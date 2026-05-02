package excavator

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/meistro57/vectoreologist/internal/models"
	qdrant "github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
)

type Excavator struct {
	client        *qdrant.Client
	vectorName    string
	vectorCombine bool
}

const maxMsgSize = 256 * 1024 * 1024 // 256 MB

func New(rawURL, vectorName string, vectorCombine bool) *Excavator {
	client, err := qdrant.NewClient(&qdrant.Config{
		Host: hostname(rawURL),
		GrpcOptions: []grpc.DialOption{
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMsgSize)),
		},
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to Qdrant: %v", err))
	}

	return &Excavator{client: client, vectorName: vectorName, vectorCombine: vectorCombine}
}

// CollectionSize returns the number of points in the named collection.
func (e *Excavator) CollectionSize(name string) (uint64, error) {
	info, err := e.client.GetCollectionInfo(context.Background(), name)
	if err != nil {
		return 0, fmt.Errorf("get collection info: %w", err)
	}
	return info.GetPointsCount(), nil
}

// StampPoints sets vectoreology_last_run on all extracted points so
// subsequent --incremental runs can skip them.
func (e *Excavator) StampPoints(collectionName string, ids []uint64, runID string) error {
	if len(ids) == 0 {
		return nil
	}
	ctx := context.Background()

	const batch = 500
	for start := 0; start < len(ids); start += batch {
		end := start + batch
		if end > len(ids) {
			end = len(ids)
		}

		pointIDs := make([]*qdrant.PointId, end-start)
		for i, id := range ids[start:end] {
			pointIDs[i] = qdrant.NewIDNum(id)
		}

		_, err := e.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
			CollectionName: collectionName,
			Payload: map[string]*qdrant.Value{
				"vectoreology_last_run": qdrant.NewValueString(runID),
			},
			PointsSelector: &qdrant.PointsSelector{
				PointsSelectorOneOf: &qdrant.PointsSelector_Points{
					Points: &qdrant.PointsIdsList{Ids: pointIDs},
				},
			},
		})
		if err != nil {
			return fmt.Errorf("stamp batch at %d: %w", start, err)
		}
	}
	return nil
}

// ExtractIncremental pulls vectors that have NOT been stamped with
// vectoreology_last_run. Uses IsEmpty filter so only unstamped points return.
func (e *Excavator) ExtractIncremental(collectionName string, limit, batchSize int, strict bool, onBatch func(batchNum, fetched, target int)) ([][]float32, []models.VectorMetadata, error) {
	ctx := context.Background()

	allVectors := make([][]float32, 0, limit)
	allMetadata := make([]models.VectorMetadata, 0, limit)

	var nextOffset *qdrant.PointId
	batchNum := 0

	// must: field "vectoreology_last_run" is empty (not set on point).
	filter := &qdrant.Filter{
		Must: []*qdrant.Condition{
			{
				ConditionOneOf: &qdrant.Condition_IsEmpty{
					IsEmpty: &qdrant.IsEmptyCondition{
						Key: "vectoreology_last_run",
					},
				},
			},
		},
	}

	for len(allVectors) < limit {
		remaining := limit - len(allVectors)
		currentBatch := batchSize
		if currentBatch > remaining {
			currentBatch = remaining
		}

		lim := uint32(currentBatch)
		req := &qdrant.ScrollPoints{
			CollectionName: collectionName,
			Limit:          &lim,
			WithVectors:    qdrant.NewWithVectors(true),
			WithPayload:    qdrant.NewWithPayload(true),
			Offset:         nextOffset,
			Filter:         filter,
		}

		points, offset, err := e.client.ScrollAndOffset(ctx, req)
		if err != nil {
			if strict {
				return nil, nil, fmt.Errorf("incremental batch %d scroll failed: %w", batchNum+1, err)
			}
			fmt.Fprintf(os.Stderr, "   ⚠ Batch %d failed: %v — stopping early\n", batchNum+1, err)
			break
		}

		for _, point := range points {
			vec, meta, ok := extractPoint(point, e.vectorName, e.vectorCombine)
			if !ok {
				continue
			}
			allVectors = append(allVectors, vec)
			allMetadata = append(allMetadata, meta)
		}

		batchNum++
		if onBatch != nil {
			onBatch(batchNum, len(allVectors), limit)
		}

		if len(points) == 0 || offset == nil {
			break
		}
		nextOffset = offset
	}

	return allVectors, allMetadata, nil
}

// Extract pulls up to limit vectors from a Qdrant collection using batched scrolling.
// batchSize controls how many points are requested per scroll call.
// When strict is true, any batch error aborts and returns an error; otherwise the
// error is logged and extraction stops early with whatever was collected.
// onBatch is called after each successful batch with (batchNum, fetched, target).
// Pass nil to suppress progress callbacks.
func (e *Excavator) Extract(collectionName string, limit, batchSize int, strict bool, onBatch func(batchNum, fetched, target int)) ([][]float32, []models.VectorMetadata, error) {
	ctx := context.Background()

	allVectors := make([][]float32, 0, limit)
	allMetadata := make([]models.VectorMetadata, 0, limit)

	var nextOffset *qdrant.PointId
	batchNum := 0

	for len(allVectors) < limit {
		remaining := limit - len(allVectors)
		currentBatch := batchSize
		if currentBatch > remaining {
			currentBatch = remaining
		}

		lim := uint32(currentBatch)
		req := &qdrant.ScrollPoints{
			CollectionName: collectionName,
			Limit:          &lim,
			WithVectors:    qdrant.NewWithVectors(true),
			WithPayload:    qdrant.NewWithPayload(true),
			Offset:         nextOffset,
		}

		points, offset, err := e.client.ScrollAndOffset(ctx, req)
		if err != nil {
			if strict {
				return nil, nil, fmt.Errorf("batch %d scroll failed: %w", batchNum+1, err)
			}
			fmt.Fprintf(os.Stderr, "   ⚠ Batch %d failed: %v — stopping early\n", batchNum+1, err)
			break
		}

		for _, point := range points {
			vec, meta, ok := extractPoint(point, e.vectorName, e.vectorCombine)
			if !ok {
				continue
			}
			allVectors = append(allVectors, vec)
			allMetadata = append(allMetadata, meta)
		}

		batchNum++
		if onBatch != nil {
			onBatch(batchNum, len(allVectors), limit)
		}

		if len(points) == 0 || offset == nil {
			break // collection exhausted
		}
		nextOffset = offset
	}

	return allVectors, allMetadata, nil
}

// extractPoint converts a RetrievedPoint into a vector and metadata.
// Returns (vec, meta, true) on success; (nil, zero, false) if the point has no vector.
func extractPoint(point *qdrant.RetrievedPoint, vectorName string, vectorCombine bool) ([]float32, models.VectorMetadata, bool) {
	var vec []float32
	if point == nil || point.Vectors == nil {
		return nil, models.VectorMetadata{}, false
	}
	if v := point.Vectors.GetVector(); v != nil {
		vec = vectorData(v)
	} else if named := point.Vectors.GetVectors(); named != nil {
		namedVectors := named.GetVectors()
		if vectorCombine {
			vec = averageNamedVectors(namedVectors)
		} else if vectorName != "" {
			vec = vectorData(namedVectors[vectorName])
		}
		if len(vec) == 0 {
			for _, nv := range namedVectors {
				vec = vectorData(nv)
				if len(vec) > 0 {
					break
				}
			}
		}
	}
	if len(vec) == 0 {
		return nil, models.VectorMetadata{}, false
	}

	// Try multiple source field names to support different collection schemas
	source := getPayloadString(point.Payload, "source", "")
	if source == "" {
		source = getPayloadString(point.Payload, "source_file", "")
	}
	if source == "" {
		source = getPayloadString(point.Payload, "source_collection", "")
	}
	if source == "" {
		source = getPayloadString(point.Payload, "source_id", "unknown")
	}

	// Build a rich fragment from available payload fields.
	// For meta_reflections: prefer claims + concepts + echoes over bare summary.
	// For mb_claims: use canonical_statement.
	// For mb_chunks: use text.
	fragment := buildFragment(point.Payload)

	meta := models.VectorMetadata{
		ID:       pointIDToUint64(point.Id),
		Fragment: fragment,
		Source:   source,
		Layer:    getPayloadString(point.Payload, "layer", getPayloadString(point.Payload, "tone", "surface")),
		RunID:    getPayloadString(point.Payload, "run_id", ""),
	}
	return vec, meta, true
}

func vectorData(v *qdrant.VectorOutput) []float32 {
	if v == nil {
		return nil
	}
	if dense := v.GetDense(); dense != nil {
		return dense.Data
	}
	return v.Data
}

func averageNamedVectors(named map[string]*qdrant.VectorOutput) []float32 {
	var sum []float32
	count := 0
	for _, v := range named {
		data := vectorData(v)
		if len(data) == 0 {
			continue
		}
		if len(sum) == 0 {
			sum = make([]float32, len(data))
		} else if len(data) != len(sum) {
			continue
		}
		for i := range data {
			sum[i] += data[i]
		}
		count++
	}
	if count == 0 {
		return nil
	}
	for i := range sum {
		sum[i] /= float32(count)
	}
	return sum
}

// buildFragment assembles a text fragment from payload fields, preferring
// rich structured data (claims, concepts, echoes) over bare summary/text.
// This gives HDBSCAN and the reasoner diverse signal instead of identical summaries.
func buildFragment(payload map[string]*qdrant.Value) string {
	var parts []string

	// Summary or canonical statement — the core single-line description.
	if s := getPayloadString(payload, "canonical_statement", ""); s != "" {
		parts = append(parts, s)
	} else if s := getPayloadString(payload, "summary", ""); s != "" {
		parts = append(parts, s)
	}

	// Claims — the most diverse and informative field in reflections.
	if claims := getPayloadList(payload, "claims"); len(claims) > 0 {
		for i, c := range claims {
			if i >= 3 {
				break // cap to keep fragment reasonable
			}
			parts = append(parts, c)
		}
	}

	// Concepts — short noun phrases, great for clustering signal.
	if concepts := getPayloadList(payload, "concepts"); len(concepts) > 0 {
		parts = append(parts, "Concepts: "+joinMax(concepts, 6))
	}

	// Echoes — cross-tradition resonances.
	if echoes := getPayloadList(payload, "echoes"); len(echoes) > 0 {
		parts = append(parts, "Echoes: "+joinMax(echoes, 4))
	}

	// Questions — what the passage raises.
	if questions := getPayloadList(payload, "questions"); len(questions) > 0 {
		if len(questions) > 2 {
			questions = questions[:2]
		}
		for _, q := range questions {
			parts = append(parts, q)
		}
	}

	// Tags from mb_claims.
	if tags := getPayloadList(payload, "tags"); len(tags) > 0 {
		parts = append(parts, "Tags: "+joinMax(tags, 6))
	}

	// Fallback to raw text field.
	if len(parts) == 0 {
		// misfit_reports: use report + verdict
		if s := getPayloadString(payload, "report", ""); s != "" {
			parts = append(parts, truncate(s, 300))
		}
		if s := getPayloadString(payload, "verdict", ""); s != "" {
			parts = append(parts, "Verdict: "+truncate(s, 150))
		}
	}

	// Final fallback to raw text field.
	if len(parts) == 0 {
		if s := getPayloadString(payload, "text", ""); s != "" {
			return truncate(s, 500)
		}
		return "N/A"
	}

	result := ""
	for i, p := range parts {
		if i > 0 {
			result += " | "
		}
		result += p
	}
	return truncate(result, 500)
}

// getPayloadList extracts a string list from a Qdrant list value.
func getPayloadList(payload map[string]*qdrant.Value, key string) []string {
	val, ok := payload[key]
	if !ok || val == nil {
		return nil
	}
	list := val.GetListValue()
	if list == nil {
		// Might be a single string value.
		if s := val.GetStringValue(); s != "" {
			return []string{s}
		}
		return nil
	}
	var out []string
	for _, item := range list.GetValues() {
		if s := item.GetStringValue(); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// joinMax joins up to n items with ", ".
func joinMax(items []string, n int) string {
	if len(items) > n {
		items = items[:n]
	}
	result := ""
	for i, item := range items {
		if i > 0 {
			result += ", "
		}
		result += item
	}
	return result
}

// truncate cuts a string to maxLen, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// pointIDToUint64 converts a Qdrant PointId to uint64.
// Numeric IDs pass through directly. UUID IDs are converted to a
// deterministic uint64 using the first 8 bytes of the UUID.
func pointIDToUint64(id *qdrant.PointId) uint64 {
	if id == nil {
		return 0
	}
	if n := id.GetNum(); n != 0 {
		return n
	}
	uuid := strings.ReplaceAll(id.GetUuid(), "-", "")
	if len(uuid) < 16 {
		return 0
	}
	bytes, err := hex.DecodeString(uuid[:16])
	if err != nil || len(bytes) != 8 {
		return 0
	}

	var out uint64
	for _, b := range bytes {
		out = (out << 8) | uint64(b)
	}
	return out
}

// hostname strips the scheme and port from a URL, returning just the host.
// The Qdrant gRPC client expects a bare hostname and manages its own port.
func hostname(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err == nil && u.Hostname() != "" {
		return u.Hostname()
	}
	return rawURL
}

func getPayloadString(payload map[string]*qdrant.Value, key, defaultVal string) string {
	if val, ok := payload[key]; ok {
		if str := val.GetStringValue(); str != "" {
			return str
		}
	}
	return defaultVal
}
