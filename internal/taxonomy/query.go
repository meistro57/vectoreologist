package taxonomy

import (
	"strings"

	"github.com/meistro57/vectoreologist/internal/models"
)

// Query defines filter criteria for taxonomy-aware cluster searches.
// Empty strings match any value; LabelMismatch=true restricts to clusters
// with a non-empty LabelWarning.
type Query struct {
	Topic            string
	Mode             string
	EpistemicPosture string
	LabelMismatch    bool
}

// MatchesCluster reports whether the given cluster satisfies all non-zero
// criteria in q.
func (q Query) MatchesCluster(c models.Cluster) bool {
	t := c.Taxonomy
	if t == nil {
		// An unclassified cluster only matches an empty query.
		return q.Topic == "" && q.Mode == "" && q.EpistemicPosture == "" && !q.LabelMismatch
	}
	if q.Topic != "" && !strings.EqualFold(t.Topic, q.Topic) {
		return false
	}
	if q.Mode != "" && !strings.EqualFold(t.Mode, q.Mode) {
		return false
	}
	if q.EpistemicPosture != "" && !strings.EqualFold(t.EpistemicPosture, q.EpistemicPosture) {
		return false
	}
	if q.LabelMismatch && t.LabelWarning == "" {
		return false
	}
	return true
}

// FilterClusters returns the subset of clusters that satisfy q.
func FilterClusters(clusters []models.Cluster, q Query) []models.Cluster {
	out := make([]models.Cluster, 0)
	for _, c := range clusters {
		if q.MatchesCluster(c) {
			out = append(out, c)
		}
	}
	return out
}
