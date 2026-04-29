package topology

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"sort"

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

// FindBridges identifies semantic connections between clusters and populates
// SampleLinks with representative cross-cluster chunk pairs.
func (t *Topology) FindBridges(clusters []models.Cluster, vectors [][]float32, metadata []models.VectorMetadata) []models.Bridge {
	idToVec := make(map[uint64][]float32, len(metadata))
	for i, m := range metadata {
		if i < len(vectors) {
			idToVec[m.ID] = vectors[i]
		}
	}

	bridges := []models.Bridge{}
	for i := 0; i < len(clusters); i++ {
		for j := i + 1; j < len(clusters); j++ {
			sim := cosineSimilarity(clusters[i].Centroid, clusters[j].Centroid)
			if sim > 0.3 {
				links := computeSampleLinks(clusters[i].VectorIDs, clusters[j].VectorIDs, idToVec, 5)
				bridges = append(bridges, models.Bridge{
					ClusterA:    clusters[i].ID,
					ClusterB:    clusters[j].ID,
					Strength:    sim,
					LinkType:    classifyLink(sim),
					SampleLinks: links,
				})
			}
		}
	}
	return bridges
}

type vecEntry struct {
	id  uint64
	vec []float32
}

// gatherVecs collects up to limit vectors from the lookup for the given IDs.
// When there are more IDs than the limit, it shuffles first so the sample
// is drawn from across the cluster rather than always from the first N.
func gatherVecs(ids []uint64, lookup map[uint64][]float32, limit int) []vecEntry {
	if len(ids) > limit {
		shuffled := make([]uint64, len(ids))
		copy(shuffled, ids)
		rand.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		ids = shuffled
	}
	out := make([]vecEntry, 0, min(len(ids), limit))
	for _, id := range ids {
		if vec, ok := lookup[id]; ok {
			out = append(out, vecEntry{id, vec})
			if len(out) >= limit {
				break
			}
		}
	}
	return out
}

// computeSampleLinks returns the top-n cross-cluster pairs by cosine similarity.
// At most 50 points per side are compared to bound cost on large clusters.
func computeSampleLinks(aIDs, bIDs []uint64, idToVec map[uint64][]float32, n int) []models.SampleLink {
	const perSide = 50
	aVecs := gatherVecs(aIDs, idToVec, perSide)
	bVecs := gatherVecs(bIDs, idToVec, perSide)
	if len(aVecs) == 0 || len(bVecs) == 0 {
		return nil
	}
	type scored struct {
		aID, bID uint64
		sim      float64
	}
	pairs := make([]scored, 0, len(aVecs)*len(bVecs))
	for _, a := range aVecs {
		for _, b := range bVecs {
			pairs = append(pairs, scored{a.id, b.id, cosineSimilarity(a.vec, b.vec)})
		}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].sim > pairs[j].sim })
	if len(pairs) > n {
		pairs = pairs[:n]
	}
	links := make([]models.SampleLink, len(pairs))
	for i, p := range pairs {
		links[i] = models.SampleLink{ChunkAID: p.aID, ChunkBID: p.bID, Similarity: p.sim}
	}
	return links
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
