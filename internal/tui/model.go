package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cyber/whisper-transcribe/internal/config"
	"github.com/cyber/whisper-transcribe/internal/tui/screens"
	"github.com/cyber/whisper-transcribe/internal/tui/styles"
)

// Model is the root Bubble Tea model for the TUI.
type Model struct {
	config *config.Config
	screen Screen
	theme  *styles.Theme

	input    *screens.InputModel
	download *screens.DownloadModel
	progress *screens.ProgressModel
	preview  *screens.PreviewModel

	width  int
	height int

	pipelineActive bool
	pendingConfig  *config.TranscriptionConfig

	program *tea.Program
}

// NewModel creates a new root TUI model.
func NewModel(cfg *config.Config) *Model {
	theme := styles.NewTheme()
	return &Model{
		config:   cfg,
		screen:   InputScreen,
		theme:    theme,
		input:    screens.NewInputModel(theme, cfg),
		download: screens.NewDownloadModel(theme),
		progress: screens.NewProgressModel(theme),
		preview:  screens.NewPreviewModel(theme),
	}
}

// SetProgram sets the program reference for external message injection.
func (m *Model) SetProgram(p *tea.Program) {
	m.program = p
}

// Init initializes the root model.
func (m Model) Init() tea.Cmd {
	return m.input.Init()
}

// Update handles all messages and delegates to screen models.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.SetSize(msg.Width, msg.Height)
		m.download.SetSize(msg.Width, msg.Height)
		m.progress.SetSize(msg.Width, msg.Height)
		m.preview.SetSize(msg.Width, msg.Height)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if !m.pipelineActive && m.screen != ProgressScreen && m.screen != ModelDownloadScreen {
				return m, tea.Quit
			}
		}

	case ScreenMsg:
		m.screen = Screen(msg)

	case ModelMissingMsg:
		m.screen = ModelDownloadScreen
		m.download.SetModel(msg.Model)
		cmds = append(cmds, m.download.Init())

	case ModelDownloadProgressMsg:
		model, cmd := m.download.Update(screens.DownloadProgressMsg{
			Downloaded: msg.Downloaded,
			Total:      msg.Total,
		})
		m.download = model.(*screens.DownloadModel)
		cmds = append(cmds, cmd)

	case ModelDownloadCompleteMsg:
		model, cmd := m.download.Update(screens.DownloadCompleteMsg{})
		m.download = model.(*screens.DownloadModel)
		cmds = append(cmds, cmd)
		// Continue with pipeline after short delay
		if m.pendingConfig != nil {
			cmds = append(cmds, RunPipeline(m.pendingConfig, m.program))
		}

	case ModelDownloadErrorMsg:
		model, cmd := m.download.Update(screens.DownloadErrorMsg{Err: msg.Err})
		m.download = model.(*screens.DownloadModel)
		cmds = append(cmds, cmd)

	case PipelineStartedMsg:
		m.screen = ProgressScreen
		m.pipelineActive = true
		cmds = append(cmds, m.progress.Init())

	case MetadataFetchedMsg:
		model, cmd := m.progress.Update(screens.MetadataMsg{
			Title:    msg.Title,
			Channel:  msg.Channel,
			Duration: msg.Duration,
		})
		m.progress = model.(*screens.ProgressModel)
		cmds = append(cmds, cmd)

	case PipelineProgressMsg:
		model, cmd := m.progress.Update(screens.ProgressMsg{
			Step:     msg.Step,
			Progress: msg.Progress,
			Message:  msg.Message,
		})
		m.progress = model.(*screens.ProgressModel)
		cmds = append(cmds, cmd)

	case TranscriptChunkMsg:
		model, cmd := m.progress.Update(screens.TranscriptMsg{
			Text:      msg.Text,
			Timestamp: msg.Timestamp,
		})
		m.progress = model.(*screens.ProgressModel)
		cmds = append(cmds, cmd)

	case PipelineCompletedMsg:
		m.screen = PreviewScreen
		m.pipelineActive = false
		m.pendingConfig = nil
		m.preview.SetResult(msg.OutputPath, msg.Stats)

	case PipelineErrorMsg:
		m.pipelineActive = false
		model, cmd := m.progress.Update(screens.ErrorMsg{
			Step: msg.Step,
			Err:  msg.Err,
		})
		m.progress = model.(*screens.ProgressModel)
		cmds = append(cmds, cmd)

	case EditorClosedMsg:
		// Editor closed, no action needed
	}

	switch m.screen {
	case InputScreen:
		model, cmd := m.input.Update(msg)
		m.input = model.(*screens.InputModel)
		cmds = append(cmds, cmd)

		if m.input.Submitted() {
			cfg := m.input.GetConfig()
			m.pendingConfig = cfg
			m.input.ClearSubmitted()
			// Check if model exists before running pipeline
			cmds = append(cmds, CheckModel(cfg.Model))
		}

	case ModelDownloadScreen:
		model, cmd := m.download.Update(msg)
		m.download = model.(*screens.DownloadModel)

		if m.download.Cancelled() {
			m.screen = InputScreen
			m.pendingConfig = nil
			m.download.Reset()
			m.input.ClearSubmitted()
			// Return with input init to restart cursor blink
			return m, m.input.Init()
		}

		cmds = append(cmds, cmd)

		if m.download.Confirmed() {
			if m.pendingConfig != nil {
				cmds = append(cmds, DownloadModel(m.pendingConfig.Model, m.program))
			}
		}

		if m.download.IsComplete() {
			// Already handled above with ModelDownloadCompleteMsg
		}

	case ProgressScreen:
		model, cmd := m.progress.Update(msg)
		m.progress = model.(*screens.ProgressModel)
		cmds = append(cmds, cmd)

	case PreviewScreen:
		model, cmd := m.preview.Update(msg)
		m.preview = model.(*screens.PreviewModel)
		cmds = append(cmds, cmd)

		if m.preview.StartNew() {
			m.screen = InputScreen
			m.input.Reset()
			m.progress.Reset()
			m.preview.Reset()
			m.download.Reset()
		}

		if m.preview.OpenEdit() {
			cmds = append(cmds, OpenInEditor(m.preview.GetOutputPath()))
		}
	}

	// If model check passed (nil message), run pipeline
	if msg == nil && m.pendingConfig != nil && m.screen == InputScreen {
		cmds = append(cmds, RunPipeline(m.pendingConfig, m.program))
	}

	return m, tea.Batch(cmds...)
}

// View renders the current screen.
func (m Model) View() string {
	switch m.screen {
	case InputScreen:
		return m.input.View()
	case ModelDownloadScreen:
		return m.download.View()
	case ProgressScreen:
		return m.progress.View()
	case PreviewScreen:
		return m.preview.View()
	default:
		return ""
	}
}
