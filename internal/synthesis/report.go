package synthesis

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

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

func (s *Synthesizer) GenerateReport(
	findings []models.Finding,
	clusters []models.Cluster,
	bridges []models.Bridge,
	moats []models.Moat,
	metadata []models.VectorMetadata,
	collection string,
) string {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	return s.generateReportWithTimestamp(findings, clusters, bridges, moats, metadata, collection, timestamp, true)
}

func (s *Synthesizer) GenerateProgressReport(
	findings []models.Finding,
	clusters []models.Cluster,
	bridges []models.Bridge,
	moats []models.Moat,
	metadata []models.VectorMetadata,
	collection string,
) string {
	return s.generateReportWithTimestamp(findings, clusters, bridges, moats, metadata, collection, "in_progress", false)
}

func (s *Synthesizer) generateReportWithTimestamp(
	findings []models.Finding,
	clusters []models.Cluster,
	bridges []models.Bridge,
	moats []models.Moat,
	metadata []models.VectorMetadata,
	collection string,
	timestamp string,
	printJSONPath bool,
) string {
	reportPath := filepath.Join(s.outputPath, fmt.Sprintf("vectoreology_%s.md", timestamp))
	os.MkdirAll(s.outputPath, 0755)
	report := renderReport(reportData{
		findings:   findings,
		clusters:   clusters,
		bridges:    bridges,
		moats:      moats,
		metadata:   metadata,
		collection: collection,
	})
	os.WriteFile(reportPath, []byte(report), 0644)
	if jsonPath := s.GenerateJSON(findings, clusters, bridges, moats, metadata, collection, timestamp); printJSONPath && jsonPath != "" {
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

// sanitizeUTF8 replaces any invalid UTF-8 byte sequences with the Unicode
// replacement character (U+FFFD). DeepSeek R1 occasionally produces truncated
// multi-byte characters (e.g. \xe2\x80 without the closing byte) that cause
// the Qdrant Go client to panic inside NewValueMap during gRPC serialization.
func sanitizeUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	return strings.ToValidUTF8(s, "\uFFFD")
}

// findingPointID returns a stable uint64 ID for a finding derived from a
// SHA-256 hash of its type, subject, and cluster list. Identical findings
// across runs produce the same ID, so successive runs upsert (overwrite)
// rather than appending duplicate rows. Different findings are extremely
// unlikely to collide (2^64 space).
func findingPointID(f models.Finding) uint64 {
	clusterStrs := make([]string, len(f.Clusters))
	for i, c := range f.Clusters {
		clusterStrs[i] = fmt.Sprintf("%d", c)
	}
	key := f.Type + "|" + f.Subject + "|" + strings.Join(clusterStrs, ",")
	sum := sha256.Sum256([]byte(key))
	return binary.LittleEndian.Uint64(sum[:8])
}

// StoreFindings writes findings back to Qdrant.
// Point IDs are derived from a hash of finding type+subject+clusters so that
// re-runs upsert (overwrite) existing findings rather than appending duplicates.
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
	points := make([]*qdrant.PointStruct, 0, len(findings))
	for _, f := range findings {
		clusterStrs := make([]string, len(f.Clusters))
		for j, c := range f.Clusters {
			clusterStrs[j] = fmt.Sprintf("%d", c)
		}

		payload := map[string]any{
			"type":            sanitizeUTF8(f.Type),
			"subject":         sanitizeUTF8(f.Subject),
			"reasoning_chain": sanitizeUTF8(f.ReasoningChain),
			"confidence":      f.Confidence,
			"is_anomaly":      f.IsAnomaly,
			"clusters":        strings.Join(clusterStrs, ","),
			"stored_at":       time.Now().Format(time.RFC3339),
		}
		if memberPointIDs, include := memberPointIDsForFinding(f, clusterMemberPointIDs); include {
			payload["member_point_ids"] = stringsToAny(memberPointIDs)
		}

		points = append(points, &qdrant.PointStruct{
			Id:      qdrant.NewIDNum(findingPointID(f)),
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
