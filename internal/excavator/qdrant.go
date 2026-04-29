package excavator

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"github.com/meistro57/vectoreologist/internal/models"
	qdrant "github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
)

type Excavator struct {
	client *qdrant.Client
}

const maxMsgSize = 256 * 1024 * 1024 // 256 MB

func New(rawURL string) *Excavator {
	client, err := qdrant.NewClient(&qdrant.Config{
		Host: hostname(rawURL),
		GrpcOptions: []grpc.DialOption{
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMsgSize)),
		},
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to Qdrant: %v", err))
	}

	return &Excavator{client: client}
}

// CollectionSize returns the number of points in the named collection.
func (e *Excavator) CollectionSize(name string) (uint64, error) {
	info, err := e.client.GetCollectionInfo(context.Background(), name)
	if err != nil {
		return 0, fmt.Errorf("get collection info: %w", err)
	}
	return info.GetPointsCount(), nil
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
			vec, meta, ok := extractPoint(point)
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
func extractPoint(point *qdrant.RetrievedPoint) ([]float32, models.VectorMetadata, bool) {
	var vec []float32
	if v := point.Vectors.GetVector(); v != nil {
		if dense := v.GetDense(); dense != nil {
			vec = dense.Data
		} else {
			vec = v.Data // fallback for pre-1.12 servers
		}
	} else if named := point.Vectors.GetVectors(); named != nil {
		for _, nv := range named.GetVectors() {
			if dense := nv.GetDense(); dense != nil {
				vec = dense.Data
			} else {
				vec = nv.Data
			}
			break
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
		source = getPayloadString(point.Payload, "source_collection", "unknown")
	}

	// Use summary or claims as the fragment if text field is absent
	fragment := getPayloadString(point.Payload, "text", "")
	if fragment == "" {
		fragment = getPayloadString(point.Payload, "summary", "")
	}
	if fragment == "" {
		fragment = "N/A"
	}

	meta := models.VectorMetadata{
		ID:       point.Id.GetNum(),
		Fragment: fragment,
		Source:   source,
		Layer:    getPayloadString(point.Payload, "layer", getPayloadString(point.Payload, "tone", "surface")),
		RunID:    getPayloadString(point.Payload, "run_id", ""),
	}
	return vec, meta, true
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
