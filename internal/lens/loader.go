package lens

import (
	"encoding/json"
	"os"

	"github.com/meistro57/vectoreologist/internal/synthesis"
)

// loadReport reads and parses a vectoreology JSON report file.
func loadReport(path string) (*synthesis.JSONReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var report synthesis.JSONReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}
	return &report, nil
}
