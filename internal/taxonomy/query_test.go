package taxonomy

import (
	"testing"

	"github.com/meistro57/vectoreologist/internal/models"
)

func clusterWithTaxonomy(id int, label, topic, mode, posture, warning string) models.Cluster {
	return models.Cluster{
		ID:    id,
		Label: label,
		Taxonomy: &models.TaxonomyLabel{
			Topic:            topic,
			Mode:             mode,
			EpistemicPosture: posture,
			LabelWarning:     warning,
		},
	}
}

var testCorpus = []models.Cluster{
	clusterWithTaxonomy(1, "Quantum concepts", "quantum_mechanics", ModeScholarlyAnnotation, PostureDoctrinalAssertion, ""),
	clusterWithTaxonomy(2, "Mind and self", "consciousness_philosophy", ModeScholarlyAnnotation, PostureExternallyReferenced, ""),
	clusterWithTaxonomy(3, "Mislabeled quantum", "consciousness_philosophy", ModeDidacticTeaching, PostureConditionalRevelation, "label claims 'quantum' but content suggests consciousness_philosophy"),
	clusterWithTaxonomy(4, "Dialogue session", "general", ModeTransformationalDialogue, PostureExperientialReframing, ""),
	clusterWithTaxonomy(5, "Formal proofs", "mathematics", ModeFunctionalDefinition, PostureDoctrinalAssertion, ""),
}

// "Find all doctrinal assertions about consciousness"
func TestQuery_DoctrinalConsciousness(t *testing.T) {
	q := Query{Topic: "consciousness_philosophy", EpistemicPosture: PostureDoctrinalAssertion}
	got := FilterClusters(testCorpus, q)
	// None in corpus have both consciousness + doctrinal.
	if len(got) != 0 {
		t.Errorf("want 0 results, got %d", len(got))
	}
}

// "Find all meta-descriptive clusters regardless of topic"
func TestQuery_AllScholarlyAnnotation(t *testing.T) {
	q := Query{Mode: ModeScholarlyAnnotation}
	got := FilterClusters(testCorpus, q)
	if len(got) != 2 {
		t.Errorf("want 2 scholarly_annotation clusters, got %d", len(got))
	}
}

// "Find clusters where label and content disagree"
func TestQuery_LabelMismatchOnly(t *testing.T) {
	q := Query{LabelMismatch: true}
	got := FilterClusters(testCorpus, q)
	if len(got) != 1 {
		t.Fatalf("want 1 mismatch cluster, got %d", len(got))
	}
	if got[0].ID != 3 {
		t.Errorf("expected cluster 3 (mislabeled quantum), got cluster %d", got[0].ID)
	}
}

// AND combination: consciousness + mismatch
func TestQuery_ConsciousnessAndMismatch(t *testing.T) {
	q := Query{Topic: "consciousness_philosophy", LabelMismatch: true}
	got := FilterClusters(testCorpus, q)
	if len(got) != 1 {
		t.Fatalf("want 1 result, got %d", len(got))
	}
	if got[0].ID != 3 {
		t.Errorf("expected cluster 3, got %d", got[0].ID)
	}
}

func TestQuery_EmptyQuery_MatchesAll(t *testing.T) {
	q := Query{}
	got := FilterClusters(testCorpus, q)
	if len(got) != len(testCorpus) {
		t.Errorf("empty query should match all %d clusters, got %d", len(testCorpus), len(got))
	}
}

func TestQuery_CaseInsensitiveTopicMatch(t *testing.T) {
	q := Query{Topic: "QUANTUM_MECHANICS"}
	got := FilterClusters(testCorpus, q)
	if len(got) != 1 {
		t.Errorf("want 1 quantum cluster, got %d", len(got))
	}
}

func TestQuery_NilTaxonomyCluster_EmptyQueryMatches(t *testing.T) {
	c := models.Cluster{ID: 99, Label: "unclassified"}
	q := Query{}
	if !q.MatchesCluster(c) {
		t.Error("empty query should match unclassified cluster")
	}
}

func TestQuery_NilTaxonomyCluster_NonEmptyQueryNoMatch(t *testing.T) {
	c := models.Cluster{ID: 99, Label: "unclassified"}
	q := Query{Topic: "quantum_mechanics"}
	if q.MatchesCluster(c) {
		t.Error("topic query should not match cluster with nil taxonomy")
	}
}

func TestFilterClusters_EmptyInput(t *testing.T) {
	got := FilterClusters(nil, Query{Topic: "quantum_mechanics"})
	if len(got) != 0 {
		t.Errorf("want empty result for nil input, got %d", len(got))
	}
}
