package workspace

import (
	"encoding/binary"
	"math"
	"testing"
	"time"

	"github.com/meistro57/vectoreologist/internal/models"
)

// --- pure unit tests (no Redis required) -------------------------------------

func TestBatchRowCount_FullBatches(t *testing.T) {
	info := workspaceInfo{BatchCount: 3, BatchSize: 5000, TotalCount: 15000}
	for b := 0; b < 2; b++ {
		if got := batchRowCount(b, info); got != 5000 {
			t.Errorf("batch %d: want 5000, got %d", b, got)
		}
	}
}

func TestBatchRowCount_LastBatch(t *testing.T) {
	info := workspaceInfo{BatchCount: 3, BatchSize: 5000, TotalCount: 12300}
	if got := batchRowCount(2, info); got != 2300 {
		t.Errorf("last batch: want 2300, got %d", got)
	}
}

func TestBatchRowCount_ExactMultiple(t *testing.T) {
	info := workspaceInfo{BatchCount: 2, BatchSize: 5000, TotalCount: 10000}
	if got := batchRowCount(1, info); got != 5000 {
		t.Errorf("exact multiple last batch: want 5000, got %d", got)
	}
}

func TestGlobalToLocal_Table(t *testing.T) {
	info := workspaceInfo{BatchCount: 3, BatchSize: 5000, TotalCount: 12000}
	tests := []struct {
		gi        int
		wantBatch int
		wantRow   int
	}{
		{0, 0, 0},
		{4999, 0, 4999},
		{5000, 1, 0},
		{9999, 1, 4999},
		{10000, 2, 0},
	}
	for _, tc := range tests {
		b, r := globalToLocal(tc.gi, info)
		if b != tc.wantBatch || r != tc.wantRow {
			t.Errorf("gi=%d: want (%d,%d), got (%d,%d)", tc.gi, tc.wantBatch, tc.wantRow, b, r)
		}
	}
}

func TestVectorBinaryRoundTrip(t *testing.T) {
	// Encode and decode using the same format StoreBatch and loadBatch use.
	vecs := [][]float32{
		{1.0, -2.5, 0.0},
		{0.5, 0.25, float32(math.Pi)},
	}
	nVecs := uint32(len(vecs))
	nDims := uint32(len(vecs[0]))

	buf := make([]byte, 8+int(nVecs)*int(nDims)*4)
	binary.LittleEndian.PutUint32(buf[0:4], nVecs)
	binary.LittleEndian.PutUint32(buf[4:8], nDims)
	off := 8
	for _, v := range vecs {
		for _, f := range v {
			binary.LittleEndian.PutUint32(buf[off:off+4], math.Float32bits(f))
			off += 4
		}
	}

	gotN := int(binary.LittleEndian.Uint32(buf[0:4]))
	gotD := int(binary.LittleEndian.Uint32(buf[4:8]))
	if gotN != 2 || gotD != 3 {
		t.Fatalf("header: want 2×3, got %d×%d", gotN, gotD)
	}
	off = 8
	for i := 0; i < gotN; i++ {
		for j := 0; j < gotD; j++ {
			got := math.Float32frombits(binary.LittleEndian.Uint32(buf[off : off+4]))
			if got != vecs[i][j] {
				t.Errorf("vec[%d][%d]: want %v, got %v", i, j, vecs[i][j], got)
			}
			off += 4
		}
	}
}

// --- integration tests (skip when Redis is unavailable) ----------------------

func newWorkspaceOrSkip(t *testing.T) *Workspace {
	t.Helper()
	ws, err := New("redis://localhost:6379", "test-"+t.Name(), 30*time.Second)
	if err != nil {
		t.Skipf("Redis not available (%v); skipping integration test", err)
	}
	t.Cleanup(func() { _ = ws.Delete() })
	return ws
}

func TestWorkspace_StoreAndLoad_RoundTrip(t *testing.T) {
	ws := newWorkspaceOrSkip(t)

	vecs := [][]float32{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}}
	meta := []models.VectorMetadata{{ID: 1}, {ID: 2}, {ID: 3}}

	if err := ws.StoreBatch(0, vecs, meta); err != nil {
		t.Fatalf("StoreBatch: %v", err)
	}

	total, err := ws.TotalVectors()
	if err != nil {
		t.Fatalf("TotalVectors: %v", err)
	}
	if total != 3 {
		t.Errorf("TotalVectors: want 3, got %d", total)
	}

	gotVecs, gotMeta, err := ws.LoadSample(10) // n > total → return all
	if err != nil {
		t.Fatalf("LoadSample: %v", err)
	}
	if len(gotVecs) != 3 {
		t.Fatalf("LoadSample: want 3 vectors, got %d", len(gotVecs))
	}
	for i := range vecs {
		for j, want := range vecs[i] {
			if gotVecs[i][j] != want {
				t.Errorf("vec[%d][%d]: want %v, got %v", i, j, want, gotVecs[i][j])
			}
		}
		if gotMeta[i].ID != meta[i].ID {
			t.Errorf("meta[%d].ID: want %d, got %d", i, meta[i].ID, gotMeta[i].ID)
		}
	}
}

func TestWorkspace_LoadSample_Subset(t *testing.T) {
	ws := newWorkspaceOrSkip(t)

	// Store 20 vectors across 2 batches.
	for b := 0; b < 2; b++ {
		vecs := make([][]float32, 10)
		meta := make([]models.VectorMetadata, 10)
		for i := range vecs {
			vecs[i] = []float32{float32(b*10 + i)}
			meta[i] = models.VectorMetadata{ID: uint64(b*10 + i)}
		}
		if err := ws.StoreBatch(b, vecs, meta); err != nil {
			t.Fatalf("StoreBatch %d: %v", b, err)
		}
	}

	gotVecs, _, err := ws.LoadSample(5)
	if err != nil {
		t.Fatalf("LoadSample: %v", err)
	}
	if len(gotVecs) != 5 {
		t.Errorf("LoadSample(5) from 20: want 5, got %d", len(gotVecs))
	}
}

func TestWorkspace_MultiBatch_TotalCount(t *testing.T) {
	ws := newWorkspaceOrSkip(t)

	for b := 0; b < 3; b++ {
		vecs := [][]float32{{float32(b), 0}, {float32(b), 1}}
		meta := []models.VectorMetadata{{ID: uint64(b * 2)}, {ID: uint64(b*2 + 1)}}
		if err := ws.StoreBatch(b, vecs, meta); err != nil {
			t.Fatalf("StoreBatch %d: %v", b, err)
		}
	}

	total, err := ws.TotalVectors()
	if err != nil {
		t.Fatalf("TotalVectors: %v", err)
	}
	if total != 6 {
		t.Errorf("want 6 total vectors across 3 batches, got %d", total)
	}
}

func TestWorkspace_Delete_CleansUp(t *testing.T) {
	ws := newWorkspaceOrSkip(t)

	if err := ws.StoreBatch(0, [][]float32{{1, 2}}, []models.VectorMetadata{{ID: 99}}); err != nil {
		t.Fatalf("StoreBatch: %v", err)
	}
	if err := ws.Delete(); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := ws.TotalVectors(); err == nil {
		t.Error("expected error after Delete (info key gone), got nil")
	}
}
