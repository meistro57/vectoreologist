package excavator

import (
	"testing"

	"github.com/meistro57/vectoreologist/internal/models"
)

// makeVectors builds n float32 vectors of dimension d, each filled with
// index-derived values so they are all distinct.
func makeVectors(n, d int) [][]float32 {
	vecs := make([][]float32, n)
	for i := range vecs {
		v := make([]float32, d)
		for j := range v {
			v[j] = float32(i*d + j)
		}
		vecs[i] = v
	}
	return vecs
}

// makeMetadata builds n VectorMetadata entries spread across nSources sources.
func makeMetadata(n, nSources int) []models.VectorMetadata {
	sources := []string{"alpha", "beta", "gamma", "delta", "epsilon"}
	if nSources > len(sources) {
		nSources = len(sources)
	}
	meta := make([]models.VectorMetadata, n)
	for i := range meta {
		meta[i] = models.VectorMetadata{
			ID:     uint64(i + 1),
			Source: sources[i%nSources],
			Layer:  "surface",
		}
	}
	return meta
}

// ---- helper: verify that Sample never returns more items than targetSize ----

func assertSampleSize(t *testing.T, gotVecs [][]float32, gotMeta []models.VectorMetadata, want int) {
	t.Helper()
	if len(gotVecs) != len(gotMeta) {
		t.Fatalf("mismatched lengths: vecs=%d meta=%d", len(gotVecs), len(gotMeta))
	}
	if len(gotVecs) > want {
		t.Errorf("got %d samples, want <= %d", len(gotVecs), want)
	}
}

// ============================================================
// Random strategy
// ============================================================

func TestRandomSample_ReturnsSampledCount(t *testing.T) {
	vecs := makeVectors(100, 4)
	meta := makeMetadata(100, 3)
	s := NewSampler(Random, 42)

	gv, gm := s.Sample(vecs, meta, 20)
	if len(gv) != 20 || len(gm) != 20 {
		t.Errorf("want 20 samples, got vecs=%d meta=%d", len(gv), len(gm))
	}
}

func TestRandomSample_EmptyInput(t *testing.T) {
	s := NewSampler(Random, 42)
	gv, gm := s.Sample(nil, nil, 10)
	if len(gv) != 0 || len(gm) != 0 {
		t.Errorf("expected empty slices, got vecs=%d meta=%d", len(gv), len(gm))
	}
}

func TestRandomSample_TargetSizeEqualsLen(t *testing.T) {
	vecs := makeVectors(10, 3)
	meta := makeMetadata(10, 2)
	s := NewSampler(Random, 0)

	gv, gm := s.Sample(vecs, meta, 10)
	// When targetSize == len, the original slices should be returned unchanged.
	if len(gv) != 10 || len(gm) != 10 {
		t.Errorf("want 10, got %d/%d", len(gv), len(gm))
	}
}

func TestRandomSample_TargetSizeGreaterThanLen(t *testing.T) {
	vecs := makeVectors(5, 3)
	meta := makeMetadata(5, 2)
	s := NewSampler(Random, 0)

	gv, gm := s.Sample(vecs, meta, 100)
	// When targetSize > len, all vectors should be returned.
	if len(gv) != 5 || len(gm) != 5 {
		t.Errorf("want 5 (all), got vecs=%d meta=%d", len(gv), len(gm))
	}
}

func TestRandomSample_SeedReproducibility(t *testing.T) {
	vecs := makeVectors(50, 4)
	meta := makeMetadata(50, 3)

	s1 := NewSampler(Random, 123)
	_, gm1 := s1.Sample(vecs, meta, 15)

	s2 := NewSampler(Random, 123)
	_, gm2 := s2.Sample(vecs, meta, 15)

	for i := range gm1 {
		if gm1[i].ID != gm2[i].ID {
			t.Errorf("index %d: run1 ID=%d run2 ID=%d; same seed should give same result",
				i, gm1[i].ID, gm2[i].ID)
		}
	}
}

func TestRandomSample_DifferentSeedsDifferentResults(t *testing.T) {
	vecs := makeVectors(100, 4)
	meta := makeMetadata(100, 3)

	s1 := NewSampler(Random, 1)
	_, gm1 := s1.Sample(vecs, meta, 30)

	s2 := NewSampler(Random, 2)
	_, gm2 := s2.Sample(vecs, meta, 30) //nolint:unused

	// It is astronomically unlikely that two different seeds produce the
	// identical 30-element permutation from 100 elements.
	same := true
	for i := range gm1 {
		if gm1[i].ID != gm2[i].ID {
			same = false
			break
		}
	}
	if same {
		t.Error("different seeds produced identical samples; expected different orderings")
	}
}

func TestRandomSample_NoDuplicates(t *testing.T) {
	vecs := makeVectors(50, 4)
	meta := makeMetadata(50, 3)
	s := NewSampler(Random, 7)

	_, gm := s.Sample(vecs, meta, 30)

	seen := make(map[uint64]bool)
	for _, m := range gm {
		if seen[m.ID] {
			t.Errorf("duplicate ID %d in sample", m.ID)
		}
		seen[m.ID] = true
	}
}

// ============================================================
// Stratified strategy
// ============================================================

func TestStratifiedSample_ReturnsSampledCount(t *testing.T) {
	vecs := makeVectors(60, 4)
	meta := makeMetadata(60, 3) // 3 sources → 20 each
	s := NewSampler(Stratified, 42)

	gv, gm := s.Sample(vecs, meta, 30)
	assertSampleSize(t, gv, gm, 30)
}

func TestStratifiedSample_EmptyInput(t *testing.T) {
	s := NewSampler(Stratified, 42)
	gv, gm := s.Sample(nil, nil, 10)
	if len(gv) != 0 || len(gm) != 0 {
		t.Errorf("expected empty, got %d/%d", len(gv), len(gm))
	}
}

func TestStratifiedSample_TargetGreaterThanLen(t *testing.T) {
	vecs := makeVectors(6, 3)
	meta := makeMetadata(6, 2)
	s := NewSampler(Stratified, 0)

	gv, gm := s.Sample(vecs, meta, 100)
	if len(gv) != 6 || len(gm) != 6 {
		t.Errorf("want all 6, got %d/%d", len(gv), len(gm))
	}
}

func TestStratifiedSample_RepresentsBothSources(t *testing.T) {
	// 40 vectors: 20 from "alpha", 20 from "beta".
	vecs := makeVectors(40, 4)
	meta := makeMetadata(40, 2)
	s := NewSampler(Stratified, 99)

	_, gm := s.Sample(vecs, meta, 20)

	counts := make(map[string]int)
	for _, m := range gm {
		counts[m.Source]++
	}
	// Each source should appear at least once.
	for _, src := range []string{"alpha", "beta"} {
		if counts[src] == 0 {
			t.Errorf("source %q not represented in stratified sample", src)
		}
	}
}

// ============================================================
// Diverse strategy — MaxMin (Farthest-First) sampling
// ============================================================

func TestDiverseSample_ReturnsSampledCount(t *testing.T) {
	vecs := makeVectors(100, 4)
	meta := makeMetadata(100, 3)
	s := NewSampler(Diverse, 42)

	gv, gm := s.Sample(vecs, meta, 25)
	if len(gv) != 25 || len(gm) != 25 {
		t.Errorf("want 25, got %d/%d", len(gv), len(gm))
	}
}

func TestDiverseSample_EmptyInput(t *testing.T) {
	s := NewSampler(Diverse, 0)
	gv, gm := s.Sample(nil, nil, 5)
	if len(gv) != 0 || len(gm) != 0 {
		t.Errorf("expected empty, got %d/%d", len(gv), len(gm))
	}
}

func TestDiverseSample_NoDuplicates(t *testing.T) {
	vecs := makeVectors(50, 4)
	meta := makeMetadata(50, 2)
	s := NewSampler(Diverse, 7)

	_, gm := s.Sample(vecs, meta, 20)

	seen := make(map[uint64]bool)
	for _, m := range gm {
		if seen[m.ID] {
			t.Errorf("duplicate ID %d in diverse sample", m.ID)
		}
		seen[m.ID] = true
	}
}

func TestDiverseSample_SpreadsBothClusters(t *testing.T) {
	// 20 vectors: first 10 near origin, last 10 far away.
	// MaxMin should select from both groups.
	n := 20
	vecs := make([][]float32, n)
	meta := makeMetadata(n, 2)
	for i := range vecs {
		v := make([]float32, 4)
		if i < 10 {
			v[0] = float32(i) * 0.001 // near (0,0,0,0)
		} else {
			v[0] = 100.0 + float32(i-10)*0.001 // far
		}
		vecs[i] = v
	}

	s := NewSampler(Diverse, 42)
	gv, gm := s.Sample(vecs, meta, 4)

	if len(gv) != 4 || len(gm) != 4 {
		t.Fatalf("want 4 samples, got %d", len(gv))
	}

	var near, far int
	for _, v := range gv {
		if v[0] < 50 {
			near++
		} else {
			far++
		}
	}
	if near == 0 || far == 0 {
		t.Errorf("diverse sample should span both clusters; near=%d far=%d", near, far)
	}
}

func TestDiverseSample_TargetGreaterThanLen(t *testing.T) {
	vecs := makeVectors(5, 4)
	meta := makeMetadata(5, 1)
	s := NewSampler(Diverse, 0)

	gv, gm := s.Sample(vecs, meta, 100)
	if len(gv) != 5 || len(gm) != 5 {
		t.Errorf("want all 5, got %d/%d", len(gv), len(gm))
	}
}

// ============================================================
// squaredL2 helper (white-box)
// ============================================================

func TestSquaredL2_BasicCases(t *testing.T) {
	cases := []struct {
		a, b []float32
		want float64
	}{
		{[]float32{0, 0, 0}, []float32{0, 0, 0}, 0},
		{[]float32{1, 0, 0}, []float32{0, 0, 0}, 1},
		{[]float32{3, 4}, []float32{0, 0}, 25}, // 3²+4²=25
		{[]float32{1, 1}, []float32{1, 1}, 0},
	}
	for _, tc := range cases {
		got := squaredL2(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("squaredL2(%v, %v) = %v; want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

// ============================================================
// Temporal strategy (currently falls back to random)
// ============================================================

func TestTemporalSample_ReturnsSampledCount(t *testing.T) {
	vecs := makeVectors(100, 4)
	meta := makeMetadata(100, 3)
	s := NewSampler(Temporal, 42)

	gv, gm := s.Sample(vecs, meta, 25)
	if len(gv) != 25 || len(gm) != 25 {
		t.Errorf("want 25, got %d/%d", len(gv), len(gm))
	}
}

func TestTemporalSample_EmptyInput(t *testing.T) {
	s := NewSampler(Temporal, 0)
	gv, gm := s.Sample(nil, nil, 5)
	if len(gv) != 0 || len(gm) != 0 {
		t.Errorf("expected empty, got %d/%d", len(gv), len(gm))
	}
}

// ============================================================
// Default/unknown strategy falls back to random
// ============================================================

func TestUnknownStrategy_FallsBackToRandom(t *testing.T) {
	vecs := makeVectors(50, 4)
	meta := makeMetadata(50, 2)
	s := NewSampler(SamplingStrategy("unknown"), 42)

	gv, gm := s.Sample(vecs, meta, 10)
	if len(gv) != 10 || len(gm) != 10 {
		t.Errorf("want 10, got %d/%d", len(gv), len(gm))
	}
}

// ============================================================
// NewSampler constructor
// ============================================================

func TestNewSampler_FieldsStored(t *testing.T) {
	s := NewSampler(Stratified, 999)
	if s.strategy != Stratified {
		t.Errorf("strategy: want %s, got %s", Stratified, s.strategy)
	}
	if s.seed != 999 {
		t.Errorf("seed: want 999, got %d", s.seed)
	}
}

// ============================================================
// min helper (package-internal, white-box test)
// ============================================================

func TestMinHelper(t *testing.T) {
	cases := []struct{ a, b, want int }{
		{3, 5, 3},
		{5, 3, 3},
		{4, 4, 4},
		{0, 1, 0},
		{-1, 0, -1},
	}
	for _, tc := range cases {
		got := min(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("min(%d,%d) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}
