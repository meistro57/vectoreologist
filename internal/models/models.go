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
	ID          int
	Label       string
	VectorIDs   []uint64
	Centroid    []float32
	Density     float64
	Size        int
	Coherence   float64
}

// Bridge represents a semantic connection between clusters
type Bridge struct {
	ClusterA   int
	ClusterB   int
	Strength   float64
	LinkType   string
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
