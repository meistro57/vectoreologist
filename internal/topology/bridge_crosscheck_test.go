package topology

import (
	"testing"

	"github.com/meistro57/vectoreologist/internal/models"
)

func TestCrosscheckBridges_AllSupported(t *testing.T) {
	bridges := []models.Bridge{
		{
			ClusterA: 0, ClusterB: 1, Strength: 0.7,
			SampleLinks: []models.SampleLink{
				{ChunkAID: 1, ChunkBID: 2, Similarity: 0.65},
				{ChunkAID: 3, ChunkBID: 4, Similarity: 0.55},
			},
		},
		{
			ClusterA: 2, ClusterB: 3, Strength: 0.5,
			SampleLinks: []models.SampleLink{
				{ChunkAID: 5, ChunkBID: 6, Similarity: 0.52},
			},
		},
	}

	result := CrosscheckBridges(bridges, 0.5)

	if result.TotalSemanticBridges != 2 {
		t.Errorf("expected 2 semantic bridges, got %d", result.TotalSemanticBridges)
	}
	if result.TotalTopologicalLinks != 2 {
		t.Errorf("expected 2 topological links, got %d", result.TotalTopologicalLinks)
	}
	if result.OverlapRatio != 1.0 {
		t.Errorf("expected overlap ratio 1.0, got %f", result.OverlapRatio)
	}
	if len(result.SemanticOnlyBridges) != 0 {
		t.Errorf("expected 0 semantic-only bridges, got %d", len(result.SemanticOnlyBridges))
	}
}

func TestCrosscheckBridges_MixedSupport(t *testing.T) {
	bridges := []models.Bridge{
		{
			ClusterA: 0, ClusterB: 1, Strength: 0.7,
			SampleLinks: []models.SampleLink{
				{ChunkAID: 1, ChunkBID: 2, Similarity: 0.65},
			},
		},
		{
			ClusterA: 2, ClusterB: 3, Strength: 0.45,
			SampleLinks: []models.SampleLink{
				{ChunkAID: 5, ChunkBID: 6, Similarity: 0.35}, // below threshold
			},
		},
		{
			ClusterA: 4, ClusterB: 5, Strength: 0.38,
			SampleLinks: nil, // no links at all
		},
	}

	result := CrosscheckBridges(bridges, 0.5)

	if result.TotalSemanticBridges != 3 {
		t.Errorf("expected 3 semantic bridges, got %d", result.TotalSemanticBridges)
	}
	if result.TotalTopologicalLinks != 1 {
		t.Errorf("expected 1 topological link, got %d", result.TotalTopologicalLinks)
	}
	if len(result.SemanticOnlyBridges) != 2 {
		t.Errorf("expected 2 semantic-only bridges, got %d", len(result.SemanticOnlyBridges))
	}

	// Overlap should be 1/3
	expectedOverlap := 1.0 / 3.0
	if result.OverlapRatio < expectedOverlap-0.01 || result.OverlapRatio > expectedOverlap+0.01 {
		t.Errorf("expected overlap ratio ~%.3f, got %f", expectedOverlap, result.OverlapRatio)
	}
}

func TestCrosscheckBridges_Empty(t *testing.T) {
	result := CrosscheckBridges(nil, 0.5)

	if result.TotalSemanticBridges != 0 {
		t.Errorf("expected 0 semantic bridges, got %d", result.TotalSemanticBridges)
	}
	if result.OverlapRatio != 0 {
		t.Errorf("expected overlap ratio 0, got %f", result.OverlapRatio)
	}
}

func TestClassifyBridgeSupport(t *testing.T) {
	tests := []struct {
		name     string
		bridge   models.Bridge
		expected string
	}{
		{
			name:     "no links",
			bridge:   models.Bridge{SampleLinks: nil},
			expected: "no_chunk_evidence",
		},
		{
			name: "all below threshold",
			bridge: models.Bridge{SampleLinks: []models.SampleLink{
				{Similarity: 0.3}, {Similarity: 0.4},
			}},
			expected: "semantic_only",
		},
		{
			name: "one above threshold",
			bridge: models.Bridge{SampleLinks: []models.SampleLink{
				{Similarity: 0.55}, {Similarity: 0.3},
			}},
			expected: "weak_topological",
		},
		{
			name: "three above threshold",
			bridge: models.Bridge{SampleLinks: []models.SampleLink{
				{Similarity: 0.6}, {Similarity: 0.55}, {Similarity: 0.52},
			}},
			expected: "moderate_topological",
		},
		{
			name: "four above threshold",
			bridge: models.Bridge{SampleLinks: []models.SampleLink{
				{Similarity: 0.7}, {Similarity: 0.65}, {Similarity: 0.6}, {Similarity: 0.55},
			}},
			expected: "strong_topological",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyBridgeSupport(tt.bridge, 0.5)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}
