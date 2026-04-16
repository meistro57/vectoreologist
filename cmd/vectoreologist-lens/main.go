package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/meistro57/vectoreologist/internal/lens"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: vectoreologist-lens <path-to-vectoreology.json>")
		fmt.Fprintln(os.Stderr, "Example: vectoreologist-lens findings/vectoreology_2026-04-15_14-30-00.json")
		os.Exit(1)
	}

	reportPath := os.Args[1]
	if _, err := os.Stat(reportPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot read %s: %v\n", reportPath, err)
		os.Exit(1)
	}

	p := tea.NewProgram(
		lens.New(reportPath),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
