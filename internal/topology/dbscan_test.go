package topology

import (
	"math"
	"testing"
)

func TestRunDBSCAN_EmptyInput(t *testing.T) {
	if labels := runDBSCAN(nil, 0.3, 2); len(labels) != 0 {
		t.Errorf("want nil/empty, got %v", labels)
	}
}

func TestRunDBSCAN_DefaultEps(t *testing.T) {
	vecs := [][]float32{{1, 0}, {0.99, 0.14}}
	l2Normalise(vecs)
	labels := runDBSCAN(vecs, 0, 1) // eps=0 → default 0.3
	if len(labels) != 2 {
		t.Errorf("want 2 labels, got %d", len(labels))
	}
}

func TestRunDBSCAN_TwoClearClusters(t *testing.T) {
	// Group A near [1,0,0,0,0], group B near [0,1,0,0,0].
	vecs := make([][]float32, 20)
	for i := 0; i < 10; i++ {
		vecs[i] = []float32{1, float32(i) * 0.01, 0, 0, 0}
	}
	for i := 10; i < 20; i++ {
		vecs[i] = []float32{0, 1, float32(i-10) * 0.01, 0, 0}
	}
	l2Normalise(vecs)

	labels := runDBSCAN(vecs, 0.3, 3)

	ids := make(map[int]bool)
	for _, l := range labels {
		if l != -1 {
			ids[l] = true
		}
	}
	if len(ids) != 2 {
		t.Fatalf("want 2 clusters, got %d: labels=%v", len(ids), labels)
	}
	idA := labels[0]
	for i := 1; i < 10; i++ {
		if labels[i] != idA {
			t.Errorf("group A[%d]: want cluster %d, got %d", i, idA, labels[i])
		}
	}
	idB := labels[10]
	for i := 11; i < 20; i++ {
		if labels[i] != idB {
			t.Errorf("group B[%d]: want cluster %d, got %d", i, idB, labels[i])
		}
	}
	if idA == idB {
		t.Error("both groups mapped to same cluster ID")
	}
}

func TestRunDBSCAN_AllNoise(t *testing.T) {
	// Orthogonal unit vectors: cosine distance = 1 between each pair.
	// With eps=0.3 and minPts=2 no point has enough neighbors → all noise.
	vecs := [][]float32{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}
	labels := runDBSCAN(vecs, 0.3, 2)
	for i, l := range labels {
		if l != -1 {
			t.Errorf("point %d should be noise, got cluster %d", i, l)
		}
	}
}

func TestRunDBSCAN_SingleCluster(t *testing.T) {
	// 5 nearly identical unit vectors should form one cluster.
	vecs := make([][]float32, 5)
	for i := range vecs {
		vecs[i] = []float32{1, float32(i) * 0.001, 0}
	}
	l2Normalise(vecs)
	labels := runDBSCAN(vecs, 0.3, 3)
	clusterID := labels[0]
	if clusterID == -1 {
		t.Fatal("expected a cluster, got noise")
	}
	for i, l := range labels {
		if l != clusterID {
			t.Errorf("point %d: want cluster %d, got %d", i, clusterID, l)
		}
	}
}

func TestBuildNeighborLists_NoSelf(t *testing.T) {
	vecs := [][]float32{{1, 0}, {0.9, 0.1}, {0, 1}}
	l2Normalise(vecs)
	nb := buildNeighborLists(vecs, 0.5)
	for i, list := range nb {
		for _, j := range list {
			if j == i {
				t.Errorf("point %d appears in its own neighbor list", i)
			}
		}
	}
}

func TestBuildNeighborLists_Symmetric(t *testing.T) {
	vecs := [][]float32{{1, 0}, {0.9, 0.1}, {0.95, 0.05}, {0, 1}}
	l2Normalise(vecs)
	nb := buildNeighborLists(vecs, 0.5)
	contains := func(list []int, v int) bool {
		for _, x := range list {
			if x == v {
				return true
			}
		}
		return false
	}
	for i, list := range nb {
		for _, j := range list {
			if !contains(nb[j], i) {
				t.Errorf("j=%d is in nb[%d] but %d is not in nb[%d]", j, i, i, j)
			}
		}
	}
}

func TestUnitCosineDistance_Identical(t *testing.T) {
	v := []float32{1, 0, 0}
	if d := unitCosineDistance(v, v); d > 1e-6 {
		t.Errorf("identical vectors: want 0, got %v", d)
	}
}

func TestUnitCosineDistance_Orthogonal(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{0, 1}
	if d := unitCosineDistance(a, b); math.Abs(d-1.0) > 1e-6 {
		t.Errorf("orthogonal unit vectors: want 1.0, got %v", d)
	}
}

func TestUnitCosineDistance_Opposite(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{-1, 0}
	if d := unitCosineDistance(a, b); math.Abs(d-2.0) > 1e-6 {
		t.Errorf("opposite unit vectors: want 2.0, got %v", d)
	}
}

func TestUnitCosineDistance_LengthMismatch(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{1, 0, 0}
	if d := unitCosineDistance(a, b); d != 2.0 {
		t.Errorf("length mismatch: want 2.0 (max distance sentinel), got %v", d)
	}
}

func TestL2Normalise_NonUnitBecomesUnit(t *testing.T) {
	vecs := [][]float32{{3, 0, 0}, {1, 1, 0}, {1, 1, 1}}
	l2Normalise(vecs)
	for i, v := range vecs {
		var norm float64
		for _, x := range v {
			norm += float64(x) * float64(x)
		}
		if math.Abs(math.Sqrt(norm)-1.0) > 1e-6 {
			t.Errorf("vector %d: want unit norm, got %v", i, math.Sqrt(norm))
		}
	}
}

func TestL2Normalise_ZeroVector(t *testing.T) {
	vecs := [][]float32{{0, 0, 0}}
	l2Normalise(vecs)
	for _, x := range vecs[0] {
		if x != 0 {
			t.Error("zero vector should remain unchanged after normalisation")
		}
	}
}
