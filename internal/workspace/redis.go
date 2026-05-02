package workspace

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"

	"github.com/meistro57/vectoreologist/internal/models"
	"github.com/redis/go-redis/v9"
)

// Workspace streams vector batches to Redis, keeping them off the Go heap.
// All keys are namespaced under veo:{runID}: and expire after ttl.
//
// Key scheme:
//
//	veo:{runID}:vec:{batchNum}   → binary [4-byte n_vecs LE][4-byte n_dims LE][n_vecs×n_dims×4 float32 LE row-major]
//	veo:{runID}:meta:{batchNum}  → JSON array of VectorMetadata
//	veo:{runID}:info             → JSON {batch_count, batch_size, total_count, dim}
type Workspace struct {
	client *redis.Client
	runID  string
	ttl    time.Duration
}

type workspaceInfo struct {
	BatchCount int `json:"batch_count"`
	BatchSize  int `json:"batch_size"`
	TotalCount int `json:"total_count"`
	Dim        int `json:"dim"`
}

// New creates a Workspace connected to the given Redis URL.
func New(redisURL, runID string, ttl time.Duration) (*Workspace, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("workspace: parse redis URL: %w", err)
	}
	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("workspace: redis ping: %w", err)
	}
	return &Workspace{client: client, runID: runID, ttl: ttl}, nil
}

// StoreBatch writes a batch of vectors and their metadata to Redis.
// batchNum should be monotonically increasing starting from 0.
func (w *Workspace) StoreBatch(batchNum int, vectors [][]float32, meta []models.VectorMetadata) error {
	if len(vectors) == 0 {
		return nil
	}
	ctx := context.Background()

	nVecs := uint32(len(vectors))
	nDims := uint32(0)
	if len(vectors[0]) > 0 {
		nDims = uint32(len(vectors[0]))
	}

	// Binary encoding: 8-byte header + row-major float32 values.
	headerSize := 8
	dataSize := int(nVecs) * int(nDims) * 4
	buf := make([]byte, headerSize+dataSize)
	binary.LittleEndian.PutUint32(buf[0:4], nVecs)
	binary.LittleEndian.PutUint32(buf[4:8], nDims)
	off := headerSize
	for _, v := range vectors {
		for _, f := range v {
			binary.LittleEndian.PutUint32(buf[off:off+4], math.Float32bits(f))
			off += 4
		}
	}

	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("workspace: marshal meta: %w", err)
	}

	vecKey := w.key("vec", batchNum)
	metaKey := w.key("meta", batchNum)

	pipe := w.client.Pipeline()
	pipe.Set(ctx, vecKey, buf, w.ttl)
	pipe.Set(ctx, metaKey, metaJSON, w.ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("workspace: store batch %d: %w", batchNum, err)
	}

	// Update info key.
	infoKey := w.infoKey()
	infoRaw, _ := w.client.Get(ctx, infoKey).Bytes()
	var info workspaceInfo
	if len(infoRaw) > 0 {
		_ = json.Unmarshal(infoRaw, &info)
	}
	info.BatchCount = batchNum + 1
	info.TotalCount += int(nVecs)
	info.Dim = int(nDims)
	if batchNum == 0 {
		info.BatchSize = int(nVecs)
	}
	infoJSON, _ := json.Marshal(info)
	if err := w.client.Set(ctx, infoKey, infoJSON, w.ttl).Err(); err != nil {
		return fmt.Errorf("workspace: update info: %w", err)
	}
	return nil
}

// LoadSample loads a random sample of n vectors and their metadata.
// If n >= total_count all vectors are returned.
func (w *Workspace) LoadSample(n int) ([][]float32, []models.VectorMetadata, error) {
	ctx := context.Background()

	info, err := w.fetchInfo(ctx)
	if err != nil {
		return nil, nil, err
	}
	if info.TotalCount == 0 {
		return nil, nil, nil
	}

	type vecRef struct {
		batch int
		row   int
	}

	var refs []vecRef
	if n >= info.TotalCount {
		refs = make([]vecRef, 0, info.TotalCount)
		for b := 0; b < info.BatchCount; b++ {
			sz := batchRowCount(b, info)
			for r := 0; r < sz; r++ {
				refs = append(refs, vecRef{b, r})
			}
		}
	} else {
		perm := rand.Perm(info.TotalCount)[:n]
		sort.Ints(perm)
		refs = make([]vecRef, 0, n)
		for _, gi := range perm {
			b, r := globalToLocal(gi, info)
			refs = append(refs, vecRef{b, r})
		}
	}

	// Group refs by batch so we load each batch at most once.
	byBatch := make(map[int][]int)
	for _, ref := range refs {
		byBatch[ref.batch] = append(byBatch[ref.batch], ref.row)
	}
	batches := make([]int, 0, len(byBatch))
	for b := range byBatch {
		batches = append(batches, b)
	}
	sort.Ints(batches)

	vectors := make([][]float32, 0, len(refs))
	meta := make([]models.VectorMetadata, 0, len(refs))

	for _, b := range batches {
		rows := byBatch[b]
		sort.Ints(rows)

		bVecs, bMeta, err := w.loadBatch(ctx, b)
		if err != nil {
			return nil, nil, err
		}
		for _, r := range rows {
			if r < len(bVecs) {
				vectors = append(vectors, bVecs[r])
				meta = append(meta, bMeta[r])
			}
		}
	}

	return vectors, meta, nil
}

// TotalVectors returns the total number of vectors stored across all batches.
func (w *Workspace) TotalVectors() (int, error) {
	ctx := context.Background()
	info, err := w.fetchInfo(ctx)
	if err != nil {
		return 0, err
	}
	return info.TotalCount, nil
}

// Delete removes all workspace keys from Redis.
func (w *Workspace) Delete() error {
	ctx := context.Background()
	pattern := fmt.Sprintf("veo:%s:*", w.runID)
	iter := w.client.Scan(ctx, 0, pattern, 0).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("workspace: scan keys: %w", err)
	}
	if len(keys) == 0 {
		return nil
	}
	if err := w.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("workspace: delete keys: %w", err)
	}
	return nil
}

// --- helpers -----------------------------------------------------------------

func (w *Workspace) key(kind string, batchNum int) string {
	return fmt.Sprintf("veo:%s:%s:%d", w.runID, kind, batchNum)
}

func (w *Workspace) infoKey() string {
	return fmt.Sprintf("veo:%s:info", w.runID)
}

func (w *Workspace) fetchInfo(ctx context.Context) (workspaceInfo, error) {
	raw, err := w.client.Get(ctx, w.infoKey()).Bytes()
	if err != nil {
		return workspaceInfo{}, fmt.Errorf("workspace: fetch info: %w", err)
	}
	var info workspaceInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return workspaceInfo{}, fmt.Errorf("workspace: parse info: %w", err)
	}
	return info, nil
}

func (w *Workspace) loadBatch(ctx context.Context, batchNum int) ([][]float32, []models.VectorMetadata, error) {
	vecKey := w.key("vec", batchNum)
	metaKey := w.key("meta", batchNum)

	pipe := w.client.Pipeline()
	vecCmd := pipe.Get(ctx, vecKey)
	metaCmd := pipe.Get(ctx, metaKey)
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, nil, fmt.Errorf("workspace: load batch %d: %w", batchNum, err)
	}

	vecBytes, err := vecCmd.Bytes()
	if err != nil {
		return nil, nil, fmt.Errorf("workspace: read vec bytes batch %d: %w", batchNum, err)
	}
	metaBytes, err := metaCmd.Bytes()
	if err != nil {
		return nil, nil, fmt.Errorf("workspace: read meta bytes batch %d: %w", batchNum, err)
	}

	if len(vecBytes) < 8 {
		return nil, nil, fmt.Errorf("workspace: vec data too short for batch %d", batchNum)
	}
	nVecs := int(binary.LittleEndian.Uint32(vecBytes[0:4]))
	nDims := int(binary.LittleEndian.Uint32(vecBytes[4:8]))

	vectors := make([][]float32, nVecs)
	off := 8
	for i := 0; i < nVecs; i++ {
		v := make([]float32, nDims)
		for j := 0; j < nDims; j++ {
			bits := binary.LittleEndian.Uint32(vecBytes[off : off+4])
			v[j] = math.Float32frombits(bits)
			off += 4
		}
		vectors[i] = v
	}

	var meta []models.VectorMetadata
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return nil, nil, fmt.Errorf("workspace: parse meta batch %d: %w", batchNum, err)
	}

	return vectors, meta, nil
}

// batchRowCount returns the number of vectors in the given batch number.
func batchRowCount(b int, info workspaceInfo) int {
	if b < info.BatchCount-1 {
		return info.BatchSize
	}
	// Last batch may be smaller.
	rem := info.TotalCount - (info.BatchCount-1)*info.BatchSize
	if rem < 0 {
		rem = 0
	}
	return rem
}

// globalToLocal converts a global vector index to (batch, row).
func globalToLocal(gi int, info workspaceInfo) (batch, row int) {
	if info.BatchSize <= 0 {
		return 0, gi
	}
	return gi / info.BatchSize, gi % info.BatchSize
}
