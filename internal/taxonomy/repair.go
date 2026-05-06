package taxonomy

import (
	"fmt"
	"strings"

	"github.com/meistro57/vectoreologist/internal/models"
)

// knownTopicKeywords maps topic domain names to representative words found in labels.
// Used to detect when a label's topic word contradicts the classifier's topic.
var knownTopicKeywords = map[string][]string{
	"quantum_mechanics":        {"quantum", "entanglement", "superposition", "bohr", "schrodinger", "heisenberg", "planck"},
	"consciousness_philosophy": {"consciousness", "qualia", "phenomenal", "sentience", "mind", "awareness"},
	"mathematics":              {"math", "theorem", "algebra", "calculus", "geometry", "topology"},
	"computer_science":         {"computer", "algorithm", "software", "code", "neural", "machine learning"},
	"philosophy":               {"philosophy", "epistemology", "ontology", "metaphysics", "ethics"},
	"biology":                  {"biology", "cell", "dna", "evolution", "organism", "gene"},
	"history":                  {"history", "historical", "ancient", "medieval", "empire", "century"},
	"theology":                 {"theology", "god", "divine", "spiritual", "sacred", "religion"},
	"psychology":               {"psychology", "cognitive", "behavior", "freud", "jung", "therapy"},
	"linguistics":              {"linguistics", "grammar", "syntax", "semantics", "phonology", "morpheme"},
}

// CheckLabelMismatch compares the classifier's TaxonomyLabel against the existing
// cluster label string and returns a non-empty warning when it detects a high-
// confidence topic or mode disagreement. Returns "" when no mismatch is found.
func CheckLabelMismatch(lbl *models.TaxonomyLabel, existingLabel string) string {
	if lbl == nil {
		return ""
	}

	lower := strings.ToLower(existingLabel)

	// Check mode mismatch first (applies even when topic is "general").
	if strings.Contains(lower, "didactic") && lbl.Mode != ModeDidacticTeaching && lbl.Mode != ModeUnknown {
		return fmt.Sprintf("label mode 'didactic' conflicts with detected mode '%s'", lbl.Mode)
	}

	// Topic checks require a specific topic signal from the classifier.
	if lbl.Topic == "" || lbl.Topic == "general" {
		return ""
	}

	// No mismatch if the existing label already references the classifier's topic.
	if labelContainsTopic(lower, lbl.Topic) {
		return ""
	}

	// Mismatch if the existing label references a DIFFERENT known topic domain.
	for domain, keywords := range knownTopicKeywords {
		if domain == lbl.Topic {
			continue
		}
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				return fmt.Sprintf("label claims '%s' but content suggests '%s'", kw, lbl.Topic)
			}
		}
	}

	return ""
}

// labelContainsTopic checks whether the label string contains any keywords
// associated with the given topic domain name.
func labelContainsTopic(label, topic string) bool {
	// Direct substring match on the topic name.
	topicBase := strings.Split(topic, "_")[0]
	if strings.Contains(label, topicBase) {
		return true
	}
	// Check known keyword list for this topic.
	keywords, ok := knownTopicKeywords[topic]
	if !ok {
		return false
	}
	for _, kw := range keywords {
		if strings.Contains(label, kw) {
			return true
		}
	}
	return false
}
