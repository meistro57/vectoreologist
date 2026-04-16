package lens

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/meistro57/vectoreologist/internal/synthesis"
)

// reportLoadedMsg carries the parsed report data back to the model.
type reportLoadedMsg struct {
	report *synthesis.JSONReport
}

// errMsg carries a load/parse error.
type errMsg struct{ err error }

// loadReportCmd reads and parses the JSON report file asynchronously.
func loadReportCmd(path string) tea.Cmd {
	return func() tea.Msg {
		report, err := loadReport(path)
		if err != nil {
			return errMsg{err}
		}
		return reportLoadedMsg{report}
	}
}
