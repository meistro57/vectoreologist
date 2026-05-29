package reasoner

import (
	"path/filepath"
	"testing"

	"github.com/meistro57/vectoreologist/internal/models"
)

func TestBudgetForProfile(t *testing.T) {
	fast := BudgetForProfile("fast")
	if fast.MaxClusters != 12 || fast.MaxBridges != 4 || fast.MaxMoats != 2 {
		t.Fatalf("fast budget mismatch: %+v", fast)
	}
	balanced := BudgetForProfile("balanced")
	if balanced.MaxClusters != 0 || balanced.MaxBridges != 10 || balanced.MaxMoats != 5 {
		t.Fatalf("balanced budget mismatch: %+v", balanced)
	}
	deep := BudgetForProfile("deep")
	if deep.MaxClusters != 0 || deep.MaxBridges != 25 || deep.MaxMoats != 12 {
		t.Fatalf("deep budget mismatch: %+v", deep)
	}
}

func TestResolveBudget_Overrides(t *testing.T) {
	budget, profile := ResolveBudget("fast", -1, 8, 0)
	if profile != "fast" {
		t.Fatalf("profile mismatch: %s", profile)
	}
	if budget.MaxClusters != 12 || budget.MaxBridges != 8 || budget.MaxMoats != 0 {
		t.Fatalf("resolved budget mismatch: %+v", budget)
	}
}

func TestApplyBudgets(t *testing.T) {
	clusters := []models.Cluster{
		{ID: 1, Size: 5, Coherence: 0.60},
		{ID: 2, Size: 10, Coherence: 0.80},
		{ID: 3, Size: 7, Coherence: 0.70},
	}
	selectedClusters := applyClusterBudget(clusters, 2)
	if len(selectedClusters) != 2 {
		t.Fatalf("cluster budget len = %d", len(selectedClusters))
	}
	if selectedClusters[0].ID != 2 || selectedClusters[1].ID != 3 {
		t.Fatalf("cluster budget selection = %+v", selectedClusters)
	}

	bridges := []models.Bridge{
		{ClusterA: 1, ClusterB: 2, Strength: 0.30},
		{ClusterA: 2, ClusterB: 3, Strength: 0.70},
		{ClusterA: 3, ClusterB: 4, Strength: 0.50},
	}
	selectedBridges := applyBridgeBudget(bridges, 2)
	if len(selectedBridges) != 2 {
		t.Fatalf("bridge budget len = %d", len(selectedBridges))
	}
	if selectedBridges[0].Strength < selectedBridges[1].Strength {
		t.Fatalf("bridge order incorrect: %+v", selectedBridges)
	}

	moats := []models.Moat{
		{ClusterA: 1, ClusterB: 2, Distance: 0.20},
		{ClusterA: 3, ClusterB: 4, Distance: 0.90},
		{ClusterA: 5, ClusterB: 6, Distance: 0.50},
	}
	selectedMoats := applyMoatBudget(moats, 2)
	if len(selectedMoats) != 2 {
		t.Fatalf("moat budget len = %d", len(selectedMoats))
	}
	if selectedMoats[0].Distance < selectedMoats[1].Distance {
		t.Fatalf("moat order incorrect: %+v", selectedMoats)
	}
}

func TestTopologyFingerprint_DeterministicAndSensitive(t *testing.T) {
	clusters := []models.Cluster{{ID: 1, Size: 3, Density: 0.71, Coherence: 0.82, Centroid: []float32{0.1, 0.2}}}
	bridges := []models.Bridge{{ClusterA: 1, ClusterB: 2, Strength: 0.63}}
	moats := []models.Moat{{ClusterA: 2, ClusterB: 3, Distance: 0.77}}
	budget := ReasoningBudget{MaxClusters: 0, MaxBridges: 10, MaxMoats: 5}

	a := TopologyFingerprint("col", 42, "deepseek-chat", budget, clusters, bridges, moats)
	b := TopologyFingerprint("col", 42, "deepseek-chat", budget, clusters, bridges, moats)
	if a != b {
		t.Fatalf("expected deterministic fingerprint")
	}

	changed := TopologyFingerprint("col", 99, "deepseek-chat", budget, clusters, bridges, moats)
	if changed == a {
		t.Fatalf("fingerprint should change when seed changes")
	}
}

func TestCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "cache")
	fingerprint := "abc123"
	findings := []models.Finding{{Type: "cluster_analysis", Subject: "Cluster 1: x", ReasoningChain: "ok", Clusters: []int{1}}}

	if err := SaveCachedFindings(cacheDir, fingerprint, findings); err != nil {
		t.Fatalf("save cache: %v", err)
	}
	loaded, hit, err := LoadCachedFindings(cacheDir, fingerprint)
	if err != nil {
		t.Fatalf("load cache: %v", err)
	}
	if !hit {
		t.Fatal("expected cache hit")
	}
	if len(loaded) != 1 || loaded[0].Subject != findings[0].Subject {
		t.Fatalf("cache payload mismatch: %+v", loaded)
	}
}
