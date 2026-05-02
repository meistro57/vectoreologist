package topology

import (
	"math"
	"testing"
)

func TestPCAReduce_NoOpWhenSmallDims(t *testing.T) {
	vecs := [][]float32{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}}
	got := pcaReduce(vecs, 10)
	if len(got) != len(vecs) || &got[0][0] != &vecs[0][0] {
		t.Error("expected pcaReduce to return input unchanged when d <= nComponents")
	}
}

func TestPCAReduce_NoOpZeroComponents(t *testing.T) {
	vecs := [][]float32{{1, 2, 3}, {4, 5, 6}}
	got := pcaReduce(vecs, 0)
	if &got[0][0] != &vecs[0][0] {
		t.Error("expected no-op for nComponents=0")
	}
}

func TestPCAReduce_EmptyInput(t *testing.T) {
	got := pcaReduce(nil, 2)
	if got != nil {
		t.Errorf("expected nil for nil input, got %v", got)
	}
}

func TestPCAReduce_OutputShape(t *testing.T) {
	vecs := make([][]float32, 20)
	for i := range vecs {
		vecs[i] = make([]float32, 8)
		for j := range vecs[i] {
			vecs[i][j] = float32(i*8 + j + 1)
		}
	}
	got := pcaReduce(vecs, 3)
	if len(got) != 20 {
		t.Fatalf("row count: want 20, got %d", len(got))
	}
	for i, row := range got {
		if len(row) != 3 {
			t.Fatalf("row %d: want 3 cols, got %d", i, len(row))
		}
	}
}

func TestPCAReduce_TwoGroupsSeparable(t *testing.T) {
	// Two tight groups along different axes; after PCA to 2D the projected
	// group means should remain clearly apart.
	vecs := make([][]float32, 0, 20)
	for i := 0; i < 10; i++ {
		vecs = append(vecs, []float32{1 + float32(i)*0.01, 0, 0, 0, 0, 0})
	}
	for i := 0; i < 10; i++ {
		vecs = append(vecs, []float32{0, 0, 0, 1 + float32(i)*0.01, 0, 0})
	}
	reduced := pcaReduce(vecs, 2)
	if len(reduced) != 20 {
		t.Fatalf("want 20 rows, got %d", len(reduced))
	}
	var m1, m2 [2]float64
	for i := 0; i < 10; i++ {
		m1[0] += float64(reduced[i][0])
		m1[1] += float64(reduced[i][1])
	}
	for i := 10; i < 20; i++ {
		m2[0] += float64(reduced[i][0])
		m2[1] += float64(reduced[i][1])
	}
	for j := range m1 {
		m1[j] /= 10
		m2[j] /= 10
	}
	dist := math.Sqrt((m1[0]-m2[0])*(m1[0]-m2[0]) + (m1[1]-m2[1])*(m1[1]-m2[1]))
	if dist < 0.1 {
		t.Errorf("projected cluster means too close (dist=%.4f); groups should remain separable", dist)
	}
}
