package styles

import "github.com/charmbracelet/lipgloss"

// Theme holds all styled components for the TUI.
type Theme struct {
	Primary   lipgloss.Style
	Secondary lipgloss.Style
	Accent    lipgloss.Style
	Success   lipgloss.Style
	Error     lipgloss.Style
	Warning   lipgloss.Style
	Dim       lipgloss.Style

	Title        lipgloss.Style
	Subtitle     lipgloss.Style
	Label        lipgloss.Style
	Header       lipgloss.Style
	Box          lipgloss.Style
	Button       lipgloss.Style
	ButtonActive lipgloss.Style
	Help         lipgloss.Style
	Spinner      lipgloss.Style
	ProgressBar  lipgloss.Style
}

// NewTheme creates a new theme with default colors.
func NewTheme() *Theme {
	primary := lipgloss.Color("#7C3AED")
	secondary := lipgloss.Color("#6366F1")
	accent := lipgloss.Color("#06B6D4")
	success := lipgloss.Color("#10B981")
	errColor := lipgloss.Color("#EF4444")
	warning := lipgloss.Color("#F59E0B")
	dim := lipgloss.Color("#6B7280")

	return &Theme{
		Primary:   lipgloss.NewStyle().Foreground(primary),
		Secondary: lipgloss.NewStyle().Foreground(secondary),
		Accent:    lipgloss.NewStyle().Foreground(accent),
		Success:   lipgloss.NewStyle().Foreground(success),
		Error:     lipgloss.NewStyle().Foreground(errColor),
		Warning:   lipgloss.NewStyle().Foreground(warning),
		Dim:       lipgloss.NewStyle().Foreground(dim),

		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(primary).
			MarginBottom(1),

		Subtitle: lipgloss.NewStyle().
			Foreground(secondary),

		Label: lipgloss.NewStyle().
			Foreground(dim).
			Italic(true),

		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(primary).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(secondary).
			Padding(1, 2).
			MarginBottom(1),

		Box: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(dim).
			Padding(1, 2),

		Button: lipgloss.NewStyle().
			Foreground(dim).
			Padding(0, 2),

		ButtonActive: lipgloss.NewStyle().
			Foreground(primary).
			Bold(true).
			Padding(0, 2),

		Help: lipgloss.NewStyle().
			Foreground(dim).
			MarginTop(1),

		Spinner: lipgloss.NewStyle().
			Foreground(accent),

		ProgressBar: lipgloss.NewStyle().
			Foreground(accent),
	}
}

// ASCIIHeader returns the application header art.
const ASCIIHeader = `
╦ ╦┬ ┬┬┌─┐┌─┐┌─┐┬─┐  ╔╦╗┬─┐┌─┐┌┐┌┌─┐┌─┐┬─┐┬┌┐ ┌─┐
║║║├─┤│└─┐├─┘├┤ ├┬┘   ║ ├┬┘├─┤│││└─┐│  ├┬┘│├┴┐├┤
╚╩╝┴ ┴┴└─┘┴  └─┘┴└─   ╩ ┴└─┴ ┴┘└┘└─┘└─┘┴└─┴└─┘└─┘`
