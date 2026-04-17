package reasoner

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/meistro57/vectoreologist/internal/models"
)

// ---- helpers ----------------------------------------------------------------

// newTestServer spins up an httptest server that always responds with the
// provided status code and body.
func newTestServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
}

// chatResponse builds a minimal OpenAI-compatible chat completion JSON.
// If reasoning is non-empty it adds the reasoning_content field (R1 format).
func chatResponse(content, reasoning string) string {
	msg := map[string]interface{}{
		"role":    "assistant",
		"content": content,
	}
	if reasoning != "" {
		msg["reasoning_content"] = reasoning
	}
	resp := map[string]interface{}{
		"choices": []interface{}{
			map[string]interface{}{"message": msg},
		},
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

// ---- callDeepSeek -----------------------------------------------------------

func TestCallDeepSeek_SuccessWithReasoningContent(t *testing.T) {
	srv := newTestServer(t, 200, chatResponse("my conclusion", "my thinking"))
	defer srv.Close()

	r := New2(srv.URL, "test-key", "deepseek-reasoner")
	resp, err := r.callDeepSeek("test prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.conclusion != "my conclusion" {
		t.Errorf("conclusion: want %q, got %q", "my conclusion", resp.conclusion)
	}
	if resp.thinking != "my thinking" {
		t.Errorf("thinking: want %q, got %q", "my thinking", resp.thinking)
	}
}

func TestCallDeepSeek_SuccessWithoutReasoningContent(t *testing.T) {
	// Plain chat format — no reasoning_content field.
	srv := newTestServer(t, 200, chatResponse("just the answer", ""))
	defer srv.Close()

	r := New2(srv.URL, "test-key", "deepseek-chat")
	resp, err := r.callDeepSeek("test prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.conclusion != "just the answer" {
		t.Errorf("conclusion: want %q, got %q", "just the answer", resp.conclusion)
	}
	if resp.thinking != "" {
		t.Errorf("thinking should be empty, got %q", resp.thinking)
	}
}

func TestCallDeepSeek_MalformedJSON(t *testing.T) {
	srv := newTestServer(t, 200, `this is not json {{{`)
	defer srv.Close()

	r := New2(srv.URL, "test-key", "deepseek-chat")
	_, err := r.callDeepSeek("test prompt")
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
	if !strings.Contains(err.Error(), "bad JSON") {
		t.Errorf("expected 'bad JSON' in error, got: %v", err)
	}
}

func TestCallDeepSeek_EmptyChoicesArray(t *testing.T) {
	body := `{"choices": []}`
	srv := newTestServer(t, 200, body)
	defer srv.Close()

	r := New2(srv.URL, "test-key", "deepseek-chat")
	_, err := r.callDeepSeek("test prompt")
	if err == nil {
		t.Fatal("expected error for empty choices, got nil")
	}
	if !strings.Contains(err.Error(), "no choices") {
		t.Errorf("expected 'no choices' in error, got: %v", err)
	}
}

func TestCallDeepSeek_MissingChoicesKey(t *testing.T) {
	body := `{"result": "ok"}`
	srv := newTestServer(t, 200, body)
	defer srv.Close()

	r := New2(srv.URL, "test-key", "deepseek-chat")
	_, err := r.callDeepSeek("test prompt")
	if err == nil {
		t.Fatal("expected error when choices key is absent, got nil")
	}
}

func TestCallDeepSeek_HTTPError(t *testing.T) {
	srv := newTestServer(t, 500, `{"error": "internal server error"}`)
	defer srv.Close()

	r := New2(srv.URL, "test-key", "deepseek-chat")
	// A 500 still returns a body; the body will be invalid for our parser.
	// The important thing is that the function does not panic and returns an error.
	_, err := r.callDeepSeek("test prompt")
	if err == nil {
		t.Fatal("expected an error on 500 response, got nil")
	}
}

func TestCallDeepSeek_NetworkTimeout(t *testing.T) {
	// Server that never responds (hangs until the client times out).
	hangSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
	}))
	defer hangSrv.Close()

	r := New2(hangSrv.URL, "test-key", "deepseek-chat")
	// Override the client timeout to something very short so the test doesn't take 5 min.
	r.client = &http.Client{Timeout: 50 * time.Millisecond}

	_, err := r.callDeepSeek("test prompt")
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestCallDeepSeek_RequestHasAuthHeader(t *testing.T) {
	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(chatResponse("ok", "")))
	}))
	defer srv.Close()

	r := New2(srv.URL, "secret-key", "deepseek-chat")
	r.callDeepSeek("hi")

	if capturedAuth != "Bearer secret-key" {
		t.Errorf("Authorization header: want %q, got %q", "Bearer secret-key", capturedAuth)
	}
}

func TestCallDeepSeek_RequestPath(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(chatResponse("ok", "")))
	}))
	defer srv.Close()

	r := New2(srv.URL, "key", "deepseek-chat")
	r.callDeepSeek("hi")

	if capturedPath != "/chat/completions" {
		t.Errorf("path: want /chat/completions, got %q", capturedPath)
	}
}

// ---- formatForReport --------------------------------------------------------

func TestFormatForReport_WithThinking(t *testing.T) {
	resp := &deepSeekResponse{thinking: "the plan", conclusion: "the result"}
	got := formatForReport(resp)
	if !strings.Contains(got, "**Thinking:**") {
		t.Error("expected '**Thinking:**' section")
	}
	if !strings.Contains(got, "the plan") {
		t.Error("expected thinking text in output")
	}
	if !strings.Contains(got, "**Conclusion:**") {
		t.Error("expected '**Conclusion:**' section")
	}
	if !strings.Contains(got, "the result") {
		t.Error("expected conclusion text in output")
	}
}

func TestFormatForReport_WithoutThinking(t *testing.T) {
	resp := &deepSeekResponse{thinking: "", conclusion: "plain answer"}
	got := formatForReport(resp)
	if got != "plain answer" {
		t.Errorf("want %q, got %q", "plain answer", got)
	}
	if strings.Contains(got, "Thinking") {
		t.Error("should not contain Thinking section when thinking is empty")
	}
}

// ---- prompt builders --------------------------------------------------------

func TestBuildClusterPrompt_ContainsFields(t *testing.T) {
	c := models.Cluster{ID: 7, Label: "my label", Size: 42, Density: 0.65, Coherence: 0.88}
	p := buildClusterPrompt(c, nil)
	checks := []string{"7", "my label", "42", "0.65", "0.88"}
	for _, s := range checks {
		if !strings.Contains(p, s) {
			t.Errorf("cluster prompt missing %q\nfull prompt: %s", s, p)
		}
	}
}

func TestBuildClusterPrompt_IncludesSnippets(t *testing.T) {
	c := models.Cluster{ID: 3, Label: "test", Size: 5, Density: 0.5, Coherence: 0.7}
	snippets := []string{"quantum gates", "error correction"}
	p := buildClusterPrompt(c, snippets)
	for _, want := range snippets {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing snippet %q\nfull prompt: %s", want, p)
		}
	}
	if !strings.Contains(p, "**Conclusion:**") {
		t.Error("prompt should ask for **Conclusion:** paragraph")
	}
}

func TestBuildBridgePrompt_ContainsFields(t *testing.T) {
	b := models.Bridge{ClusterA: 3, ClusterB: 9, Strength: 0.72, LinkType: "strong_semantic"}
	p := buildBridgePrompt(b)
	for _, s := range []string{"3", "9", "0.72", "strong_semantic"} {
		if !strings.Contains(p, s) {
			t.Errorf("bridge prompt missing %q", s)
		}
	}
}

func TestBuildMoatPrompt_ContainsFields(t *testing.T) {
	m := models.Moat{ClusterA: 1, ClusterB: 5, Distance: 0.95}
	p := buildMoatPrompt(m)
	for _, s := range []string{"1", "5", "0.95"} {
		if !strings.Contains(p, s) {
			t.Errorf("moat prompt missing %q", s)
		}
	}
}

// ---- ReasonAboutTopology integration (mocked) -------------------------------

func TestReasonAboutTopology_ProducesFindings(t *testing.T) {
	// One cluster, no bridges or moats → 1 API call → 1 finding.
	srv := newTestServer(t, 200, chatResponse("cluster insight", ""))
	defer srv.Close()

	r := New2(srv.URL, "key", "deepseek-chat")
	clusters := []models.Cluster{
		{ID: 1, Label: "test cluster", Size: 10, Coherence: 0.6},
	}
	findings := r.ReasonAboutTopology(clusters, nil, nil, nil)
	if len(findings) != 1 {
		t.Fatalf("want 1 finding, got %d", len(findings))
	}
	if findings[0].Type != "cluster_analysis" {
		t.Errorf("type: want cluster_analysis, got %q", findings[0].Type)
	}
}

func TestReasonAboutTopology_BridgesAndMoatsTruncated(t *testing.T) {
	// 12 bridges → only top 10 should be reasoned about;
	// 7 moats → only top 5 should be reasoned about.
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(chatResponse("ok", "")))
	}))
	defer srv.Close()

	r := New2(srv.URL, "key", "deepseek-chat")

	var bridges []models.Bridge
	for i := 0; i < 12; i++ {
		bridges = append(bridges, models.Bridge{ClusterA: i, ClusterB: i + 1, Strength: float64(i) * 0.05})
	}
	var moats []models.Moat
	for i := 0; i < 7; i++ {
		moats = append(moats, models.Moat{ClusterA: i, ClusterB: i + 10, Distance: float64(i) * 0.1})
	}

	findings := r.ReasonAboutTopology(nil, bridges, moats, nil)

	// 0 clusters + 10 bridges + 5 moats = 15 calls
	if callCount != 15 {
		t.Errorf("expected 15 API calls (0+10+5), got %d", callCount)
	}
	if len(findings) != 15 {
		t.Errorf("expected 15 findings, got %d", len(findings))
	}
}

func TestReasonAboutTopology_SkipsOnAPIError(t *testing.T) {
	// Server always returns error JSON → findings should be empty (errors are skipped).
	srv := newTestServer(t, 200, `{not valid json`)
	defer srv.Close()

	r := New2(srv.URL, "key", "deepseek-chat")
	clusters := []models.Cluster{{ID: 1, Label: "x", Coherence: 0.7}}
	findings := r.ReasonAboutTopology(clusters, nil, nil, nil)
	if len(findings) != 0 {
		t.Errorf("errors should be skipped; want 0 findings, got %d", len(findings))
	}
}

func TestReasonAboutTopology_IsAnomalySetForLowCoherence(t *testing.T) {
	srv := newTestServer(t, 200, chatResponse("ok", ""))
	defer srv.Close()

	r := New2(srv.URL, "key", "deepseek-chat")
	clusters := []models.Cluster{
		{ID: 1, Label: "coherent", Coherence: 0.8},
		{ID: 2, Label: "incoherent", Coherence: 0.3},
	}
	findings := r.ReasonAboutTopology(clusters, nil, nil, nil)
	if len(findings) != 2 {
		t.Fatalf("want 2 findings, got %d", len(findings))
	}

	// Find by subject to be order-independent.
	bySubject := make(map[string]models.Finding)
	for _, f := range findings {
		bySubject[f.Subject] = f
	}
	if bySubject["Cluster 1: coherent"].IsAnomaly {
		t.Error("coherent cluster should not be IsAnomaly")
	}
	if !bySubject["Cluster 2: incoherent"].IsAnomaly {
		t.Error("incoherent cluster should be IsAnomaly")
	}
}
