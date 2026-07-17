package excavator

import (
	"context"
	"fmt"
	"hash/fnv"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/meistro57/vectoreologist/internal/models"
	qdrant "github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
)

type Excavator struct {
	client        *qdrant.Client
	vectorName    string
	vectorCombine bool
}

const maxMsgSize = 256 * 1024 * 1024

const (
	idNamespaceNumeric = "numeric"
	idNamespaceUUID    = "uuid"
)

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

func (e *Excavator) CollectionSize(name string) (uint64, error) {
	info, err := e.client.GetCollectionInfo(context.Background(), name)
	if err != nil {
		return 0, fmt.Errorf("get collection info: %w", err)
	}
	return info.GetPointsCount(), nil
}

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

func (e *Excavator) StampMetadataPoints(collectionName string, metadata []models.VectorMetadata, runID string) error {
	if len(metadata) == 0 {
		return nil
	}
	pointIDs := make([]*qdrant.PointId, 0, len(metadata))
	for _, m := range metadata {
		if id := metadataPointID(m); id != nil {
			pointIDs = append(pointIDs, id)
		}
	}
	if len(pointIDs) == 0 {
		return nil
	}

	ctx := context.Background()
	const batch = 500
	for start := 0; start < len(pointIDs); start += batch {
		end := start + batch
		if end > len(pointIDs) {
			end = len(pointIDs)
		}
		_, err := e.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
			CollectionName: collectionName,
			Payload: map[string]*qdrant.Value{
				"vectoreology_last_run": qdrant.NewValueString(runID),
			},
			PointsSelector: &qdrant.PointsSelector{
				PointsSelectorOneOf: &qdrant.PointsSelector_Points{
					Points: &qdrant.PointsIdsList{Ids: pointIDs[start:end]},
				},
			},
		})
		if err != nil {
			return fmt.Errorf("stamp metadata batch at %d: %w", start, err)
		}
	}
	return nil
}

func metadataPointID(m models.VectorMetadata) *qdrant.PointId {
	raw := strings.TrimSpace(m.RawPointID)
	if strings.HasPrefix(raw, "uuid:") {
		u := strings.TrimSpace(strings.TrimPrefix(raw, "uuid:"))
		if u != "" {
			return qdrant.NewIDUUID(u)
		}
	}
	if strings.HasPrefix(raw, "num:") {
		ns := strings.TrimSpace(strings.TrimPrefix(raw, "num:"))
		if n, err := strconv.ParseUint(ns, 10, 64); err == nil {
			return qdrant.NewIDNum(n)
		}
	}
	if m.IDNamespace == idNamespaceUUID && raw != "" {
		return qdrant.NewIDUUID(raw)
	}
	if m.ID != 0 {
		return qdrant.NewIDNum(m.ID)
	}
	return nil
}

func (e *Excavator) ExtractIncremental(collectionName string, limit, batchSize int, strict bool, onBatch func(batchNum, fetched, target int)) ([][]float32, []models.VectorMetadata, error) {
	ctx := context.Background()

	allVectors := make([][]float32, 0, limit)
	allMetadata := make([]models.VectorMetadata, 0, limit)

	var nextOffset *qdrant.PointId
	batchNum := 0

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
			break
		}
		nextOffset = offset
	}

	return allVectors, allMetadata, nil
}

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

	source := getPayloadString(point.Payload, "source", "")
	if source == "" {
		source = getPayloadString(point.Payload, "source_file", "")
	}
	if source == "" {
		source = getPayloadString(point.Payload, "source_collection", "")
	}
	if source == "" {
		source = getPayloadString(point.Payload, "source_id", "")
	}
	if source == "" {
		// keystones have no single source. Label by the concept instead, so the
		// report's "source balance" becomes a per-cluster concept composition
		// rather than an alphabetical artifact of the sorted source_ids list.
		source = getPayloadString(point.Payload, "concept", "")
	}
	if source == "" {
		if ids := getPayloadList(point.Payload, "source_ids"); len(ids) > 0 {
			source = ids[0]
		}
	}
	if source == "" {
		source = "unknown"
	}

	fragment := buildFragment(point.Payload)
	normalizedID, rawPointID, namespace, _ := normalizePointID(point.Id)

	meta := models.VectorMetadata{
		ID:          normalizedID,
		RawPointID:  rawPointID,
		IDNamespace: namespace,
		Timestamp:   extractPointTimestamp(point.Payload),
		Fragment:    fragment,
		Source:      source,
		Layer:       getPayloadString(point.Payload, "layer", getPayloadString(point.Payload, "tone", "surface")),
		RunID:       getPayloadString(point.Payload, "run_id", ""),
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

func buildFragment(payload map[string]*qdrant.Value) string {
	var parts []string

	if s := getPayloadString(payload, "canonical_statement", ""); s != "" {
		parts = append(parts, s)
	} else if s := getPayloadString(payload, "statement", ""); s != "" {
		parts = append(parts, s)
	} else if s := getPayloadString(payload, "summary", ""); s != "" {
		parts = append(parts, s)
	}

	// keystones-specific fields (canon / recursive pass)
	if s := getPayloadString(payload, "one_liner", ""); s != "" {
		parts = append(parts, s)
	}
	if s := getPayloadString(payload, "concept", ""); s != "" {
		parts = append(parts, "Concept: "+s)
	}
	if themes := getPayloadList(payload, "themes"); len(themes) > 0 {
		parts = append(parts, "Themes: "+joinMax(themes, 6))
	}

	if claims := getPayloadList(payload, "claims"); len(claims) > 0 {
		for i, c := range claims {
			if i >= 3 {
				break
			}
			parts = append(parts, c)
		}
	}

	if concepts := getPayloadList(payload, "concepts"); len(concepts) > 0 {
		parts = append(parts, "Concepts: "+joinMax(concepts, 6))
	}

	if echoes := getPayloadList(payload, "echoes"); len(echoes) > 0 {
		parts = append(parts, "Echoes: "+joinMax(echoes, 4))
	}

	if questions := getPayloadList(payload, "questions"); len(questions) > 0 {
		if len(questions) > 2 {
			questions = questions[:2]
		}
		for _, q := range questions {
			parts = append(parts, q)
		}
	}

	if tags := getPayloadList(payload, "tags"); len(tags) > 0 {
		parts = append(parts, "Tags: "+joinMax(tags, 6))
	}

	if len(parts) == 0 {
		if s := getPayloadString(payload, "report", ""); s != "" {
			parts = append(parts, truncate(s, 300))
		}
		if s := getPayloadString(payload, "verdict", ""); s != "" {
			parts = append(parts, "Verdict: "+truncate(s, 150))
		}
	}

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

func getPayloadList(payload map[string]*qdrant.Value, key string) []string {
	val, ok := payload[key]
	if !ok || val == nil {
		return nil
	}
	list := val.GetListValue()
	if list == nil {
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

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func normalizePointID(id *qdrant.PointId) (uint64, string, string, bool) {
	if id == nil {
		return 0, "", "", false
	}
	uuid := strings.TrimSpace(id.GetUuid())
	if uuid != "" {
		raw := "uuid:" + strings.ToLower(uuid)
		return stablePointIDHash(raw), raw, idNamespaceUUID, true
	}
	n := id.GetNum()
	raw := "num:" + strconv.FormatUint(n, 10)
	return stablePointIDHash(raw), raw, idNamespaceNumeric, true
}

func stablePointIDHash(raw string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(raw))
	return h.Sum64()
}

func pointIDToUint64(id *qdrant.PointId) uint64 {
	normalized, _, _, ok := normalizePointID(id)
	if !ok {
		return 0
	}
	return normalized
}

func extractPointTimestamp(payload map[string]*qdrant.Value) int64 {
	keys := []string{"timestamp", "ts", "created_at", "updated_at", "date", "datetime", "time"}
	for _, key := range keys {
		val, ok := payload[key]
		if !ok || val == nil {
			continue
		}
		if n := val.GetIntegerValue(); n != 0 {
			return normalizeUnixTimestamp(n)
		}
		if f := val.GetDoubleValue(); f != 0 {
			return normalizeUnixTimestamp(int64(f))
		}
		if s := strings.TrimSpace(val.GetStringValue()); s != "" {
			if n, err := strconv.ParseInt(s, 10, 64); err == nil {
				return normalizeUnixTimestamp(n)
			}
			for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
				if t, err := time.Parse(layout, s); err == nil {
					return t.Unix()
				}
			}
		}
	}
	return 0
}

func normalizeUnixTimestamp(v int64) int64 {
	if v <= 0 {
		return 0
	}
	if v > 1_000_000_000_000 {
		return v / 1000
	}
	return v
}

type IDNormalizationAudit struct {
	Total              int
	NumericIDs         int
	UUIDIDs            int
	MissingRawID       int
	MissingNamespace   int
	InvalidUUID        int
	HashCollisions     int
	NonDeterministicID int
}

func (a IDNormalizationAudit) Summary() string {
	return fmt.Sprintf(
		"total=%d numeric=%d uuid=%d missing_raw=%d missing_namespace=%d invalid_uuid=%d collisions=%d nondeterministic=%d",
		a.Total,
		a.NumericIDs,
		a.UUIDIDs,
		a.MissingRawID,
		a.MissingNamespace,
		a.InvalidUUID,
		a.HashCollisions,
		a.NonDeterministicID,
	)
}

func AuditIDNormalization(metadata []models.VectorMetadata) IDNormalizationAudit {
	audit := IDNormalizationAudit{Total: len(metadata)}
	seen := make(map[uint64]string, len(metadata))
	for _, m := range metadata {
		raw := strings.TrimSpace(m.RawPointID)
		ns := strings.TrimSpace(m.IDNamespace)
		if raw == "" {
			audit.MissingRawID++
		}
		if ns == "" {
			audit.MissingNamespace++
		}
		switch ns {
		case idNamespaceNumeric:
			audit.NumericIDs++
		case idNamespaceUUID:
			audit.UUIDIDs++
			if !isUUIDRaw(raw) {
				audit.InvalidUUID++
			}
		}
		if raw != "" {
			expected := stablePointIDHash(raw)
			if expected != m.ID {
				audit.NonDeterministicID++
			}
		}
		if prev, ok := seen[m.ID]; ok && raw != "" && prev != raw {
			audit.HashCollisions++
		} else if raw != "" {
			seen[m.ID] = raw
		}
	}
	return audit
}

func isUUIDRaw(raw string) bool {
	raw = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(raw)), "uuid:")
	stripped := strings.ReplaceAll(raw, "-", "")
	if len(stripped) != 32 {
		return false
	}
	for _, c := range stripped {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

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
