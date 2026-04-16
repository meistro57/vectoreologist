package lens

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/meistro57/vectoreologist/internal/synthesis"
)

// ViewMode controls which panel is shown on the left.
type ViewMode int

const (
	clusterView ViewMode = iota
	bridgeView
	anomalyView
	searchView
)

// SortField controls cluster list ordering.
type SortField int

const (
	sortByID SortField = iota
	sortByCoherence
	sortByDensity
	sortBySize
)

// Model is the Bubbletea model for Vectoreologist Lens.
type Model struct {
	reportPath string
	report     *synthesis.JSONReport

	// displayed list (after filter + sort)
	visibleClusters []synthesis.JSONCluster
	visibleBridges  []synthesis.JSONBridge
	visibleAnomalies []synthesis.JSONAnomaly

	viewMode      ViewMode
	selectedIndex int
	detailScroll  int

	// filters
	showAnomaliesOnly bool
	sortField         SortField
	sortAsc           bool

	// search
	searchQuery   string
	searchActive  bool
	searchResults []searchResult

	// export
	showExportMenu bool
	exportStatus   string

	width   int
	height  int
	err     error
	loading bool
}

type searchResult struct {
	kind  string // "cluster", "bridge", "anomaly"
	index int    // index in the original slice
	label string
}

// New creates a fresh Model ready to load the given report path.
func New(reportPath string) Model {
	return Model{
		reportPath: reportPath,
		viewMode:   clusterView,
		sortField:  sortByCoherence,
		sortAsc:    false,
		loading:    true,
	}
}

func (m Model) Init() tea.Cmd {
	return loadReportCmd(m.reportPath)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case reportLoadedMsg:
		m.report = msg.report
		m.loading = false
		m.applyFilters()
		return m, nil

	case errMsg:
		m.err = msg.err
		m.loading = false
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.searchActive {
		return m.handleSearchKey(msg)
	}
	if m.showExportMenu {
		return m.handleExportKey(msg)
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return *m, tea.Quit

	case "up", "k":
		if m.selectedIndex > 0 {
			m.selectedIndex--
			m.detailScroll = 0
		}

	case "down", "j":
		max := m.listLen() - 1
		if m.selectedIndex < max {
			m.selectedIndex++
			m.detailScroll = 0
		}

	case "K":
		m.detailScroll = max(0, m.detailScroll-3)

	case "J":
		m.detailScroll += 3

	case "tab":
		m.viewMode = (m.viewMode + 1) % 3
		m.selectedIndex = 0
		m.detailScroll = 0

	case "b":
		m.viewMode = bridgeView
		m.selectedIndex = 0
		m.detailScroll = 0

	case "a":
		m.viewMode = anomalyView
		m.selectedIndex = 0
		m.detailScroll = 0

	case "c":
		m.viewMode = clusterView
		m.selectedIndex = 0
		m.detailScroll = 0

	case "/":
		m.searchActive = true
		m.searchQuery = ""
		m.viewMode = searchView
		m.selectedIndex = 0

	case "esc":
		if m.viewMode == searchView {
			m.searchActive = false
			m.viewMode = clusterView
			m.selectedIndex = 0
		}

	case "f":
		m.showAnomaliesOnly = !m.showAnomaliesOnly
		m.selectedIndex = 0
		m.applyFilters()

	case "s":
		m.cycleSortField()
		m.applyFilters()

	case "e":
		m.showExportMenu = true
		m.exportStatus = ""

	case "r":
		m.loading = true
		return *m, loadReportCmd(m.reportPath)

	case "enter":
		if m.viewMode == searchView && len(m.searchResults) > 0 {
			sr := m.searchResults[m.selectedIndex]
			switch sr.kind {
			case "cluster":
				m.viewMode = clusterView
				m.jumpToCluster(sr.index)
			case "bridge":
				m.viewMode = bridgeView
				m.selectedIndex = sr.index
			case "anomaly":
				m.viewMode = anomalyView
				m.selectedIndex = sr.index
			}
			m.searchActive = false
			m.detailScroll = 0
		}
	}
	return *m, nil
}

func (m *Model) handleExportKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.showExportMenu = false
		m.exportStatus = ""
	case "j":
		m.exportStatus = m.exportCurrentItem()
	case "v":
		m.exportStatus = m.exportVisibleList()
	}
	return *m, nil
}

func (m *Model) handleSearchKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.searchActive = false
		m.searchQuery = ""
		m.viewMode = clusterView
		m.selectedIndex = 0

	case "enter":
		if len(m.searchResults) > 0 {
			sr := m.searchResults[m.selectedIndex]
			switch sr.kind {
			case "cluster":
				m.viewMode = clusterView
				m.jumpToCluster(sr.index)
			case "bridge":
				m.viewMode = bridgeView
				m.selectedIndex = sr.index
			case "anomaly":
				m.viewMode = anomalyView
				m.selectedIndex = sr.index
			}
			m.searchActive = false
			m.detailScroll = 0
		}

	case "up", "k":
		if m.selectedIndex > 0 {
			m.selectedIndex--
		}

	case "down", "j":
		if m.selectedIndex < len(m.searchResults)-1 {
			m.selectedIndex++
		}

	case "backspace":
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			m.runSearch()
		}

	default:
		if len(msg.String()) == 1 {
			m.searchQuery += msg.String()
			m.runSearch()
			m.selectedIndex = 0
		}
	}
	return *m, nil
}

func (m *Model) runSearch() {
	if m.report == nil {
		return
	}
	q := strings.ToLower(m.searchQuery)
	m.searchResults = nil

	for i, c := range m.report.Clusters {
		if strings.Contains(strings.ToLower(c.Label), q) ||
			strings.Contains(strings.ToLower(c.Reasoning), q) {
			m.searchResults = append(m.searchResults, searchResult{"cluster", i, c.Label})
		}
	}
	for i, b := range m.report.Bridges {
		label := fmt.Sprintf("Cluster %d ↔ %d (%s)", b.ClusterA, b.ClusterB, b.LinkType)
		if strings.Contains(strings.ToLower(label), q) ||
			strings.Contains(strings.ToLower(b.Reasoning), q) {
			m.searchResults = append(m.searchResults, searchResult{"bridge", i, label})
		}
	}
	for i, an := range m.report.Anomalies {
		if strings.Contains(strings.ToLower(an.Subject), q) ||
			strings.Contains(strings.ToLower(an.ReasoningChain), q) {
			m.searchResults = append(m.searchResults, searchResult{"anomaly", i, an.Subject})
		}
	}
}

func (m *Model) applyFilters() {
	if m.report == nil {
		return
	}

	clusters := m.report.Clusters
	if m.showAnomaliesOnly {
		filtered := clusters[:0:0]
		for _, c := range clusters {
			if c.IsAnomaly {
				filtered = append(filtered, c)
			}
		}
		clusters = filtered
	}

	// Sort.
	sorted := make([]synthesis.JSONCluster, len(clusters))
	copy(sorted, clusters)
	sort.Slice(sorted, func(i, j int) bool {
		var less bool
		switch m.sortField {
		case sortByCoherence:
			less = sorted[i].Coherence < sorted[j].Coherence
		case sortByDensity:
			less = sorted[i].Density < sorted[j].Density
		case sortBySize:
			less = sorted[i].Size < sorted[j].Size
		default:
			less = sorted[i].ID < sorted[j].ID
		}
		if !m.sortAsc {
			less = !less
		}
		return less
	})
	m.visibleClusters = sorted
	m.visibleBridges = m.report.Bridges
	m.visibleAnomalies = m.report.Anomalies
}

func (m *Model) cycleSortField() {
	if m.sortField == sortBySize {
		m.sortField = sortByID
		m.sortAsc = true
	} else {
		m.sortField++
		m.sortAsc = false
	}
}

func (m *Model) jumpToCluster(reportIndex int) {
	if m.report == nil {
		return
	}
	target := m.report.Clusters[reportIndex].ID
	for i, c := range m.visibleClusters {
		if c.ID == target {
			m.selectedIndex = i
			m.detailScroll = 0
			return
		}
	}
	m.selectedIndex = 0
}

func (m Model) listLen() int {
	switch m.viewMode {
	case bridgeView:
		return len(m.visibleBridges)
	case anomalyView:
		return len(m.visibleAnomalies)
	case searchView:
		return len(m.searchResults)
	default:
		return len(m.visibleClusters)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
