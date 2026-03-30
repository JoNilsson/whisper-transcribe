package screens

import (
	"strings"

	"github.com/charmbracelet/bubbles/filepicker"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cyber/whisper-transcribe/internal/tui/styles"
)

// FilepickerModel is a directory-browser screen built on bubbles/filepicker.
// Navigation uses the standard filepicker keys (arrows, enter to descend,
// esc/backspace to ascend). Pressing space confirms the current directory.
// Pressing q cancels and returns to the input screen.
type FilepickerModel struct {
	theme     *styles.Theme
	fp        filepicker.Model
	selected  string // non-empty when a directory has been confirmed
	cancelled bool
	width     int
	height    int
}

// NewFilepickerModel creates a directory picker starting at startDir.
func NewFilepickerModel(theme *styles.Theme, startDir string) *FilepickerModel {
	fp := filepicker.New()
	fp.CurrentDirectory = startDir
	// Files are not selectable — we only want directory navigation.
	// Confirming with space takes the CurrentDirectory field.
	fp.DirAllowed = false
	fp.FileAllowed = false
	fp.ShowHidden = false
	fp.ShowPermissions = false
	fp.ShowSize = false
	fp.AutoHeight = true

	return &FilepickerModel{
		theme: theme,
		fp:    fp,
	}
}

// Init starts the filepicker.
func (m *FilepickerModel) Init() tea.Cmd {
	return m.fp.Init()
}

// Update handles key events, delegating navigation to the bubbles filepicker.
func (m *FilepickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			m.cancelled = true
			return m, nil
		case "esc":
			// If at root or the user wants to cancel, pop back via filepicker's
			// Back binding; but if they press esc twice quickly the model will
			// already have set cancelled via the q path. Let the filepicker
			// handle esc for directory traversal.
		case " ":
			// Confirm current directory.
			m.selected = m.fp.CurrentDirectory
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.fp, cmd = m.fp.Update(msg)
	return m, cmd
}

// View renders the directory browser with a status bar and help text.
func (m *FilepickerModel) View() string {
	var b strings.Builder

	header := m.theme.Header.Render(styles.ASCIIHeader)
	b.WriteString(header)
	b.WriteString("\n\n")

	label := m.theme.Primary.Render("Select Output Directory")
	b.WriteString(label)
	b.WriteString("\n")

	dirLine := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#06B6D4")).
		Bold(true).
		Render("  " + m.fp.CurrentDirectory)
	b.WriteString(dirLine)
	b.WriteString("\n\n")

	b.WriteString(m.fp.View())
	b.WriteString("\n")

	help := m.theme.Help.Render("↑/↓ navigate • enter open dir • space confirm • backspace go up • q cancel")
	b.WriteString(help)

	return b.String()
}

// SelectedDir returns the confirmed directory path, or empty string if none yet.
func (m *FilepickerModel) SelectedDir() string {
	return m.selected
}

// Cancelled returns true if the user pressed q to cancel.
func (m *FilepickerModel) Cancelled() bool {
	return m.cancelled
}

// Reset clears selection and cancellation state.
func (m *FilepickerModel) Reset(startDir string) {
	m.selected = ""
	m.cancelled = false
	m.fp.CurrentDirectory = startDir
}

// SetSize updates the screen dimensions.
func (m *FilepickerModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	// Reserve rows for header, label, dir line, help bar.
	m.fp.Height = max(5, h-12)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
