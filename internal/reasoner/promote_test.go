package reasoner

import (
	"strings"
	"testing"

	"github.com/meistro57/vectoreologist/internal/models"
)

// ---- ExtractConclusionLabel -------------------------------------------------

func TestExtractConclusionLabel_FromR1Format(t *testing.T) {
	chain := "**Thinking:**\nsome long reasoning here\n\n**Conclusion:**\nThis cluster represents quantum error correction methods."
	got := ExtractConclusionLabel(chain)
	if got != "This cluster represents quantum error correction methods." {
		t.Errorf("got %q", got)
	}
}

func TestExtractConclusionLabel_PlainText(t *testing.T) {
	chain := "This cluster is about ancient philosophy texts."
	got := ExtractConclusionLabel(chain)
	if got != "This cluster is about ancient philosophy texts." {
		t.Errorf("got %q", got)
	}
}

func TestExtractConclusionLabel_TakesFirstSentenceOnly(t *testing.T) {
	chain := "**Conclusion:**\nFirst sentence here. Second sentence follows."
	got := ExtractConclusionLabel(chain)
	if got != "First sentence here." {
		t.Errorf("got %q", got)
	}
}

func TestExtractConclusionLabel_StripMarkdown(t *testing.T) {
	chain := "**Conclusion:**\n**Bold** and *italic* text forms a concept."
	got := ExtractConclusionLabel(chain)
	if strings.Contains(got, "**") || strings.Contains(got, "*") {
		t.Errorf("markdown not stripped: %q", got)
	}
}

func TestExtractConclusionLabel_CapsAt80(t *testing.T) {
	long := strings.Repeat("x", 100)
	chain := "**Conclusion:**\n" + long + "."
	got := ExtractConclusionLabel(chain)
	if len(got) > 80 {
		t.Errorf("label too long: %d chars: %q", len(got), got)
	}
}

func TestExtractConclusionLabel_EmptyChain(t *testing.T) {
	if got := ExtractConclusionLabel(""); got != "" {
		t.Errorf("want empty, got %q", got)
	}
}

// ---- PromoteClusterLabels ---------------------------------------------------

func TestPromoteClusterLabels_ReplacesLabel(t *testing.T) {
	clusters := []models.Cluster{
		{ID: 1, Label: "surface / unknown"},
	}
	findings := []models.Finding{
		{
			Type:           "cluster_analysis",
			Subject:        "Cluster 1: surface / unknown",
			ReasoningChain: "**Conclusion:**\nThis cluster represents Stoic philosophy.",
		},
	}
	out := PromoteClusterLabels(findings, clusters)
	if out[0].Label != "This cluster represents Stoic philosophy." {
		t.Errorf("label: got %q", out[0].Label)
	}
	if out[0].Source != "surface / unknown" {
		t.Errorf("source: got %q", out[0].Source)
	}
}

func TestPromoteClusterLabels_OriginalsNotModified(t *testing.T) {
	clusters := []models.Cluster{
		{ID: 1, Label: "surface / original"},
	}
	findings := []models.Finding{
		{Type: "cluster_analysis", Subject: "Cluster 1: x", ReasoningChain: "**Conclusion:**\nNew label."},
	}
	_ = PromoteClusterLabels(findings, clusters)
	if clusters[0].Label != "surface / original" {
		t.Error("original slice was mutated")
	}
}

func TestPromoteClusterLabels_NoFindingUnchanged(t *testing.T) {
	clusters := []models.Cluster{
		{ID: 2, Label: "surface / keep"},
	}
	out := PromoteClusterLabels(nil, clusters)
	if out[0].Label != "surface / keep" || out[0].Source != "" {
		t.Errorf("no-finding cluster should be unchanged: label=%q source=%q", out[0].Label, out[0].Source)
	}
}

// ---- PromoteBridgeLabels ----------------------------------------------------

func TestPromoteBridgeLabels_SetsLabel(t *testing.T) {
	bridges := []models.Bridge{
		{ClusterA: 1, ClusterB: 3, Strength: 0.6},
	}
	findings := []models.Finding{
		{Type: "bridge_analysis", Subject: "Bridge: 1 ↔ 3", ReasoningChain: "**Conclusion:**\nShared metaphysical grounding."},
	}
	out := PromoteBridgeLabels(findings, bridges)
	if out[0].Label != "Shared metaphysical grounding." {
		t.Errorf("bridge label: got %q", out[0].Label)
	}
}

func TestPromoteBridgeLabels_OriginalsNotModified(t *testing.T) {
	bridges := []models.Bridge{
		{ClusterA: 1, ClusterB: 2},
	}
	findings := []models.Finding{
		{Type: "bridge_analysis", Subject: "Bridge: 1 ↔ 2", ReasoningChain: "**Conclusion:**\nSomething."},
	}
	_ = PromoteBridgeLabels(findings, bridges)
	if bridges[0].Label != "" {
		t.Error("original bridge slice was mutated")
	}
}
