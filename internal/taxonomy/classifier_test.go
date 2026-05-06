package taxonomy

import (
	"testing"

	"github.com/meistro57/vectoreologist/internal/models"
)

// ---- helpers ---------------------------------------------------------------

func classify(frags []string, label string) *models.TaxonomyLabel {
	return New().Classify(frags, label)
}

// ---- Mode classification ----------------------------------------------------

func TestClassify_ModeScholarlyAnnotation(t *testing.T) {
	frags := []string{
		"According to Smith et al (2019), the primary mechanism involves...",
		"As cited in the peer-reviewed literature, these findings suggest...",
		"The study by Jones et al found that consciousness is correlated with...",
	}
	lbl := classify(frags, "surface / unknown")
	if lbl.Mode != ModeScholarlyAnnotation {
		t.Errorf("mode = %q, want %q", lbl.Mode, ModeScholarlyAnnotation)
	}
}

func TestClassify_ModeMetaDescriptiveSummary(t *testing.T) {
	frags := []string{
		"This section provides an overview of the key concepts discussed below.",
		"In summary, this chapter examines the relationship between mind and matter.",
		"The following text presents an introduction to the topic of consciousness.",
	}
	lbl := classify(frags, "didactic / General Content")
	if lbl.Mode != ModeMetaDescriptiveSummary {
		t.Errorf("mode = %q, want %q", lbl.Mode, ModeMetaDescriptiveSummary)
	}
}

func TestClassify_ModeDidacticTeaching(t *testing.T) {
	frags := []string{
		"In other words, the concept of emergence refers to properties arising from complexity.",
		"Note that this means the whole is greater than the sum of its parts.",
		"To understand this, remember that entropy always increases in a closed system.",
	}
	lbl := classify(frags, "surface / unknown")
	if lbl.Mode != ModeDidacticTeaching {
		t.Errorf("mode = %q, want %q", lbl.Mode, ModeDidacticTeaching)
	}
}

func TestClassify_ModeTransformationalDialogue(t *testing.T) {
	frags := []string{
		"Q: What is the nature of consciousness?",
		"A: Consciousness arises from the integration of information.",
		"The interviewer asked about the hard problem; the respondent replied at length.",
	}
	lbl := classify(frags, "surface / unknown")
	if lbl.Mode != ModeTransformationalDialogue {
		t.Errorf("mode = %q, want %q", lbl.Mode, ModeTransformationalDialogue)
	}
}

func TestClassify_ModeFunctionalDefinition(t *testing.T) {
	frags := []string{
		"We define X := the set of all coherent states satisfying the constraint.",
		"Formally, let S be a Hilbert space with inner product defined as follows.",
		"By definition, a manifold is a topological space that is locally Euclidean.",
	}
	lbl := classify(frags, "surface / unknown")
	if lbl.Mode != ModeFunctionalDefinition {
		t.Errorf("mode = %q, want %q", lbl.Mode, ModeFunctionalDefinition)
	}
}

// ---- Epistemic posture classification ----------------------------------------

func TestClassify_PostureConditionalRevelation(t *testing.T) {
	frags := []string{
		"If consciousness arises from quantum processes, then we must reconsider the measurement problem.",
		"Assuming that the Copenhagen interpretation holds, then wave function collapse is observer-dependent.",
		"Were we to accept panpsychism, it follows that all matter has some form of experience.",
	}
	lbl := classify(frags, "surface / unknown")
	if lbl.EpistemicPosture != PostureConditionalRevelation {
		t.Errorf("posture = %q, want %q", lbl.EpistemicPosture, PostureConditionalRevelation)
	}
}

func TestClassify_PostureExternallyReferenced(t *testing.T) {
	frags := []string{
		"According to the literature, this phenomenon is well-documented.",
		"Research indicates a strong correlation between meditation and neural coherence.",
		"Studies show that the default mode network is activated during self-referential thought.",
	}
	lbl := classify(frags, "surface / unknown")
	if lbl.EpistemicPosture != PostureExternallyReferenced {
		t.Errorf("posture = %q, want %q", lbl.EpistemicPosture, PostureExternallyReferenced)
	}
}

func TestClassify_PostureExperientialReframing(t *testing.T) {
	frags := []string{
		"In my experience, this approach consistently produces deeper insights.",
		"From my perspective, the standard model is insufficient to explain consciousness.",
		"Personally, I find that reframing the question reveals hidden assumptions.",
	}
	lbl := classify(frags, "surface / unknown")
	if lbl.EpistemicPosture != PostureExperientialReframing {
		t.Errorf("posture = %q, want %q", lbl.EpistemicPosture, PostureExperientialReframing)
	}
}

// ---- Topic classification ---------------------------------------------------

func TestClassify_TopicConsciousnessPhilosophy(t *testing.T) {
	frags := []string{
		"The hard problem of consciousness concerns the explanatory gap between neural activity and qualia.",
		"Subjective experience cannot be fully reduced to third-person descriptions of brain states.",
		"Phenomenal consciousness refers to the 'what it is like' aspect of mental states.",
	}
	lbl := classify(frags, "other / general")
	if lbl.Topic != "consciousness_philosophy" {
		t.Errorf("topic = %q, want consciousness_philosophy", lbl.Topic)
	}
}

func TestClassify_TopicQuantumMechanics(t *testing.T) {
	frags := []string{
		"Quantum entanglement allows instantaneous correlation between distant particles.",
		"The Schrödinger equation describes the evolution of a quantum wave function over time.",
		"Superposition states collapse upon measurement according to the Copenhagen interpretation.",
	}
	lbl := classify(frags, "other / general")
	if lbl.Topic != "quantum_mechanics" {
		t.Errorf("topic = %q, want quantum_mechanics", lbl.Topic)
	}
}

// ---- Failing example corrections (acceptance criteria) ----------------------

// Example 1: cluster mislabeled "other / quantum_mechanics" but content is about consciousness.
func TestClassify_FailingExample1_QuantumMislabel(t *testing.T) {
	frags := []string{
		"Consciousness cannot be explained purely in terms of physical processes.",
		"The subjective quality of experience — qualia — resists objective description.",
		"Awareness of the self is a hallmark of phenomenal consciousness.",
		"Sentience may be a fundamental property of the universe, not an emergent one.",
	}
	lbl := classify(frags, "other / quantum_mechanics")
	// Topic should be consciousness, NOT quantum.
	if lbl.Topic == "quantum_mechanics" {
		t.Error("topic should not be quantum_mechanics for consciousness content")
	}
	if lbl.Topic != "consciousness_philosophy" {
		t.Errorf("topic = %q, want consciousness_philosophy", lbl.Topic)
	}
	// A label warning should be set because the label claims "quantum".
	if lbl.LabelWarning == "" {
		t.Error("LabelWarning should be set when label says quantum but content is consciousness")
	}
}

// Example 2: "didactic / General Content" should be refined to scholarly_annotation mode.
func TestClassify_FailingExample2_DidacticJunkDrawer(t *testing.T) {
	frags := []string{
		"As noted by Smith et al (2022), consciousness is a fundamental puzzle.",
		"According to the literature, several competing theories exist.",
		"The peer-reviewed study by Jones found correlates in the prefrontal cortex.",
	}
	lbl := classify(frags, "didactic / General Content")
	if lbl.Mode != ModeScholarlyAnnotation {
		t.Errorf("mode = %q, want scholarly_annotation (not didactic junk drawer)", lbl.Mode)
	}
	// The label warning should flag the mode mismatch.
	if lbl.LabelWarning == "" {
		t.Error("LabelWarning should be set when label says 'didactic' but mode is scholarly_annotation")
	}
}

// Example 3: conditional philosophy text labeled "surface / unknown" gets posture classified.
func TestClassify_FailingExample3_ConditionalPhilosophy(t *testing.T) {
	frags := []string{
		"If consciousness arises from quantum processes, then the observer problem is fundamental.",
		"Assuming the Copenhagen interpretation holds, then wave function collapse requires sentience.",
		"Were we to accept this premise, it follows that physics is incomplete without mind.",
	}
	lbl := classify(frags, "surface / unknown")
	if lbl.EpistemicPosture != PostureConditionalRevelation {
		t.Errorf("posture = %q, want conditional_revelation", lbl.EpistemicPosture)
	}
	// No label warning needed for "surface / unknown" since it's a generic fallback.
}

// ---- Edge cases -------------------------------------------------------------

func TestClassify_EmptyFragments(t *testing.T) {
	lbl := classify(nil, "surface / unknown")
	if lbl == nil {
		t.Fatal("nil TaxonomyLabel for empty input")
	}
	if lbl.Mode != ModeUnknown {
		t.Errorf("mode = %q, want unknown for empty input", lbl.Mode)
	}
}

func TestClassify_ConfidenceRange(t *testing.T) {
	frags := []string{
		"According to Smith et al (2019), consciousness is correlated with neural binding.",
	}
	lbl := classify(frags, "surface / unknown")
	if lbl.Confidence < 0 || lbl.Confidence > 1 {
		t.Errorf("confidence %.3f outside [0,1]", lbl.Confidence)
	}
}

func TestClassify_SemanticConceptPreserved(t *testing.T) {
	existing := "surface / The Kybalion"
	lbl := classify([]string{"The principle of mentalism states that all is mind."}, existing)
	if lbl.SemanticConcept != existing {
		t.Errorf("SemanticConcept = %q, want %q", lbl.SemanticConcept, existing)
	}
}

// ---- ClassifyClusters -------------------------------------------------------

func TestClassifyClusters_SetsOnAllClusters(t *testing.T) {
	clusters := []models.Cluster{
		{ID: 1, Label: "surface / unknown", VectorIDs: []uint64{1, 2}},
		{ID: 2, Label: "surface / unknown", VectorIDs: []uint64{3}},
	}
	meta := []models.VectorMetadata{
		{ID: 1, Fragment: "According to the literature, consciousness is complex."},
		{ID: 2, Fragment: "Studies show that qualia resist physical explanation."},
		{ID: 3, Fragment: "In summary, this chapter explores quantum mechanics."},
	}
	result := New().ClassifyClusters(clusters, meta)
	for _, c := range result {
		if c.Taxonomy == nil {
			t.Errorf("cluster %d: Taxonomy should not be nil after ClassifyClusters", c.ID)
		}
	}
}

func TestClassifyClusters_OriginalNotMutated(t *testing.T) {
	clusters := []models.Cluster{
		{ID: 1, Label: "original", VectorIDs: []uint64{1}},
	}
	meta := []models.VectorMetadata{
		{ID: 1, Fragment: "some text"},
	}
	_ = New().ClassifyClusters(clusters, meta)
	if clusters[0].Taxonomy != nil {
		t.Error("ClassifyClusters should not mutate the input slice")
	}
}
