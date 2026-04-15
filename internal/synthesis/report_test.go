package synthesis

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/meistro57/vectoreologist/internal/models"
)

// newTestSynthesizer creates a Synthesizer directly, bypassing the Qdrant
// constructor that would dial a live gRPC endpoint.  This is safe because
// none of the functions under test access the client field.
func newTestSynthesizer(outputPath string) *Synthesizer {
	return &Synthesizer{
		qdrantURL:  "http://localhost:6333",
		outputPath: outputPath,
		client:     nil, // not used by GenerateReport
	}
}

// ---- GenerateReport ---------------------------------------------------------

func TestGenerateReport_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	s := newTestSynthesizer(dir)

	path := s.GenerateReport(nil, nil, nil, nil)
	if path == "" {
		t.Fatal("GenerateReport returned empty path")
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("report file does not exist at %s", path)
	}
}

func TestGenerateReport_FilePathIsInsideOutputDir(t *testing.T) {
	dir := t.TempDir()
	s := newTestSynthesizer(dir)

	path := s.GenerateReport(nil, nil, nil, nil)
	// Ensure path is under our output directory.
	rel, err := filepath.Rel(dir, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		t.Errorf("report path %q is not under output dir %q", path, dir)
	}
}

func TestGenerateReport_ContainsTitleAndTimestamp(t *testing.T) {
	dir := t.TempDir()
	s := newTestSynthesizer(dir)

	path := s.GenerateReport(nil, nil, nil, nil)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read report: %v", err)
	}

	body := string(content)
	if !strings.Contains(body, "# Vectoreology Report") {
		t.Error("report missing title '# Vectoreology Report'")
	}
	if !strings.Contains(body, "**Generated:**") {
		t.Error("report missing **Generated:** timestamp")
	}
}

func TestGenerateReport_TopologySummaryCounts(t *testing.T) {
	dir := t.TempDir()
	s := newTestSynthesizer(dir)

	clusters := []models.Cluster{
		{ID: 1}, {ID: 2}, {ID: 3},
	}
	bridges := []models.Bridge{
		{ClusterA: 1, ClusterB: 2},
	}
	moats := []models.Moat{
		{ClusterA: 1, ClusterB: 3},
		{ClusterA: 2, ClusterB: 3},
	}

	path := s.GenerateReport(nil, clusters, bridges, moats)
	content, _ := os.ReadFile(path)
	body := string(content)

	if !strings.Contains(body, "**Clusters:** 3") {
		t.Errorf("report should say Clusters: 3\n%s", body)
	}
	if !strings.Contains(body, "**Bridges:** 1") {
		t.Errorf("report should say Bridges: 1\n%s", body)
	}
	if !strings.Contains(body, "**Moats:** 2") {
		t.Errorf("report should say Moats: 2\n%s", body)
	}
}

func TestGenerateReport_ClusterAnalysisSection(t *testing.T) {
	dir := t.TempDir()
	s := newTestSynthesizer(dir)

	findings := []models.Finding{
		{
			Type:           "cluster_analysis",
			Subject:        "Cluster 1: science",
			ReasoningChain: "This represents scientific concepts.",
		},
	}

	path := s.GenerateReport(findings, nil, nil, nil)
	content, _ := os.ReadFile(path)
	body := string(content)

	if !strings.Contains(body, "## Cluster Analysis") {
		t.Error("missing '## Cluster Analysis' section")
	}
	if !strings.Contains(body, "### Cluster 1: science") {
		t.Error("missing cluster finding heading")
	}
	if !strings.Contains(body, "This represents scientific concepts.") {
		t.Error("missing reasoning chain text")
	}
}

func TestGenerateReport_BridgeAnalysisSection(t *testing.T) {
	dir := t.TempDir()
	s := newTestSynthesizer(dir)

	findings := []models.Finding{
		{
			Type:           "bridge_analysis",
			Subject:        "Bridge: 1 ↔ 2",
			ReasoningChain: "Connected by shared semantics.",
		},
	}

	path := s.GenerateReport(findings, nil, nil, nil)
	content, _ := os.ReadFile(path)
	body := string(content)

	if !strings.Contains(body, "## Semantic Bridges") {
		t.Error("missing '## Semantic Bridges' section")
	}
	if !strings.Contains(body, "### Bridge: 1 ↔ 2") {
		t.Error("missing bridge finding heading")
	}
	if !strings.Contains(body, "Connected by shared semantics.") {
		t.Error("missing bridge reasoning chain")
	}
}

func TestGenerateReport_MoatAnalysisSection(t *testing.T) {
	dir := t.TempDir()
	s := newTestSynthesizer(dir)

	findings := []models.Finding{
		{
			Type:           "moat_analysis",
			Subject:        "Moat: 1 ⊥ 3",
			ReasoningChain: "No shared vocabulary.",
		},
	}

	path := s.GenerateReport(findings, nil, nil, nil)
	content, _ := os.ReadFile(path)
	body := string(content)

	if !strings.Contains(body, "## Knowledge Moats") {
		t.Error("missing '## Knowledge Moats' section")
	}
	if !strings.Contains(body, "### Moat: 1 ⊥ 3") {
		t.Error("missing moat finding heading")
	}
	if !strings.Contains(body, "No shared vocabulary.") {
		t.Error("missing moat reasoning chain")
	}
}

func TestGenerateReport_FiltersFindingsByType(t *testing.T) {
	// An "anomaly" type finding should NOT appear in any section.
	dir := t.TempDir()
	s := newTestSynthesizer(dir)

	findings := []models.Finding{
		{Type: "cluster_analysis", Subject: "Cluster A", ReasoningChain: "cluster text"},
		{Type: "coherence_anomaly", Subject: "Anomaly X", ReasoningChain: "anomaly text"},
	}

	path := s.GenerateReport(findings, nil, nil, nil)
	content, _ := os.ReadFile(path)
	body := string(content)

	if strings.Contains(body, "anomaly text") {
		t.Error("anomaly finding should not appear in the report sections")
	}
	if !strings.Contains(body, "cluster text") {
		t.Error("cluster finding should appear in the report")
	}
}

func TestGenerateReport_EmptyFindingsStillProducesStructure(t *testing.T) {
	dir := t.TempDir()
	s := newTestSynthesizer(dir)

	path := s.GenerateReport(nil, nil, nil, nil)
	content, _ := os.ReadFile(path)
	body := string(content)

	for _, section := range []string{
		"## Topology Summary",
		"## Cluster Analysis",
		"## Semantic Bridges",
		"## Knowledge Moats",
	} {
		if !strings.Contains(body, section) {
			t.Errorf("missing section %q in empty report", section)
		}
	}
}

func TestGenerateReport_OutputDirCreatedIfAbsent(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "sub", "dir")
	// The nested directory does not exist yet.
	s := newTestSynthesizer(nested)

	path := s.GenerateReport(nil, nil, nil, nil)
	if path == "" {
		t.Fatal("GenerateReport returned empty path")
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("report not created in missing directory: %s", path)
	}
}

// ---- hostname helper (shared with excavator; tested here for the synthesis copy) ----

func TestHostname_StripScheme(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"http://localhost:6333", "localhost"},
		{"https://qdrant.example.com:6334", "qdrant.example.com"},
		{"bare-hostname", "bare-hostname"},
		{"", ""},
	}
	for _, tc := range tests {
		got := hostname(tc.input)
		if got != tc.want {
			t.Errorf("hostname(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// ---- StoreFindings ----------------------------------------------------------

func TestStoreFindings_NoopReturnsNil(t *testing.T) {
	// StoreFindings is a stub; it must not panic and must return nil.
	s := newTestSynthesizer(t.TempDir())
	findings := []models.Finding{
		{Type: "cluster_analysis", Subject: "x"},
	}
	if err := s.StoreFindings(findings); err != nil {
		t.Errorf("StoreFindings should return nil (stub), got: %v", err)
	}
}

func TestStoreFindings_EmptySlice(t *testing.T) {
	s := newTestSynthesizer(t.TempDir())
	if err := s.StoreFindings(nil); err != nil {
		t.Errorf("StoreFindings with nil should return nil, got: %v", err)
	}
}
