package models

// VectorMetadata holds excavated metadata from Qdrant points
type VectorMetadata struct {
	ID       uint64
	Fragment string
	Source   string
	Layer    string
	RunID    string
}

// TaxonomyLabel is the multi-axis knowledge classification for a cluster,
// produced by the taxonomy classifier after clustering and label promotion.
type TaxonomyLabel struct {
	Topic            string  `json:"topic"`
	Mode             string  `json:"mode"`
	EpistemicPosture string  `json:"epistemic_posture"`
	SourceFamily     string  `json:"source_family,omitempty"`
	Confidence       float64 `json:"confidence"`
	LabelWarning     string  `json:"label_warning,omitempty"`
	SemanticConcept  string  `json:"semantic_concept,omitempty"`
}

// Cluster represents a semantic concept cluster in vector space
type Cluster struct {
	ID        int
	Label     string   // semantic label (promoted from R1 conclusion, or source-based fallback)
	Source    string   // original layer/source-based label produced by clustering
	VectorIDs []uint64
	Centroid  []float32
	Density   float64
	Size      int
	Coherence float64
	Taxonomy  *TaxonomyLabel // set by taxonomy.Classifier after clustering
}

// SampleLink is a representative cross-cluster chunk pair that justifies a bridge.
type SampleLink struct {
	ChunkAID   uint64  `json:"chunk_a_id"`
	ChunkBID   uint64  `json:"chunk_b_id"`
	Similarity float64 `json:"similarity"`
}

// Bridge represents a semantic connection between clusters
type Bridge struct {
	ClusterA    int
	ClusterB    int
	Strength    float64
	LinkType    string
	Label       string // short semantic description from R1 conclusion, if available
	SampleLinks []SampleLink
}

// Moat represents isolation between knowledge domains
type Moat struct {
	ClusterA    int
	ClusterB    int
	Distance    float64
	Explanation string
}

// Finding represents a DeepSeek R1 reasoning result or a detected anomaly.
type Finding struct {
	Type           string
	Subject        string
	ReasoningChain string
	Confidence     float64
	IsAnomaly      bool
	Clusters       []int
	// Structured anomaly fields — populated by the anomaly detector for new anomaly types.
	AnomalyType    string   `json:"anomaly_type,omitempty"`
	Evidence       string   `json:"evidence,omitempty"`
	PossibleCauses []string `json:"possible_causes,omitempty"`
	RequiresReview bool     `json:"requires_review,omitempty"`
}
