package lens

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/meistro57/vectoreologist/internal/synthesis"
)

func TestLoadReport(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	reportPath := filepath.Join(tmp, "vectoreology_2026-04-16_10-00-00.json")

	input := synthesis.JSONReport{
		Timestamp:  "2026-04-16T10-00-00",
		Collection: "my_collection",
		Summary: synthesis.JSONSummary{
			TotalClusters:  2,
			TotalBridges:   1,
			TotalMoats:     0,
			TotalAnomalies: 1,
		},
		Clusters: []synthesis.JSONCluster{{ID: 1, Label: "alpha", Size: 4, Coherence: 0.9, Density: 0.7}},
		Bridges:  []synthesis.JSONBridge{{ClusterA: 1, ClusterB: 2, Strength: 0.81, LinkType: "strong_semantic"}},
		Anomalies: []synthesis.JSONAnomaly{{
			Type:      "coherence_anomaly",
			Subject:   "Cluster 2: beta",
			ClusterID: 2,
		}},
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	if err := os.WriteFile(reportPath, data, 0o644); err != nil {
		t.Fatalf("write report file: %v", err)
	}

	got, err := loadReport(reportPath)
	if err != nil {
		t.Fatalf("loadReport returned error: %v", err)
	}

	if got.Collection != input.Collection {
		t.Fatalf("collection mismatch: got %q want %q", got.Collection, input.Collection)
	}
	if len(got.Clusters) != 1 || got.Clusters[0].Label != "alpha" {
		t.Fatalf("unexpected clusters: %+v", got.Clusters)
	}
	if len(got.Bridges) != 1 || got.Bridges[0].Strength != 0.81 {
		t.Fatalf("unexpected bridges: %+v", got.Bridges)
	}
	if len(got.Anomalies) != 1 || got.Anomalies[0].ClusterID != 2 {
		t.Fatalf("unexpected anomalies: %+v", got.Anomalies)
	}
}
