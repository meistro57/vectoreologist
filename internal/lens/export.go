package lens

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/meistro57/vectoreologist/internal/synthesis"
)

type exportEnvelope struct {
	ExportedAt string      `json:"exported_at"`
	SourceFile string      `json:"source_file"`
	Kind       string      `json:"kind"`
	Data       interface{} `json:"data"`
}

// exportCurrentItem writes the currently selected item to a JSON file
// and returns a short status message.
func (m *Model) exportCurrentItem() string {
	if m.report == nil {
		return "no report loaded"
	}
	switch m.viewMode {
	case clusterView:
		if len(m.visibleClusters) == 0 {
			return "no clusters visible"
		}
		c := m.visibleClusters[m.selectedIndex]
		return m.writeExport(fmt.Sprintf("cluster_%d", c.ID), c)
	case bridgeView:
		if len(m.visibleBridges) == 0 {
			return "no bridges visible"
		}
		b := m.visibleBridges[m.selectedIndex]
		return m.writeExport(fmt.Sprintf("bridge_%d_%d", b.ClusterA, b.ClusterB), b)
	case anomalyView:
		if len(m.visibleAnomalies) == 0 {
			return "no anomalies visible"
		}
		return m.writeExport("anomaly", m.visibleAnomalies[m.selectedIndex])
	default:
		return "switch to clusters/bridges/anomalies first"
	}
}

// exportVisibleList writes all currently visible items in the active view to JSON.
func (m *Model) exportVisibleList() string {
	if m.report == nil {
		return "no report loaded"
	}
	switch m.viewMode {
	case clusterView:
		return m.writeExport(fmt.Sprintf("clusters_%d", len(m.visibleClusters)), m.visibleClusters)
	case bridgeView:
		return m.writeExport(fmt.Sprintf("bridges_%d", len(m.visibleBridges)), m.visibleBridges)
	case anomalyView:
		return m.writeExport(fmt.Sprintf("anomalies_%d", len(m.visibleAnomalies)), m.visibleAnomalies)
	default:
		return m.writeExport("all", struct {
			Clusters  []synthesis.JSONCluster `json:"clusters"`
			Bridges   []synthesis.JSONBridge  `json:"bridges"`
			Anomalies []synthesis.JSONAnomaly `json:"anomalies"`
		}{m.visibleClusters, m.visibleBridges, m.visibleAnomalies})
	}
}

func (m *Model) writeExport(kind string, payload interface{}) string {
	ts := time.Now().Format("2006-01-02_15-04-05")
	name := fmt.Sprintf("export_%s_%s.json", kind, ts)
	path := filepath.Join(filepath.Dir(m.reportPath), name)

	env := exportEnvelope{
		ExportedAt: ts,
		SourceFile: filepath.Base(m.reportPath),
		Kind:       kind,
		Data:       payload,
	}
	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return fmt.Sprintf("✗ marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Sprintf("✗ write: %v", err)
	}
	return "✓ " + path
}
