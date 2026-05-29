package excavator

import (
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/meistro57/vectoreologist/internal/models"
)

const diversePoolCap = 15000

type SamplingStrategy string

const (
	Random     SamplingStrategy = "random"
	Stratified SamplingStrategy = "stratified"
	Temporal   SamplingStrategy = "temporal"
	Diverse    SamplingStrategy = "diverse"
)

type Sampler struct {
	strategy SamplingStrategy
	seed     int64
}

func NewSampler(strategy SamplingStrategy, seed int64) *Sampler {
	return &Sampler{
		strategy: strategy,
		seed:     seed,
	}
}

func (s *Sampler) Sample(
	vectors [][]float32,
	metadata []models.VectorMetadata,
	targetSize int,
) ([][]float32, []models.VectorMetadata) {
	if len(vectors) <= targetSize {
		return vectors, metadata
	}

	switch s.strategy {
	case Stratified:
		return s.stratifiedSample(vectors, metadata, targetSize)
	case Diverse:
		return s.diverseSample(vectors, metadata, targetSize)
	case Temporal:
		return s.temporalSample(vectors, metadata, targetSize)
	default:
		return s.randomSample(vectors, metadata, targetSize)
	}
}

func (s *Sampler) randomSample(
	vectors [][]float32,
	metadata []models.VectorMetadata,
	targetSize int,
) ([][]float32, []models.VectorMetadata) {
	rng := rand.New(rand.NewSource(s.seed))
	indices := rng.Perm(len(vectors))[:targetSize]

	sampledVecs := make([][]float32, targetSize)
	sampledMeta := make([]models.VectorMetadata, targetSize)

	for i, idx := range indices {
		sampledVecs[i] = vectors[idx]
		sampledMeta[i] = metadata[idx]
	}

	return sampledVecs, sampledMeta
}

func (s *Sampler) stratifiedSample(
	vectors [][]float32,
	metadata []models.VectorMetadata,
	targetSize int,
) ([][]float32, []models.VectorMetadata) {
	sourceGroups := make(map[string][]int)
	for i, meta := range metadata {
		sourceGroups[meta.Source] = append(sourceGroups[meta.Source], i)
	}

	sampledVecs := make([][]float32, 0, targetSize)
	sampledMeta := make([]models.VectorMetadata, 0, targetSize)

	rng := rand.New(rand.NewSource(s.seed))
	perSource := targetSize / len(sourceGroups)

	for _, indices := range sourceGroups {
		sampleCount := min(perSource, len(indices))
		shuffled := rng.Perm(len(indices))[:sampleCount]

		for _, localIdx := range shuffled {
			globalIdx := indices[localIdx]
			sampledVecs = append(sampledVecs, vectors[globalIdx])
			sampledMeta = append(sampledMeta, metadata[globalIdx])
		}
	}

	if len(sampledVecs) >= targetSize {
		return sampledVecs[:targetSize], sampledMeta[:targetSize]
	}

	remaining := targetSize - len(sampledVecs)
	picked := map[int]bool{}
	for _, m := range sampledMeta {
		if m.ID != 0 {
			picked[int(m.ID)] = true
		}
	}
	perm := rng.Perm(len(vectors))
	for _, idx := range perm {
		if remaining == 0 {
			break
		}
		if metadata[idx].ID != 0 && picked[int(metadata[idx].ID)] {
			continue
		}
		sampledVecs = append(sampledVecs, vectors[idx])
		sampledMeta = append(sampledMeta, metadata[idx])
		remaining--
	}

	return sampledVecs, sampledMeta
}

func (s *Sampler) diverseSample(
	vectors [][]float32,
	metadata []models.VectorMetadata,
	targetSize int,
) ([][]float32, []models.VectorMetadata) {
	rng := rand.New(rand.NewSource(s.seed))
	poolIndices := diversePoolIndices(metadata, len(vectors), diversePoolCap, rng)
	if len(poolIndices) == 0 {
		poolIndices = make([]int, len(vectors))
		for i := range vectors {
			poolIndices[i] = i
		}
	}
	if len(poolIndices) <= targetSize {
		return gatherByIndices(vectors, metadata, poolIndices)
	}

	poolVecs := make([][]float32, len(poolIndices))
	poolMeta := make([]models.VectorMetadata, len(poolIndices))
	for i, idx := range poolIndices {
		poolVecs[i] = vectors[idx]
		poolMeta[i] = metadata[idx]
	}

	pool := len(poolVecs)
	minDist := make([]float64, pool)
	for i := range minDist {
		minDist[i] = math.MaxFloat64
	}

	selected := make([]int, 0, targetSize)
	first := rng.Intn(pool)
	selected = append(selected, first)
	minDist[first] = 0
	updateMinDist(minDist, poolVecs, first)

	for len(selected) < targetSize {
		best, bestD := -1, -1.0
		for i, d := range minDist {
			if d > bestD {
				bestD = d
				best = i
			}
		}
		if best < 0 {
			break
		}
		selected = append(selected, best)
		minDist[best] = 0
		updateMinDist(minDist, poolVecs, best)
	}

	out := make([][]float32, len(selected))
	outM := make([]models.VectorMetadata, len(selected))
	for i, idx := range selected {
		out[i] = poolVecs[idx]
		outM[i] = poolMeta[idx]
	}
	return out, outM
}

func diversePoolIndices(metadata []models.VectorMetadata, vectorCount, capSize int, rng *rand.Rand) []int {
	if vectorCount == 0 || capSize <= 0 {
		return nil
	}
	if capSize >= vectorCount {
		all := make([]int, vectorCount)
		for i := 0; i < vectorCount; i++ {
			all[i] = i
		}
		return all
	}
	groups := map[string][]int{}
	for i := 0; i < vectorCount; i++ {
		source := "unknown"
		layer := "surface"
		if i < len(metadata) {
			if strings.TrimSpace(metadata[i].Source) != "" {
				source = metadata[i].Source
			}
			if strings.TrimSpace(metadata[i].Layer) != "" {
				layer = metadata[i].Layer
			}
		}
		key := source + "|" + layer
		groups[key] = append(groups[key], i)
	}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		rng.Shuffle(len(groups[k]), func(i, j int) {
			groups[k][i], groups[k][j] = groups[k][j], groups[k][i]
		})
	}

	out := make([]int, 0, capSize)
	cursor := map[string]int{}
	for len(out) < capSize {
		progress := false
		for _, k := range keys {
			pos := cursor[k]
			if pos >= len(groups[k]) {
				continue
			}
			out = append(out, groups[k][pos])
			cursor[k] = pos + 1
			progress = true
			if len(out) >= capSize {
				break
			}
		}
		if !progress {
			break
		}
	}
	return out
}

func gatherByIndices(vectors [][]float32, metadata []models.VectorMetadata, indices []int) ([][]float32, []models.VectorMetadata) {
	outV := make([][]float32, len(indices))
	outM := make([]models.VectorMetadata, len(indices))
	for i, idx := range indices {
		outV[i] = vectors[idx]
		if idx < len(metadata) {
			outM[i] = metadata[idx]
		}
	}
	return outV, outM
}

func updateMinDist(minDist []float64, vecs [][]float32, pivot int) {
	pv := vecs[pivot]
	for i, d := range minDist {
		if d == 0 {
			continue
		}
		if dist := squaredL2(pv, vecs[i]); dist < d {
			minDist[i] = dist
		}
	}
}

func squaredL2(a, b []float32) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var sum float64
	for i := 0; i < n; i++ {
		d := float64(a[i] - b[i])
		sum += d * d
	}
	return sum
}

func (s *Sampler) temporalSample(
	vectors [][]float32,
	metadata []models.VectorMetadata,
	targetSize int,
) ([][]float32, []models.VectorMetadata) {
	type item struct {
		idx int
		ts  int64
	}
	items := make([]item, 0, len(vectors))
	for i := range vectors {
		ts := int64(0)
		if i < len(metadata) {
			ts = metadata[i].Timestamp
			if ts == 0 {
				ts = parseTimestampFromRunID(metadata[i].RunID)
			}
		}
		if ts > 0 {
			items = append(items, item{idx: i, ts: ts})
		}
	}
	if len(items) < targetSize/2 || len(items) == 0 {
		return s.randomSample(vectors, metadata, targetSize)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].ts == items[j].ts {
			return items[i].idx < items[j].idx
		}
		return items[i].ts < items[j].ts
	})

	windowCount := max(3, min(8, targetSize/8+1))
	if windowCount > len(items) {
		windowCount = len(items)
	}
	windows := make([][]int, windowCount)
	for i := 0; i < windowCount; i++ {
		start := i * len(items) / windowCount
		end := (i + 1) * len(items) / windowCount
		for _, it := range items[start:end] {
			windows[i] = append(windows[i], it.idx)
		}
	}

	weights := make([]float64, windowCount)
	totalWeight := 0.0
	for i := range windows {
		if len(windows[i]) == 0 {
			continue
		}
		w := 1.0 + 1.2*(float64(i)/float64(max(1, windowCount-1)))
		weights[i] = w
		totalWeight += w
	}
	if totalWeight == 0 {
		return s.randomSample(vectors, metadata, targetSize)
	}

	quotas := make([]int, windowCount)
	remaining := targetSize
	for i := range windows {
		if len(windows[i]) == 0 {
			continue
		}
		q := int(math.Round((weights[i] / totalWeight) * float64(targetSize)))
		if q < 1 {
			q = 1
		}
		if q > len(windows[i]) {
			q = len(windows[i])
		}
		quotas[i] = q
		remaining -= q
	}

	for remaining > 0 {
		progress := false
		for i := windowCount - 1; i >= 0 && remaining > 0; i-- {
			if quotas[i] < len(windows[i]) {
				quotas[i]++
				remaining--
				progress = true
			}
		}
		if !progress {
			break
		}
	}
	for remaining < 0 {
		progress := false
		for i := 0; i < windowCount && remaining < 0; i++ {
			if quotas[i] > 1 {
				quotas[i]--
				remaining++
				progress = true
			}
		}
		if !progress {
			break
		}
	}

	rng := rand.New(rand.NewSource(s.seed))
	selected := make([]int, 0, targetSize)
	seen := make(map[int]bool, targetSize)
	for i := 0; i < windowCount; i++ {
		if quotas[i] == 0 || len(windows[i]) == 0 {
			continue
		}
		perm := rng.Perm(len(windows[i]))
		for j := 0; j < quotas[i] && j < len(perm); j++ {
			idx := windows[i][perm[j]]
			if seen[idx] {
				continue
			}
			selected = append(selected, idx)
			seen[idx] = true
		}
	}

	if len(selected) < targetSize {
		for i := len(items) - 1; i >= 0 && len(selected) < targetSize; i-- {
			idx := items[i].idx
			if seen[idx] {
				continue
			}
			selected = append(selected, idx)
			seen[idx] = true
		}
	}
	if len(selected) < targetSize {
		perm := rng.Perm(len(vectors))
		for _, idx := range perm {
			if len(selected) >= targetSize {
				break
			}
			if seen[idx] {
				continue
			}
			selected = append(selected, idx)
			seen[idx] = true
		}
	}

	return gatherByIndices(vectors, metadata, selected[:targetSize])
}

func parseTimestampFromRunID(runID string) int64 {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return 0
	}
	if n, err := strconv.ParseInt(runID, 10, 64); err == nil {
		if n > 1e12 {
			return n / 1000
		}
		return n
	}
	if t, err := time.Parse("20060102T150405Z", runID); err == nil {
		return t.Unix()
	}
	if t, err := time.Parse(time.RFC3339, runID); err == nil {
		return t.Unix()
	}
	return 0
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
