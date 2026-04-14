package topology

import (
	"math"

	"github.com/meistro57/vectoreologist/internal/models"
)

type Topology struct {
	neighbors int
	minDist   float64
}

func New() *Topology {
	return &Topology{
		neighbors: 15,
		minDist:   0.1,
	}
}

// AnalyzeClusters performs dimensionality reduction and clustering
// TODO: Implement proper UMAP + HDBSCAN
func (t *Topology) AnalyzeClusters(vectors [][]float32, metadata []models.VectorMetadata) []models.Cluster {
	// Placeholder: Return sample cluster for structure
	return []models.Cluster{
		{
			ID:        1,
			Label:     "Consciousness & Phenomenology",
			VectorIDs: []uint64{1, 2, 3},
			Centroid:  make([]float32, len(vectors[0])),
			Density:   0.85,
			Size:      3,
			Coherence: 0.92,
		},
	}
}

// FindBridges identifies semantic connections between clusters
func (t *Topology) FindBridges(clusters []models.Cluster) []models.Bridge {
	bridges := []models.Bridge{}
	
	for i := 0; i < len(clusters); i++ {
		for j := i + 1; j < len(clusters); j++ {
			similarity := t.clusterSimilarity(clusters[i], clusters[j])
			
			if similarity > 0.3 {
				bridges = append(bridges, models.Bridge{
					ClusterA: clusters[i].ID,
					ClusterB: clusters[j].ID,
					Strength: similarity,
					LinkType: classifyLink(similarity),
				})
			}
		}
	}
	
	return bridges
}

// FindMoats identifies isolated cluster pairs
func (t *Topology) FindMoats(clusters []models.Cluster) []models.Moat {
	moats := []models.Moat{}
	
	for i := 0; i < len(clusters); i++ {
		for j := i + 1; j < len(clusters); j++ {
			similarity := t.clusterSimilarity(clusters[i], clusters[j])
			
			if similarity < 0.1 {
				moats = append(moats, models.Moat{
					ClusterA:    clusters[i].ID,
					ClusterB:    clusters[j].ID,
					Distance:    1.0 - similarity,
					Explanation: "No semantic bridge detected",
				})
			}
		}
	}
	
	return moats
}

func (t *Topology) clusterSimilarity(a, b models.Cluster) float64 {
	return cosineSimilarity(a.Centroid, b.Centroid)
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0.0
	}
	
	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i] * b[i])
		normA += float64(a[i] * a[i])
		normB += float64(b[i] * b[i])
	}
	
	if normA == 0 || normB == 0 {
		return 0.0
	}
	
	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

func classifyLink(similarity float64) string {
	switch {
	case similarity > 0.7:
		return "strong_semantic"
	case similarity > 0.5:
		return "moderate_bridge"
	case similarity > 0.3:
		return "weak_connection"
	default:
		return "isolated"
	}
}
