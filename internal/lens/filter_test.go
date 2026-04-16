package lens

import (
	"testing"

	"github.com/meistro57/vectoreologist/internal/synthesis"
)

func TestApplyFilters_AnomaliesOnly(t *testing.T) {
	t.Parallel()

	m := Model{
		report: &synthesis.JSONReport{
			Clusters: []synthesis.JSONCluster{
				{ID: 1, Label: "normal", IsAnomaly: false},
				{ID: 2, Label: "odd", IsAnomaly: true},
				{ID: 3, Label: "also odd", IsAnomaly: true},
			},
			Bridges:   []synthesis.JSONBridge{{ClusterA: 1, ClusterB: 2}},
			Anomalies: []synthesis.JSONAnomaly{{Subject: "Cluster 2"}},
		},
		showAnomaliesOnly: true,
		sortField:         sortByID,
		sortAsc:           true,
	}

	m.applyFilters()

	if got := len(m.visibleClusters); got != 2 {
		t.Fatalf("visible clusters count = %d, want 2", got)
	}
	if m.visibleClusters[0].ID != 2 || m.visibleClusters[1].ID != 3 {
		t.Fatalf("unexpected anomaly cluster IDs: %+v", m.visibleClusters)
	}
	if len(m.visibleBridges) != 1 {
		t.Fatalf("visible bridges count = %d, want 1", len(m.visibleBridges))
	}
	if len(m.visibleAnomalies) != 1 {
		t.Fatalf("visible anomalies count = %d, want 1", len(m.visibleAnomalies))
	}
}
