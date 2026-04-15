package topology

import (
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/meistro57/vectoreologist/internal/models"
)

// ---- cosineSimilarity -------------------------------------------------------

func TestCosineSimilarity_IdenticalVectors(t *testing.T) {
	a := []float32{1, 2, 3}
	got := cosineSimilarity(a, a)
	if math.Abs(got-1.0) > 1e-6 {
		t.Errorf("identical vectors: want 1.0, got %v", got)
	}
}

func TestCosineSimilarity_OppositeVectors(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{-1, 0, 0}
	got := cosineSimilarity(a, b)
	if math.Abs(got-(-1.0)) > 1e-6 {
		t.Errorf("opposite vectors: want -1.0, got %v", got)
	}
}

func TestCosineSimilarity_OrthogonalVectors(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{0, 1}
	got := cosineSimilarity(a, b)
	if math.Abs(got) > 1e-6 {
		t.Errorf("orthogonal vectors: want 0.0, got %v", got)
	}
}

func TestCosineSimilarity_ZeroVector(t *testing.T) {
	a := []float32{0, 0, 0}
	b := []float32{1, 2, 3}
	got := cosineSimilarity(a, b)
	if got != 0.0 {
		t.Errorf("zero vector: want 0.0, got %v", got)
	}
}

func TestCosineSimilarity_LengthMismatch(t *testing.T) {
	a := []float32{1, 2, 3}
	b := []float32{1, 2}
	got := cosineSimilarity(a, b)
	if got != 0.0 {
		t.Errorf("length mismatch: want 0.0, got %v", got)
	}
}

func TestCosineSimilarity_EmptyVectors(t *testing.T) {
	got := cosineSimilarity(nil, nil)
	if got != 0.0 {
		t.Errorf("nil vectors: want 0.0, got %v", got)
	}
}

func TestCosineSimilarity_KnownValue(t *testing.T) {
	// [1,1] vs [1,0] → cos(45°) = 1/√2 ≈ 0.7071
	a := []float32{1, 1}
	b := []float32{1, 0}
	got := cosineSimilarity(a, b)
	want := 1.0 / math.Sqrt2
	if math.Abs(got-want) > 1e-5 {
		t.Errorf("known value: want %.6f, got %.6f", want, got)
	}
}

// ---- classifyLink -----------------------------------------------------------

func TestClassifyLink_Table(t *testing.T) {
	tests := []struct {
		sim  float64
		want string
	}{
		{0.8, "strong_semantic"},
		{0.71, "strong_semantic"},
		{0.70, "moderate_bridge"}, // exactly 0.70 is NOT > 0.7, falls through to moderate_bridge
		{0.65, "moderate_bridge"},
		{0.50, "weak_connection"}, // exactly 0.50 is NOT > 0.5, falls through to weak_connection
		{0.45, "weak_connection"},
		{0.30, "isolated"}, // exactly 0.30 is NOT > 0.3, falls through to isolated
		{0.20, "isolated"},
		{0.0, "isolated"},
		{-0.5, "isolated"},
	}
	for _, tc := range tests {
		got := classifyLink(tc.sim)
		if got != tc.want {
			t.Errorf("classifyLink(%.2f) = %q, want %q", tc.sim, got, tc.want)
		}
	}
}

func TestClassifyLink_StrictBoundaries(t *testing.T) {
	// Confirm strict > comparisons at each boundary.
	if classifyLink(0.7) != "moderate_bridge" {
		t.Error("0.7 is NOT > 0.7, should be moderate_bridge")
	}
	if classifyLink(0.5) != "weak_connection" {
		t.Error("0.5 is NOT > 0.5, should be weak_connection")
	}
	if classifyLink(0.3) != "isolated" {
		t.Error("0.3 is NOT > 0.3, should be isolated")
	}
}

// ---- FindBridges ------------------------------------------------------------

func TestFindBridges_Empty(t *testing.T) {
	top := New()
	got := top.FindBridges(nil)
	if len(got) != 0 {
		t.Errorf("empty input: want 0 bridges, got %d", len(got))
	}
}

func TestFindBridges_SingleCluster(t *testing.T) {
	top := New()
	c := models.Cluster{ID: 1, Centroid: []float32{1, 0}}
	got := top.FindBridges([]models.Cluster{c})
	if len(got) != 0 {
		t.Errorf("single cluster: want 0 bridges, got %d", len(got))
	}
}

func TestFindBridges_HighSimilarity(t *testing.T) {
	top := New()
	// Nearly identical centroids → similarity > 0.3 → bridge.
	c1 := models.Cluster{ID: 1, Centroid: []float32{1, 0, 0}}
	c2 := models.Cluster{ID: 2, Centroid: []float32{1, 0, 0}}
	got := top.FindBridges([]models.Cluster{c1, c2})
	if len(got) != 1 {
		t.Fatalf("want 1 bridge, got %d", len(got))
	}
	if got[0].ClusterA != 1 || got[0].ClusterB != 2 {
		t.Errorf("bridge endpoints: want (1,2), got (%d,%d)", got[0].ClusterA, got[0].ClusterB)
	}
}

func TestFindBridges_LowSimilarity(t *testing.T) {
	top := New()
	// Orthogonal centroids → similarity = 0.0 → no bridge.
	c1 := models.Cluster{ID: 1, Centroid: []float32{1, 0}}
	c2 := models.Cluster{ID: 2, Centroid: []float32{0, 1}}
	got := top.FindBridges([]models.Cluster{c1, c2})
	if len(got) != 0 {
		t.Errorf("orthogonal centroids: want 0 bridges, got %d", len(got))
	}
}

func TestFindBridges_StrengthAndLinkType(t *testing.T) {
	top := New()
	// Identical unit vectors → sim = 1.0 → strong_semantic.
	c1 := models.Cluster{ID: 1, Centroid: []float32{1, 0}}
	c2 := models.Cluster{ID: 2, Centroid: []float32{1, 0}}
	got := top.FindBridges([]models.Cluster{c1, c2})
	if len(got) != 1 {
		t.Fatalf("want 1 bridge, got %d", len(got))
	}
	if got[0].LinkType != "strong_semantic" {
		t.Errorf("LinkType: want strong_semantic, got %q", got[0].LinkType)
	}
	if math.Abs(got[0].Strength-1.0) > 1e-5 {
		t.Errorf("Strength: want ~1.0, got %v", got[0].Strength)
	}
}

func TestFindBridges_NoDuplicatePairs(t *testing.T) {
	top := New()
	// Three clusters all with the same centroid → 3 pairs: (1,2),(1,3),(2,3).
	c1 := models.Cluster{ID: 1, Centroid: []float32{1, 0}}
	c2 := models.Cluster{ID: 2, Centroid: []float32{1, 0}}
	c3 := models.Cluster{ID: 3, Centroid: []float32{1, 0}}
	got := top.FindBridges([]models.Cluster{c1, c2, c3})
	if len(got) != 3 {
		t.Errorf("want 3 bridges for 3 similar clusters, got %d", len(got))
	}
	// Verify no (A,B) pair appears twice.
	seen := make(map[[2]int]bool)
	for _, b := range got {
		key := [2]int{b.ClusterA, b.ClusterB}
		if seen[key] {
			t.Errorf("duplicate bridge pair (%d,%d)", b.ClusterA, b.ClusterB)
		}
		seen[key] = true
	}
}

// ---- FindMoats --------------------------------------------------------------

func TestFindMoats_Empty(t *testing.T) {
	top := New()
	got := top.FindMoats(nil)
	if len(got) != 0 {
		t.Errorf("empty input: want 0 moats, got %d", len(got))
	}
}

func TestFindMoats_OrthogonalClusters(t *testing.T) {
	top := New()
	// Cosine sim = 0.0, which is < 0.1 → moat.
	c1 := models.Cluster{ID: 1, Centroid: []float32{1, 0}}
	c2 := models.Cluster{ID: 2, Centroid: []float32{0, 1}}
	got := top.FindMoats([]models.Cluster{c1, c2})
	if len(got) != 1 {
		t.Fatalf("want 1 moat, got %d", len(got))
	}
	if got[0].ClusterA != 1 || got[0].ClusterB != 2 {
		t.Errorf("moat endpoints: want (1,2), got (%d,%d)", got[0].ClusterA, got[0].ClusterB)
	}
}

func TestFindMoats_HighSimilarity_NoMoat(t *testing.T) {
	top := New()
	// Identical centroids → sim = 1.0, not < 0.1 → no moat.
	c1 := models.Cluster{ID: 1, Centroid: []float32{1, 0}}
	c2 := models.Cluster{ID: 2, Centroid: []float32{1, 0}}
	got := top.FindMoats([]models.Cluster{c1, c2})
	if len(got) != 0 {
		t.Errorf("similar clusters should produce no moat, got %d", len(got))
	}
}

func TestFindMoats_Distance(t *testing.T) {
	top := New()
	c1 := models.Cluster{ID: 1, Centroid: []float32{1, 0}}
	c2 := models.Cluster{ID: 2, Centroid: []float32{0, 1}}
	got := top.FindMoats([]models.Cluster{c1, c2})
	if len(got) != 1 {
		t.Fatalf("want 1 moat, got %d", len(got))
	}
	// sim = 0, so distance = 1.0 - 0 = 1.0
	if math.Abs(got[0].Distance-1.0) > 1e-5 {
		t.Errorf("distance: want 1.0, got %v", got[0].Distance)
	}
	if got[0].Explanation == "" {
		t.Error("moat explanation should not be empty")
	}
}

func TestFindMoats_NoDuplicatePairs(t *testing.T) {
	top := New()
	c1 := models.Cluster{ID: 1, Centroid: []float32{1, 0}}
	c2 := models.Cluster{ID: 2, Centroid: []float32{0, 1}}
	c3 := models.Cluster{ID: 3, Centroid: []float32{-1, 0}}
	got := top.FindMoats([]models.Cluster{c1, c2, c3})

	seen := make(map[[2]int]bool)
	for _, m := range got {
		key := [2]int{m.ClusterA, m.ClusterB}
		if seen[key] {
			t.Errorf("duplicate moat pair (%d,%d)", m.ClusterA, m.ClusterB)
		}
		seen[key] = true
	}
}

// ---- New constructor --------------------------------------------------------

func TestNew_DefaultParams(t *testing.T) {
	top := New()
	if top.neighbors != 15 {
		t.Errorf("neighbors: want 15, got %d", top.neighbors)
	}
	if top.minDist != 0.1 {
		t.Errorf("minDist: want 0.1, got %v", top.minDist)
	}
}

// ---- AnalyzeClusters --------------------------------------------------------

// checkPythonDeps returns true if python3 is available and umap-learn/hdbscan
// are installed.  If not, the test should be skipped.
func checkPythonDeps() bool {
	if _, err := exec.LookPath("python3"); err != nil {
		return false
	}
	cmd := exec.Command("python3", "-c", "import umap, hdbscan, numpy")
	return cmd.Run() == nil
}

func TestAnalyzeClusters_EmptyInput(t *testing.T) {
	top := New()
	got := top.AnalyzeClusters(nil, nil)
	if len(got) != 0 {
		t.Errorf("empty input: want nil/empty, got %d clusters", len(got))
	}
}

func TestAnalyzeClusters_WithPython(t *testing.T) {
	if !checkPythonDeps() {
		t.Skip("python3 with umap-learn+hdbscan not available; skipping AnalyzeClusters test")
	}

	top := New()
	// Build 30 simple 2-D vectors forming two loose blobs.
	var vecs [][]float32
	var meta []models.VectorMetadata
	for i := 0; i < 15; i++ {
		vecs = append(vecs, []float32{float32(i) * 0.1, 0})
		meta = append(meta, models.VectorMetadata{ID: uint64(i), Source: "src_a", Layer: "deep"})
	}
	for i := 15; i < 30; i++ {
		vecs = append(vecs, []float32{float32(i) * 0.1, 1.0})
		meta = append(meta, models.VectorMetadata{ID: uint64(i), Source: "src_b", Layer: "surface"})
	}

	clusters := top.AnalyzeClusters(vecs, meta)
	// With HDBSCAN on this well-separated data we should get at least 1 cluster.
	if len(clusters) == 0 {
		t.Error("expected at least 1 cluster from non-trivial input")
	}
}

func TestAnalyzeClusters_WithFakePython(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake python script injection not supported on Windows")
	}

	// Build a minimal well-formed cluster JSON response.
	fakeOutput := clusterOutput{
		Clusters: []struct {
			ID        int       `json:"id"`
			Label     string    `json:"label"`
			VectorIDs []uint64  `json:"vector_ids"`
			Centroid  []float32 `json:"centroid"`
			Density   float64   `json:"density"`
			Size      int       `json:"size"`
			Coherence float64   `json:"coherence"`
		}{
			{
				ID:        1,
				Label:     "surface / src",
				VectorIDs: []uint64{1, 2},
				Centroid:  []float32{0.5, 0.5},
				Density:   0.6,
				Size:      2,
				Coherence: 0.8,
			},
		},
		NoiseCount:   0,
		TotalVectors: 2,
	}
	outJSON, err := json.Marshal(fakeOutput)
	if err != nil {
		t.Fatalf("could not marshal fake output: %v", err)
	}

	fakeDir := t.TempDir()

	// Write the JSON response to a file; the fake script will cat it to stdout.
	// This avoids shell quoting issues that arise when embedding JSON in echo.
	jsonFile := filepath.Join(fakeDir, "response.json")
	if err := os.WriteFile(jsonFile, outJSON, 0644); err != nil {
		t.Fatalf("could not write response JSON: %v", err)
	}

	// Write a fake python3 stub that cats the pre-built JSON file.
	fakePy := filepath.Join(fakeDir, "python3")
	script := "#!/bin/sh\ncat " + jsonFile + "\n"
	if err := os.WriteFile(fakePy, []byte(script), 0755); err != nil {
		t.Fatalf("could not write fake python3: %v", err)
	}

	// Prepend fakeDir to PATH so exec.Command("python3",...) finds our stub.
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", fakeDir+string(os.PathListSeparator)+origPath)
	defer os.Setenv("PATH", origPath)

	top := New()
	vecs := [][]float32{{1, 0}, {0, 1}}
	meta := []models.VectorMetadata{
		{ID: 1, Source: "src", Layer: "surface"},
		{ID: 2, Source: "src", Layer: "surface"},
	}
	clusters := top.AnalyzeClusters(vecs, meta)

	if len(clusters) != 1 {
		t.Fatalf("want 1 cluster from fake python, got %d", len(clusters))
	}
	if clusters[0].ID != 1 {
		t.Errorf("cluster ID: want 1, got %d", clusters[0].ID)
	}
	if clusters[0].Label != "surface / src" {
		t.Errorf("cluster label: want 'surface / src', got %q", clusters[0].Label)
	}
	if clusters[0].Coherence != 0.8 {
		t.Errorf("coherence: want 0.8, got %v", clusters[0].Coherence)
	}
}

func TestAnalyzeClusters_FakePythonBadJSON(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake python script injection not supported on Windows")
	}

	fakeDir := t.TempDir()

	// Write a response file containing invalid JSON.
	badFile := filepath.Join(fakeDir, "bad.json")
	if err := os.WriteFile(badFile, []byte("not valid json"), 0644); err != nil {
		t.Fatalf("could not write bad JSON file: %v", err)
	}

	fakePy := filepath.Join(fakeDir, "python3")
	script := "#!/bin/sh\ncat " + badFile + "\n"
	if err := os.WriteFile(fakePy, []byte(script), 0755); err != nil {
		t.Fatalf("could not write fake python3: %v", err)
	}

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", fakeDir+string(os.PathListSeparator)+origPath)
	defer os.Setenv("PATH", origPath)

	top := New()
	vecs := [][]float32{{1, 0}}
	meta := []models.VectorMetadata{{ID: 1}}
	clusters := top.AnalyzeClusters(vecs, meta)
	// On bad JSON the function should return nil gracefully.
	if len(clusters) != 0 {
		t.Errorf("bad JSON from python: want 0 clusters, got %d", len(clusters))
	}
}
