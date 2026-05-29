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
		{
			Type:           "topology_diagnostics",
			Subject:        "DBSCAN params (configured defaults): eps=0.300 minPts=5",
			ReasoningChain: "requested_eps=0.300 requested_minPts=5 sampled_pairs=0 reason=Using configured DBSCAN defaults.",
		},
	}

	path := s.GenerateJSON(findings, clusters, bridges, nil, nil, "test_collection", "2026-01-01_00-00-00")
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
	if report.Topology == nil {
		t.Fatal("topology diagnostics should be present")
	}
	if report.Topology.Parameters == "" {
		t.Fatal("topology parameters should be populated")
	}
}

func TestGenerateJSON_ExposesDiagnosticsAndAttractors(t *testing.T) {
	dir := t.TempDir()
	s := &Synthesizer{outputPath: dir}

	clusters := []models.Cluster{
		{ID: 1, Label: "surface / x", Source: "surface / x", VectorIDs: []uint64{1, 2, 3, 4}, Size: 4, Density: 0.99, Coherence: 0.99},
		{ID: 2, Label: "surface / y", Source: "surface / y", VectorIDs: []uint64{5, 6, 7}, Size: 3, Density: 0.7, Coherence: 0.8},
	}
	metadata := []models.VectorMetadata{
		{ID: 1, Source: "mb_dom", Fragment: "alpha one"},
		{ID: 2, Source: "mb_dom", Fragment: "alpha two"},
		{ID: 3, Source: "mb_dom", Fragment: "alpha three"},
		{ID: 4, Source: "mb_dom", Fragment: "alpha four"},
		{ID: 5, Source: "mb_a", Fragment: "beta one"},
		{ID: 6, Source: "mb_b", Fragment: "beta two"},
		{ID: 7, Source: "mb_c", Fragment: "beta three"},
	}
	bridges := []models.Bridge{{ClusterA: 1, ClusterB: 2, Strength: 0.8, SampleLinks: []models.SampleLink{{ChunkAID: 1, ChunkBID: 99}}}}
	findings := []models.Finding{{
		Type: "cluster_analysis", Subject: "Cluster 1: x", Clusters: []int{1},
		ReasoningChain: "Shared archetype concept", Confidence: 0.8,
	}, {
		Type: "cluster_analysis", Subject: "Cluster 2: y", Clusters: []int{2},
		ReasoningChain: "Shared archetype concept", Confidence: 0.6,
	}, {
		Type:           "orphan_cluster",
		Subject:        "surface / x",
		Clusters:       []int{1},
		IsAnomaly:      true,
		Confidence:     0.77,
		ConfidenceBand: "high",
		ReasoningChain: "isolated",
	}}

	path := s.GenerateJSON(findings, clusters, bridges, nil, metadata, "mb_test", "2026-02-02_00-00-00")
	if path == "" {
		t.Fatal("GenerateJSON returned empty path")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read JSON: %v", err)
	}
	var report JSONReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if report.Summary.DuplicateHeavyClusters != 1 {
		t.Errorf("duplicate_heavy_clusters = %d, want 1", report.Summary.DuplicateHeavyClusters)
	}
	if report.Summary.OverSampledClusters != 1 {
		t.Errorf("oversampled_clusters = %d, want 1", report.Summary.OverSampledClusters)
	}
	if report.Summary.SkippedBridges != 1 {
		t.Errorf("skipped_bridges = %d, want 1", report.Summary.SkippedBridges)
	}
	if len(report.Attractors) == 0 {
		t.Fatalf("expected at least one attractor")
	}
	if len(report.Recommendations) == 0 {
		t.Fatal("expected recommendations")
	}
	if len(report.ReviewItems) == 0 {
		t.Fatal("expected review items")
	}

	// Cluster 1 should carry diagnostics + suggested label + snippets.
	c1 := report.Clusters[0]
	if !c1.DuplicateHeavy {
		t.Error("cluster 1 should be flagged duplicate_heavy in JSON")
	}
	if !c1.OverSampled {
		t.Error("cluster 1 should be flagged oversampled in JSON")
	}
	if c1.SuggestedLabel == "" {
		t.Error("cluster 1 should expose suggested_label")
	}
	if len(c1.Snippets) == 0 {
		t.Error("cluster 1 should expose representative_snippets")
	}
	if len(c1.SourceBalance) == 0 {
		t.Error("cluster 1 should expose source_balance")
	}
	if len(c1.ReviewFlags) == 0 {
		t.Error("cluster 1 should expose review_flags")
	}

	// Bridge should be flagged skipped with insufficient-evidence flags.
	if !report.Bridges[0].Skipped {
		t.Error("bridge should be flagged skipped in JSON")
	}
	if report.Bridges[0].SharedConcept == "" {
		t.Error("bridge should expose shared_concept")
	}
	if len(report.Anomalies) != 1 {
		t.Fatalf("expected one anomaly, got %d", len(report.Anomalies))
	}
	if report.Anomalies[0].ConfidenceBand != "high" {
		t.Fatalf("expected confidence band high, got %q", report.Anomalies[0].ConfidenceBand)
	}
}
