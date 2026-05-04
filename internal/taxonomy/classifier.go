package taxonomy

import (
	"strings"

	"github.com/meistro57/vectoreologist/internal/models"
)

// signal is a (phrase, weight) pair used for keyword scoring.
type signal struct {
	phrase string
	weight float64
}

// modeSignals maps each Mode constant to its diagnostic signals.
// Longer / more specific phrases carry higher weight.
var modeSignals = map[string][]signal{
	ModeDidacticTeaching: {
		{"in other words", 2.0},
		{"that is to say", 2.0},
		{"note that", 1.5},
		{"remember that", 1.5},
		{"the key concept", 2.0},
		{"to understand", 1.5},
		{"this means that", 2.0},
		{"we say that", 2.0},
		{"is called", 1.0},
		{"refers to", 1.0},
		{"pedagogical", 3.0},
		{"tutorial", 2.0},
		{"lesson", 1.5},
		{"teaches", 1.5},
		{"understand", 0.5},
		{"i.e.", 1.0},
		{"for example", 0.8},
		{"such as", 0.5},
	},
	ModeMetaDescriptiveSummary: {
		{"this section", 2.0},
		{"this chapter", 2.0},
		{"this document", 2.0},
		{"this paper", 2.0},
		{"this text", 2.0},
		{"this article", 2.0},
		{"the following", 1.5},
		{"an overview", 2.5},
		{"in summary", 2.5},
		{"to summarize", 2.5},
		{"in conclusion", 2.0},
		{"introduction to", 2.0},
		{"presents", 1.0},
		{"discusses", 1.0},
		{"explores", 1.0},
		{"examines", 1.0},
		{"outlines", 1.5},
		{"overview", 1.5},
		{"abstract", 1.5},
		{"summarizes", 2.0},
		{"describes", 0.8},
	},
	ModeScholarlyAnnotation: {
		{"et al", 3.0},
		{"ibid", 3.0},
		{"as noted by", 2.5},
		{"according to", 2.0},
		{"as cited", 2.5},
		{"see also", 1.5},
		{"cf.", 2.0},
		{"op. cit.", 3.0},
		{"bibliography", 2.0},
		{"references", 1.5},
		{"literature", 1.5},
		{"cited in", 2.0},
		{"scholars", 1.5},
		{"academic", 1.0},
		{"peer-reviewed", 3.0},
		{"proceedings", 2.0},
		{"journal", 1.5},
		{"footnote", 2.0},
		{"endnote", 2.0},
		{"[1]", 1.5},
		{"[2]", 1.5},
		{"(2019)", 1.0},
		{"(2020)", 1.0},
		{"(2021)", 1.0},
		{"(2022)", 1.0},
		{"(2023)", 1.0},
		{"(2024)", 1.0},
	},
	ModeTransformationalDialogue: {
		{"q:", 3.0},
		{"a:", 3.0},
		{"question:", 2.5},
		{"answer:", 2.5},
		{"asked", 1.5},
		{"replied", 1.5},
		{"responded", 1.5},
		{"said to", 1.5},
		{"dialogue", 2.0},
		{"conversation", 2.0},
		{"you asked", 2.5},
		{"i asked", 2.5},
		{"interview", 2.0},
		{"exchange", 1.0},
		{"interlocutor", 3.0},
		{"inquired", 2.0},
	},
	ModeFunctionalDefinition: {
		{"is defined as", 3.0},
		{"we define", 2.5},
		{"by definition", 2.5},
		{"let x be", 2.0},
		{"let us define", 2.5},
		{"denoted by", 2.0},
		{"denote", 1.5},
		{"formally,", 2.5},
		{"definition:", 3.0},
		{"def.", 2.0},
		{":=", 2.5},
		{"denotes", 1.5},
		{"we denote", 2.5},
		{"where x", 1.5},
		{"where the", 0.5},
		{"notation", 1.5},
		{"formula", 1.0},
	},
}

// postureSignals maps each EpistemicPosture constant to its diagnostic signals.
var postureSignals = map[string][]signal{
	PostureDoctrinalAssertion: {
		{"it is established", 3.0},
		{"it is proven", 3.0},
		{"it is true that", 2.5},
		{"the truth is", 2.5},
		{"it is known", 2.5},
		{"undeniably", 3.0},
		{"certainly", 1.5},
		{"must be", 1.5},
		{"always", 1.0},
		{"law of", 2.0},
		{"axiom", 2.5},
		{"principle of", 1.5},
		{"proven", 2.0},
		{"established", 1.5},
		{"fact", 1.0},
		{"necessarily", 1.5},
	},
	PostureDescriptiveAbstract: {
		{"typically", 2.0},
		{"generally", 2.0},
		{"tends to", 2.0},
		{"often", 1.5},
		{"usually", 1.5},
		{"may be", 1.5},
		{"can be", 1.0},
		{"appears to", 2.0},
		{"seems to", 2.0},
		{"is thought to", 2.5},
		{"is believed to", 2.5},
		{"might be", 1.5},
		{"could be", 1.5},
		{"in many cases", 2.0},
		{"in general", 2.0},
		{"for the most part", 2.0},
	},
	PostureExternallyReferenced: {
		{"according to", 2.5},
		{"as stated by", 2.5},
		{"as cited in", 2.5},
		{"studies show", 2.5},
		{"research indicates", 2.5},
		{"the literature suggests", 3.0},
		{"evidence suggests", 2.5},
		{"findings show", 2.5},
		{"data shows", 2.5},
		{"surveys indicate", 2.5},
		{"reports indicate", 2.0},
		{"as per", 1.5},
		{"sources say", 2.0},
		{"experts say", 2.0},
		{"research shows", 2.5},
	},
	PostureExperientialReframing: {
		{"in my experience", 3.0},
		{"from my perspective", 3.0},
		{"in my view", 2.5},
		{"i have found", 2.5},
		{"i noticed", 2.0},
		{"i believe", 1.5},
		{"personally,", 2.5},
		{"looking at this differently", 3.0},
		{"from another angle", 2.5},
		{"reframing", 3.0},
		{"reconsidering", 2.0},
		{"in my own", 2.0},
		{"my understanding", 2.5},
		{"subjectively", 2.5},
		{"experientially", 3.0},
		{"i find that", 2.0},
	},
	PostureConditionalRevelation: {
		{"if we assume", 3.0},
		{"assuming that", 2.5},
		{"given that", 2.0},
		{"provided that", 2.5},
		{"under the condition", 3.0},
		{"in the case where", 2.5},
		{"were we to", 2.5},
		{"suppose that", 2.5},
		{"imagine that", 2.0},
		{"only if", 2.0},
		{"unless", 1.5},
		{"if and only if", 3.0},
		{"if x then", 2.5},
		{"if consciousness", 2.5},
		{"if quantum", 2.5},
		{"then we", 1.5},
		{"it follows that", 2.0},
	},
}

// topicDomains maps topic names to their keyword signals.
var topicDomains = map[string][]signal{
	"consciousness_philosophy": {
		{"consciousness", 3.0},
		{"qualia", 4.0},
		{"subjective experience", 3.5},
		{"phenomenal", 3.0},
		{"sentience", 3.0},
		{"self-awareness", 3.0},
		{"awareness", 2.0},
		{"hard problem", 3.5},
		{"mind-body", 3.0},
		{"intentionality", 3.0},
		{"phenomenology", 3.5},
		{"perception", 1.5},
		{"conscious", 2.0},
	},
	"quantum_mechanics": {
		{"quantum", 3.0},
		{"entanglement", 4.0},
		{"superposition", 4.0},
		{"wave function", 4.0},
		{"photon", 3.0},
		{"electron", 2.5},
		{"planck", 3.5},
		{"uncertainty principle", 4.0},
		{"bohr", 3.0},
		{"schrodinger", 4.0},
		{"heisenberg", 3.5},
		{"quantum mechanics", 4.0},
		{"quantum physics", 4.0},
		{"wave-particle", 4.0},
	},
	"mathematics": {
		{"theorem", 3.0},
		{"proof", 2.5},
		{"equation", 2.5},
		{"formula", 2.0},
		{"matrix", 2.5},
		{"calculus", 3.0},
		{"algebra", 3.0},
		{"derivative", 3.0},
		{"integral", 2.5},
		{"manifold", 3.5},
		{"topology", 3.0},
		{"eigenvalue", 4.0},
		{"polynomial", 3.0},
		{"convergence", 2.5},
	},
	"computer_science": {
		{"algorithm", 2.5},
		{"data structure", 3.5},
		{"machine learning", 3.5},
		{"neural network", 3.5},
		{"software", 2.0},
		{"programming", 2.5},
		{"database", 2.5},
		{"cpu", 3.0},
		{"compiler", 3.0},
		{"runtime", 2.5},
		{"recursion", 3.0},
		{"complexity", 2.0},
		{"binary", 2.0},
	},
	"philosophy": {
		{"epistemology", 4.0},
		{"ontology", 4.0},
		{"metaphysics", 4.0},
		{"ethics", 2.5},
		{"phenomenology", 3.5},
		{"dialectic", 3.5},
		{"hermeneutics", 4.0},
		{"teleology", 4.0},
		{"being", 1.0},
		{"truth", 1.0},
		{"knowledge", 1.0},
		{"rationalism", 3.5},
		{"empiricism", 3.5},
	},
	"biology": {
		{"cell", 2.0},
		{"organism", 2.5},
		{"dna", 3.5},
		{"protein", 3.0},
		{"evolution", 2.5},
		{"gene", 3.0},
		{"species", 2.0},
		{"metabolism", 3.5},
		{"neural", 2.5},
		{"neuron", 3.0},
		{"enzyme", 3.5},
		{"chromosome", 4.0},
		{"mitosis", 4.0},
	},
	"history": {
		{"century", 2.0},
		{"war", 1.5},
		{"empire", 2.5},
		{"civilization", 2.5},
		{"ancient", 2.0},
		{"medieval", 3.0},
		{"revolution", 2.0},
		{"dynasty", 3.0},
		{"historical", 2.5},
		{"circa", 3.0},
		{"b.c.", 3.0},
		{"a.d.", 2.5},
		{"archaeological", 3.5},
	},
	"theology": {
		{"god", 2.0},
		{"divine", 2.5},
		{"spiritual", 2.0},
		{"sacred", 2.5},
		{"prayer", 3.0},
		{"faith", 2.0},
		{"soul", 2.0},
		{"transcendent", 3.0},
		{"mystical", 3.0},
		{"salvation", 3.5},
		{"scripture", 3.5},
		{"theological", 3.5},
		{"heresy", 3.5},
	},
	"psychology": {
		{"behavior", 2.0},
		{"cognitive", 2.5},
		{"emotional", 2.0},
		{"anxiety", 2.5},
		{"depression", 2.5},
		{"personality", 2.5},
		{"unconscious", 3.0},
		{"freud", 3.5},
		{"jung", 3.5},
		{"trauma", 2.5},
		{"psychotherapy", 3.5},
		{"attachment", 2.5},
		{"cognitive bias", 3.5},
	},
	"linguistics": {
		{"grammar", 2.5},
		{"syntax", 3.0},
		{"semantics", 3.0},
		{"phonology", 4.0},
		{"morpheme", 4.0},
		{"phoneme", 4.0},
		{"discourse", 2.5},
		{"pragmatics", 3.5},
		{"lexicon", 3.0},
		{"etymology", 3.5},
		{"dialect", 3.0},
		{"language acquisition", 3.5},
	},
}

// scoreSignals counts weighted hits of signal phrases in lowercased combined text.
func scoreSignals(text string, signals []signal) float64 {
	var total float64
	for _, s := range signals {
		if strings.Contains(text, s.phrase) {
			total += s.weight
		}
	}
	return total
}

// scoreAll runs scoreSignals across a table and returns the winner, its score,
// and the confidence ratio (winner - runner_up) / (winner + 1e-9).
func scoreAll(text string, table map[string][]signal) (winner string, score, confidence float64) {
	best, second := 0.0, 0.0
	for candidate, sigs := range table {
		s := scoreSignals(text, sigs)
		if s > best {
			second = best
			best = s
			winner = candidate
		} else if s > second {
			second = s
		}
	}
	confidence = (best - second) / (best + 1e-9)
	return winner, best, confidence
}

// Classify assigns a TaxonomyLabel to a set of text fragments from a cluster.
// existingLabel is the current cluster label (layer/source or promoted R1 label)
// and is used only to populate SemanticConcept and to feed the label-repair check.
func (c *Classifier) Classify(fragments []string, existingLabel string) *models.TaxonomyLabel {
	combined := strings.ToLower(strings.Join(fragments, " "))
	if combined == "" {
		return &models.TaxonomyLabel{
			Mode:             ModeUnknown,
			EpistemicPosture: PostureUnknown,
			Topic:            "general",
			Confidence:       0.0,
			SemanticConcept:  existingLabel,
		}
	}

	mode, modeScore, modeConf := scoreAll(combined, modeSignals)
	if modeScore < 1.0 {
		mode = ModeUnknown
		modeConf = 0
	}

	posture, postureScore, postureConf := scoreAll(combined, postureSignals)
	if postureScore < 1.0 {
		posture = PostureUnknown
		postureConf = 0
	}

	topic, topicScore, topicConf := scoreAll(combined, topicDomains)
	if topicScore < 2.0 {
		topic = "general"
		topicConf = 0
	}

	// Overall confidence is the average of the three per-axis confidences.
	confidence := (modeConf + postureConf + topicConf) / 3.0

	sourceFamily := extractSourceFamily(existingLabel)

	lbl := &models.TaxonomyLabel{
		Mode:             mode,
		EpistemicPosture: posture,
		Topic:            topic,
		SourceFamily:     sourceFamily,
		Confidence:       confidence,
		SemanticConcept:  existingLabel,
	}
	lbl.LabelWarning = CheckLabelMismatch(lbl, existingLabel)
	return lbl
}

// extractSourceFamily pulls the source family from labels like "surface / Foo" → "Foo".
func extractSourceFamily(label string) string {
	if idx := strings.Index(label, " / "); idx >= 0 {
		return strings.TrimSpace(label[idx+3:])
	}
	return label
}

// ClassifyClusters runs the taxonomy classifier over all clusters and attaches
// a TaxonomyLabel to each one. Clusters without any fragment data get a
// zero-confidence label. Returns a new slice; originals are not mutated.
func (c *Classifier) ClassifyClusters(clusters []models.Cluster, metadata []models.VectorMetadata) []models.Cluster {
	byID := make(map[uint64]string, len(metadata))
	for _, m := range metadata {
		if m.Fragment != "" && m.Fragment != "N/A" {
			byID[m.ID] = m.Fragment
		}
	}

	out := make([]models.Cluster, len(clusters))
	copy(out, clusters)
	for i, cl := range out {
		frags := clusterFragments(cl, byID, 12)
		out[i].Taxonomy = c.Classify(frags, cl.Label)
	}
	return out
}

// clusterFragments collects up to n unique non-empty text fragments for a cluster.
func clusterFragments(cl models.Cluster, byID map[uint64]string, n int) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, n)
	for _, id := range cl.VectorIDs {
		if len(out) >= n {
			break
		}
		frag, ok := byID[id]
		if !ok || seen[frag] {
			continue
		}
		seen[frag] = true
		// Truncate very long fragments so scoring stays fast.
		if len(frag) > 300 {
			frag = frag[:300]
		}
		out = append(out, frag)
	}
	return out
}
