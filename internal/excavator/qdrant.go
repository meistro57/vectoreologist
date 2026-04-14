package excavator

import (
	"context"
	"fmt"

	"github.com/meistro57/vectoreologist/internal/models"
	qdrant "github.com/qdrant/go-client/qdrant"
)

type Excavator struct {
	client *qdrant.Client
}

func New(url string) *Excavator {
	client, err := qdrant.NewClient(&qdrant.Config{
		Host: url,
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
	points, err := e.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: collectionName,
		Limit:          uint32(limit),
		WithVectors:    qdrant.NewWithVectors(true),
		WithPayload:    qdrant.NewWithPayload(true),
	})
	
	if err != nil {
		return nil, nil, fmt.Errorf("scroll failed: %w", err)
	}

	vectors := make([][]float32, 0, len(points))
	metadata := make([]models.VectorMetadata, 0, len(points))

	for _, point := range points {
		// Extract vector
		vec := point.Vectors.GetVector().Data
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

func getPayloadString(payload map[string]*qdrant.Value, key, defaultVal string) string {
	if val, ok := payload[key]; ok {
		if str := val.GetStringValue(); str != "" {
			return str
		}
	}
	return defaultVal
}
