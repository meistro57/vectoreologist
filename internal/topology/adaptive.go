package topology

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/meistro57/vectoreologist/internal/models"
)

type DBSCANDiagnostics struct {
	AutoTuned       bool
	FallbackUsed    bool
	RequestedEps    float64
	RequestedMinPts int
	ChosenEps       float64
	ChosenMinPts    int
	SampledPairs    int
	P10             float64
	P25             float64
	P50             float64
	P75             float64
	P90             float64
	Reason          string
}

func (t *Topology) selectDBSCANParams(reduced [][]float32) (float64, int, DBSCANDiagnostics) {
	diag := DBSCANDiagnostics{
		RequestedEps:    t.epsilon,
		RequestedMinPts: t.minClusterSize,
		ChosenEps:       t.epsilon,
		ChosenMinPts:    t.minClusterSize,
		Reason:          "Using configured DBSCAN defaults.",
	}
	if !t.autoTune {
		return diag.ChosenEps, diag.ChosenMinPts, diag
	}
	if len(reduced) < 8 {
		diag.FallbackUsed = true
		diag.Reason = fmt.Sprintf("Auto-tune skipped: only %d vectors available.", len(reduced))
		return diag.ChosenEps, diag.ChosenMinPts, diag
	}

	dists := t.samplePairwiseDistances(reduced, 4096)
	if len(dists) < 32 {
		diag.FallbackUsed = true
		diag.Reason = fmt.Sprintf("Auto-tune skipped: sampled only %d pairwise distances.", len(dists))
		return diag.ChosenEps, diag.ChosenMinPts, diag
	}
	sort.Float64s(dists)
	p10 := percentileSorted(dists, 0.10)
	p25 := percentileSorted(dists, 0.25)
	p50 := percentileSorted(dists, 0.50)
	p75 := percentileSorted(dists, 0.75)
	p90 := percentileSorted(dists, 0.90)

	diag.SampledPairs = len(dists)
	diag.P10 = p10
	diag.P25 = p25
	diag.P50 = p50
	diag.P75 = p75
	diag.P90 = p90

	spread := p90 - p10
	if spread < 0.03 {
		diag.FallbackUsed = true
		diag.Reason = fmt.Sprintf("Auto-tune skipped: distance spread too narrow (p90-p10=%.3f).", spread)
		return diag.ChosenEps, diag.ChosenMinPts, diag
	}

	iqr := p75 - p25
	eps := clamp(p25+0.35*iqr, 0.12, 0.6)
	minPts := int(math.Round(math.Log(float64(len(reduced))) * 1.8))
	if p25 < 0.22 {
		minPts++
	}
	if p25 > 0.35 {
		minPts--
	}
	minPts = clampInt(minPts, 3, 12)

	diag.AutoTuned = true
	diag.ChosenEps = eps
	diag.ChosenMinPts = minPts
	diag.Reason = fmt.Sprintf("Auto-tuned from sampled pairwise distances (p25=%.3f, iqr=%.3f, spread=%.3f).", p25, iqr, spread)
	return eps, minPts, diag
}

func (t *Topology) samplePairwiseDistances(vectors [][]float32, maxPairs int) []float64 {
	n := len(vectors)
	if n < 2 {
		return nil
	}
	totalPairs := n * (n - 1) / 2
	if maxPairs > totalPairs {
		maxPairs = totalPairs
	}
	if maxPairs <= 0 {
		return nil
	}
	out := make([]float64, 0, maxPairs)
	seen := make(map[uint64]bool, maxPairs)
	for len(out) < maxPairs {
		i := t.rng.Intn(n)
		j := t.rng.Intn(n)
		if i == j {
			continue
		}
		if i > j {
			i, j = j, i
		}
		key := uint64(i)*uint64(n) + uint64(j)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, unitCosineDistance(vectors[i], vectors[j]))
	}
	return out
}

func percentileSorted(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[len(sorted)-1]
	}
	idx := p * float64(len(sorted)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return sorted[lo]
	}
	frac := idx - float64(lo)
	return sorted[lo] + frac*(sorted[hi]-sorted[lo])
}

func hybridClusterLabel(
	metadata []models.VectorMetadata,
	indices []int,
	vectors [][]float32,
	centroid []float32,
	fallback string,
) string {
	meta := topMetadataSignature(metadata, indices)
	exemplars := nearestExemplarTitles(metadata, indices, vectors, centroid, 2)
	if len(exemplars) == 0 {
		if meta != "" {
			return meta
		}
		return fallback
	}
	if meta == "" {
		return strings.Join(exemplars, " | ")
	}
	return fmt.Sprintf("%s · %s", meta, strings.Join(exemplars, " | "))
}

func topMetadataSignature(metadata []models.VectorMetadata, indices []int) string {
	sourceCounts := map[string]int{}
	layerCounts := map[string]int{}
	runCounts := map[string]int{}
	for _, idx := range indices {
		if idx < 0 || idx >= len(metadata) {
			continue
		}
		m := metadata[idx]
		source := strings.TrimSpace(m.Source)
		if source == "" {
			source = "unknown"
		}
		layer := strings.TrimSpace(m.Layer)
		if layer == "" {
			layer = "surface"
		}
		run := strings.TrimSpace(m.RunID)
		if run == "" {
			run = "unknown"
		}
		sourceCounts[source]++
		layerCounts[layer]++
		runCounts[run]++
	}
	topLayer := pluralityKey(layerCounts, "surface")
	topSource := pluralityKey(sourceCounts, "unknown")
	topRun := pluralityKey(runCounts, "unknown")
	if topRun != "unknown" {
		return fmt.Sprintf("layer=%s source=%s run=%s", topLayer, topSource, topRun)
	}
	return fmt.Sprintf("layer=%s source=%s", topLayer, topSource)
}

func nearestExemplarTitles(
	metadata []models.VectorMetadata,
	indices []int,
	vectors [][]float32,
	centroid []float32,
	limit int,
) []string {
	type candidate struct {
		title string
		sim   float64
	}
	cands := make([]candidate, 0, len(indices))
	for _, idx := range indices {
		if idx < 0 || idx >= len(metadata) || idx >= len(vectors) {
			continue
		}
		frag := strings.TrimSpace(metadata[idx].Fragment)
		if frag == "" || frag == "N/A" {
			continue
		}
		title := snippetTitle(frag)
		if title == "" {
			continue
		}
		cands = append(cands, candidate{title: title, sim: cosineSimilarity(vectors[idx], centroid)})
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].sim == cands[j].sim {
			return cands[i].title < cands[j].title
		}
		return cands[i].sim > cands[j].sim
	})
	seen := map[string]bool{}
	out := make([]string, 0, limit)
	for _, c := range cands {
		if seen[c.title] {
			continue
		}
		seen[c.title] = true
		out = append(out, c.title)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func snippetTitle(fragment string) string {
	words := strings.Fields(fragment)
	if len(words) == 0 {
		return ""
	}
	if len(words) > 6 {
		words = words[:6]
	}
	return strings.Trim(strings.Join(words, " "), ".,;:!?\"'`")
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
