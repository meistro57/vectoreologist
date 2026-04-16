package excavator

import (
	"context"
	"fmt"
	"net/url"

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

// Extract pulls vectors and metadata from a Qdrant collection
func (e *Excavator) Extract(collectionName string, limit int) ([][]float32, []models.VectorMetadata, error) {
	ctx := context.Background()
	
	// Scroll through collection to get points
	lim := uint32(limit)
	points, err := e.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: collectionName,
		Limit:          &lim,
		WithVectors:    qdrant.NewWithVectors(true),
		WithPayload:    qdrant.NewWithPayload(true),
	})
	
	if err != nil {
		return nil, nil, fmt.Errorf("scroll failed: %w", err)
	}

	vectors := make([][]float32, 0, len(points))
	metadata := make([]models.VectorMetadata, 0, len(points))

	for _, point := range points {
		// Extract vector — handle both unnamed and named vector collections.
		// In Qdrant ≥1.12 the dense vector lives in VectorOutput.GetDense().Data;
		// the top-level .Data field is deprecated and will be empty on newer servers.
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
			continue
		}
		vectors = append(vectors, vec)
		
		// Extract metadata
		meta := models.VectorMetadata{
			ID:        point.Id.GetNum(),
			Fragment:  getPayloadString(point.Payload, "text", "N/A"),
			Source:    getPayloadString(point.Payload, "source", "unknown"),
			Layer:     getPayloadString(point.Payload, "layer", "surface"),
			RunID:     getPayloadString(point.Payload, "run_id", ""),
		}
		metadata = append(metadata, meta)
	}

	return vectors, metadata, nil
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
