package anomaly

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/meistro57/vectoreologist/internal/models"
	"github.com/meistro57/vectoreologist/internal/taxonomy"
)

type Detector struct {
	densityThreshold   float64
	coherenceThreshold float64
	isolationThreshold float64
}

func New() *Detector {
	return &Detector{
		densityThreshold:   0.3,
		coherenceThreshold: 0.5,
		isolationThreshold: 0.1,
	}
}

// DetectClusterAnomalies identifies unusual cluster properties
func (d *Detector) DetectClusterAnomalies(clusters []models.Cluster) []models.Finding {
	anomalies := []models.Finding{}
	if len(clusters) == 0 {
		return anomalies
	}
	coherences := make([]float64, 0, len(clusters))
	densities := make([]float64, 0, len(clusters))
	for _, cluster := range clusters {
		coherences = append(coherences, cluster.Coherence)
		densities = append(densities, cluster.Density)
	}

	for _, cluster := range clusters {
		if cluster.Coherence < d.coherenceThreshold {
			severity := clamp01((d.coherenceThreshold - cluster.Coherence) / d.coherenceThreshold)
			tail := blendedTailStrength(lowTailStrength(coherences, cluster.Coherence), lowZTailStrength(coherences, cluster.Coherence))
			confidence := calibratedConfidence(severity, tail)
			anomalies = append(anomalies, models.Finding{
				Type:           "coherence_anomaly",
				Subject:        cluster.Label,
				IsAnomaly:      true,
				Clusters:       []int{cluster.ID},
				Confidence:     confidence,
				ConfidenceBand: confidenceBand(confidence),
				ReasoningChain: "Low coherence suggests contradictory concepts grouped together. " +
					"Vectors in this cluster may represent opposing viewpoints or conflicting information.",
			})
		}

		if cluster.Density < d.densityThreshold {
			severity := clamp01((d.densityThreshold - cluster.Density) / d.densityThreshold)
			tail := blendedTailStrength(lowTailStrength(densities, cluster.Density), lowZTailStrength(densities, cluster.Density))
			confidence := calibratedConfidence(severity, tail)
			anomalies = append(anomalies, models.Finding{
				Type:           "density_anomaly",
				Subject:        cluster.Label,
				IsAnomaly:      true,
				Clusters:       []int{cluster.ID},
				Confidence:     confidence,
				ConfidenceBand: confidenceBand(confidence),
				ReasoningChain: "Unusually low density indicates dispersed concept. " +
					"May represent an over-broad semantic category or noisy clustering.",
			})
		}

		if cluster.Density > 0.95 {
			severity := clamp01((cluster.Density - 0.95) / 0.05)
			tail := blendedTailStrength(highTailStrength(densities, cluster.Density), highZTailStrength(densities, cluster.Density))
			confidence := calibratedConfidence(severity, tail)
			anomalies = append(anomalies, models.Finding{
				Type:           "density_anomaly",
				Subject:        cluster.Label,
				IsAnomaly:      true,
				Clusters:       []int{cluster.ID},
				Confidence:     confidence,
				ConfidenceBand: confidenceBand(confidence),
				ReasoningChain: "Extremely high density suggests near-duplicate vectors. " +
					"May indicate redundant content or over-sampling from a single source.",
			})
		}
	}

	return anomalies
}

// DetectOrphans finds clusters with no bridges to other domains
func (d *Detector) DetectOrphans(clusters []models.Cluster, bridges []models.Bridge) []models.Finding {
	orphans := []models.Finding{}
	if len(clusters) == 0 {
		return orphans
	}
	connected := make(map[int]bool)
	degree := make(map[int]int)
	for _, bridge := range bridges {
		connected[bridge.ClusterA] = true
		connected[bridge.ClusterB] = true
		degree[bridge.ClusterA]++
		degree[bridge.ClusterB]++
	}

	connectedCount := 0
	sizes := make([]float64, 0, len(clusters))
	degrees := make([]float64, 0, len(clusters))
	for _, cluster := range clusters {
		sizes = append(sizes, float64(cluster.Size))
		degrees = append(degrees, float64(degree[cluster.ID]))
		if connected[cluster.ID] {
			connectedCount++
		}
	}
	connectedRatio := float64(connectedCount) / float64(len(clusters))

	for _, cluster := range clusters {
		if !connected[cluster.ID] {
			sizeTail := blendedTailStrength(highTailStrength(sizes, float64(cluster.Size)), highZTailStrength(sizes, float64(cluster.Size)))
			isolationTail := blendedTailStrength(lowTailStrength(degrees, float64(degree[cluster.ID])), lowZTailStrength(degrees, float64(degree[cluster.ID])))
			severity := clamp01(0.35 + 0.35*connectedRatio + 0.30*isolationTail)
			confidence := calibratedConfidence(severity, sizeTail)
			orphans = append(orphans, models.Finding{
				Type:           "orphan_cluster",
				Subject:        cluster.Label,
				IsAnomaly:      true,
				Clusters:       []int{cluster.ID},
				Confidence:     confidence,
				ConfidenceBand: confidenceBand(confidence),
				ReasoningChain: "Isolated concept with no semantic bridges to other knowledge domains. " +
					"May represent unique/specialized knowledge or a disconnected information silo.",
			})
		}
	}

	return orphans
}

// DetectContradictions finds clusters with opposing metadata but high vector similarity
func (d *Detector) DetectContradictions(
	clusters []models.Cluster,
	metadata []models.VectorMetadata,
) []models.Finding {
	contradictions := []models.Finding{}
	if len(clusters) == 0 {
		return contradictions
	}

	sourceByID := make(map[uint64]string, len(metadata))
	for _, meta := range metadata {
		sourceByID[meta.ID] = meta.Source
	}

	coherences := make([]float64, 0, len(clusters))
	diversities := make([]float64, 0, len(clusters))
	clusterSources := make(map[int]map[string]int, len(clusters))
	for _, cluster := range clusters {
		coherences = append(coherences, cluster.Coherence)
		sources := make(map[string]int)
		for _, vecID := range cluster.VectorIDs {
			source := strings.TrimSpace(sourceByID[vecID])
			if source == "" {
				source = "unknown"
			}
			sources[source]++
		}
		clusterSources[cluster.ID] = sources
		diversities = append(diversities, float64(len(sources)))
	}

	for _, cluster := range clusters {
		sources := clusterSources[cluster.ID]
		if cluster.Coherence > 0.8 && len(sources) > 3 {
			sourceNames := make([]string, 0, len(sources))
			for source := range sources {
				sourceNames = append(sourceNames, source)
			}
			sort.Strings(sourceNames)
			coherenceSeverity := clamp01((cluster.Coherence - 0.8) / 0.2)
			diversitySeverity := clamp01(float64(len(sources)-3) / 5.0)
			severity := clamp01(0.5*coherenceSeverity + 0.5*diversitySeverity)
			coherenceTail := blendedTailStrength(highTailStrength(coherences, cluster.Coherence), highZTailStrength(coherences, cluster.Coherence))
			diversityTail := blendedTailStrength(highTailStrength(diversities, float64(len(sources))), highZTailStrength(diversities, float64(len(sources))))
			confidence := calibratedConfidence(severity, clamp01(0.5*coherenceTail+0.5*diversityTail))
			contradictions = append(contradictions, models.Finding{
				Type:           "source_contradiction",
				Subject:        cluster.Label,
				IsAnomaly:      true,
				Clusters:       []int{cluster.ID},
				Confidence:     confidence,
				ConfidenceBand: confidenceBand(confidence),
				ReasoningChain: "High coherence with diverse sources suggests consensus across domains: " + strings.Join(sourceNames, ", ") +
					". This may indicate convergent truth or a widely-propagated narrative.",
			})
		}
	}

	return contradictions
}

// DetectTaxonomyAnomalies runs all taxonomy-aware anomaly detectors in one pass.
// Requires clusters to already have Taxonomy set by taxonomy.Classifier.
func (d *Detector) DetectTaxonomyAnomalies(clusters []models.Cluster, metadata []models.VectorMetadata) []models.Finding {
	var out []models.Finding
	out = append(out, d.DetectLabelMismatches(clusters)...)
	out = append(out, d.DetectSourceOversampling(clusters, metadata)...)
	out = append(out, d.DetectSummaryArtifacts(clusters)...)
	out = append(out, d.DetectEmbeddingBias(clusters, metadata)...)
	return out
}

// DetectLabelMismatches flags clusters where the taxonomy classifier's LabelWarning is set,
// indicating the cluster label does not match the actual content.
func (d *Detector) DetectLabelMismatches(clusters []models.Cluster) []models.Finding {
	var out []models.Finding
	for _, c := range clusters {
		if c.Taxonomy == nil || c.Taxonomy.LabelWarning == "" {
			continue
		}
		out = append(out, models.Finding{
			Type:        taxonomy.AnomalyLabelMismatch,
			Subject:     c.Label,
			IsAnomaly:   true,
			Clusters:    []int{c.ID},
			AnomalyType: taxonomy.AnomalyLabelMismatch,
			Evidence:    c.Taxonomy.LabelWarning,
			PossibleCauses: []string{
				"source metadata assigned to wrong collection",
				"content repurposed from unrelated document",
				"embedding model conflated semantically distant concepts",
			},
			RequiresReview: true,
			ReasoningChain: fmt.Sprintf(
				"Cluster label '%s' conflicts with classified content (topic=%s, mode=%s): %s",
				c.Label, c.Taxonomy.Topic, c.Taxonomy.Mode, c.Taxonomy.LabelWarning,
			),
		})
	}
	return out
}

// DetectSourceOversampling flags clusters where a single source contributes more
// than 70 % of the member vectors, suggesting over-representation.
func (d *Detector) DetectSourceOversampling(clusters []models.Cluster, metadata []models.VectorMetadata) []models.Finding {
	byID := make(map[uint64]string, len(metadata))
	for _, m := range metadata {
		byID[m.ID] = m.Source
	}

	var out []models.Finding
	for _, c := range clusters {
		if len(c.VectorIDs) == 0 {
			continue
		}
		counts := make(map[string]int)
		for _, id := range c.VectorIDs {
			src := byID[id]
			if src == "" {
				src = "unknown"
			}
			counts[src]++
		}
		top, topN := "", 0
		for src, n := range counts {
			if n > topN {
				top, topN = src, n
			}
		}
		ratio := float64(topN) / float64(len(c.VectorIDs))
		if ratio > 0.70 && len(counts) > 1 {
			out = append(out, models.Finding{
				Type:        taxonomy.AnomalySourceOversampling,
				Subject:     c.Label,
				IsAnomaly:   true,
				Clusters:    []int{c.ID},
				AnomalyType: taxonomy.AnomalySourceOversampling,
				Evidence:    fmt.Sprintf("source '%s' contributes %.0f%% of %d vectors", top, ratio*100, len(c.VectorIDs)),
				PossibleCauses: []string{
					"unbalanced dataset extraction",
					"source dominates collection",
					"sampling strategy did not diversify across sources",
				},
				RequiresReview: ratio > 0.90,
				ReasoningChain: fmt.Sprintf(
					"Source oversampling: '%s' accounts for %.0f%% of cluster '%s'. Other sources present: %d.",
					top, ratio*100, c.Label, len(counts)-1,
				),
			})
		}
	}
	return out
}

// DetectSummaryArtifacts flags clusters whose taxonomy mode is meta_descriptive_summary
// but whose label indicates a specific content domain — suggesting the cluster contains
// document scaffolding rather than primary content.
func (d *Detector) DetectSummaryArtifacts(clusters []models.Cluster) []models.Finding {
	var out []models.Finding
	for _, c := range clusters {
		if c.Taxonomy == nil || c.Taxonomy.Mode != taxonomy.ModeMetaDescriptiveSummary {
			continue
		}
		// Flag when a specific (non-generic) topic is attached to a summary cluster.
		topic := c.Taxonomy.Topic
		if topic == "" || topic == "general" {
			continue
		}
		// Only flag if the label references the topic (content looks like a summary of that topic).
		lower := strings.ToLower(c.Label)
		if !strings.Contains(lower, strings.Split(topic, "_")[0]) {
			continue
		}
		out = append(out, models.Finding{
			Type:        taxonomy.AnomalySummaryTemplateArtifact,
			Subject:     c.Label,
			IsAnomaly:   true,
			Clusters:    []int{c.ID},
			AnomalyType: taxonomy.AnomalySummaryTemplateArtifact,
			Evidence: fmt.Sprintf(
				"mode=meta_descriptive_summary with topic='%s' suggests document scaffold, not primary content",
				topic,
			),
			PossibleCauses: []string{
				"introductory or index sections were embedded as content",
				"table-of-contents or chapter summaries included in extraction",
				"template text repeated across source documents",
			},
			RequiresReview: true,
			ReasoningChain: fmt.Sprintf(
				"Cluster '%s' contains summary/overview text about topic '%s'. "+
					"This may be structural document scaffolding rather than primary knowledge.",
				c.Label, topic,
			),
		})
	}
	return out
}

// DetectEmbeddingBias flags clusters that combine extremely high density with
// single-source dominance — a pattern consistent with near-duplicate embeddings
// from one source, possibly indicating model bias or content repetition.
func (d *Detector) DetectEmbeddingBias(clusters []models.Cluster, metadata []models.VectorMetadata) []models.Finding {
	byID := make(map[uint64]string, len(metadata))
	for _, m := range metadata {
		byID[m.ID] = m.Source
	}

	var out []models.Finding
	for _, c := range clusters {
		if c.Density <= 0.95 || len(c.VectorIDs) == 0 {
			continue
		}
		counts := make(map[string]int)
		for _, id := range c.VectorIDs {
			src := byID[id]
			if src == "" {
				src = "unknown"
			}
			counts[src]++
		}
		if len(counts) != 1 {
			continue // bias signal only when there is exactly one source
		}
		var onlySource string
		for src := range counts {
			onlySource = src
		}
		out = append(out, models.Finding{
			Type:        taxonomy.AnomalyEmbeddingBiasSuspected,
			Subject:     c.Label,
			IsAnomaly:   true,
			Clusters:    []int{c.ID},
			AnomalyType: taxonomy.AnomalyEmbeddingBiasSuspected,
			Evidence: fmt.Sprintf(
				"density=%.2f, all %d vectors from single source '%s'",
				c.Density, len(c.VectorIDs), onlySource,
			),
			PossibleCauses: []string{
				"near-duplicate content from one source embedded repeatedly",
				"embedding model collapses semantically similar text from this source",
				"extraction pulled too many similar chunks from the same document",
			},
			RequiresReview: true,
			ReasoningChain: fmt.Sprintf(
				"Cluster '%s' has density %.2f (>0.95) and all vectors come from '%s'. "+
					"High density + single source strongly suggests near-duplicate embeddings.",
				c.Label, c.Density, onlySource,
			),
		})
	}
	return out
}

func calibratedConfidence(severity, tail float64) float64 {
	return clamp01(0.55*severity + 0.45*tail)
}

func blendedTailStrength(percentileTail, zTail float64) float64 {
	return clamp01(0.6*percentileTail + 0.4*zTail)
}

func lowZTailStrength(values []float64, value float64) float64 {
	mean, std := meanStd(values)
	if std == 0 {
		return 0.5
	}
	z := (value - mean) / std
	if z >= 0 {
		return 0
	}
	return clamp01((-z) / 3.0)
}

func highZTailStrength(values []float64, value float64) float64 {
	mean, std := meanStd(values)
	if std == 0 {
		return 0.5
	}
	z := (value - mean) / std
	if z <= 0 {
		return 0
	}
	return clamp01(z / 3.0)
}

func meanStd(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	mean := 0.0
	for _, v := range values {
		mean += v
	}
	mean /= float64(len(values))
	variance := 0.0
	for _, v := range values {
		d := v - mean
		variance += d * d
	}
	variance /= float64(len(values))
	return mean, math.Sqrt(variance)
}

func confidenceBand(confidence float64) string {
	switch {
	case confidence >= 0.85:
		return "critical"
	case confidence >= 0.70:
		return "high"
	case confidence >= 0.50:
		return "medium"
	default:
		return "low"
	}
}

func lowTailStrength(values []float64, value float64) float64 {
	if len(values) == 0 {
		return 0.5
	}
	rank := percentileRank(values, value)
	return 1.0 - rank
}

func highTailStrength(values []float64, value float64) float64 {
	if len(values) == 0 {
		return 0.5
	}
	return percentileRank(values, value)
}

func percentileRank(values []float64, value float64) float64 {
	if len(values) == 0 {
		return 0.5
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	count := 0
	for _, v := range sorted {
		if v <= value {
			count++
		}
	}
	return float64(count) / float64(len(sorted))
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// ScoreAnomaly calculates anomaly weight (higher = more interesting)
func (d *Detector) ScoreAnomaly(finding models.Finding, cluster models.Cluster) float64 {
	var score float64

	switch finding.Type {
	case "coherence_anomaly":
		// Lower coherence = higher anomaly score
		score = 1.0 - cluster.Coherence
	case "density_anomaly":
		// Distance from ideal density (0.6-0.8 range)
		idealDensity := 0.7
		score = math.Abs(cluster.Density - idealDensity)
	case "orphan_cluster":
		// Orphans are always high-value anomalies
		score = 0.9
	case "source_contradiction":
		// Contradictions are critical findings
		score = 0.95
	default:
		score = 0.5
	}

	return score
}
