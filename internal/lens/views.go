package lens

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/meistro57/vectoreologist/internal/synthesis"
)

// View renders the full TUI screen.
func (m Model) View() string {
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("\n  ✗ Error: %v\n\n  Press q to quit.", m.err))
	}
	if m.loading {
		return titleStyle.Render("\n  ⚙ Loading report…")
	}
	if m.report == nil {
		return dimStyle.Render("\n  No report loaded.")
	}

	header := m.renderHeader()
	footer := m.renderFooter()

	// Available height for content area.
	headerH := lipgloss.Height(header)
	footerH := lipgloss.Height(footer)
	exportH := 0
	if m.showExportMenu {
		exportH = 3
	}
	contentH := m.height - headerH - footerH - exportH
	if contentH < 4 {
		contentH = 4
	}

	var content string
	switch m.viewMode {
	case bridgeView:
		content = m.renderBridgeView(contentH)
	case anomalyView:
		content = m.renderAnomalyView(contentH)
	case searchView:
		content = m.renderSearchView(contentH)
	default:
		content = m.renderClusterView(contentH)
	}

	if m.showExportMenu {
		return header + "\n" + content + "\n" + m.renderExportMenu() + "\n" + footer
	}
	return header + "\n" + content + "\n" + footer
}

// renderHeader renders the top status bar.
func (m Model) renderHeader() string {
	r := m.report
	tabs := m.renderTabs()
	info := fmt.Sprintf(
		"%s  %s clusters  %s bridges  %s anomalies",
		dimStyle.Render(r.Timestamp),
		titleStyle.Render(fmt.Sprintf("%d", r.Summary.TotalClusters)),
		bridgeStyle.Render(fmt.Sprintf("%d", r.Summary.TotalBridges)),
		anomalyStyle.Render(fmt.Sprintf("%d", r.Summary.TotalAnomalies)),
	)
	collectionLabel := titleStyle.Render("🏺 " + r.Collection)

	width := m.width
	if width < 40 {
		width = 40
	}

	top := lipgloss.JoinHorizontal(lipgloss.Top,
		collectionLabel,
		dimStyle.Render("  "+info),
	)
	return headerStyle.Width(width-4).Render(top) + "\n" + tabs
}

func (m Model) renderTabs() string {
	views := []struct {
		mode  ViewMode
		label string
		key   string
	}{
		{clusterView, "Clusters", "c"},
		{bridgeView, "Bridges", "b"},
		{anomalyView, "Anomalies", "a"},
		{searchView, "Search", "/"},
	}

	var parts []string
	for _, v := range views {
		label := fmt.Sprintf("[%s] %s", v.key, v.label)
		if m.viewMode == v.mode {
			parts = append(parts, tabActiveStyle.Render(label))
		} else {
			parts = append(parts, tabInactiveStyle.Render(label))
		}
	}

	filters := ""
	if m.showAnomaliesOnly {
		filters += anomalyStyle.Render(" ⚠ anomalies-only")
	}
	filters += dimStyle.Render(fmt.Sprintf("  sort:%s", sortName(m.sortField, m.sortAsc)))

	return "  " + strings.Join(parts, dimStyle.Render("  │  ")) + filters
}

func sortName(f SortField, asc bool) string {
	dir := "↓"
	if asc {
		dir = "↑"
	}
	switch f {
	case sortByCoherence:
		return "coherence" + dir
	case sortByDensity:
		return "density" + dir
	case sortBySize:
		return "size" + dir
	default:
		return "id" + dir
	}
}

// renderFooter renders the keybinding help bar.
func (m Model) renderFooter() string {
	hints := []string{
		"↑↓/jk navigate",
		"JK scroll detail",
		"c/b/a views",
		"/ search",
		"f filter",
		"s sort",
		"e export",
		"r reload",
		"q quit",
	}
	return footerStyle.Width(m.width).Render(strings.Join(hints, dimStyle.Render("  ·  ")))
}

// renderExportMenu renders the export overlay panel.
func (m Model) renderExportMenu() string {
	hints := dimStyle.Render("[j] current item  ·  [v] visible list  ·  [esc] close")
	line := titleStyle.Render("Export") + "  " + hints
	if m.exportStatus != "" {
		line += "\n  " + m.exportStatus
	}
	width := m.width - 4
	if width < 20 {
		width = 20
	}
	return exportMenuStyle.Width(width).Render(line)
}

// ── Cluster View ──────────────────────────────────────────────────────────────

func (m Model) renderClusterView(height int) string {
	listW := m.width * 2 / 5
	detailW := m.width - listW - 3

	list := m.renderClusterList(listW, height)
	detail := m.renderClusterDetail(detailW, height)

	return lipgloss.JoinHorizontal(lipgloss.Top,
		panelBorderStyle.Width(listW).Height(height).Render(list),
		detailPanelStyle.Width(detailW).Height(height).Render(detail),
	)
}

func (m Model) renderClusterList(width, height int) string {
	if len(m.visibleClusters) == 0 {
		return dimStyle.Render("No clusters.")
	}

	// Scrolling window.
	visH := height - 2
	if visH < 1 {
		visH = 1
	}
	start := 0
	if m.selectedIndex >= visH {
		start = m.selectedIndex - visH + 1
	}
	end := start + visH
	if end > len(m.visibleClusters) {
		end = len(m.visibleClusters)
	}

	var sb strings.Builder
	sb.WriteString(titleStyle.Render(fmt.Sprintf("Clusters (%d)", len(m.visibleClusters))) + "\n")

	for i := start; i < end; i++ {
		c := m.visibleClusters[i]
		prefix := "  "
		style := normalStyle
		if i == m.selectedIndex {
			prefix = "▶ "
			style = selectedStyle
		}
		anomalyMark := ""
		if c.IsAnomaly {
			anomalyMark = anomalyStyle.Render(" ⚠")
		}
		coherence := coherenceColor(c.Coherence).Render(fmt.Sprintf("%.2f", c.Coherence))
		line := fmt.Sprintf("%s#%d %s%s  coh:%s  n:%d",
			prefix, c.ID,
			truncate(c.Label, width-28),
			anomalyMark,
			coherence,
			c.Size,
		)
		sb.WriteString(style.Render(line) + "\n")
	}
	return sb.String()
}

func (m Model) renderClusterDetail(width, height int) string {
	if len(m.visibleClusters) == 0 {
		return dimStyle.Render("Select a cluster.")
	}
	c := m.visibleClusters[m.selectedIndex]

	var lines []string
	anomalyMark := ""
	if c.IsAnomaly {
		anomalyMark = anomalyStyle.Render(" ⚠ ANOMALY")
	}
	lines = append(lines, titleStyle.Render(fmt.Sprintf("Cluster #%d%s", c.ID, anomalyMark)))
	lines = append(lines, dimStyle.Render(c.Label))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("%s  %s  %s  %s",
		labelVal("size", fmt.Sprintf("%d", c.Size)),
		labelVal("coherence", coherenceColor(c.Coherence).Render(fmt.Sprintf("%.3f", c.Coherence))),
		labelVal("density", fmt.Sprintf("%.3f", c.Density)),
		labelVal("vectors", fmt.Sprintf("%d", len(c.VectorIDs))),
	))
	lines = append(lines, "")

	if c.Reasoning != "" {
		lines = append(lines, subtitleStyle.Render("── Reasoning ──"))
		lines = append(lines, "")
		for _, l := range wrapLines(c.Reasoning, width-2) {
			lines = append(lines, reasoningStyle.Render(l))
		}
	} else {
		lines = append(lines, dimStyle.Render("No reasoning available."))
		lines = append(lines, dimStyle.Render("Run with a DeepSeek API key to generate analysis."))
	}

	return scrollLines(lines, m.detailScroll, height-2)
}

// ── Bridge View ───────────────────────────────────────────────────────────────

func (m Model) renderBridgeView(height int) string {
	listW := m.width * 2 / 5
	detailW := m.width - listW - 3

	list := m.renderBridgeList(listW, height)
	detail := m.renderBridgeDetail(detailW, height)

	return lipgloss.JoinHorizontal(lipgloss.Top,
		panelBorderStyle.Width(listW).Height(height).Render(list),
		detailPanelStyle.Width(detailW).Height(height).Render(detail),
	)
}

func (m Model) renderBridgeList(width, height int) string {
	bridges := m.visibleBridges
	if len(bridges) == 0 {
		return dimStyle.Render("No bridges.")
	}

	visH := height - 2
	start := 0
	if m.selectedIndex >= visH {
		start = m.selectedIndex - visH + 1
	}
	end := start + visH
	if end > len(bridges) {
		end = len(bridges)
	}

	var sb strings.Builder
	sb.WriteString(titleStyle.Render(fmt.Sprintf("Bridges (%d)", len(bridges))) + "\n")

	for i := start; i < end; i++ {
		b := bridges[i]
		prefix := "  "
		style := normalStyle
		if i == m.selectedIndex {
			prefix = "▶ "
			style = selectedStyle
		}
		strengthBar := strengthBar(b.Strength, 6)
		line := fmt.Sprintf("%s#%d↔#%d  %s  %s  %s",
			prefix, b.ClusterA, b.ClusterB,
			strengthBar,
			bridgeStyle.Render(fmt.Sprintf("%.2f", b.Strength)),
			dimStyle.Render(b.LinkType),
		)
		sb.WriteString(style.Render(line) + "\n")
	}
	return sb.String()
}

func (m Model) renderBridgeDetail(width, height int) string {
	if len(m.visibleBridges) == 0 {
		return dimStyle.Render("No bridges found.")
	}
	b := m.visibleBridges[m.selectedIndex]

	labelA := clusterLabel(m.report.Clusters, b.ClusterA)
	labelB := clusterLabel(m.report.Clusters, b.ClusterB)

	var lines []string
	lines = append(lines, titleStyle.Render(fmt.Sprintf("Bridge: #%d ↔ #%d", b.ClusterA, b.ClusterB)))
	lines = append(lines, dimStyle.Render(fmt.Sprintf("%s  ↔  %s", truncate(labelA, width/2-3), truncate(labelB, width/2-3))))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("%s  %s",
		labelVal("strength", bridgeStyle.Render(fmt.Sprintf("%.4f", b.Strength))),
		labelVal("type", b.LinkType),
	))
	lines = append(lines, "")

	if b.Reasoning != "" {
		lines = append(lines, subtitleStyle.Render("── Reasoning ──"))
		for _, l := range wrapLines(b.Reasoning, width-2) {
			lines = append(lines, reasoningStyle.Render(l))
		}
	} else {
		lines = append(lines, dimStyle.Render("No reasoning available."))
	}

	return scrollLines(lines, m.detailScroll, height-2)
}

// ── Anomaly View ──────────────────────────────────────────────────────────────

func (m Model) renderAnomalyView(height int) string {
	listW := m.width * 2 / 5
	detailW := m.width - listW - 3

	list := m.renderAnomalyList(listW, height)
	detail := m.renderAnomalyDetail(detailW, height)

	return lipgloss.JoinHorizontal(lipgloss.Top,
		panelBorderStyle.Width(listW).Height(height).Render(list),
		detailPanelStyle.Width(detailW).Height(height).Render(detail),
	)
}

func (m Model) renderAnomalyList(width, height int) string {
	anomalies := m.visibleAnomalies
	if len(anomalies) == 0 {
		return dimStyle.Render("No anomalies detected. 🎉")
	}

	visH := height - 2
	start := 0
	if m.selectedIndex >= visH {
		start = m.selectedIndex - visH + 1
	}
	end := start + visH
	if end > len(anomalies) {
		end = len(anomalies)
	}

	var sb strings.Builder
	sb.WriteString(titleStyle.Render(fmt.Sprintf("Anomalies (%d)", len(anomalies))) + "\n")

	for i := start; i < end; i++ {
		an := anomalies[i]
		prefix := "  "
		style := normalStyle
		if i == m.selectedIndex {
			prefix = "▶ "
			style = selectedStyle
		}
		marker := anomalyStyle.Render("⚠ ")
		subject := style.Render(truncate(an.Subject, width-20))
		typ := dimStyle.Render("  " + an.Type)
		sb.WriteString(normalStyle.Render(prefix) + marker + subject + typ + "\n")
	}
	return sb.String()
}

func (m Model) renderAnomalyDetail(width, height int) string {
	if len(m.visibleAnomalies) == 0 {
		return dimStyle.Render("No anomalies.")
	}
	an := m.visibleAnomalies[m.selectedIndex]

	var lines []string
	lines = append(lines, anomalyStyle.Render("⚠ "+an.Subject))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("%s  %s",
		labelVal("type", an.Type),
		labelVal("cluster", fmt.Sprintf("#%d", an.ClusterID)),
	))
	lines = append(lines, "")

	if an.ReasoningChain != "" {
		lines = append(lines, subtitleStyle.Render("── Reasoning Chain ──"))
		for _, l := range wrapLines(an.ReasoningChain, width-2) {
			lines = append(lines, reasoningStyle.Render(l))
		}
	}

	return scrollLines(lines, m.detailScroll, height-2)
}

// ── Search View ───────────────────────────────────────────────────────────────

func (m Model) renderSearchView(height int) string {
	var sb strings.Builder

	query := m.searchQuery
	cursor := searchCursorStyle.Render(" ")
	prompt := searchPromptStyle.Render("/") + " " + query + cursor
	sb.WriteString(panelBorderStyle.Render(prompt) + "\n")

	if len(m.searchResults) == 0 {
		if query == "" {
			sb.WriteString(dimStyle.Render("  Type to search across clusters, bridges, and anomalies."))
		} else {
			sb.WriteString(dimStyle.Render(fmt.Sprintf("  No results for %q", query)))
		}
		return sb.String()
	}

	sb.WriteString(dimStyle.Render(fmt.Sprintf("  %d results\n\n", len(m.searchResults))))

	for i, r := range m.searchResults {
		prefix := "  "
		style := normalStyle
		if i == m.selectedIndex {
			prefix = "▶ "
			style = selectedStyle
		}
		var kindStyle lipgloss.Style
		switch r.kind {
		case "bridge":
			kindStyle = bridgeStyle
		case "anomaly":
			kindStyle = anomalyStyle
		default:
			kindStyle = subtitleStyle
		}
		line := fmt.Sprintf("%s[%s] %s", prefix, r.kind, r.label)
		sb.WriteString(kindStyle.Render(prefix) + style.Render(line[len(prefix):]) + "\n")
		if i >= height-5 {
			break
		}
	}
	return sb.String()
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func labelVal(label, val string) string {
	return dimStyle.Render(label+":") + " " + val
}

func truncate(s string, n int) string {
	if n < 4 {
		n = 4
	}
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "…"
}

func wrapLines(text string, width int) []string {
	if width < 10 {
		width = 10
	}
	var result []string
	for _, para := range strings.Split(text, "\n") {
		if para == "" {
			result = append(result, "")
			continue
		}
		for len(para) > width {
			// Find last space before width.
			cut := width
			if idx := strings.LastIndex(para[:cut], " "); idx > 0 {
				cut = idx
			}
			result = append(result, para[:cut])
			para = para[cut:]
			if len(para) > 0 && para[0] == ' ' {
				para = para[1:]
			}
		}
		result = append(result, para)
	}
	return result
}

func scrollLines(lines []string, offset, height int) string {
	if offset > len(lines)-1 {
		offset = len(lines) - 1
	}
	if offset < 0 {
		offset = 0
	}
	end := offset + height
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[offset:end], "\n")
}

func strengthBar(v float64, width int) string {
	filled := int(v * float64(width))
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return bridgeStyle.Render(bar)
}

func clusterLabel(clusters []synthesis.JSONCluster, id int) string {
	for _, c := range clusters {
		if c.ID == id {
			return c.Label
		}
	}
	return fmt.Sprintf("#%d", id)
}
