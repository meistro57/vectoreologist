package lens

import "github.com/charmbracelet/lipgloss"

var (
	colorCyan    = lipgloss.Color("#00ff9f")
	colorBlue    = lipgloss.Color("#00d4ff")
	colorMagenta = lipgloss.Color("#ff00ff")
	colorGray    = lipgloss.Color("#606060")
	colorDimGray = lipgloss.Color("#404040")
	colorRed     = lipgloss.Color("#ff6b6b")
	colorYellow  = lipgloss.Color("#ffd700")
	colorWhite   = lipgloss.Color("#e0e0e0")

	headerStyle = lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorBlue).
			Padding(0, 1)

	titleStyle = lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(colorBlue)

	selectedStyle = lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true).
			Background(lipgloss.Color("#001a0d"))

	normalStyle = lipgloss.NewStyle().
			Foreground(colorWhite)

	dimStyle = lipgloss.NewStyle().
			Foreground(colorGray)

	anomalyStyle = lipgloss.NewStyle().
			Foreground(colorMagenta).
			Bold(true)

	bridgeStyle = lipgloss.NewStyle().
			Foreground(colorBlue)

	moatStyle = lipgloss.NewStyle().
			Foreground(colorYellow)

	exportMenuStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(colorYellow).
				Padding(0, 1)

	errorStyle = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true)

	panelBorderStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(colorBlue).
				Padding(0, 1)

	detailPanelStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(colorCyan).
				Padding(0, 1)

	footerStyle = lipgloss.NewStyle().
			Foreground(colorDimGray).
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(colorDimGray)

	tabActiveStyle = lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true).
			Underline(true)

	tabInactiveStyle = lipgloss.NewStyle().
				Foreground(colorGray)

	coherenceHighStyle = lipgloss.NewStyle().Foreground(colorCyan)
	coherenceMidStyle  = lipgloss.NewStyle().Foreground(colorYellow)
	coherenceLowStyle  = lipgloss.NewStyle().Foreground(colorRed)

	searchPromptStyle = lipgloss.NewStyle().
				Foreground(colorCyan).
				Bold(true)

	searchCursorStyle = lipgloss.NewStyle().
				Background(colorCyan).
				Foreground(lipgloss.Color("#000000"))

	reasoningStyle = lipgloss.NewStyle().
			Foreground(colorGray).
			Italic(true)
)

func coherenceColor(c float64) lipgloss.Style {
	switch {
	case c >= 0.7:
		return coherenceHighStyle
	case c >= 0.4:
		return coherenceMidStyle
	default:
		return coherenceLowStyle
	}
}
