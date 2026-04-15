package anomaly

import (
	"math"
	"testing"

	"github.com/meistro57/vectoreologist/internal/models"
)

// ---- helpers ---------------------------------------------------------------

func clusterWith(id int, label string, coherence, density float64) models.Cluster {
	return models.Cluster{
		ID:        id,
		Label:     label,
		Coherence: coherence,
		Density:   density,
		Size:      10,
	}
}

func findingsOfType(fs []models.Finding, typ string) []models.Finding {
	var out []models.Finding
	for _, f := range fs {
		if f.Type == typ {
			out = append(out, f)
		}
	}
	return out
}

// ---- DetectClusterAnomalies -------------------------------------------------

func TestDetectClusterAnomalies_EmptyInput(t *testing.T) {
	d := New()
	got := d.DetectClusterAnomalies(nil)
	if len(got) != 0 {
		t.Errorf("want 0 anomalies for empty input, got %d", len(got))
	}
}

func TestDetectClusterAnomalies_HealthyCluster(t *testing.T) {
	// Coherence >= 0.5, density in (0.3, 0.95) → no anomaly.
	d := New()
	cluster := clusterWith(1, "healthy", 0.7, 0.6)
	got := d.DetectClusterAnomalies([]models.Cluster{cluster})
	if len(got) != 0 {
		t.Errorf("healthy cluster should produce no anomalies, got %d", len(got))
	}
}

func TestDetectClusterAnomalies_LowCoherence(t *testing.T) {
	d := New()
	cluster := clusterWith(1, "incoherent", 0.3, 0.5) // coherence < 0.5
	got := d.DetectClusterAnomalies([]models.Cluster{cluster})

	cohAnoms := findingsOfType(got, "coherence_anomaly")
	if len(cohAnoms) != 1 {
		t.Fatalf("want 1 coherence_anomaly, got %d", len(cohAnoms))
	}
	if !cohAnoms[0].IsAnomaly {
		t.Error("IsAnomaly should be true")
	}
	if cohAnoms[0].Subject != "incoherent" {
		t.Errorf("Subject: want 'incoherent', got %q", cohAnoms[0].Subject)
	}
	if len(cohAnoms[0].Clusters) == 0 || cohAnoms[0].Clusters[0] != 1 {
		t.Errorf("Clusters should contain cluster ID 1, got %v", cohAnoms[0].Clusters)
	}
}

func TestDetectClusterAnomalies_CoherenceBoundary(t *testing.T) {
	d := New()
	// Exactly at threshold (0.5) should NOT trigger.
	at := clusterWith(1, "at", 0.5, 0.5)
	// Just below should trigger.
	below := clusterWith(2, "below", 0.499, 0.5)

	gotAt := d.DetectClusterAnomalies([]models.Cluster{at})
	if len(findingsOfType(gotAt, "coherence_anomaly")) != 0 {
		t.Error("coherence == threshold should not trigger anomaly")
	}

	gotBelow := d.DetectClusterAnomalies([]models.Cluster{below})
	if len(findingsOfType(gotBelow, "coherence_anomaly")) != 1 {
		t.Error("coherence just below threshold should trigger anomaly")
	}
}

func TestDetectClusterAnomalies_LowDensity(t *testing.T) {
	d := New()
	cluster := clusterWith(2, "sparse", 0.7, 0.1) // density < 0.3
	got := d.DetectClusterAnomalies([]models.Cluster{cluster})

	densAnoms := findingsOfType(got, "density_anomaly")
	if len(densAnoms) != 1 {
		t.Fatalf("want 1 density_anomaly for low density, got %d", len(densAnoms))
	}
	if densAnoms[0].Subject != "sparse" {
		t.Errorf("Subject: want 'sparse', got %q", densAnoms[0].Subject)
	}
}

func TestDetectClusterAnomalies_HighDensity(t *testing.T) {
	d := New()
	cluster := clusterWith(3, "dense", 0.7, 0.96) // density > 0.95
	got := d.DetectClusterAnomalies([]models.Cluster{cluster})

	densAnoms := findingsOfType(got, "density_anomaly")
	if len(densAnoms) != 1 {
		t.Fatalf("want 1 density_anomaly for high density, got %d", len(densAnoms))
	}
}

func TestDetectClusterAnomalies_DensityBoundaryValues(t *testing.T) {
	d := New()
	tests := []struct {
		label   string
		density float64
		want    int // expected density_anomaly count
	}{
		{"exactly 0.30", 0.30, 0},  // not < 0.3
		{"exactly 0.29", 0.29, 1},  // < 0.3
		{"exactly 0.95", 0.95, 0},  // not > 0.95
		{"exactly 0.96", 0.96, 1},  // > 0.95
	}
	for _, tc := range tests {
		c := clusterWith(1, tc.label, 0.7, tc.density)
		got := findingsOfType(d.DetectClusterAnomalies([]models.Cluster{c}), "density_anomaly")
		if len(got) != tc.want {
			t.Errorf("%s: want %d density_anomaly, got %d", tc.label, tc.want, len(got))
		}
	}
}

func TestDetectClusterAnomalies_BothAnomaliesOnSameCluster(t *testing.T) {
	// Low coherence AND low density → two findings.
	d := New()
	cluster := clusterWith(5, "bad", 0.2, 0.1)
	got := d.DetectClusterAnomalies([]models.Cluster{cluster})
	if len(got) != 2 {
		t.Errorf("want 2 findings (coherence + density), got %d", len(got))
	}
}

func TestDetectClusterAnomalies_MultipleClusters(t *testing.T) {
	d := New()
	clusters := []models.Cluster{
		clusterWith(1, "healthy", 0.8, 0.6),
		clusterWith(2, "low-coh", 0.2, 0.6),
		clusterWith(3, "low-den", 0.8, 0.1),
	}
	got := d.DetectClusterAnomalies(clusters)
	if len(got) != 2 {
		t.Errorf("want 2 total anomalies, got %d", len(got))
	}
}

// ---- DetectOrphans ----------------------------------------------------------

func TestDetectOrphans_AllConnected(t *testing.T) {
	d := New()
	clusters := []models.Cluster{
		clusterWith(1, "A", 0.7, 0.5),
		clusterWith(2, "B", 0.7, 0.5),
	}
	bridges := []models.Bridge{
		{ClusterA: 1, ClusterB: 2, Strength: 0.5},
	}
	got := d.DetectOrphans(clusters, bridges)
	if len(got) != 0 {
		t.Errorf("all clusters are connected; want 0 orphans, got %d", len(got))
	}
}

func TestDetectOrphans_AllIsolated(t *testing.T) {
	d := New()
	clusters := []models.Cluster{
		clusterWith(1, "A", 0.7, 0.5),
		clusterWith(2, "B", 0.7, 0.5),
	}
	got := d.DetectOrphans(clusters, nil)
	if len(got) != 2 {
		t.Errorf("no bridges; want 2 orphans, got %d", len(got))
	}
	for _, f := range got {
		if f.Type != "orphan_cluster" {
			t.Errorf("wrong finding type: want orphan_cluster, got %q", f.Type)
		}
		if !f.IsAnomaly {
			t.Error("IsAnomaly should be true for orphan")
		}
	}
}

func TestDetectOrphans_SomeConnected(t *testing.T) {
	d := New()
	clusters := []models.Cluster{
		clusterWith(1, "A", 0.7, 0.5),
		clusterWith(2, "B", 0.7, 0.5),
		clusterWith(3, "C", 0.7, 0.5), // no bridge
	}
	bridges := []models.Bridge{
		{ClusterA: 1, ClusterB: 2},
	}
	got := d.DetectOrphans(clusters, bridges)
	if len(got) != 1 {
		t.Fatalf("want 1 orphan, got %d", len(got))
	}
	if got[0].Subject != "C" {
		t.Errorf("orphan should be cluster C, got %q", got[0].Subject)
	}
}

func TestDetectOrphans_EmptyClusters(t *testing.T) {
	d := New()
	got := d.DetectOrphans(nil, nil)
	if len(got) != 0 {
		t.Errorf("empty input: want 0, got %d", len(got))
	}
}

func TestDetectOrphans_BridgeBothDirections(t *testing.T) {
	// A bridge (A,B) should mark BOTH A and B as connected.
	d := New()
	clusters := []models.Cluster{
		clusterWith(10, "X", 0.7, 0.5),
		clusterWith(20, "Y", 0.7, 0.5),
	}
	bridges := []models.Bridge{{ClusterA: 10, ClusterB: 20}}
	got := d.DetectOrphans(clusters, bridges)
	if len(got) != 0 {
		t.Errorf("both endpoints should be marked connected; got %d orphans", len(got))
	}
}

// ---- DetectContradictions ---------------------------------------------------

func makeMetadata(vectors []uint64, sources []string) []models.VectorMetadata {
	meta := make([]models.VectorMetadata, len(vectors))
	for i, id := range vectors {
		meta[i] = models.VectorMetadata{
			ID:     id,
			Source: sources[i%len(sources)],
		}
	}
	return meta
}

func TestDetectContradictions_HighCoherenceManySources(t *testing.T) {
	d := New()
	// 4 distinct sources + coherence > 0.8 → contradiction.
	vids := []uint64{1, 2, 3, 4, 5, 6, 7, 8}
	sources := []string{"src1", "src2", "src3", "src4"}
	cluster := models.Cluster{
		ID:        1,
		Label:     "multi-source",
		VectorIDs: vids,
		Coherence: 0.9,
	}
	meta := makeMetadata(vids, sources)

	got := d.DetectContradictions([]models.Cluster{cluster}, meta)
	contra := findingsOfType(got, "source_contradiction")
	if len(contra) != 1 {
		t.Fatalf("want 1 source_contradiction, got %d", len(contra))
	}
	if !contra[0].IsAnomaly {
		t.Error("contradiction should be marked IsAnomaly")
	}
}

func TestDetectContradictions_HighCoherenceFewSources(t *testing.T) {
	d := New()
	// Only 2 distinct sources → no contradiction even if coherence is high.
	vids := []uint64{1, 2, 3, 4}
	sources := []string{"src1", "src2"}
	cluster := models.Cluster{
		ID:        1,
		Label:     "dual-source",
		VectorIDs: vids,
		Coherence: 0.9,
	}
	meta := makeMetadata(vids, sources)

	got := d.DetectContradictions([]models.Cluster{cluster}, meta)
	if len(got) != 0 {
		t.Errorf("few sources should not trigger contradiction, got %d", len(got))
	}
}

func TestDetectContradictions_LowCoherenceManySources(t *testing.T) {
	d := New()
	// Low coherence → no contradiction despite many sources.
	vids := []uint64{1, 2, 3, 4, 5, 6, 7, 8}
	sources := []string{"src1", "src2", "src3", "src4"}
	cluster := models.Cluster{
		ID:        1,
		Label:     "incoherent-multi",
		VectorIDs: vids,
		Coherence: 0.5, // not > 0.8
	}
	meta := makeMetadata(vids, sources)

	got := d.DetectContradictions([]models.Cluster{cluster}, meta)
	if len(got) != 0 {
		t.Errorf("low coherence should not trigger contradiction, got %d", len(got))
	}
}

func TestDetectContradictions_EmptyInput(t *testing.T) {
	d := New()
	got := d.DetectContradictions(nil, nil)
	if len(got) != 0 {
		t.Errorf("empty input: want 0, got %d", len(got))
	}
}

func TestDetectContradictions_ExactlyThreeSources(t *testing.T) {
	d := New()
	// Exactly 3 sources is NOT > 3, so no contradiction.
	vids := []uint64{1, 2, 3, 4, 5, 6}
	sources := []string{"src1", "src2", "src3"}
	cluster := models.Cluster{
		ID:        1,
		Label:     "three-source",
		VectorIDs: vids,
		Coherence: 0.9,
	}
	meta := makeMetadata(vids, sources)

	got := d.DetectContradictions([]models.Cluster{cluster}, meta)
	if len(got) != 0 {
		t.Errorf("exactly 3 sources should NOT trigger contradiction (need > 3), got %d", len(got))
	}
}

// ---- ScoreAnomaly -----------------------------------------------------------

func TestScoreAnomaly_CoherenceAnomaly(t *testing.T) {
	d := New()
	f := models.Finding{Type: "coherence_anomaly"}

	// Score = 1.0 - coherence
	tests := []struct {
		coherence float64
		want      float64
	}{
		{0.0, 1.0},
		{0.5, 0.5},
		{1.0, 0.0},
		{0.3, 0.7},
	}
	for _, tc := range tests {
		c := clusterWith(1, "x", tc.coherence, 0.5)
		got := d.ScoreAnomaly(f, c)
		if math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("coherence=%.2f: want score %.4f, got %.4f", tc.coherence, tc.want, got)
		}
	}
}

func TestScoreAnomaly_DensityAnomaly(t *testing.T) {
	d := New()
	f := models.Finding{Type: "density_anomaly"}
	idealDensity := 0.7

	tests := []struct {
		density float64
	}{
		{0.0},
		{0.5},
		{0.7},
		{0.95},
		{1.0},
	}
	for _, tc := range tests {
		c := clusterWith(1, "x", 0.5, tc.density)
		got := d.ScoreAnomaly(f, c)
		want := math.Abs(tc.density - idealDensity)
		if math.Abs(got-want) > 1e-9 {
			t.Errorf("density=%.2f: want %.4f, got %.4f", tc.density, want, got)
		}
	}
}

func TestScoreAnomaly_OrphanCluster(t *testing.T) {
	d := New()
	f := models.Finding{Type: "orphan_cluster"}
	c := clusterWith(1, "x", 0.5, 0.5)
	got := d.ScoreAnomaly(f, c)
	if got != 0.9 {
		t.Errorf("orphan_cluster score: want 0.9, got %v", got)
	}
}

func TestScoreAnomaly_SourceContradiction(t *testing.T) {
	d := New()
	f := models.Finding{Type: "source_contradiction"}
	c := clusterWith(1, "x", 0.5, 0.5)
	got := d.ScoreAnomaly(f, c)
	if got != 0.95 {
		t.Errorf("source_contradiction score: want 0.95, got %v", got)
	}
}

func TestScoreAnomaly_UnknownType(t *testing.T) {
	d := New()
	f := models.Finding{Type: "something_else"}
	c := clusterWith(1, "x", 0.5, 0.5)
	got := d.ScoreAnomaly(f, c)
	if got != 0.5 {
		t.Errorf("unknown type score: want 0.5, got %v", got)
	}
}

// ---- New constructor --------------------------------------------------------

func TestNew_DefaultThresholds(t *testing.T) {
	d := New()
	if d.densityThreshold != 0.3 {
		t.Errorf("densityThreshold: want 0.3, got %v", d.densityThreshold)
	}
	if d.coherenceThreshold != 0.5 {
		t.Errorf("coherenceThreshold: want 0.5, got %v", d.coherenceThreshold)
	}
	if d.isolationThreshold != 0.1 {
		t.Errorf("isolationThreshold: want 0.1, got %v", d.isolationThreshold)
	}
}
