package models

// VectorMetadata holds excavated metadata from Qdrant points
type VectorMetadata struct {
	ID       uint64
	Fragment string
	Source   string
	Layer    string
	RunID    string
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

// Finding represents a DeepSeek R1 reasoning result
type Finding struct {
	Type           string
	Subject        string
	ReasoningChain string
	Confidence     float64
	IsAnomaly      bool
	Clusters       []int
}
