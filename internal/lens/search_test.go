package lens

import (
	"testing"

	"github.com/meistro57/vectoreologist/internal/synthesis"
)

func TestRunSearch_FindsAcrossEntities(t *testing.T) {
	t.Parallel()

	m := Model{
		report: &synthesis.JSONReport{
			Clusters: []synthesis.JSONCluster{{ID: 1, Label: "Sceptical epistemology", Reasoning: "A robust ontology cluster."}},
			Bridges:  []synthesis.JSONBridge{{ClusterA: 1, ClusterB: 2, LinkType: "semantic", Reasoning: "Shared ontology keywords."}},
			Anomalies: []synthesis.JSONAnomaly{{
				Type:           "coherence_anomaly",
				Subject:        "Cluster 2: Chaotic notes",
				ReasoningChain: "Low semantic consistency in ontology terms.",
			}},
		},
	}

	m.searchQuery = "ontology"
	m.runSearch()

	if len(m.searchResults) != 3 {
		t.Fatalf("search results count = %d, want 3; results=%+v", len(m.searchResults), m.searchResults)
	}

	wantKinds := map[string]int{"cluster": 1, "bridge": 1, "anomaly": 1}
	for _, res := range m.searchResults {
		wantKinds[res.kind]--
	}
	for kind, remaining := range wantKinds {
		if remaining != 0 {
			t.Fatalf("kind %q mismatch remaining=%d results=%+v", kind, remaining, m.searchResults)
		}
	}
}
