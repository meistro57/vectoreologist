🔥 VECTOREOLOGIST LENS - CLAUDE CODE INSTRUCTION
Paste this entire thing into Claude Code:

PROJECT CONTEXT
You're working on Vectoreologist — a Go-based knowledge archaeology engine that analyzes vector embeddings from Qdrant using UMAP/HDBSCAN clustering and DeepSeek R1 reasoning.
Current State:

CLI tool works perfectly ✅
Generates markdown reports in findings/vectoreology_TIMESTAMP.md ✅
Has full test coverage, CI/CD, cross-platform builds ✅
Problem: Output is buried in markdown files — no way to explore findings interactively ❌

Goal: Build Vectoreologist Lens — a Bubbletea TUI for interactive exploration of vector topology findings.

REPOSITORY STRUCTURE
vectoreologist/
├── cmd/vectoreologist/main.go          # Existing CLI
├── cmd/vectoreologist-lens/main.go     # NEW: TUI entry point (you'll create)
├── internal/
│   ├── models/models.go                # Shared types (Cluster, Bridge, Moat, Finding)
│   ├── synthesis/
│   │   ├── report.go                   # Existing markdown generator
│   │   └── json.go                     # NEW: JSON export (you'll create)
│   └── lens/                           # NEW: TUI package (you'll create)
│       ├── model.go                    # Bubbletea model
│       ├── views.go                    # View rendering functions
│       ├── commands.go                 # Bubbletea commands (load, filter, sort)
│       ├── styles.go                   # Lipgloss styles
│       └── loader.go                   # JSON loader
├── go.mod
├── go.sum
└── Makefile

TASK 1: ADD JSON EXPORT TO SYNTHESIS PACKAGE
File: internal/synthesis/json.go
Create a new file that exports findings to JSON alongside the markdown report.
Requirements:

Add a GenerateJSON() method to the Synthesizer struct
Write JSON file to same directory as markdown report
Filename: vectoreology_TIMESTAMP.json (matching the markdown filename pattern)
JSON structure:

json{
  "timestamp": "2026-04-15T14:30:00Z",
  "collection": "kae_chunks",
  "summary": {
    "total_clusters": 22,
    "total_bridges": 205,
    "total_moats": 0,
    "total_anomalies": 11
  },
  "clusters": [
    {
      "id": 1,
      "label": "surface / kae_chunks",
      "size": 42,
      "density": 0.73,
      "coherence": 0.91,
      "centroid": [0.1, 0.2, 0.3],
      "vector_ids": [1, 2, 3],
      "reasoning": "**Thinking:**\n...\n**Conclusion:**\n...",
      "is_anomaly": false
    }
  ],
  "bridges": [
    {
      "cluster_a": 1,
      "cluster_b": 7,
      "strength": 0.89,
      "link_type": "strong_semantic",
      "reasoning": "Why these connect..."
    }
  ],
  "moats": [
    {
      "cluster_a": 1,
      "cluster_b": 15,
      "distance": 0.95,
      "explanation": "Why isolated...",
      "reasoning": "DeepSeek explanation..."
    }
  ],
  "anomalies": [
    {
      "type": "coherence_anomaly",
      "subject": "Cluster 4: Pseudo-psychology",
      "cluster_id": 4,
      "reasoning_chain": "Low coherence suggests...",
      "is_anomaly": true
    }
  ]
}
Implementation Notes:

Enrich clusters with is_anomaly flag by checking if any Finding references that cluster
Enrich bridges/moats with their reasoning chains from findings
Match findings to clusters/bridges/moats by parsing the Subject field
Use time.Now().Format("2006-01-02_15-04-05") for timestamp (same as markdown)

File: internal/synthesis/report.go
Modify GenerateReport() to also call GenerateJSON():
gofunc (s *Synthesizer) GenerateReport(
	findings []models.Finding,
	clusters []models.Cluster,
	bridges []models.Bridge,
	moats []models.Moat,
) string {
	// ... existing markdown generation ...
	
	// NEW: Also generate JSON
	jsonPath := s.GenerateJSON(findings, clusters, bridges, moats)
	fmt.Printf("   ✓ JSON written to %s\n", jsonPath)
	
	return reportPath
}

TASK 2: CREATE BUBBLETEA TUI PACKAGE
File: internal/lens/model.go
Create the main Bubbletea model:
gopackage lens

import (
	"github.com/charmbracelet/bubbletea"
	"github.com/meistro57/vectoreologist/internal/models"
)

type ViewMode int

const (
	ClusterView ViewMode = iota
	BridgeView
	AnomalyView
	SearchView
)

type Model struct {
	// Data
	reportPath  string
	timestamp   string
	collection  string
	clusters    []models.Cluster
	bridges     []models.Bridge
	moats       []models.Moat
	anomalies   []models.Finding
	
	// UI State
	viewMode      ViewMode
	selectedIndex int
	scrollOffset  int
	
	// Filters
	showAnomaliesOnly bool
	minCoherence      float64
	maxDensity        float64
	showOrphansOnly   bool
	sortBy            string // "coherence", "density", "size"
	
	// Search
	searchQuery string
	searchResults []interface{} // can be clusters, bridges, or anomalies
	
	// Dimensions
	width  int
	height int
	
	// Status
	err error
}

func New(reportPath string) Model {
	return Model{
		reportPath:    reportPath,
		viewMode:      ClusterView,
		selectedIndex: 0,
		minCoherence:  0.0,
		maxDensity:    1.0,
		sortBy:        "coherence",
	}
}

func (m Model) Init() tea.Cmd {
	return LoadReport(m.reportPath)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle messages (keyboard, window size, loaded data, etc.)
	// TODO: Implement
	return m, nil
}

func (m Model) View() string {
	// Render current view
	// TODO: Implement
	return ""
}
File: internal/lens/commands.go
Bubbletea commands for async operations:
gopackage lens

import (
	"encoding/json"
	"os"
	
	"github.com/charmbracelet/bubbletea"
)

type ReportLoadedMsg struct {
	Timestamp  string
	Collection string
	Clusters   []models.Cluster
	Bridges    []models.Bridge
	Moats      []models.Moat
	Anomalies  []models.Finding
}

type ErrMsg struct{ Err error }

func LoadReport(path string) tea.Cmd {
	return func() tea.Msg {
		// Read and parse JSON
		// Return ReportLoadedMsg or ErrMsg
	}
}
File: internal/lens/views.go
View rendering functions:
gopackage lens

import (
	"fmt"
	"strings"
	
	"github.com/charmbracelet/lipgloss"
)

func (m Model) renderClusterView() string {
	// Render cluster list + detail panel
}

func (m Model) renderBridgeView() string {
	// Render bridge navigator
}

func (m Model) renderAnomalyView() string {
	// Render anomaly inspector
}

func (m Model) renderSearchView() string {
	// Render search interface
}

func (m Model) renderHeader() string {
	// Top status bar
}

func (m Model) renderFooter() string {
	// Keybinding help
}
File: internal/lens/styles.go
Lipgloss styling (cyberpunk theme to match KAE Lens):
gopackage lens

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	colorCyan    = lipgloss.Color("#00ff9f")
	colorBlue    = lipgloss.Color("#00d4ff")
	colorMagenta = lipgloss.Color("#ff00ff")
	colorGray    = lipgloss.Color("#808080")
	colorRed     = lipgloss.Color("#ff6b6b")
	
	// Styles
	headerStyle = lipgloss.NewStyle().
		Foreground(colorCyan).
		Bold(true).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(colorBlue).
		Padding(0, 1)
	
	selectedStyle = lipgloss.NewStyle().
		Foreground(colorCyan).
		Bold(true)
	
	anomalyStyle = lipgloss.NewStyle().
		Foreground(colorMagenta).
		Bold(true)
	
	// ... more styles
)

TASK 3: CREATE TUI ENTRY POINT
File: cmd/vectoreologist-lens/main.go
gopackage main

import (
	"fmt"
	"os"
	
	tea "github.com/charmbracelet/bubbletea"
	"github.com/meistro57/vectoreologist/internal/lens"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: vectoreologist-lens <path-to-vectoreology-json>")
		fmt.Println("Example: vectoreologist-lens findings/vectoreology_2026-04-15_14-30-00.json")
		os.Exit(1)
	}
	
	reportPath := os.Args[1]
	
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

TASK 4: UPDATE DEPENDENCIES
Add to go.mod:
bashgo get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/lipgloss@latest

TASK 5: UPDATE MAKEFILE
Add new build targets:
makefile# Add to existing Makefile

lens:
	go build $(LDFLAGS) -o vectoreologist-lens ./cmd/vectoreologist-lens

install-lens: lens
	cp vectoreologist-lens /usr/local/bin/

run-lens:
	./vectoreologist-lens findings/vectoreology_*.json

FUNCTIONAL REQUIREMENTS
Cluster View

List all clusters with: ID, label, size, coherence, density
Mark anomalies with ⚠ symbol
Selected cluster shows full reasoning chain in detail panel
Arrow keys to navigate, Enter to expand reasoning

Bridge View (accessed via b key from cluster view)

Show all bridges from selected cluster
Display: ClusterA ↔ ClusterB, strength, link type
Selected bridge shows full reasoning
Enter to jump to connected cluster

Anomaly View (accessed via a key)

List all anomalies grouped by type
Show: type, cluster ID, subject
Selected anomaly shows reasoning + affected clusters
Enter to jump to affected cluster

Search (accessed via / key)

Fuzzy search across cluster labels, reasoning chains
Show matching clusters, bridges, anomalies
Enter to jump to result

Filters (toggled via f key)

 Show anomalies only
 Coherence < X (adjustable)
 Density > Y (adjustable)
 Orphans only

Sort Options (cycled via s key)

Coherence (ascending/descending)
Density (ascending/descending)
Size (ascending/descending)
ID (default)

Export (via e key)

Export current cluster to JSON
Export filtered clusters to CSV
Export all anomalies to JSON
Copy reasoning chain to clipboard

Keybindings
↑↓      Navigate list
Enter   Expand/Select
b       Bridge view (from cluster)
a       Anomaly view
/       Search
f       Toggle filters
s       Cycle sort
e       Export menu
r       Reload report
tab     Switch view mode
esc     Back/Cancel
q       Quit

UI LAYOUT REQUIREMENTS
┌─ Header ──────────────────────────────────────────────────┐
│ Collection name • Stats • Timestamp                       │
├───────────────────────────────────────────────────────────┤
│                                                            │
│ [Left Panel: List]        [Right Panel: Details/Filters]  │
│                                                            │
│ Scrollable list           Selected item details           │
│ with navigation           or filter controls              │
│                                                            │
├───────────────────────────────────────────────────────────┤
│ Footer: Keybinding help                                   │
└───────────────────────────────────────────────────────────┘
Colors:

Cyan (#00ff9f) for highlights/selected
Blue (#00d4ff) for borders
Magenta (#ff00ff) for anomalies
Gray for normal text
Red for errors


TESTING REQUIREMENTS
Write tests for:

JSON loading (lens/loader_test.go)
Filter logic (lens/filter_test.go)
Sort logic (lens/sort_test.go)
Search matching (lens/search_test.go)


SUCCESS CRITERIA
✅ Run ./vectoreologist --collection kae_chunks → generates JSON + markdown
✅ Run ./vectoreologist-lens findings/vectoreology_*.json → TUI launches
✅ Navigate clusters with arrow keys
✅ See full DeepSeek reasoning chains
✅ Filter by anomaly/coherence/density
✅ Search across all findings
✅ Export selected findings to JSON/CSV
✅ Jump between clusters via bridges
✅ No crashes, graceful error handling
✅ Works on Linux/macOS

IMPLEMENTATION ORDER

Start with JSON export — modify synthesis/ package first
Build basic TUI skeleton — get something on screen with Bubbletea
Add cluster list view — load JSON, display clusters
Add detail panel — show selected cluster info
Add filters — implement filter logic
Add bridge/anomaly views — navigation between views
Add search — fuzzy matching
Add export — JSON/CSV output
Polish UI — styling, keybindings, help text
Write tests — unit tests for logic


CODING STYLE
Match existing Vectoreologist conventions:

Standard library only in tests
Errors via fmt.Fprintf(os.Stderr, ...)
Table-driven tests
No godoc comments for obvious functions
Keep packages focused (lens/ should only do UI)


EXAMPLE RUN
bash# Generate report with JSON
./vectoreologist --collection kae_chunks --sample 5000

# Launch TUI
./vectoreologist-lens findings/vectoreology_2026-04-15_14-30-00.json

# User navigates with arrow keys, presses 'b' to see bridges,
# presses 'a' to see anomalies, presses '/' to search,
# presses 'e' to export, presses 'q' to quit

START HERE
Begin with:
bashtouch internal/synthesis/json.go
touch internal/lens/model.go
touch cmd/vectoreologist-lens/main.go
Build incrementally, test frequently, commit often.
GO BUILD VECTOREOLOGIST LENS! 🔥
