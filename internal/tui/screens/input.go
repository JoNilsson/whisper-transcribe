package screens

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cyber/whisper-transcribe/internal/config"
	"github.com/cyber/whisper-transcribe/internal/downloader"
	"github.com/cyber/whisper-transcribe/internal/tui/styles"
)

// InputModel handles the URL input and configuration screen.
type InputModel struct {
	theme     *styles.Theme
	urlInput  textinput.Model
	submitted bool
	err       error

	url        string
	model      string
	timestamps bool
	outputDir  string

	focusIndex int
	models     []string

	width  int
	height int
}

// NewInputModel creates a new input screen model.
func NewInputModel(theme *styles.Theme, cfg *config.Config) *InputModel {
	ti := textinput.New()
	ti.Placeholder = "https://www.youtube.com/watch?v=..."
	ti.Focus()
	ti.CharLimit = 200
	ti.Width = 60

	return &InputModel{
		theme:      theme,
		urlInput:   ti,
		model:      cfg.DefaultModel,
		outputDir:  cfg.OutputDir,
		timestamps: cfg.Timestamps,
		models:     config.ModelOptions(),
		focusIndex: 0,
	}
}

// Init initializes the input model.
func (m *InputModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles input events.
func (m *InputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "down":
			m.focusIndex = (m.focusIndex + 1) % 4
			m.updateFocus()
		case "shift+tab", "up":
			m.focusIndex = (m.focusIndex - 1 + 4) % 4
			m.updateFocus()
		case "left":
			if m.focusIndex == 1 {
				idx := indexOf(m.models, m.model)
				if idx > 0 {
					m.model = m.models[idx-1]
				}
			}
		case "right":
			if m.focusIndex == 1 {
				idx := indexOf(m.models, m.model)
				if idx < len(m.models)-1 {
					m.model = m.models[idx+1]
				}
			}
		case " ":
			if m.focusIndex == 2 {
				m.timestamps = !m.timestamps
			}
		case "enter":
			if m.focusIndex == 3 {
				m.url = m.urlInput.Value()
				if err := downloader.ValidateURL(m.url); err != nil {
					m.err = err
				} else {
					m.err = nil
					m.submitted = true
				}
			}
		}
	}

	if m.focusIndex == 0 {
		m.urlInput, cmd = m.urlInput.Update(msg)
	}

	return m, cmd
}

func (m *InputModel) updateFocus() {
	if m.focusIndex == 0 {
		m.urlInput.Focus()
	} else {
		m.urlInput.Blur()
	}
}

// View renders the input screen.
func (m *InputModel) View() string {
	var b strings.Builder

	header := m.theme.Header.Render(styles.ASCIIHeader)
	b.WriteString(header)
	b.WriteString("\n\n")

	urlLabel := "YouTube URL"
	if m.focusIndex == 0 {
		urlLabel = m.theme.Primary.Render("▶ " + urlLabel)
	} else {
		urlLabel = m.theme.Dim.Render("  " + urlLabel)
	}
	b.WriteString(urlLabel)
	b.WriteString("\n")
	b.WriteString("  ")
	b.WriteString(m.urlInput.View())
	b.WriteString("\n")

	if m.err != nil {
		b.WriteString("  ")
		b.WriteString(m.theme.Error.Render(m.err.Error()))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	modelLabel := "Model"
	if m.focusIndex == 1 {
		modelLabel = m.theme.Primary.Render("▶ " + modelLabel)
	} else {
		modelLabel = m.theme.Dim.Render("  " + modelLabel)
	}
	b.WriteString(modelLabel)
	b.WriteString("\n  ")
	for _, model := range m.models {
		if model == m.model {
			b.WriteString(m.theme.Accent.Render("◉ " + model + "  "))
		} else {
			b.WriteString(m.theme.Dim.Render("○ " + model + "  "))
		}
	}
	b.WriteString("\n\n")

	tsLabel := "Include Timestamps"
	if m.focusIndex == 2 {
		tsLabel = m.theme.Primary.Render("▶ " + tsLabel)
	} else {
		tsLabel = m.theme.Dim.Render("  " + tsLabel)
	}
	b.WriteString(tsLabel)
	b.WriteString("  ")
	if m.timestamps {
		b.WriteString(m.theme.Success.Render("☑ Yes"))
	} else {
		b.WriteString(m.theme.Dim.Render("☐ No"))
	}
	b.WriteString("\n\n")

	b.WriteString(m.theme.Dim.Render(fmt.Sprintf("  Output: %s", m.outputDir)))
	b.WriteString("\n\n")

	startBtn := "[ Start Transcription ]"
	if m.focusIndex == 3 {
		startBtn = m.theme.ButtonActive.Render(startBtn)
	} else {
		startBtn = m.theme.Button.Render(startBtn)
	}
	b.WriteString(lipgloss.NewStyle().MarginLeft(20).Render(startBtn))
	b.WriteString("\n\n")

	help := m.theme.Help.Render("↑/↓ navigate • ←/→ select model • space toggle • enter submit • q quit")
	b.WriteString(help)

	return b.String()
}

// Submitted returns true if the form has been submitted.
func (m *InputModel) Submitted() bool {
	return m.submitted
}

// GetConfig returns the transcription configuration.
func (m *InputModel) GetConfig() *config.TranscriptionConfig {
	return &config.TranscriptionConfig{
		URL:        m.url,
		Model:      m.model,
		Timestamps: m.timestamps,
		OutputDir:  m.outputDir,
	}
}

// Reset resets the input screen for a new transcription.
func (m *InputModel) Reset() {
	m.submitted = false
	m.url = ""
	m.err = nil
	m.urlInput.SetValue("")
	m.urlInput.Focus()
	m.focusIndex = 0
}

// SetSize updates the screen dimensions.
func (m *InputModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.urlInput.Width = min(60, w-10)
}

func indexOf(slice []string, item string) int {
	for i, s := range slice {
		if s == item {
			return i
		}
	}
	return -1
}
