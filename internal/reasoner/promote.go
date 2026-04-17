package reasoner

import (
	"fmt"
	"strings"

	"github.com/meistro57/vectoreologist/internal/models"
)

// ExtractConclusionLabel parses the first sentence from a reasoning chain's
// Conclusion section, strips markdown formatting, and caps at ~80 chars.
// Works on both R1 format ("**Thinking:**\n...\n\n**Conclusion:**\n...") and
// plain chat format (raw conclusion text).
func ExtractConclusionLabel(reasoningChain string) string {
	text := reasoningChain
	if idx := strings.Index(text, "**Conclusion:**"); idx >= 0 {
		text = text[idx+len("**Conclusion:**"):]
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	// First sentence only.
	if idx := strings.IndexAny(text, ".!?"); idx >= 0 {
		text = text[:idx+1]
	}
	text = stripMarkdown(text)
	text = strings.TrimSpace(text)
	if len(text) > 80 {
		text = text[:77] + "..."
	}
	return text
}

func stripMarkdown(s string) string {
	for _, tok := range []string{"**", "__", "*", "_", "`"} {
		s = strings.ReplaceAll(s, tok, "")
	}
	return s
}

// PromoteClusterLabels promotes each cluster's Label to the first sentence of
// its R1 Conclusion finding, moving the old source-based label into Source.
// Clusters with no matching finding are returned unchanged.
func PromoteClusterLabels(findings []models.Finding, clusters []models.Cluster) []models.Cluster {
	reasoning := make(map[int]string, len(findings))
	for _, f := range findings {
		if f.Type != "cluster_analysis" {
			continue
		}
		var id int
		fmt.Sscanf(f.Subject, "Cluster %d", &id)
		if id > 0 {
			reasoning[id] = f.ReasoningChain
		}
	}

	out := make([]models.Cluster, len(clusters))
	copy(out, clusters)
	for i, c := range out {
		chain, ok := reasoning[c.ID]
		if !ok {
			continue
		}
		label := ExtractConclusionLabel(chain)
		if label == "" {
			continue
		}
		out[i].Source = c.Label
		out[i].Label = label
	}
	return out
}

// PromoteBridgeLabels sets a short semantic label on each bridge from its
// finding's Conclusion. Bridges with no matching finding are unchanged.
func PromoteBridgeLabels(findings []models.Finding, bridges []models.Bridge) []models.Bridge {
	type pair struct{ a, b int }
	reasoning := make(map[pair]string)
	for _, f := range findings {
		if f.Type != "bridge_analysis" {
			continue
		}
		idx := strings.Index(f.Subject, ": ")
		if idx < 0 {
			continue
		}
		rest := f.Subject[idx+2:]
		sidx := strings.Index(rest, "↔")
		if sidx < 0 {
			continue
		}
		var a, b int
		fmt.Sscanf(strings.TrimSpace(rest[:sidx]), "%d", &a)
		fmt.Sscanf(strings.TrimSpace(rest[sidx+len("↔"):]), "%d", &b)
		if a > 0 {
			label := ExtractConclusionLabel(f.ReasoningChain)
			reasoning[pair{a, b}] = label
			reasoning[pair{b, a}] = label
		}
	}

	out := make([]models.Bridge, len(bridges))
	copy(out, bridges)
	for i, b := range out {
		if label := reasoning[pair{b.ClusterA, b.ClusterB}]; label != "" {
			out[i].Label = label
		}
	}
	return out
}
