package topology

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"

	"github.com/meistro57/vectoreologist/internal/models"
)

//go:embed cluster.py
var clusterScript []byte

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

type clusterInput struct {
	Vectors  [][]float32         `json:"vectors"`
	Metadata []map[string]string `json:"metadata"`
	Params   map[string]any      `json:"params"`
}

type clusterOutput struct {
	Clusters []struct {
		ID        int       `json:"id"`
		Label     string    `json:"label"`
		VectorIDs []uint64  `json:"vector_ids"`
		Centroid  []float32 `json:"centroid"`
		Density   float64   `json:"density"`
		Size      int       `json:"size"`
		Coherence float64   `json:"coherence"`
	} `json:"clusters"`
	NoiseCount   int `json:"noise_count"`
	TotalVectors int `json:"total_vectors"`
}

// AnalyzeClusters runs UMAP + HDBSCAN via an embedded Python script.
func (t *Topology) AnalyzeClusters(vectors [][]float32, metadata []models.VectorMetadata) []models.Cluster {
	if len(vectors) == 0 {
		return nil
	}

	// Write the embedded Python script to a temp file.
	scriptF, err := os.CreateTemp("", "vectoreologist-cluster-*.py")
	if err != nil {
		fmt.Fprintf(os.Stderr, "clustering: create script temp: %v\n", err)
		return nil
	}
	defer os.Remove(scriptF.Name())
	scriptF.Write(clusterScript)
	scriptF.Close()

	// Build and write the JSON input.
	metaMaps := make([]map[string]string, len(metadata))
	for i, m := range metadata {
		metaMaps[i] = map[string]string{
			"id":     fmt.Sprintf("%d", m.ID),
			"source": m.Source,
			"layer":  m.Layer,
			"run_id": m.RunID,
		}
	}
	input := clusterInput{
		Vectors:  vectors,
		Metadata: metaMaps,
		Params: map[string]any{
			"n_neighbors": t.neighbors,
			"min_dist":    t.minDist,
		},
	}

	inputF, err := os.CreateTemp("", "vectoreologist-input-*.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "clustering: create input temp: %v\n", err)
		return nil
	}
	defer os.Remove(inputF.Name())
	if err := json.NewEncoder(inputF).Encode(input); err != nil {
		fmt.Fprintf(os.Stderr, "clustering: encode input: %v\n", err)
		return nil
	}
	inputF.Close()

	// Run the script.
	cmd := exec.Command("python3", scriptF.Name(), inputF.Name())
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "clustering: python script failed: %v\n", err)
		fmt.Fprintln(os.Stderr, "  Install deps: pip install umap-learn hdbscan numpy")
		return nil
	}

	var result clusterOutput
	if err := json.Unmarshal(out, &result); err != nil {
		fmt.Fprintf(os.Stderr, "clustering: parse output: %v\n", err)
		return nil
	}

	if result.NoiseCount > 0 {
		fmt.Printf("   ℹ %d/%d vectors classified as noise\n", result.NoiseCount, result.TotalVectors)
	}

	clusters := make([]models.Cluster, len(result.Clusters))
	for i, c := range result.Clusters {
		clusters[i] = models.Cluster{
			ID:        c.ID,
			Label:     c.Label,
			VectorIDs: c.VectorIDs,
			Centroid:  c.Centroid,
			Density:   c.Density,
			Size:      c.Size,
			Coherence: c.Coherence,
		}
	}
	return clusters
}

// FindBridges identifies semantic connections between clusters.
func (t *Topology) FindBridges(clusters []models.Cluster) []models.Bridge {
	bridges := []models.Bridge{}
	for i := 0; i < len(clusters); i++ {
		for j := i + 1; j < len(clusters); j++ {
			sim := cosineSimilarity(clusters[i].Centroid, clusters[j].Centroid)
			if sim > 0.3 {
				bridges = append(bridges, models.Bridge{
					ClusterA: clusters[i].ID,
					ClusterB: clusters[j].ID,
					Strength: sim,
					LinkType: classifyLink(sim),
				})
			}
		}
	}
	return bridges
}

// FindMoats identifies isolated cluster pairs.
func (t *Topology) FindMoats(clusters []models.Cluster) []models.Moat {
	moats := []models.Moat{}
	for i := 0; i < len(clusters); i++ {
		for j := i + 1; j < len(clusters); j++ {
			sim := cosineSimilarity(clusters[i].Centroid, clusters[j].Centroid)
			if sim < 0.1 {
				moats = append(moats, models.Moat{
					ClusterA:    clusters[i].ID,
					ClusterB:    clusters[j].ID,
					Distance:    1.0 - sim,
					Explanation: "No semantic bridge detected",
				})
			}
		}
	}
	return moats
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0.0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i] * b[i])
		normA += float64(a[i] * a[i])
		normB += float64(b[i] * b[i])
	}
	if normA == 0 || normB == 0 {
		return 0.0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func classifyLink(sim float64) string {
	switch {
	case sim > 0.7:
		return "strong_semantic"
	case sim > 0.5:
		return "moderate_bridge"
	case sim > 0.3:
		return "weak_connection"
	default:
		return "isolated"
	}
}
