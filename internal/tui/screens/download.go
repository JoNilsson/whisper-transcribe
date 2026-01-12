package screens

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cyber/whisper-transcribe/internal/models"
	"github.com/cyber/whisper-transcribe/internal/tui/styles"
)

// DownloadState represents the current state of the download screen.
type DownloadState int

const (
	DownloadStatePrompt DownloadState = iota
	DownloadStateDownloading
	DownloadStateComplete
	DownloadStateError
)

// DownloadModel handles the model download prompt and progress screen.
type DownloadModel struct {
	theme    *styles.Theme
	spinner  spinner.Model
	progress progress.Model

	state         DownloadState
	model         string
	modelSize     string
	focusedButton int

	downloaded int64
	total      int64

	err error

	confirmed bool
	cancelled bool

	width  int
	height int
}

// NewDownloadModel creates a new download screen model.
func NewDownloadModel(theme *styles.Theme) *DownloadModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = theme.Spinner

	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(50),
	)

	return &DownloadModel{
		theme:    theme,
		spinner:  s,
		progress: p,
		state:    DownloadStatePrompt,
	}
}

// Init initializes the download model.
func (m *DownloadModel) Init() tea.Cmd {
	return m.spinner.Tick
}

// SetModel sets the model to download.
func (m *DownloadModel) SetModel(model string) {
	m.model = model
	m.state = DownloadStatePrompt
	m.focusedButton = 0
	m.confirmed = false
	m.cancelled = false
	m.err = nil
	m.downloaded = 0
	m.total = 0

	if info, err := models.GetModelInfo(model); err == nil {
		m.modelSize = info.Size
	} else {
		m.modelSize = "unknown size"
	}
}

// Update handles download screen events.
func (m *DownloadModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.state == DownloadStatePrompt {
			switch msg.String() {
			case "left", "h":
				m.focusedButton = 0
			case "right", "l":
				m.focusedButton = 1
			case "y", "Y":
				m.confirmed = true
			case "n", "N", "q", "esc":
				m.cancelled = true
			case "enter":
				if m.focusedButton == 0 {
					m.confirmed = true
				} else {
					m.cancelled = true
				}
			}
		} else if m.state == DownloadStateError {
			switch msg.String() {
			case "enter", "q", "esc":
				m.cancelled = true
			}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case progress.FrameMsg:
		model, cmd := m.progress.Update(msg)
		m.progress = model.(progress.Model)
		cmds = append(cmds, cmd)

	case DownloadProgressMsg:
		m.state = DownloadStateDownloading
		m.downloaded = msg.Downloaded
		m.total = msg.Total
		if msg.Total > 0 {
			cmds = append(cmds, m.progress.SetPercent(float64(msg.Downloaded)/float64(msg.Total)))
		}

	case DownloadCompleteMsg:
		m.state = DownloadStateComplete

	case DownloadErrorMsg:
		m.state = DownloadStateError
		m.err = msg.Err
	}

	return m, tea.Batch(cmds...)
}

// View renders the download screen.
func (m *DownloadModel) View() string {
	var b strings.Builder

	switch m.state {
	case DownloadStatePrompt:
		b.WriteString(m.renderPrompt())
	case DownloadStateDownloading:
		b.WriteString(m.renderDownloading())
	case DownloadStateComplete:
		b.WriteString(m.renderComplete())
	case DownloadStateError:
		b.WriteString(m.renderError())
	}

	return b.String()
}

func (m *DownloadModel) renderPrompt() string {
	var b strings.Builder

	title := m.theme.Warning.Render("Model Not Found")
	b.WriteString(m.theme.Title.Render(title))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("The whisper model '%s' is not installed locally.\n\n", m.model))
	b.WriteString(fmt.Sprintf("Model size: %s\n", m.theme.Accent.Render(m.modelSize)))
	b.WriteString(fmt.Sprintf("Download location: %s\n\n", m.theme.Dim.Render(models.GetModelsDir())))

	b.WriteString("Would you like to download it now?\n\n")

	yesBtn := "[ Yes, download ]"
	noBtn := "[ No, cancel ]"

	if m.focusedButton == 0 {
		yesBtn = m.theme.ButtonActive.Render(yesBtn)
		noBtn = m.theme.Button.Render(noBtn)
	} else {
		yesBtn = m.theme.Button.Render(yesBtn)
		noBtn = m.theme.ButtonActive.Render(noBtn)
	}

	b.WriteString(fmt.Sprintf("  %s  %s\n\n", yesBtn, noBtn))

	help := m.theme.Help.Render("←/→ select • y/n quick select • enter confirm")
	b.WriteString(help)

	return b.String()
}

func (m *DownloadModel) renderDownloading() string {
	var b strings.Builder

	title := m.theme.Accent.Render("Downloading Model")
	b.WriteString(m.theme.Title.Render(title))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("  %s Downloading %s...\n\n", m.spinner.View(), m.model))

	b.WriteString("  ")
	b.WriteString(m.progress.View())
	b.WriteString("\n\n")

	if m.total > 0 {
		downloaded := models.FormatBytes(m.downloaded)
		total := models.FormatBytes(m.total)
		b.WriteString(fmt.Sprintf("  %s / %s\n", downloaded, total))
	}

	return b.String()
}

func (m *DownloadModel) renderComplete() string {
	var b strings.Builder

	title := m.theme.Success.Render("Download Complete")
	b.WriteString(m.theme.Title.Render(title))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("  %s Model '%s' downloaded successfully!\n\n",
		m.theme.Success.Render("✓"), m.model))

	b.WriteString("  Continuing with transcription...\n")

	return b.String()
}

func (m *DownloadModel) renderError() string {
	var b strings.Builder

	title := m.theme.Error.Render("Download Failed")
	b.WriteString(m.theme.Title.Render(title))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("  %s Failed to download model '%s'\n\n",
		m.theme.Error.Render("✗"), m.model))

	if m.err != nil {
		errBox := m.theme.Box.Render(fmt.Sprintf("Error: %v", m.err))
		b.WriteString(errBox)
		b.WriteString("\n\n")
	}

	help := m.theme.Help.Render("Press enter to return")
	b.WriteString(help)

	return b.String()
}

// Confirmed returns true if user confirmed download.
func (m *DownloadModel) Confirmed() bool {
	if m.confirmed {
		m.confirmed = false
		m.state = DownloadStateDownloading
		return true
	}
	return false
}

// Cancelled returns true if user cancelled.
func (m *DownloadModel) Cancelled() bool {
	if m.cancelled {
		m.cancelled = false
		return true
	}
	return false
}

// IsComplete returns true if download is complete.
func (m *DownloadModel) IsComplete() bool {
	return m.state == DownloadStateComplete
}

// SetSize updates the screen dimensions.
func (m *DownloadModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.progress = progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(max(30, w-20)),
	)
}

// Reset resets the download screen.
func (m *DownloadModel) Reset() {
	m.state = DownloadStatePrompt
	m.model = ""
	m.modelSize = ""
	m.focusedButton = 0
	m.confirmed = false
	m.cancelled = false
	m.err = nil
	m.downloaded = 0
	m.total = 0
}

// DownloadProgressMsg reports download progress.
type DownloadProgressMsg struct {
	Downloaded int64
	Total      int64
}

// DownloadCompleteMsg signals download completion.
type DownloadCompleteMsg struct{}

// DownloadErrorMsg signals a download error.
type DownloadErrorMsg struct {
	Err error
}
