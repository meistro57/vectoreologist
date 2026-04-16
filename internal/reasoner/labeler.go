package reasoner

import (
	"fmt"
	"strings"

	"github.com/meistro57/vectoreologist/internal/models"
)

// LabelClusters uses DeepSeek to replace each cluster's Label with a
// semantically meaningful 3–6 word description derived from its text fragments.
// It always uses deepseek-chat (fast) regardless of the reasoner's configured model,
// since chain-of-thought adds no value for labeling.
// Returns a new slice; the originals are not modified.
func (r *Reasoner) LabelClusters(clusters []models.Cluster, metadata []models.VectorMetadata) []models.Cluster {
	byID := make(map[uint64]models.VectorMetadata, len(metadata))
	for _, m := range metadata {
		byID[m.ID] = m
	}

	// Always use deepseek-chat for this task.
	labeler := &Reasoner{
		apiURL: r.apiURL,
		apiKey: r.apiKey,
		model:  "deepseek-chat",
		client: r.client,
	}

	labeled := make([]models.Cluster, len(clusters))
	copy(labeled, clusters)

	for i, c := range labeled {
		fmt.Printf("\r   labeling %d/%d: cluster #%d …          ", i+1, len(clusters), c.ID)
		if label := labeler.labelOneCluster(c, byID); label != "" {
			labeled[i].Label = label
		}
	}
	fmt.Printf("\r   ✓ semantic labels applied (%d clusters)                    \n", len(clusters))
	return labeled
}

func (r *Reasoner) labelOneCluster(c models.Cluster, byID map[uint64]models.VectorMetadata) string {
	const maxSamples = 8
	var samples []string
	for _, vid := range c.VectorIDs {
		if m, ok := byID[vid]; ok && m.Fragment != "" && m.Fragment != "N/A" {
			samples = append(samples, m.Fragment)
			if len(samples) >= maxSamples {
				break
			}
		}
	}
	if len(samples) == 0 {
		return ""
	}

	resp, err := r.callDeepSeek(buildLabelPrompt(c, samples))
	if err != nil {
		return ""
	}

	label := strings.TrimSpace(resp.conclusion)
	label = strings.Trim(label, "\"'`")
	// Take only the first line in case the model adds explanation.
	if idx := strings.IndexByte(label, '\n'); idx >= 0 {
		label = strings.TrimSpace(label[:idx])
	}
	return label
}

func buildLabelPrompt(c models.Cluster, samples []string) string {
	return fmt.Sprintf(`You are labeling clusters in a knowledge archaeology system.

Cluster: %d vectors, coherence %.2f.

Sample content:
%s

Respond with ONLY a concise 3–6 word semantic label. No explanation. No quotes.`,
		c.Size, c.Coherence, strings.Join(samples, "\n---\n"))
}
