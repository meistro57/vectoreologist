package synthesis

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/meistro57/vectoreologist/internal/models"
	qdrant "github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
)

const findingsCollection = "vectoreology_findings"

type Synthesizer struct {
	qdrantURL  string
	outputPath string
	client     *qdrant.Client
}

const maxMsgSize = 256 * 1024 * 1024 // 256 MB

func New(qdrantURL, outputPath string) *Synthesizer {
	client, err := qdrant.NewClient(&qdrant.Config{
		Host: hostname(qdrantURL),
		GrpcOptions: []grpc.DialOption{
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMsgSize)),
		},
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to Qdrant: %v", err))
	}

	return &Synthesizer{
		qdrantURL:  qdrantURL,
		outputPath: outputPath,
		client:     client,
	}
}

func hostname(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err == nil && u.Hostname() != "" {
		return u.Hostname()
	}
	return rawURL
}

// GenerateReport creates a living markdown synthesis document and a matching JSON file.
func (s *Synthesizer) GenerateReport(
	findings []models.Finding,
	clusters []models.Cluster,
	bridges []models.Bridge,
	moats []models.Moat,
	collection string,
) string {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	reportPath := filepath.Join(s.outputPath, fmt.Sprintf("vectoreology_%s.md", timestamp))

	os.MkdirAll(s.outputPath, 0755)

	var sb strings.Builder
	sb.WriteString("# Vectoreology Report\n\n")
	sb.WriteString(fmt.Sprintf("**Generated:** %s\n\n", time.Now().Format(time.RFC3339)))
	
	sb.WriteString("## Topology Summary\n\n")
	sb.WriteString(fmt.Sprintf("- **Clusters:** %d\n", len(clusters)))
	sb.WriteString(fmt.Sprintf("- **Bridges:** %d\n", len(bridges)))
	sb.WriteString(fmt.Sprintf("- **Moats:** %d\n\n", len(moats)))

	sb.WriteString("## Cluster Analysis\n\n")
	for _, finding := range findings {
		if finding.Type == "cluster_analysis" {
			sb.WriteString(fmt.Sprintf("### %s\n\n", finding.Subject))
			sb.WriteString(fmt.Sprintf("%s\n\n", finding.ReasoningChain))
		}
	}

	sb.WriteString("## Semantic Bridges\n\n")
	for _, finding := range findings {
		if finding.Type == "bridge_analysis" {
			sb.WriteString(fmt.Sprintf("### %s\n\n", finding.Subject))
			sb.WriteString(fmt.Sprintf("%s\n\n", finding.ReasoningChain))
		}
	}

	sb.WriteString("## Knowledge Moats\n\n")
	for _, finding := range findings {
		if finding.Type == "moat_analysis" {
			sb.WriteString(fmt.Sprintf("### %s\n\n", finding.Subject))
			sb.WriteString(fmt.Sprintf("%s\n\n", finding.ReasoningChain))
		}
	}

	os.WriteFile(reportPath, []byte(sb.String()), 0644)

	// Also generate JSON for the TUI lens.
	if jsonPath := s.GenerateJSON(findings, clusters, bridges, moats, collection, timestamp); jsonPath != "" {
		fmt.Printf("   ✓ JSON written to %s\n", jsonPath)
	}

	return reportPath
}

func buildClusterMemberPointIDs(clusters []models.Cluster) map[int][]string {
	clusterMemberPointIDs := make(map[int][]string, len(clusters))
	for _, cluster := range clusters {
		memberPointIDs := make([]string, len(cluster.VectorIDs))
		for i, vectorID := range cluster.VectorIDs {
			memberPointIDs[i] = fmt.Sprintf("%d", vectorID)
		}
		clusterMemberPointIDs[cluster.ID] = memberPointIDs
	}
	return clusterMemberPointIDs
}

func parseClusterIDsFromSubject(findingType, subject string) []int {
	if findingType == "cluster_analysis" {
		var clusterID int
		if _, err := fmt.Sscanf(subject, "Cluster %d:", &clusterID); err == nil {
			return []int{clusterID}
		}
	}
	if findingType == "bridge_analysis" {
		var clusterA, clusterB int
		if _, err := fmt.Sscanf(subject, "Bridge: %d ↔ %d", &clusterA, &clusterB); err == nil {
			return []int{clusterA, clusterB}
		}
	}
	return nil
}

func memberPointIDsForFinding(f models.Finding, clusterMemberPointIDs map[int][]string) ([]string, bool) {
	switch f.Type {
	case "cluster_analysis", "bridge_analysis", "density_anomaly", "coherence_anomaly":
	default:
		return nil, false
	}

	clusterIDs := f.Clusters
	if len(clusterIDs) == 0 {
		clusterIDs = parseClusterIDsFromSubject(f.Type, f.Subject)
	}

	seen := make(map[string]bool)
	memberPointIDs := make([]string, 0)
	for _, clusterID := range clusterIDs {
		for _, memberPointID := range clusterMemberPointIDs[clusterID] {
			if !seen[memberPointID] {
				seen[memberPointID] = true
				memberPointIDs = append(memberPointIDs, memberPointID)
			}
		}
	}

	return memberPointIDs, true
}

func stringsToAny(values []string) []any {
	converted := make([]any, len(values))
	for i, value := range values {
		converted[i] = value
	}
	return converted
}

// StoreFindings writes findings back to Qdrant.
// Uses a 1-dimensional confidence vector; payload holds all finding fields.
func (s *Synthesizer) StoreFindings(findings []models.Finding, clusters []models.Cluster) error {
	if len(findings) == 0 {
		return nil
	}

	ctx := context.Background()

	exists, err := s.client.CollectionExists(ctx, findingsCollection)
	if err != nil {
		return fmt.Errorf("checking collection: %w", err)
	}
	if !exists {
		if err := s.client.CreateCollection(ctx, &qdrant.CreateCollection{
			CollectionName: findingsCollection,
			VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
				Size:     1,
				Distance: qdrant.Distance_Cosine,
			}),
		}); err != nil {
			return fmt.Errorf("creating collection: %w", err)
		}
	}

	clusterMemberPointIDs := buildClusterMemberPointIDs(clusters)
	base := uint64(time.Now().UnixMilli())
	points := make([]*qdrant.PointStruct, 0, len(findings))
	for i, f := range findings {
		clusterStrs := make([]string, len(f.Clusters))
		for j, c := range f.Clusters {
			clusterStrs[j] = fmt.Sprintf("%d", c)
		}

		payload := map[string]any{
			"type":            f.Type,
			"subject":         f.Subject,
			"reasoning_chain": f.ReasoningChain,
			"confidence":      f.Confidence,
			"is_anomaly":      f.IsAnomaly,
			"clusters":        strings.Join(clusterStrs, ","),
			"stored_at":       time.Now().Format(time.RFC3339),
		}
		if memberPointIDs, include := memberPointIDsForFinding(f, clusterMemberPointIDs); include {
			payload["member_point_ids"] = stringsToAny(memberPointIDs)
		}

		points = append(points, &qdrant.PointStruct{
			Id:      qdrant.NewIDNum(base + uint64(i)),
			Vectors: qdrant.NewVectors(float32(f.Confidence)),
			Payload: qdrant.NewValueMap(payload),
		})
	}

	if _, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: findingsCollection,
		Points:         points,
	}); err != nil {
		return fmt.Errorf("upserting findings: %w", err)
	}

	return nil
}
