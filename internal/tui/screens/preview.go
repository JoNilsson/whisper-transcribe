package screens

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/cyber/whisper-transcribe/internal/pipeline"
	"github.com/cyber/whisper-transcribe/internal/tui/styles"
)

// PreviewModel handles the completion and preview screen.
type PreviewModel struct {
	theme    *styles.Theme
	viewport viewport.Model
	renderer *glamour.TermRenderer

	outputPath string
	stats      pipeline.Stats
	markdown   string

	focusedButton int
	buttons       []string

	startNew bool
	openEdit bool

	width  int
	height int
}

// NewPreviewModel creates a new preview screen model.
func NewPreviewModel(theme *styles.Theme) *PreviewModel {
	vp := viewport.New(80, 20)

	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)

	return &PreviewModel{
		theme:    theme,
		viewport: vp,
		renderer: renderer,
		buttons:  []string{"New Transcription", "Open in Editor", "Quit"},
	}
}

// Init initializes the preview model.
func (m *PreviewModel) Init() tea.Cmd {
	return nil
}

// SetResult sets the transcription result for display.
func (m *PreviewModel) SetResult(outputPath string, stats pipeline.Stats) {
	m.outputPath = outputPath
	m.stats = stats

	content, err := os.ReadFile(outputPath)
	if err != nil {
		m.markdown = fmt.Sprintf("Error reading file: %v", err)
	} else {
		m.markdown = string(content)
	}

	rendered, err := m.renderer.Render(m.markdown)
	if err != nil {
		rendered = m.markdown
	}
	m.viewport.SetContent(rendered)
}

// Update handles preview events.
func (m *PreviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h":
			m.focusedButton = max(0, m.focusedButton-1)
		case "right", "l":
			m.focusedButton = min(len(m.buttons)-1, m.focusedButton+1)
		case "enter":
			switch m.focusedButton {
			case 0:
				m.startNew = true
			case 1:
				m.openEdit = true
			case 2:
				return m, tea.Quit
			}
		case "up", "k":
			m.viewport.LineUp(1)
		case "down", "j":
			m.viewport.LineDown(1)
		case "pgup":
			m.viewport.ViewUp()
		case "pgdown":
			m.viewport.ViewDown()
		case "n":
			m.startNew = true
		case "e":
			m.openEdit = true
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the preview screen.
func (m *PreviewModel) View() string {
	var b strings.Builder

	header := m.theme.Success.Render("✓ Transcription Complete")
	b.WriteString(header)
	b.WriteString("\n\n")

	previewLabel := m.theme.Label.Render("─ Preview ")
	b.WriteString(previewLabel)
	b.WriteString("\n")

	preview := m.theme.Box.
		Width(m.width - 4).
		Height(m.height - 15).
		Render(m.viewport.View())
	b.WriteString(preview)
	b.WriteString("\n\n")

	stats := fmt.Sprintf(
		"Saved to: %s\nDuration: %s  •  Words: %d  •  Model: %s",
		m.outputPath,
		m.stats.Duration,
		m.stats.WordCount,
		m.stats.Model,
	)
	b.WriteString(m.theme.Dim.Render(stats))
	b.WriteString("\n\n")

	var buttons []string
	for i, btn := range m.buttons {
		if i == m.focusedButton {
			buttons = append(buttons, m.theme.ButtonActive.Render("[ "+btn+" ]"))
		} else {
			buttons = append(buttons, m.theme.Button.Render("[ "+btn+" ]"))
		}
	}
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Center, buttons...))
	b.WriteString("\n\n")

	help := m.theme.Help.Render("←/→ select • enter confirm • ↑/↓ scroll • n new • e edit • q quit")
	b.WriteString(help)

	return b.String()
}

// StartNew returns true if user wants a new transcription.
func (m *PreviewModel) StartNew() bool {
	if m.startNew {
		m.startNew = false
		return true
	}
	return false
}

// OpenEdit returns true if user wants to open editor.
func (m *PreviewModel) OpenEdit() bool {
	if m.openEdit {
		m.openEdit = false
		return true
	}
	return false
}

// GetOutputPath returns the output file path.
func (m *PreviewModel) GetOutputPath() string {
	return m.outputPath
}

// SetSize updates the screen dimensions.
func (m *PreviewModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.viewport.Width = max(40, w-8)
	m.viewport.Height = max(5, h-20)

	m.renderer, _ = glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(max(40, w-12)),
	)
}

// Reset resets the preview screen.
func (m *PreviewModel) Reset() {
	m.outputPath = ""
	m.markdown = ""
	m.focusedButton = 0
	m.startNew = false
	m.openEdit = false
	m.viewport.SetContent("")
}
