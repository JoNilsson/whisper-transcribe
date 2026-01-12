package screens

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cyber/whisper-transcribe/internal/tui/styles"
)

// StepStatus represents the status of a pipeline step.
type StepStatus int

const (
	StepPending StepStatus = iota
	StepInProgress
	StepCompleted
	StepError
)

// PipelineStep represents a step in the transcription pipeline.
type PipelineStep struct {
	Name     string
	Key      string
	Status   StepStatus
	Progress float64
	Message  string
}

// ProgressModel handles the progress display screen.
type ProgressModel struct {
	theme    *styles.Theme
	spinner  spinner.Model
	progress progress.Model
	viewport viewport.Model

	title string
	steps []PipelineStep

	transcript strings.Builder

	err error

	width  int
	height int
}

// NewProgressModel creates a new progress screen model.
func NewProgressModel(theme *styles.Theme) *ProgressModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = theme.Spinner

	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(50),
	)

	vp := viewport.New(60, 8)

	return &ProgressModel{
		theme:    theme,
		spinner:  s,
		progress: p,
		viewport: vp,
		steps: []PipelineStep{
			{Name: "Fetching video metadata", Key: "metadata", Status: StepPending},
			{Name: "Downloading audio", Key: "download", Status: StepPending},
			{Name: "Transcribing audio", Key: "transcribe", Status: StepPending},
			{Name: "Formatting Markdown", Key: "format", Status: StepPending},
			{Name: "Validating output", Key: "validate", Status: StepPending},
		},
	}
}

// Init initializes the progress model.
func (m *ProgressModel) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update handles progress events.
func (m *ProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case progress.FrameMsg:
		model, cmd := m.progress.Update(msg)
		m.progress = model.(progress.Model)
		cmds = append(cmds, cmd)

	case MetadataMsg:
		m.title = msg.Title

	case ProgressMsg:
		m.updateStepStatus(msg.Step, msg.Progress, msg.Message)
		if msg.Progress > 0 && msg.Progress < 1 {
			cmds = append(cmds, m.progress.SetPercent(msg.Progress))
		}

	case TranscriptMsg:
		m.transcript.WriteString(msg.Text)
		m.transcript.WriteString(" ")
		m.viewport.SetContent(m.transcript.String())
		m.viewport.GotoBottom()

	case ErrorMsg:
		m.err = msg.Err
		m.setStepError(msg.Step)
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *ProgressModel) updateStepStatus(key string, prog float64, message string) {
	for i := range m.steps {
		if m.steps[i].Key == key {
			if prog >= 1.0 {
				m.steps[i].Status = StepCompleted
			} else {
				m.steps[i].Status = StepInProgress
			}
			m.steps[i].Progress = prog
			m.steps[i].Message = message
			break
		}
	}
}

func (m *ProgressModel) setStepError(key string) {
	for i := range m.steps {
		if m.steps[i].Key == key {
			m.steps[i].Status = StepError
			break
		}
	}
}

// View renders the progress screen.
func (m *ProgressModel) View() string {
	var b strings.Builder

	titleText := "Processing"
	if m.title != "" {
		titleText = fmt.Sprintf("Processing: %q", m.title)
	}
	b.WriteString(m.theme.Title.Render(titleText))
	b.WriteString("\n\n")

	for _, step := range m.steps {
		icon := m.stepIcon(step.Status)
		name := step.Name
		var status string

		switch step.Status {
		case StepPending:
			status = m.theme.Dim.Render("pending")
		case StepInProgress:
			status = m.spinner.View()
		case StepCompleted:
			status = m.theme.Success.Render("done")
		case StepError:
			status = m.theme.Error.Render("error")
		}

		line := fmt.Sprintf("  %s %s %s", icon, name, status)
		b.WriteString(line)
		b.WriteString("\n")

		if step.Status == StepInProgress && step.Progress > 0 && step.Progress < 1 {
			b.WriteString("    ")
			b.WriteString(m.progress.View())
			b.WriteString("\n")
		}
	}

	if m.err != nil {
		b.WriteString("\n")
		errBox := m.theme.Error.Render(fmt.Sprintf("Error: %v", m.err))
		b.WriteString(m.theme.Box.Render(errBox))
		b.WriteString("\n")
	}

	if m.transcript.Len() > 0 {
		b.WriteString("\n")
		previewLabel := m.theme.Label.Render("Live Preview:")
		b.WriteString(previewLabel)
		b.WriteString("\n")
		previewBox := m.theme.Box.Width(m.width - 4).Render(m.viewport.View())
		b.WriteString(previewBox)
	}

	return b.String()
}

func (m *ProgressModel) stepIcon(status StepStatus) string {
	switch status {
	case StepCompleted:
		return m.theme.Success.Render("✓")
	case StepInProgress:
		return m.theme.Accent.Render("◐")
	case StepError:
		return m.theme.Error.Render("✗")
	default:
		return m.theme.Dim.Render("○")
	}
}

// SetError sets an error on the progress screen.
func (m *ProgressModel) SetError(err error) {
	m.err = err
}

// SetSize updates the screen dimensions.
func (m *ProgressModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.viewport.Width = max(40, w-10)
	m.viewport.Height = 8
	m.progress = progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(max(30, w-20)),
	)
}

// Reset resets the progress screen for a new transcription.
func (m *ProgressModel) Reset() {
	m.title = ""
	m.err = nil
	m.transcript.Reset()
	m.viewport.SetContent("")
	for i := range m.steps {
		m.steps[i].Status = StepPending
		m.steps[i].Progress = 0
		m.steps[i].Message = ""
	}
}

// MetadataMsg contains video metadata.
type MetadataMsg struct {
	Title    string
	Channel  string
	Duration string
}

// ProgressMsg reports step progress.
type ProgressMsg struct {
	Step     string
	Progress float64
	Message  string
}

// TranscriptMsg streams transcription text.
type TranscriptMsg struct {
	Text      string
	Timestamp string
}

// ErrorMsg signals an error.
type ErrorMsg struct {
	Step string
	Err  error
}
