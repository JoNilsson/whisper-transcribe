package screens

import (
	"strings"

	"github.com/charmbracelet/bubbles/filepicker"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cyber/whisper-transcribe/internal/tui/styles"
)

// PickerMode determines whether the filepicker selects directories or files.
type PickerMode int

const (
	PickDir  PickerMode = iota
	PickFile            // select audio files
)

// FilepickerModel is a browser screen built on bubbles/filepicker.
// Navigation uses the standard filepicker keys (arrows, enter to descend,
// esc/backspace to ascend). Pressing space confirms the selection.
// Pressing q cancels and returns to the input screen.
type FilepickerModel struct {
	theme     *styles.Theme
	fp        filepicker.Model
	mode      PickerMode
	selected  string // non-empty when confirmed
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
		mode:  PickDir,
	}
}

// audioExtensions lists file extensions shown when picking audio files.
var audioExtensions = []string{".wav", ".mp3", ".m4a", ".ogg", ".flac", ".webm", ".mp4"}

// NewAudioFilepickerModel creates an audio-file picker starting at startDir.
func NewAudioFilepickerModel(theme *styles.Theme, startDir string) *FilepickerModel {
	fp := filepicker.New()
	fp.CurrentDirectory = startDir
	fp.DirAllowed = false
	fp.FileAllowed = true
	fp.AllowedTypes = audioExtensions
	fp.ShowHidden = false
	fp.ShowPermissions = false
	fp.ShowSize = true
	fp.AutoHeight = true

	return &FilepickerModel{
		theme: theme,
		fp:    fp,
		mode:  PickFile,
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
			// Let the filepicker handle esc for directory traversal.
		case " ":
			if m.mode == PickDir {
				m.selected = m.fp.CurrentDirectory
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	m.fp, cmd = m.fp.Update(msg)

	// In file mode, check if the filepicker confirmed a file selection.
	if m.mode == PickFile {
		if didSelect, path := m.fp.DidSelectFile(msg); didSelect {
			m.selected = path
		}
	}

	return m, cmd
}

// View renders the browser with a status bar and help text.
func (m *FilepickerModel) View() string {
	var b strings.Builder

	header := m.theme.Header.Render(styles.ASCIIHeader)
	b.WriteString(header)
	b.WriteString("\n\n")

	var title, help string
	if m.mode == PickFile {
		title = "Select Audio File"
		help = "↑/↓ navigate • enter select/open dir • backspace go up • q cancel"
	} else {
		title = "Select Output Directory"
		help = "↑/↓ navigate • enter open dir • space confirm • backspace go up • q cancel"
	}

	label := m.theme.Primary.Render(title)
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

	b.WriteString(m.theme.Help.Render(help))

	return b.String()
}

// SelectedDir returns the confirmed directory path, or empty string if none yet.
func (m *FilepickerModel) SelectedDir() string {
	if m.mode == PickDir {
		return m.selected
	}
	return ""
}

// SelectedFile returns the confirmed file path, or empty string if none yet.
func (m *FilepickerModel) SelectedFile() string {
	if m.mode == PickFile {
		return m.selected
	}
	return ""
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
