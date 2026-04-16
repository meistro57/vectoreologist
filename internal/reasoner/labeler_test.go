package reasoner

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/meistro57/vectoreologist/internal/models"
)

func TestLabelClusters_ReplacesLabel(t *testing.T) {
	srv := newTestServer(t, 200, chatResponse("Quantum Error Correction", ""))
	defer srv.Close()

	r := New2(srv.URL, "key", "deepseek-reasoner")
	clusters := []models.Cluster{
		{ID: 1, Label: "layer/source", VectorIDs: []uint64{1, 2}},
	}
	metadata := []models.VectorMetadata{
		{ID: 1, Fragment: "quantum gates and error correction codes"},
		{ID: 2, Fragment: "stabilizer codes and surface codes"},
	}

	labeled := r.LabelClusters(clusters, metadata)
	if len(labeled) != 1 {
		t.Fatalf("want 1 cluster, got %d", len(labeled))
	}
	if labeled[0].Label != "Quantum Error Correction" {
		t.Errorf("label: want %q, got %q", "Quantum Error Correction", labeled[0].Label)
	}
}

func TestLabelClusters_SkipsEmptyFragments(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(chatResponse("some label", "")))
	}))
	defer srv.Close()

	r := New2(srv.URL, "key", "deepseek-chat")
	clusters := []models.Cluster{
		{ID: 1, Label: "original", VectorIDs: []uint64{1, 2}},
	}
	metadata := []models.VectorMetadata{
		{ID: 1, Fragment: ""},
		{ID: 2, Fragment: "N/A"},
	}

	labeled := r.LabelClusters(clusters, metadata)
	if callCount != 0 {
		t.Errorf("expected 0 API calls for empty/N/A fragments, got %d", callCount)
	}
	if labeled[0].Label != "original" {
		t.Errorf("label should be unchanged, got %q", labeled[0].Label)
	}
}

func TestLabelClusters_AlwaysUsesDeepseekChat(t *testing.T) {
	var capturedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		capturedBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(chatResponse("label", "")))
	}))
	defer srv.Close()

	// Configured with deepseek-reasoner — labeler should still use deepseek-chat.
	r := New2(srv.URL, "key", "deepseek-reasoner")
	clusters := []models.Cluster{
		{ID: 1, Label: "old", VectorIDs: []uint64{1}},
	}
	metadata := []models.VectorMetadata{
		{ID: 1, Fragment: "some text fragment"},
	}

	r.LabelClusters(clusters, metadata)

	if !strings.Contains(capturedBody, "deepseek-chat") {
		t.Errorf("labeler should use deepseek-chat, got request body: %s", capturedBody)
	}
}

func TestLabelClusters_OriginalsNotModified(t *testing.T) {
	srv := newTestServer(t, 200, chatResponse("New Label", ""))
	defer srv.Close()

	r := New2(srv.URL, "key", "deepseek-chat")
	original := []models.Cluster{
		{ID: 1, Label: "original label", VectorIDs: []uint64{1}},
	}
	metadata := []models.VectorMetadata{
		{ID: 1, Fragment: "text"},
	}

	_ = r.LabelClusters(original, metadata)

	if original[0].Label != "original label" {
		t.Errorf("original slice was mutated; want %q, got %q", "original label", original[0].Label)
	}
}

func TestLabelClusters_APIErrorKeepsOriginalLabel(t *testing.T) {
	srv := newTestServer(t, 200, `{bad json`)
	defer srv.Close()

	r := New2(srv.URL, "key", "deepseek-chat")
	clusters := []models.Cluster{
		{ID: 1, Label: "keep me", VectorIDs: []uint64{1}},
	}
	metadata := []models.VectorMetadata{
		{ID: 1, Fragment: "some text"},
	}

	labeled := r.LabelClusters(clusters, metadata)
	if labeled[0].Label != "keep me" {
		t.Errorf("label should be unchanged on error, got %q", labeled[0].Label)
	}
}

func TestBuildLabelPrompt_ContainsFragments(t *testing.T) {
	c := models.Cluster{ID: 3, Size: 15, Coherence: 0.72}
	samples := []string{"vector embeddings", "semantic search"}
	p := buildLabelPrompt(c, samples)

	for _, want := range []string{"15", "0.72", "vector embeddings", "semantic search"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q\nfull prompt:\n%s", want, p)
		}
	}
}
