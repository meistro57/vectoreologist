package excavator

import (
	"testing"

	"github.com/meistro57/vectoreologist/internal/models"
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

func buildNamedPoint(id uint64, vectors map[string][]float32, payload map[string]string) *qdrant.RetrievedPoint {
	p := &qdrant.RetrievedPoint{
		Id:      qdrant.NewIDNum(id),
		Payload: make(map[string]*qdrant.Value, len(payload)),
	}
	for k, v := range payload {
		p.Payload[k] = &qdrant.Value{Kind: &qdrant.Value_StringValue{StringValue: v}}
	}
	named := make(map[string]*qdrant.VectorOutput, len(vectors))
	for name, data := range vectors {
		named[name] = &qdrant.VectorOutput{Data: data}
	}
	p.Vectors = &qdrant.VectorsOutput{
		VectorsOptions: &qdrant.VectorsOutput_Vectors{
			Vectors: &qdrant.NamedVectorsOutput{Vectors: named},
		},
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

	got, meta, ok := extractPoint(pt, "", false)
	if !ok {
		t.Fatal("expected ok=true, got false")
	}
	if len(got) != 3 {
		t.Fatalf("vector length: want 3, got %d", len(got))
	}
	wantID := stablePointIDHash("num:42")
	if meta.ID != wantID {
		t.Errorf("ID: want %d, got %d", wantID, meta.ID)
	}
	if meta.RawPointID != "num:42" {
		t.Errorf("RawPointID: want num:42, got %q", meta.RawPointID)
	}
	if meta.IDNamespace != idNamespaceNumeric {
		t.Errorf("IDNamespace: want %q, got %q", idNamespaceNumeric, meta.IDNamespace)
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

	_, _, ok := extractPoint(pt, "", false)
	if ok {
		t.Error("expected ok=false for point with no vector, got true")
	}
}

func TestExtractPoint_DefaultPayload(t *testing.T) {
	vec := []float32{0.1}
	pt := buildPoint(1, vec, nil) // no payload keys

	_, meta, ok := extractPoint(pt, "", false)
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

	_, meta, ok := extractPoint(pt, "", false)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if meta.RunID != "run-abc" {
		t.Errorf("RunID: want 'run-abc', got %q", meta.RunID)
	}
}

func TestExtractPoint_NamedVectorByName(t *testing.T) {
	pt := buildNamedPoint(55, map[string][]float32{
		"claims_vec":  {1, 2, 3},
		"summary_vec": {4, 5, 6},
	}, map[string]string{"text": "hello"})

	got, _, ok := extractPoint(pt, "summary_vec", false)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if len(got) != 3 || got[0] != 4 || got[1] != 5 || got[2] != 6 {
		t.Fatalf("vector mismatch: got %v", got)
	}
}

func TestExtractPoint_NamedVectorCombine(t *testing.T) {
	pt := buildNamedPoint(56, map[string][]float32{
		"claims_vec":  {2, 4, 6},
		"summary_vec": {4, 6, 8},
	}, map[string]string{"text": "hello"})

	got, _, ok := extractPoint(pt, "", true)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if len(got) != 3 || got[0] != 3 || got[1] != 5 || got[2] != 7 {
		t.Fatalf("vector mismatch: got %v", got)
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

func TestPointIDToUint64_UUID(t *testing.T) {
	id := qdrant.NewIDUUID("123e4567-e89b-12d3-a456-426614174000")
	got := pointIDToUint64(id)
	want := stablePointIDHash("uuid:123e4567-e89b-12d3-a456-426614174000")
	if got != want {
		t.Fatalf("pointIDToUint64(UUID) = %d, want %d", got, want)
	}
}

func TestPointIDToUint64_InvalidUUIDStillDeterministic(t *testing.T) {
	id := qdrant.NewIDUUID("bad")
	want := stablePointIDHash("uuid:bad")
	if got := pointIDToUint64(id); got != want {
		t.Fatalf("pointIDToUint64(invalid UUID) = %d, want %d", got, want)
	}
}

func TestNormalizePointID_NumericNamespace(t *testing.T) {
	normalized, raw, ns, ok := normalizePointID(qdrant.NewIDNum(99))
	if !ok {
		t.Fatal("expected ok")
	}
	if raw != "num:99" {
		t.Fatalf("raw = %q", raw)
	}
	if ns != idNamespaceNumeric {
		t.Fatalf("namespace = %q", ns)
	}
	if normalized != stablePointIDHash(raw) {
		t.Fatalf("normalized mismatch")
	}
}

func TestMetadataPointID_UUIDAndNumeric(t *testing.T) {
	uuidMeta := models.VectorMetadata{RawPointID: "uuid:123e4567-e89b-12d3-a456-426614174000", IDNamespace: idNamespaceUUID}
	numMeta := models.VectorMetadata{RawPointID: "num:77", IDNamespace: idNamespaceNumeric}
	if got := metadataPointID(uuidMeta); got == nil || got.GetUuid() == "" {
		t.Fatal("expected UUID point id")
	}
	if got := metadataPointID(numMeta); got == nil || got.GetNum() != 77 {
		t.Fatal("expected numeric point id")
	}
}

func TestAuditIDNormalization_DetectsIssues(t *testing.T) {
	meta := []models.VectorMetadata{
		{ID: stablePointIDHash("num:1"), RawPointID: "num:1", IDNamespace: idNamespaceNumeric},
		{ID: stablePointIDHash("uuid:123e4567-e89b-12d3-a456-426614174000"), RawPointID: "uuid:123e4567-e89b-12d3-a456-426614174000", IDNamespace: idNamespaceUUID},
		{ID: 123, RawPointID: "num:2", IDNamespace: idNamespaceNumeric},
		{ID: 456, RawPointID: "uuid:not-a-uuid", IDNamespace: idNamespaceUUID},
		{ID: 789},
	}
	audit := AuditIDNormalization(meta)
	if audit.Total != 5 {
		t.Fatalf("total = %d", audit.Total)
	}
	if audit.NonDeterministicID == 0 {
		t.Fatal("expected nondeterministic IDs")
	}
	if audit.InvalidUUID == 0 {
		t.Fatal("expected invalid uuid count")
	}
	if audit.MissingRawID == 0 || audit.MissingNamespace == 0 {
		t.Fatal("expected missing raw/namespace counts")
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
