package anomaly

import (
	"math"

	"github.com/meistro57/vectoreologist/internal/models"
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

	for _, cluster := range clusters {
		// Low coherence = contradictory vectors in same cluster
		if cluster.Coherence < d.coherenceThreshold {
			anomalies = append(anomalies, models.Finding{
				Type:      "coherence_anomaly",
				Subject:   cluster.Label,
				IsAnomaly: true,
				Clusters:  []int{cluster.ID},
				ReasoningChain: "Low coherence suggests contradictory concepts grouped together. " +
					"Vectors in this cluster may represent opposing viewpoints or conflicting information.",
			})
		}

		// Unusual density = either extremely tight or extremely loose clustering
		if cluster.Density < d.densityThreshold {
			anomalies = append(anomalies, models.Finding{
				Type:      "density_anomaly",
				Subject:   cluster.Label,
				IsAnomaly: true,
				Clusters:  []int{cluster.ID},
				ReasoningChain: "Unusually low density indicates dispersed concept. " +
					"May represent an over-broad semantic category or noisy clustering.",
			})
		}

		if cluster.Density > 0.95 {
			anomalies = append(anomalies, models.Finding{
				Type:      "density_anomaly",
				Subject:   cluster.Label,
				IsAnomaly: true,
				Clusters:  []int{cluster.ID},
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
	
	// Build connectivity map
	connected := make(map[int]bool)
	for _, bridge := range bridges {
		connected[bridge.ClusterA] = true
		connected[bridge.ClusterB] = true
	}

	for _, cluster := range clusters {
		if !connected[cluster.ID] {
			orphans = append(orphans, models.Finding{
				Type:      "orphan_cluster",
				Subject:   cluster.Label,
				IsAnomaly: true,
				Clusters:  []int{cluster.ID},
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

	// Check for clusters with high internal similarity but contradictory sources
	for _, cluster := range clusters {
		sources := make(map[string]int)
		for _, vecID := range cluster.VectorIDs {
			// Find metadata for this vector
			for _, meta := range metadata {
				if meta.ID == vecID {
					sources[meta.Source]++
					break
				}
			}
		}

		// If cluster has high coherence but multiple distinct sources, it's potentially contradictory
		if cluster.Coherence > 0.8 && len(sources) > 3 {
			sourceList := ""
			for src := range sources {
				sourceList += src + ", "
			}

			contradictions = append(contradictions, models.Finding{
				Type:      "source_contradiction",
				Subject:   cluster.Label,
				IsAnomaly: true,
				Clusters:  []int{cluster.ID},
				ReasoningChain: "High coherence with diverse sources suggests consensus across domains: " + sourceList +
					"This may indicate convergent truth or a widely-propagated narrative.",
			})
		}
	}

	return contradictions
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
