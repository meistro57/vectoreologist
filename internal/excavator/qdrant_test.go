package excavator

import (
	"testing"

	qdrant "github.com/qdrant/go-client/qdrant"
)

// buildPoint constructs a minimal RetrievedPoint for use in tests.
func buildPoint(id uint64, vec []float32, payload map[string]string) *qdrant.RetrievedPoint {
	p := &qdrant.RetrievedPoint{
		Id:      qdrant.NewIDNum(id),
		Payload: make(map[string]*qdrant.Value, len(payload)),
	}
	for k, v := range payload {
		p.Payload[k] = &qdrant.Value{Kind: &qdrant.Value_StringValue{StringValue: v}}
	}
	if len(vec) > 0 {
		p.Vectors = &qdrant.VectorsOutput{
			VectorsOptions: &qdrant.VectorsOutput_Vector{
				Vector: &qdrant.VectorOutput{
					Data: vec, // uses the pre-1.12 fallback path in extractPoint
				},
			},
		}
	}
	return p
}

// ============================================================
// extractPoint
// ============================================================

func TestExtractPoint_BasicVector(t *testing.T) {
	vec := []float32{1.0, 2.0, 3.0}
	pt := buildPoint(42, vec, map[string]string{
		"source": "mysource",
		"layer":  "deep",
		"text":   "hello world",
	})

	got, meta, ok := extractPoint(pt)
	if !ok {
		t.Fatal("expected ok=true, got false")
	}
	if len(got) != 3 {
		t.Fatalf("vector length: want 3, got %d", len(got))
	}
	if meta.ID != 42 {
		t.Errorf("ID: want 42, got %d", meta.ID)
	}
	if meta.Source != "mysource" {
		t.Errorf("Source: want mysource, got %s", meta.Source)
	}
	if meta.Layer != "deep" {
		t.Errorf("Layer: want deep, got %s", meta.Layer)
	}
	if meta.Fragment != "hello world" {
		t.Errorf("Fragment: want 'hello world', got %q", meta.Fragment)
	}
}

func TestExtractPoint_NoVector_ReturnsFalse(t *testing.T) {
	pt := buildPoint(7, nil, nil)

	_, _, ok := extractPoint(pt)
	if ok {
		t.Error("expected ok=false for point with no vector, got true")
	}
}

func TestExtractPoint_DefaultPayload(t *testing.T) {
	vec := []float32{0.1}
	pt := buildPoint(1, vec, nil) // no payload keys

	_, meta, ok := extractPoint(pt)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if meta.Source != "unknown" {
		t.Errorf("Source default: want 'unknown', got %q", meta.Source)
	}
	if meta.Layer != "surface" {
		t.Errorf("Layer default: want 'surface', got %q", meta.Layer)
	}
	if meta.Fragment != "N/A" {
		t.Errorf("Fragment default: want 'N/A', got %q", meta.Fragment)
	}
}

func TestExtractPoint_RunID(t *testing.T) {
	vec := []float32{1.0}
	pt := buildPoint(99, vec, map[string]string{"run_id": "run-abc"})

	_, meta, ok := extractPoint(pt)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if meta.RunID != "run-abc" {
		t.Errorf("RunID: want 'run-abc', got %q", meta.RunID)
	}
}

// ============================================================
// getPayloadString
// ============================================================

func TestGetPayloadString_Present(t *testing.T) {
	payload := map[string]*qdrant.Value{
		"key": {Kind: &qdrant.Value_StringValue{StringValue: "val"}},
	}
	got := getPayloadString(payload, "key", "default")
	if got != "val" {
		t.Errorf("want 'val', got %q", got)
	}
}

func TestGetPayloadString_Missing(t *testing.T) {
	got := getPayloadString(nil, "key", "fallback")
	if got != "fallback" {
		t.Errorf("want 'fallback', got %q", got)
	}
}

func TestGetPayloadString_EmptyString(t *testing.T) {
	payload := map[string]*qdrant.Value{
		"key": {Kind: &qdrant.Value_StringValue{StringValue: ""}},
	}
	got := getPayloadString(payload, "key", "default")
	if got != "default" {
		t.Errorf("empty string value should return default, got %q", got)
	}
}

// ============================================================
// hostname helper
// ============================================================

func TestHostname_StripsScheme(t *testing.T) {
	cases := []struct{ in, want string }{
		{"http://localhost:6333", "localhost"},
		{"https://my.qdrant.host", "my.qdrant.host"},
		{"localhost", "localhost"},
		{"bare-host", "bare-host"},
	}
	for _, tc := range cases {
		got := hostname(tc.in)
		if got != tc.want {
			t.Errorf("hostname(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
