package excavator

import (
	"math/rand"

	"github.com/meistro57/vectoreologist/internal/models"
)

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

func (s *Sampler) diverseSample(
	vectors [][]float32,
	metadata []models.VectorMetadata,
	targetSize int,
) ([][]float32, []models.VectorMetadata) {
	// TODO: Implement MaxMin or FarthestFirst sampling for diversity
	// For now, fallback to random
	return s.randomSample(vectors, metadata, targetSize)
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
