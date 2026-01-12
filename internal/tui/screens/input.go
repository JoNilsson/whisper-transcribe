package screens

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cyber/whisper-transcribe/internal/config"
	"github.com/cyber/whisper-transcribe/internal/downloader"
	"github.com/cyber/whisper-transcribe/internal/tui/styles"
)

// SourceType represents the input source type.
type SourceType int

const (
	SourceURL SourceType = iota
	SourceLocalFile
)

// InputModel handles the URL input and configuration screen.
type InputModel struct {
	theme      *styles.Theme
	urlInput   textinput.Model
	fileInput  textinput.Model
	submitted  bool
	err        error

	sourceType SourceType
	url        string
	localFile  string
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

	fi := textinput.New()
	fi.Placeholder = "/path/to/audio.wav"
	fi.CharLimit = 500
	fi.Width = 60

	return &InputModel{
		theme:      theme,
		urlInput:   ti,
		fileInput:  fi,
		sourceType: SourceURL,
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
			m.focusIndex = (m.focusIndex + 1) % 5
			m.updateFocus()
		case "shift+tab", "up":
			m.focusIndex = (m.focusIndex - 1 + 5) % 5
			m.updateFocus()
		case "left":
			if m.focusIndex == 0 {
				// Toggle source type
				if m.sourceType == SourceLocalFile {
					m.sourceType = SourceURL
				}
			} else if m.focusIndex == 2 {
				idx := indexOf(m.models, m.model)
				if idx > 0 {
					m.model = m.models[idx-1]
				}
			}
		case "right":
			if m.focusIndex == 0 {
				// Toggle source type
				if m.sourceType == SourceURL {
					m.sourceType = SourceLocalFile
				}
			} else if m.focusIndex == 2 {
				idx := indexOf(m.models, m.model)
				if idx < len(m.models)-1 {
					m.model = m.models[idx+1]
				}
			}
		case " ":
			if m.focusIndex == 0 {
				// Toggle source type
				if m.sourceType == SourceURL {
					m.sourceType = SourceLocalFile
				} else {
					m.sourceType = SourceURL
				}
			} else if m.focusIndex == 3 {
				m.timestamps = !m.timestamps
			}
		case "enter":
			if m.focusIndex == 4 {
				if m.sourceType == SourceURL {
					m.url = m.urlInput.Value()
					if err := downloader.ValidateURL(m.url); err != nil {
						m.err = err
					} else {
						m.err = nil
						m.submitted = true
					}
				} else {
					m.localFile = m.fileInput.Value()
					if err := validateLocalFile(m.localFile); err != nil {
						m.err = err
					} else {
						m.err = nil
						m.submitted = true
					}
				}
			}
		}
	}

	if m.focusIndex == 1 {
		if m.sourceType == SourceURL {
			m.urlInput, cmd = m.urlInput.Update(msg)
		} else {
			m.fileInput, cmd = m.fileInput.Update(msg)
		}
	}

	return m, cmd
}

func (m *InputModel) updateFocus() {
	m.urlInput.Blur()
	m.fileInput.Blur()

	if m.focusIndex == 1 {
		if m.sourceType == SourceURL {
			m.urlInput.Focus()
		} else {
			m.fileInput.Focus()
		}
	}
}

// View renders the input screen.
func (m *InputModel) View() string {
	var b strings.Builder

	header := m.theme.Header.Render(styles.ASCIIHeader)
	b.WriteString(header)
	b.WriteString("\n\n")

	// Source type selector
	sourceLabel := "Source Type"
	if m.focusIndex == 0 {
		sourceLabel = m.theme.Primary.Render("▶ " + sourceLabel)
	} else {
		sourceLabel = m.theme.Dim.Render("  " + sourceLabel)
	}
	b.WriteString(sourceLabel)
	b.WriteString("\n  ")
	if m.sourceType == SourceURL {
		b.WriteString(m.theme.Accent.Render("◉ YouTube URL  "))
		b.WriteString(m.theme.Dim.Render("○ Local File"))
	} else {
		b.WriteString(m.theme.Dim.Render("○ YouTube URL  "))
		b.WriteString(m.theme.Accent.Render("◉ Local File"))
	}
	b.WriteString("\n\n")

	// Input field (URL or file path)
	var inputLabel string
	if m.sourceType == SourceURL {
		inputLabel = "YouTube URL"
	} else {
		inputLabel = "Audio File Path"
	}
	if m.focusIndex == 1 {
		inputLabel = m.theme.Primary.Render("▶ " + inputLabel)
	} else {
		inputLabel = m.theme.Dim.Render("  " + inputLabel)
	}
	b.WriteString(inputLabel)
	b.WriteString("\n")
	b.WriteString("  ")
	if m.sourceType == SourceURL {
		b.WriteString(m.urlInput.View())
	} else {
		b.WriteString(m.fileInput.View())
	}
	b.WriteString("\n")

	if m.err != nil {
		b.WriteString("  ")
		b.WriteString(m.theme.Error.Render(m.err.Error()))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	modelLabel := "Model"
	if m.focusIndex == 2 {
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
	if m.focusIndex == 3 {
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
	if m.focusIndex == 4 {
		startBtn = m.theme.ButtonActive.Render(startBtn)
	} else {
		startBtn = m.theme.Button.Render(startBtn)
	}
	b.WriteString(lipgloss.NewStyle().MarginLeft(20).Render(startBtn))
	b.WriteString("\n\n")

	help := m.theme.Help.Render("↑/↓ navigate • ←/→ select • space toggle • enter submit • q quit")
	b.WriteString(help)

	return b.String()
}

// Submitted returns true if the form has been submitted.
func (m *InputModel) Submitted() bool {
	return m.submitted
}

// GetConfig returns the transcription configuration.
func (m *InputModel) GetConfig() *config.TranscriptionConfig {
	cfg := &config.TranscriptionConfig{
		Model:      m.model,
		Timestamps: m.timestamps,
		OutputDir:  m.outputDir,
	}
	if m.sourceType == SourceURL {
		cfg.URL = m.url
	} else {
		cfg.LocalFile = m.localFile
	}
	return cfg
}

// Reset resets the input screen for a new transcription.
func (m *InputModel) Reset() {
	m.submitted = false
	m.url = ""
	m.localFile = ""
	m.sourceType = SourceURL
	m.err = nil
	m.urlInput.SetValue("")
	m.fileInput.SetValue("")
	m.urlInput.Focus()
	m.focusIndex = 0
}

// SetSize updates the screen dimensions.
func (m *InputModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	inputWidth := min(60, w-10)
	m.urlInput.Width = inputWidth
	m.fileInput.Width = inputWidth
}

func indexOf(slice []string, item string) int {
	for i, s := range slice {
		if s == item {
			return i
		}
	}
	return -1
}

func validateLocalFile(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("file path is required")
	}

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", path)
	}
	if err != nil {
		return fmt.Errorf("cannot access file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("path is a directory, not a file")
	}

	// Check for supported audio extensions
	ext := strings.ToLower(strings.TrimPrefix(strings.ToLower(path[strings.LastIndex(path, "."):]), "."))
	supportedExts := []string{"wav", "mp3", "m4a", "ogg", "flac", "webm", "mp4"}
	supported := false
	for _, e := range supportedExts {
		if ext == e {
			supported = true
			break
		}
	}
	if !supported {
		return fmt.Errorf("unsupported audio format: .%s (supported: wav, mp3, m4a, ogg, flac, webm, mp4)", ext)
	}

	return nil
}
