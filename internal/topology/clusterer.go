package topology

import (
	"fmt"
	"math"
	"math/rand"
	"sort"

	"github.com/meistro57/vectoreologist/internal/models"
)

// MaxTopologyTotal is the total vector count fed into topology analysis.
// When the input exceeds this, vectors are randomly sampled. Clustered topology
// on a representative sample is more coherent than processing the full dataset
// anyway. Use --sample at extraction time to keep the full dataset under this
// limit.
const MaxTopologyTotal = 20000

// Topology holds configuration for cluster analysis.
type Topology struct {
	minClusterSize int
	epsilon        float64
}

// New returns a Topology with sensible defaults.
func New() *Topology {
	return &Topology{
		minClusterSize: 5,
		epsilon:        0.3,
	}
}

// SetClusterParams updates the DBSCAN parameters. Non-positive values are
// ignored (existing values are kept).
func (t *Topology) SetClusterParams(minClusterSize int, epsilon float64) {
	if minClusterSize > 0 {
		t.minClusterSize = minClusterSize
	}
	if epsilon > 0 {
		t.epsilon = epsilon
	}
}

// AnalyzeClusters runs PCA + DBSCAN on the provided vectors.
// All computation is in-process — no Python subprocess is spawned.
func (t *Topology) AnalyzeClusters(vectors [][]float32, metadata []models.VectorMetadata) []models.Cluster {
	if len(vectors) == 0 {
		return nil
	}

	// If the input exceeds the safe topology total, randomly subsample.
	if len(vectors) > MaxTopologyTotal {
		fmt.Printf("   ℹ Topology: sampling %d of %d vectors to stay within memory limits (use --sample %d to cap at extraction)\n",
			MaxTopologyTotal, len(vectors), MaxTopologyTotal)
		indices := rand.Perm(len(vectors))[:MaxTopologyTotal]
		sort.Ints(indices)
		sampledVecs := make([][]float32, MaxTopologyTotal)
		sampledMeta := make([]models.VectorMetadata, MaxTopologyTotal)
		for i, idx := range indices {
			sampledVecs[i] = vectors[idx]
			sampledMeta[i] = metadata[idx]
		}
		vectors = sampledVecs
		metadata = sampledMeta
	}

	// Work on a copy of the vectors so we don't mutate the caller's data during
	// normalisation and PCA projection.
	n := len(vectors)
	working := make([][]float32, n)
	for i, v := range vectors {
		cp := make([]float32, len(v))
		copy(cp, v)
		working[i] = cp
	}

	// PCA pre-reduction: shrink high-dimensional vectors to pcaDims to reduce
	// DBSCAN neighbour-search cost. Skip if already small enough.
	d := 0
	if len(working[0]) > 0 {
		d = len(working[0])
	}
	reduced := working
	if d > pcaDims {
		reduced = pcaReduce(working, pcaDims)
	}

	// L2-normalise so that Euclidean distance in the normalised space equals
	// cosine distance (unitCosineDistance assumes unit vectors).
	l2Normalise(reduced)

	// DBSCAN clustering.
	labels := runDBSCAN(reduced, t.epsilon, t.minClusterSize)

	// Count noise for informational output.
	noiseCount := 0
	for _, l := range labels {
		if l == -1 {
			noiseCount++
		}
	}
	if noiseCount > 0 {
		fmt.Printf("   ℹ %d/%d vectors classified as noise\n", noiseCount, n)
	}

	// Group indices by cluster ID.
	clusterMap := make(map[int][]int)
	for idx, lbl := range labels {
		if lbl == -1 {
			continue
		}
		clusterMap[lbl] = append(clusterMap[lbl], idx)
	}

	if len(clusterMap) == 0 {
		return nil
	}

	// Sort cluster IDs for deterministic output.
	clusterIDs := make([]int, 0, len(clusterMap))
	for id := range clusterMap {
		clusterIDs = append(clusterIDs, id)
	}
	sort.Ints(clusterIDs)

	clusters := make([]models.Cluster, 0, len(clusterIDs))
	for rank, cid := range clusterIDs {
		indices := clusterMap[cid]

		// Centroid computed in ORIGINAL (pre-PCA) vector space.
		dim := len(vectors[indices[0]])
		centroid := make([]float64, dim)
		for _, idx := range indices {
			for j, val := range vectors[idx] {
				centroid[j] += float64(val)
			}
		}
		invSize := 1.0 / float64(len(indices))
		centroid32 := make([]float32, dim)
		for j := range centroid {
			centroid32[j] = float32(centroid[j] * invSize)
		}

		// Coherence: mean cosine similarity of each original vector to centroid.
		coherence := meanCosineSimilarityToCenter(vectors, indices, centroid32)

		// Density: compactness in the reduced (post-PCA, normalised) space.
		density := clusterDensity(reduced, indices)

		// Dominant layer / source label.
		label := dominantLabel(metadata, indices)

		// Vector IDs from original metadata.
		vectorIDs := make([]uint64, len(indices))
		for i, idx := range indices {
			vectorIDs[i] = metadata[idx].ID
		}

		clusters = append(clusters, models.Cluster{
			ID:        rank + 1,
			Label:     label,
			VectorIDs: vectorIDs,
			Centroid:  centroid32,
			Density:   density,
			Size:      len(indices),
			Coherence: coherence,
		})
	}

	return clusters
}

// meanCosineSimilarityToCenter returns the mean cosine similarity of vectors at
// the given indices to the provided centroid.
func meanCosineSimilarityToCenter(vectors [][]float32, indices []int, centroid []float32) float64 {
	var sum float64
	for _, idx := range indices {
		sum += cosineSimilarity(vectors[idx], centroid)
	}
	if len(indices) == 0 {
		return 0
	}
	return sum / float64(len(indices))
}

// clusterDensity returns 1/(1+mean_spread) where mean_spread is the mean
// Euclidean distance from each cluster member to the cluster mean in the
// reduced space.
func clusterDensity(reduced [][]float32, indices []int) float64 {
	if len(indices) == 0 {
		return 0
	}
	dim := len(reduced[indices[0]])
	mean := make([]float64, dim)
	for _, idx := range indices {
		for j, val := range reduced[idx] {
			mean[j] += float64(val)
		}
	}
	invN := 1.0 / float64(len(indices))
	for j := range mean {
		mean[j] *= invN
	}
	var spread float64
	for _, idx := range indices {
		var sq float64
		for j, val := range reduced[idx] {
			d := float64(val) - mean[j]
			sq += d * d
		}
		spread += math.Sqrt(sq)
	}
	spread /= float64(len(indices))
	return 1.0 / (1.0 + spread)
}

// dominantLabel returns "topLayer / topSource" based on plurality vote across
// the cluster members.
func dominantLabel(metadata []models.VectorMetadata, indices []int) string {
	sources := make(map[string]int)
	layers := make(map[string]int)
	for _, idx := range indices {
		if idx < len(metadata) {
			s := metadata[idx].Source
			if s == "" {
				s = "unknown"
			}
			l := metadata[idx].Layer
			if l == "" {
				l = "surface"
			}
			sources[s]++
			layers[l]++
		}
	}
	topSource := pluralityKey(sources, "unknown")
	topLayer := pluralityKey(layers, "surface")
	return topLayer + " / " + topSource
}

func pluralityKey(m map[string]int, fallback string) string {
	best := fallback
	bestN := -1
	for k, n := range m {
		if n > bestN || (n == bestN && k < best) {
			best = k
			bestN = n
		}
	}
	return best
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

