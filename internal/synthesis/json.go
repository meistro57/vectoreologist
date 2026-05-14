package synthesis

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/meistro57/vectoreologist/internal/models"
)

// JSONReport is the top-level JSON export structure.
type JSONReport struct {
	Timestamp       string             `json:"timestamp"`
	Collection      string             `json:"collection"`
	Summary         JSONSummary        `json:"summary"`
	Clusters        []JSONCluster      `json:"clusters"`
	Bridges         []JSONBridge       `json:"bridges"`
	Moats           []JSONMoat         `json:"moats"`
	Anomalies       []JSONAnomaly      `json:"anomalies"`
	Attractors      []JSONAttractor    `json:"attractors,omitempty"`
	Recommendations []string           `json:"recommendations,omitempty"`
	ReviewItems     []string           `json:"review_items,omitempty"`
}

type JSONSummary struct {
	TotalClusters          int `json:"total_clusters"`
	TotalBridges           int `json:"total_bridges"`
	TotalMoats             int `json:"total_moats"`
	TotalAnomalies         int `json:"total_anomalies"`
	DuplicateHeavyClusters int `json:"duplicate_heavy_clusters"`
	OverSampledClusters    int `json:"oversampled_clusters"`
	SkippedBridges         int `json:"skipped_bridges"`
}

// JSONAttractor describes a recurring semantic concept across clusters/bridges.
type JSONAttractor struct {
	Name        string   `json:"name"`
	Clusters    []int    `json:"clusters"`
	Bridges     []string `json:"bridges"`
	Confidence  float64  `json:"confidence"`
	Explanation string   `json:"explanation,omitempty"`
}

// JSONTaxonomy is the JSON representation of a TaxonomyLabel.
type JSONTaxonomy struct {
	Topic            string  `json:"topic"`
	Mode             string  `json:"mode"`
	EpistemicPosture string  `json:"epistemic_posture"`
	SourceFamily     string  `json:"source_family,omitempty"`
	Confidence       float64 `json:"confidence"`
	LabelWarning     string  `json:"label_warning,omitempty"`
	SemanticConcept  string  `json:"semantic_concept,omitempty"`
}

type JSONCluster struct {
	ID             int                `json:"id"`
	Label          string             `json:"label"`
	SuggestedLabel string             `json:"suggested_label,omitempty"`
	Source         string             `json:"source,omitempty"` // original layer/source-based label
	Size           int                `json:"size"`
	Density        float64            `json:"density"`
	Coherence      float64            `json:"coherence"`
	Centroid       []float32          `json:"centroid"`
	VectorIDs      []uint64           `json:"vector_ids"`
	Reasoning      string             `json:"reasoning"`
	IsAnomaly      bool               `json:"is_anomaly"`
	Taxonomy       *JSONTaxonomy      `json:"taxonomy,omitempty"`
	Confidence     float64            `json:"confidence,omitempty"`
	FinalConcept   string             `json:"final_concept,omitempty"`
	Snippets       []string           `json:"representative_snippets,omitempty"`
	SourceBalance  map[string]float64 `json:"source_balance,omitempty"`
	ReviewFlags    []string           `json:"review_flags,omitempty"`
	DuplicateHeavy bool               `json:"duplicate_heavy,omitempty"`
	OverSampled    bool               `json:"oversampled,omitempty"`
}

type JSONSampleLink struct {
	ChunkAID   uint64  `json:"chunk_a_id"`
	ChunkBID   uint64  `json:"chunk_b_id"`
	Similarity float64 `json:"similarity"`
}

type JSONBridge struct {
	ClusterA      int              `json:"cluster_a"`
	ClusterB      int              `json:"cluster_b"`
	Strength      float64          `json:"strength"`
	LinkType      string           `json:"link_type"`
	Label         string           `json:"label,omitempty"` // short semantic description from R1 conclusion
	SampleLinks   []JSONSampleLink `json:"sample_links"`
	Reasoning     string           `json:"reasoning"`
	LabelA        string           `json:"label_a,omitempty"`
	LabelB        string           `json:"label_b,omitempty"`
	EvidenceA     []string         `json:"evidence_a,omitempty"`
	EvidenceB     []string         `json:"evidence_b,omitempty"`
	SharedConcept string           `json:"shared_concept,omitempty"`
	Confidence    float64          `json:"confidence,omitempty"`
	ReviewFlags   []string         `json:"review_flags,omitempty"`
	Skipped       bool             `json:"skipped,omitempty"`
}

type JSONMoat struct {
	ClusterA    int     `json:"cluster_a"`
	ClusterB    int     `json:"cluster_b"`
	Distance    float64 `json:"distance"`
	Explanation string  `json:"explanation"`
	Reasoning   string  `json:"reasoning"`
}

type JSONAnomaly struct {
	Type           string   `json:"type"`
	Subject        string   `json:"subject"`
	ClusterID      int      `json:"cluster_id"`
	ReasoningChain string   `json:"reasoning_chain"`
	IsAnomaly      bool     `json:"is_anomaly"`
	AnomalyType    string   `json:"anomaly_type,omitempty"`
	Evidence       string   `json:"evidence,omitempty"`
	PossibleCauses []string `json:"possible_causes,omitempty"`
	RequiresReview bool     `json:"requires_review,omitempty"`
}

// GenerateJSON exports findings to a JSON file alongside the markdown report.
// Returns the path of the written file.
func (s *Synthesizer) GenerateJSON(
	findings []models.Finding,
	clusters []models.Cluster,
	bridges []models.Bridge,
	moats []models.Moat,
	metadata []models.VectorMetadata,
	collection string,
	timestamp string,
) string {
	jsonPath := filepath.Join(s.outputPath, fmt.Sprintf("vectoreology_%s.json", timestamp))

	artifact := buildReportArtifact(reportData{
		findings:   findings,
		clusters:   clusters,
		bridges:    bridges,
		moats:      moats,
		metadata:   metadata,
		collection: collection,
	})
	clusterSectionByID := make(map[int]clusterSection, len(artifact.clusterSections))
	for _, section := range artifact.clusterSections {
		clusterSectionByID[section.cluster.ID] = section
	}
	bridgeSectionByPair := make(map[[2]int]bridgeSection, len(artifact.bridgeSections))
	for _, section := range artifact.bridgeSections {
		bridgeSectionByPair[[2]int{section.bridge.ClusterA, section.bridge.ClusterB}] = section
	}

	// Build cluster ID → reasoning/anomaly from findings.
	// Subject format from reasoner: "Cluster N: label"
	clusterReasoning := make(map[int]string)
	clusterAnomalous := make(map[int]bool)
	for _, f := range findings {
		if f.Type == "cluster_analysis" {
			cid := parseClusterID(f.Subject)
			if cid > 0 {
				clusterReasoning[cid] = f.ReasoningChain
				if f.IsAnomaly {
					clusterAnomalous[cid] = true
				}
			}
		}
	}

	// Build bridge (A,B) → reasoning lookup.
	// Subject format from reasoner: "Bridge: A ↔ B"
	type pair struct{ a, b int }
	bridgeReasoning := make(map[pair]string)
	for _, f := range findings {
		if f.Type == "bridge_analysis" {
			a, b := parsePair(f.Subject, "↔")
			if a > 0 {
				bridgeReasoning[pair{a, b}] = f.ReasoningChain
				bridgeReasoning[pair{b, a}] = f.ReasoningChain
			}
		}
	}

	// Build moat (A,B) → reasoning lookup.
	// Subject format from reasoner: "Moat: A ⊥ B"
	moatReasoning := make(map[pair]string)
	for _, f := range findings {
		if f.Type == "moat_analysis" {
			a, b := parsePair(f.Subject, "⊥")
			if a > 0 {
				moatReasoning[pair{a, b}] = f.ReasoningChain
				moatReasoning[pair{b, a}] = f.ReasoningChain
			}
		}
	}

	// Gather anomaly findings.
	var anomalies []JSONAnomaly
	for _, f := range findings {
		if !f.IsAnomaly {
			continue
		}
		cid := parseClusterID(f.Subject)
		if cid == 0 && len(f.Clusters) > 0 {
			cid = f.Clusters[0]
		}
		anomalies = append(anomalies, JSONAnomaly{
			Type:           f.Type,
			Subject:        f.Subject,
			ClusterID:      cid,
			ReasoningChain: f.ReasoningChain,
			IsAnomaly:      true,
			AnomalyType:    f.AnomalyType,
			Evidence:       f.Evidence,
			PossibleCauses: f.PossibleCauses,
			RequiresReview: f.RequiresReview,
		})
	}

	// Enrich clusters.
	jClusters := make([]JSONCluster, len(clusters))
	for i, c := range clusters {
		jc := JSONCluster{
			ID:        c.ID,
			Label:     c.Label,
			Source:    c.Source,
			Size:      c.Size,
			Density:   c.Density,
			Coherence: c.Coherence,
			Centroid:  c.Centroid,
			VectorIDs: c.VectorIDs,
			Reasoning: clusterReasoning[c.ID],
			IsAnomaly: clusterAnomalous[c.ID],
		}
		if c.Taxonomy != nil {
			jc.Taxonomy = &JSONTaxonomy{
				Topic:            c.Taxonomy.Topic,
				Mode:             c.Taxonomy.Mode,
				EpistemicPosture: c.Taxonomy.EpistemicPosture,
				SourceFamily:     c.Taxonomy.SourceFamily,
				Confidence:       c.Taxonomy.Confidence,
				LabelWarning:     c.Taxonomy.LabelWarning,
				SemanticConcept:  c.Taxonomy.SemanticConcept,
			}
		}
		if section, ok := clusterSectionByID[c.ID]; ok {
			jc.SuggestedLabel = section.suggestedLabel
			jc.Confidence = section.confidence
			jc.FinalConcept = section.finalConcept
			jc.Snippets = section.snippets
			jc.SourceBalance = section.sourceBalance
			jc.ReviewFlags = section.flags
			jc.DuplicateHeavy = section.duplicateHeavy
			jc.OverSampled = section.overSampled
		}
		jClusters[i] = jc
	}

	// Enrich bridges.
	jBridges := make([]JSONBridge, len(bridges))
	for i, b := range bridges {
		jb := JSONBridge{
			ClusterA:    b.ClusterA,
			ClusterB:    b.ClusterB,
			Strength:    b.Strength,
			LinkType:    b.LinkType,
			Label:       b.Label,
			SampleLinks: toJSONSampleLinks(b.SampleLinks),
			Reasoning:   bridgeReasoning[pair{b.ClusterA, b.ClusterB}],
		}
		if section, ok := bridgeSectionByPair[[2]int{b.ClusterA, b.ClusterB}]; ok {
			jb.LabelA = section.labelA
			jb.LabelB = section.labelB
			jb.EvidenceA = section.evidenceA
			jb.EvidenceB = section.evidenceB
			jb.SharedConcept = section.sharedConcept
			jb.Confidence = section.confidence
			jb.ReviewFlags = section.flags
			jb.Skipped = section.skipped
		}
		jBridges[i] = jb
	}

	// Enrich moats.
	jMoats := make([]JSONMoat, len(moats))
	for i, m := range moats {
		jMoats[i] = JSONMoat{
			ClusterA:    m.ClusterA,
			ClusterB:    m.ClusterB,
			Distance:    m.Distance,
			Explanation: m.Explanation,
			Reasoning:   moatReasoning[pair{m.ClusterA, m.ClusterB}],
		}
	}

	jAttractors := make([]JSONAttractor, 0, len(artifact.attractors))
	for _, attr := range artifact.attractors {
		jAttractors = append(jAttractors, JSONAttractor{
			Name:        attr.name,
			Clusters:    attr.clusters,
			Bridges:     attr.bridges,
			Confidence:  attr.confidence,
			Explanation: attr.explanation,
		})
	}

	report := JSONReport{
		Timestamp:  strings.ReplaceAll(timestamp, "_", "T"),
		Collection: collection,
		Summary: JSONSummary{
			TotalClusters:          len(clusters),
			TotalBridges:           len(bridges),
			TotalMoats:             len(moats),
			TotalAnomalies:         len(anomalies),
			DuplicateHeavyClusters: artifact.diagnostics.duplicateHeavyClusters,
			OverSampledClusters:    artifact.diagnostics.overSampledClusters,
			SkippedBridges:         artifact.diagnostics.skippedBridges,
		},
		Clusters:        jClusters,
		Bridges:         jBridges,
		Moats:           jMoats,
		Anomalies:       anomalies,
		Attractors:      jAttractors,
		Recommendations: artifact.recommendations,
		ReviewItems:     artifact.reviewItems,
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "synthesis: marshal JSON: %v\n", err)
		return ""
	}
	if err := os.WriteFile(jsonPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "synthesis: write JSON: %v\n", err)
		return ""
	}
	return jsonPath
}

// parseClusterID extracts the cluster ID from subjects like "Cluster 7: label".
func parseClusterID(subject string) int {
	// Subject: "Cluster N: ..." or "Cluster N anomaly" etc.
	var id int
	fmt.Sscanf(subject, "Cluster %d", &id)
	return id
}

// toJSONSampleLinks converts model sample links to their JSON representation.
// Returns an empty (non-nil) slice so JSON marshaling produces [] rather than null.
func toJSONSampleLinks(links []models.SampleLink) []JSONSampleLink {
	out := make([]JSONSampleLink, len(links))
	for i, l := range links {
		out[i] = JSONSampleLink{ChunkAID: l.ChunkAID, ChunkBID: l.ChunkBID, Similarity: l.Similarity}
	}
	return out
}

// parsePair extracts two integer IDs from subjects like "Bridge: A ↔ B" or "Moat: A ⊥ B".
func parsePair(subject, sep string) (int, int) {
	// Strip prefix up to ": ".
	idx := strings.Index(subject, ": ")
	if idx < 0 {
		return 0, 0
	}
	rest := subject[idx+2:]
	sidx := strings.Index(rest, sep)
	if sidx < 0 {
		return 0, 0
	}
	left := strings.TrimSpace(rest[:sidx])
	right := strings.TrimSpace(rest[sidx+len(sep):])
	var a, b int
	fmt.Sscanf(left, "%d", &a)
	fmt.Sscanf(right, "%d", &b)
	return a, b
}
