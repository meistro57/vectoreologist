package synthesis

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/meistro57/vectoreologist/internal/models"
)

func TestParseClusterID(t *testing.T) {
	tests := []struct {
		subject string
		want    int
	}{
		{"Cluster 1: surface / The Kybalion", 1},
		{"Cluster 7: surface / The Republic", 7},
		{"Cluster 14: deep / arxiv", 14},
		{"not a cluster", 0},
		{"", 0},
	}
	for _, tt := range tests {
		if got := parseClusterID(tt.subject); got != tt.want {
			t.Errorf("parseClusterID(%q) = %d, want %d", tt.subject, got, tt.want)
		}
	}
}

func TestParsePair(t *testing.T) {
	tests := []struct {
		subject string
		sep     string
		wantA   int
		wantB   int
	}{
		{"Bridge: 3 ↔ 7", "↔", 3, 7},
		{"Moat: 2 ⊥ 9", "⊥", 2, 9},
		{"Bridge: 10 ↔ 1", "↔", 10, 1},
		{"not a pair", "↔", 0, 0},
	}
	for _, tt := range tests {
		a, b := parsePair(tt.subject, tt.sep)
		if a != tt.wantA || b != tt.wantB {
			t.Errorf("parsePair(%q, %q) = (%d,%d), want (%d,%d)",
				tt.subject, tt.sep, a, b, tt.wantA, tt.wantB)
		}
	}
}

func TestGenerateJSON_ReasoningAttachedToClusters(t *testing.T) {
	dir := t.TempDir()
	s := &Synthesizer{outputPath: dir}

	clusters := []models.Cluster{
		{ID: 1, Label: "surface / Foo", Size: 10, Density: 0.8, Coherence: 0.9},
		{ID: 2, Label: "surface / Bar", Size: 5, Density: 0.7, Coherence: 0.4},
	}
	bridges := []models.Bridge{
		{ClusterA: 1, ClusterB: 2, Strength: 0.75, LinkType: "strong_semantic"},
	}
	findings := []models.Finding{
		{
			Type:           "cluster_analysis",
			Subject:        "Cluster 1: surface / Foo",
			ReasoningChain: "Foo reasoning text",
			IsAnomaly:      false,
		},
		{
			Type:           "cluster_analysis",
			Subject:        "Cluster 2: surface / Bar",
			ReasoningChain: "Bar reasoning text",
			IsAnomaly:      true,
		},
		{
			Type:           "bridge_analysis",
			Subject:        "Bridge: 1 ↔ 2",
			ReasoningChain: "bridge reasoning",
		},
	}

	path := s.GenerateJSON(findings, clusters, bridges, nil, "test_collection", "2026-01-01_00-00-00")
	if path == "" {
		t.Fatal("GenerateJSON returned empty path")
	}

	data, err := os.ReadFile(filepath.Join(dir, "vectoreology_2026-01-01_00-00-00.json"))
	if err != nil {
		t.Fatalf("read JSON: %v", err)
	}

	var report JSONReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if report.Collection != "test_collection" {
		t.Errorf("collection = %q, want %q", report.Collection, "test_collection")
	}
	if len(report.Clusters) != 2 {
		t.Fatalf("clusters len = %d, want 2", len(report.Clusters))
	}
	if report.Clusters[0].Reasoning != "Foo reasoning text" {
		t.Errorf("cluster 1 reasoning = %q", report.Clusters[0].Reasoning)
	}
	if report.Clusters[1].Reasoning != "Bar reasoning text" {
		t.Errorf("cluster 2 reasoning = %q", report.Clusters[1].Reasoning)
	}
	if !report.Clusters[1].IsAnomaly {
		t.Error("cluster 2 should be anomaly")
	}
	if report.Bridges[0].Reasoning != "bridge reasoning" {
		t.Errorf("bridge reasoning = %q", report.Bridges[0].Reasoning)
	}
	if len(report.Anomalies) != 1 {
		t.Errorf("anomalies len = %d, want 1", len(report.Anomalies))
	}
}
