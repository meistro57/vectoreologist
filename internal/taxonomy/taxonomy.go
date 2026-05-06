package taxonomy

// Mode controlled vocabulary — what function does the text perform?
const (
	ModeDidacticTeaching         = "didactic_teaching"
	ModeMetaDescriptiveSummary   = "meta_descriptive_summary"
	ModeScholarlyAnnotation      = "scholarly_annotation"
	ModeTransformationalDialogue = "transformational_dialogue"
	ModeFunctionalDefinition     = "functional_definition"
	ModeUnknown                  = "unknown"
)

// EpistemicPosture controlled vocabulary — what certainty posture does the text take?
const (
	PostureDoctrinalAssertion    = "doctrinal_assertion"
	PostureDescriptiveAbstract   = "descriptive_abstract"
	PostureExternallyReferenced  = "externally_referenced"
	PostureExperientialReframing = "experiential_reframing"
	PostureConditionalRevelation = "conditional_revelation"
	PostureUnknown               = "unknown"
)

// AnomalyType controlled vocabulary — taxonomy-derived anomaly classifications.
const (
	AnomalyHighCrossSourceCoherence = "high_cross_source_coherence"
	AnomalySourceOversampling       = "source_oversampling"
	AnomalySummaryTemplateArtifact  = "summary_template_artifact"
	AnomalyLabelMismatch            = "label_mismatch"
	AnomalyEmbeddingBiasSuspected   = "embedding_bias_suspected"
)

// Classifier holds the vocabulary tables for rule-based classification.
type Classifier struct{}

// New returns a ready-to-use Classifier.
func New() *Classifier { return &Classifier{} }
