package excavator

import (
	"math"
	"math/rand"

	"github.com/meistro57/vectoreologist/internal/models"
)

// diversePoolCap limits the candidate pool fed to MaxMin sampling.
// MaxMin is O(pool × target × dims), so capping keeps it practical.
// Raised from 3000 to 15000 to support larger collections.
const diversePoolCap = 15000

// SamplingStrategy determines how vectors are selected from a collection
type SamplingStrategy string

const (
	Random     SamplingStrategy = "random"     // Random sampling
	Stratified SamplingStrategy = "stratified" // Sample proportionally from each source
	Temporal   SamplingStrategy = "temporal"   // Sample across time windows
	Diverse    SamplingStrategy = "diverse"    // Maximize vector diversity
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

// Sample selects vectors according to the sampling strategy
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
	// Group by source
	sourceGroups := make(map[string][]int)
	for i, meta := range metadata {
		sourceGroups[meta.Source] = append(sourceGroups[meta.Source], i)
	}

	// Sample proportionally from each source
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

	return sampledVecs, sampledMeta
}

// diverseSample implements greedy MaxMin (Farthest-First) sampling.
// It iteratively picks the point maximally distant from the current selection,
// guaranteeing that the selected set spans the vector space.
func (s *Sampler) diverseSample(
	vectors [][]float32,
	metadata []models.VectorMetadata,
	targetSize int,
) ([][]float32, []models.VectorMetadata) {
	// Pre-filter to a manageable pool to bound O(pool × target × dims) cost.
	poolVecs, poolMeta := vectors, metadata
	if len(vectors) > diversePoolCap {
		poolVecs, poolMeta = s.randomSample(vectors, metadata, diversePoolCap)
	}
	pool := len(poolVecs)
	if pool <= targetSize {
		return poolVecs, poolMeta
	}

	rng := rand.New(rand.NewSource(s.seed))

	// minDist[i] = min squared-L2 distance from point i to any selected point.
	// Zero means the point is already selected.
	minDist := make([]float64, pool)
	for i := range minDist {
		minDist[i] = math.MaxFloat64
	}

	selected := make([]int, 0, targetSize)

	// Seed with a random point.
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

// updateMinDist sets minDist[i] = min(minDist[i], squaredL2(vecs[pivot], vecs[i]))
// for all unselected points (minDist[i] > 0).
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

// squaredL2 returns the squared Euclidean distance between two vectors.
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
	// TODO: Parse temporal metadata and sample across time windows
	// For now, fallback to random
	return s.randomSample(vectors, metadata, targetSize)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
