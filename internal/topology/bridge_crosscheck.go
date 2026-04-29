package topology

import (
	"github.com/meistro57/vectoreologist/internal/models"
)

// BridgeCrosscheck compares semantic bridges (from centroid similarity) against
// topological links (from actual chunk-to-chunk nearest neighbors). Bridges that
// exist only as centroid-level similarity without supporting chunk-level links
// are flagged as "semantic-only" — the suspect set.
//
// This addresses the epistemic overreach risk: just because two cluster centroids
// are somewhat similar doesn't mean individual chunks actually bridge the domains.

// CrosscheckResult holds the comparison between semantic and topological bridges.
type CrosscheckResult struct {
	// TotalSemanticBridges is the count of bridges found via centroid similarity.
	TotalSemanticBridges int

	// TotalTopologicalLinks is the count of bridges with actual chunk-level support.
	TotalTopologicalLinks int

	// OverlapRatio is TopologicalLinks / SemanticBridges (0-1).
	// Low overlap means most bridges are centroid artifacts.
	OverlapRatio float64

	// SemanticOnlyBridges are bridges with no chunk-level support — the suspect set.
	SemanticOnlyBridges []models.Bridge

	// SupportedBridges have real cross-cluster chunk pairs backing them.
	SupportedBridges []models.Bridge
}

// CrosscheckBridges compares semantic bridges against topological evidence.
//
// A bridge is "topologically supported" if at least one of its SampleLinks has
// similarity above the support threshold. A bridge with no SampleLinks or all
// links below threshold is "semantic-only".
//
// Parameters:
//   - bridges: the bridges found by FindBridges (centroid-based)
//   - supportThreshold: minimum chunk-pair similarity to count as real support (e.g., 0.5)
func CrosscheckBridges(bridges []models.Bridge, supportThreshold float64) CrosscheckResult {
	result := CrosscheckResult{
		TotalSemanticBridges: len(bridges),
	}

	for _, b := range bridges {
		if hasTopologicalSupport(b, supportThreshold) {
			result.SupportedBridges = append(result.SupportedBridges, b)
		} else {
			result.SemanticOnlyBridges = append(result.SemanticOnlyBridges, b)
		}
	}

	result.TotalTopologicalLinks = len(result.SupportedBridges)

	if result.TotalSemanticBridges > 0 {
		result.OverlapRatio = float64(result.TotalTopologicalLinks) / float64(result.TotalSemanticBridges)
	}

	return result
}

// hasTopologicalSupport returns true if at least minLinks chunk-pairs exceed
// the similarity threshold.
func hasTopologicalSupport(b models.Bridge, threshold float64) bool {
	if len(b.SampleLinks) == 0 {
		return false
	}

	supported := 0
	for _, link := range b.SampleLinks {
		if link.Similarity >= threshold {
			supported++
		}
	}

	// Require at least 1 chunk-pair above threshold.
	return supported >= 1
}

// ClassifyBridgeSupport returns a human-readable support classification.
func ClassifyBridgeSupport(b models.Bridge, threshold float64) string {
	if len(b.SampleLinks) == 0 {
		return "no_chunk_evidence"
	}

	above := 0
	for _, link := range b.SampleLinks {
		if link.Similarity >= threshold {
			above++
		}
	}

	switch {
	case above == 0:
		return "semantic_only"
	case above == 1:
		return "weak_topological"
	case above <= 3:
		return "moderate_topological"
	default:
		return "strong_topological"
	}
}
