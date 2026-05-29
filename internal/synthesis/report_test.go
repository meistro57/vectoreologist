package synthesis

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/meistro57/vectoreologist/internal/models"
)

func newTestSynthesizer(outputPath string) *Synthesizer {
	return &Synthesizer{
		qdrantURL:  "http://localhost:6333",
		outputPath: outputPath,
		client:     nil,
	}
}

func TestGenerateReport_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	s := newTestSynthesizer(dir)

	path := s.GenerateReport(nil, nil, nil, nil, nil, "test")
	if path == "" {
		t.Fatal("GenerateReport returned empty path")
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("report file does not exist at %s", path)
	}
}

func TestGenerateReport_ContainsExecutiveSummaryAndRecommendations(t *testing.T) {
	dir := t.TempDir()
	s := newTestSynthesizer(dir)

	clusters := []models.Cluster{{ID: 1, Label: "surface / x", Source: "surface / x", VectorIDs: []uint64{1, 2, 3}, Size: 3, Density: 0.70, Coherence: 0.80}}
	metadata := []models.VectorMetadata{
		{ID: 1, Source: "mb_a", Fragment: "Alpha snippet one"},
		{ID: 2, Source: "mb_b", Fragment: "Alpha snippet two"},
		{ID: 3, Source: "mb_c", Fragment: "Alpha snippet three"},
	}
	findings := []models.Finding{{
		Type:           "topology_diagnostics",
		Subject:        "DBSCAN params (configured defaults): eps=0.300 minPts=5",
		ReasoningChain: "requested_eps=0.300 requested_minPts=5 sampled_pairs=0 reason=Using configured DBSCAN defaults.",
	}}

	path := s.GenerateReport(findings, clusters, nil, nil, metadata, "mb_test")
	content, _ := os.ReadFile(path)
	body := string(content)

	for _, section := range []string{"## Executive Summary", "## Cluster Analysis", "## Semantic Attractors", "## Recommendations"} {
		if !strings.Contains(body, section) {
			t.Fatalf("missing section %q", section)
		}
	}
	if !strings.Contains(body, "- Total clusters: 1") {
		t.Fatalf("summary missing cluster count:\n%s", body)
	}
	if !strings.Contains(body, "DBSCAN diagnostics") {
		t.Fatalf("summary missing dbscan diagnostics:\n%s", body)
	}
}

func TestGenerateReport_RemovesReasoningLeakage(t *testing.T) {
	dir := t.TempDir()
	s := newTestSynthesizer(dir)

	clusters := []models.Cluster{{ID: 1, Label: "surface / x", Source: "surface / x", VectorIDs: []uint64{1, 2, 3}, Size: 3, Density: 0.7, Coherence: 0.8}}
	metadata := []models.VectorMetadata{
		{ID: 1, Source: "mb_a", Fragment: "Snippet one"},
		{ID: 2, Source: "mb_b", Fragment: "Snippet two"},
		{ID: 3, Source: "mb_c", Fragment: "Snippet three"},
	}
	findings := []models.Finding{{
		Type:           "cluster_analysis",
		Subject:        "Cluster 1: surface / x",
		Clusters:       []int{1},
		ReasoningChain: "We need to analyze this cluster. I'll produce a final answer. Conclusion: Clean interpretation here.",
	}}

	path := s.GenerateReport(findings, clusters, nil, nil, metadata, "mb_test")
	content, _ := os.ReadFile(path)
	body := string(content)

	if strings.Contains(strings.ToLower(body), "we need to analyze") || strings.Contains(strings.ToLower(body), "i'll produce") {
		t.Fatalf("report leaked reasoning text:\n%s", body)
	}
}

func TestGenerateReport_InsufficientBridgeEvidence(t *testing.T) {
	dir := t.TempDir()
	s := newTestSynthesizer(dir)

	clusters := []models.Cluster{
		{ID: 1, Label: "A", Source: "surface / mb_a", VectorIDs: []uint64{1, 2, 3}, Size: 3, Density: 0.6, Coherence: 0.7},
		{ID: 2, Label: "B", Source: "surface / mb_b", VectorIDs: []uint64{4, 5, 6}, Size: 3, Density: 0.6, Coherence: 0.7},
	}
	metadata := []models.VectorMetadata{
		{ID: 1, Source: "mb_a", Fragment: "left 1"},
		{ID: 2, Source: "mb_a", Fragment: "left 2"},
		{ID: 3, Source: "mb_a", Fragment: "left 3"},
		{ID: 4, Source: "mb_b", Fragment: "right 1"},
	}
	bridges := []models.Bridge{{ClusterA: 1, ClusterB: 2, Strength: 0.8, SampleLinks: []models.SampleLink{{ChunkAID: 2, ChunkBID: 99, Similarity: 0.8}}}}

	path := s.GenerateReport(nil, clusters, bridges, nil, metadata, "mb_test")
	content, _ := os.ReadFile(path)
	body := string(content)

	if !strings.Contains(body, "Insufficient evidence to interpret this bridge.") {
		t.Fatalf("bridge evidence guard missing:\n%s", body)
	}
}

func TestGenerateReport_FlagsDuplicateHeavyAndOversampling(t *testing.T) {
	dir := t.TempDir()
	s := newTestSynthesizer(dir)

	clusters := []models.Cluster{{ID: 1, Label: "surface / x", Source: "surface / x", VectorIDs: []uint64{1, 2, 3, 4}, Size: 4, Density: 0.99, Coherence: 0.99}}
	metadata := []models.VectorMetadata{
		{ID: 1, Source: "mb_dom", Fragment: "one"},
		{ID: 2, Source: "mb_dom", Fragment: "two"},
		{ID: 3, Source: "mb_dom", Fragment: "three"},
		{ID: 4, Source: "mb_other", Fragment: "four"},
	}

	path := s.GenerateReport(nil, clusters, nil, nil, metadata, "mb_test")
	content, _ := os.ReadFile(path)
	body := string(content)

	if !strings.Contains(body, "Potential near-duplicate cluster. Human review recommended before interpretation.") {
		t.Fatalf("missing duplicate-heavy flag:\n%s", body)
	}
	if !strings.Contains(body, "Source oversampling detected. Interpretive confidence may be inflated.") {
		t.Fatalf("missing oversampling flag:\n%s", body)
	}
}

func TestGenerateReport_IncludesAnomalyConfidenceBands(t *testing.T) {
	dir := t.TempDir()
	s := newTestSynthesizer(dir)

	clusters := []models.Cluster{{ID: 1, Label: "surface / x", Source: "surface / x", VectorIDs: []uint64{1, 2, 3}, Size: 3, Density: 0.7, Coherence: 0.8}}
	metadata := []models.VectorMetadata{
		{ID: 1, Source: "mb_a", Fragment: "Snippet one"},
		{ID: 2, Source: "mb_b", Fragment: "Snippet two"},
		{ID: 3, Source: "mb_c", Fragment: "Snippet three"},
	}
	findings := []models.Finding{{
		Type:           "orphan_cluster",
		Subject:        "surface / x",
		Clusters:       []int{1},
		IsAnomaly:      true,
		Confidence:     0.77,
		ConfidenceBand: "high",
		ReasoningChain: "isolated",
	}}

	path := s.GenerateReport(findings, clusters, nil, nil, metadata, "mb_test")
	content, _ := os.ReadFile(path)
	body := string(content)
	if !strings.Contains(body, "orphan_cluster (high 0.77)") {
		t.Fatalf("missing anomaly confidence label in markdown report:\n%s", body)
	}
}

func TestGenerateReport_PendingAnalysisWhenFindingMissing(t *testing.T) {
	dir := t.TempDir()
	s := newTestSynthesizer(dir)

	clusters := []models.Cluster{{ID: 1, Label: "surface / x", Source: "surface / x", VectorIDs: []uint64{1, 2, 3}, Size: 3, Density: 0.7, Coherence: 0.8}}
	metadata := []models.VectorMetadata{
		{ID: 1, Source: "mb_a", Fragment: "Snippet one"},
		{ID: 2, Source: "mb_b", Fragment: "Snippet two"},
		{ID: 3, Source: "mb_c", Fragment: "Snippet three"},
	}

	path := s.GenerateReport(nil, clusters, nil, nil, metadata, "mb_test")
	body, _ := os.ReadFile(path)
	text := string(body)

	if strings.Contains(text, "Insufficient evidence to interpret this cluster") {
		t.Fatalf("missing finding should not produce 'Insufficient evidence' when snippets are present:\n%s", text)
	}
	if !strings.Contains(text, "No interpretive analysis available for this cluster.") {
		t.Fatalf("expected 'No interpretive analysis available' placeholder:\n%s", text)
	}
	if !strings.Contains(text, "Awaiting interpretive analysis") {
		t.Fatalf("expected 'Awaiting interpretive analysis' flag:\n%s", text)
	}
}

func TestKeywordTokens_FiltersGenericMetricWords(t *testing.T) {
	tokens := keywordTokens("High density coherence material consciousness evolution")
	joined := strings.Join(tokens, ",")
	for _, banned := range []string{"high", "density", "coherence", "material"} {
		if strings.Contains(joined, banned) {
			t.Fatalf("generic metric word %q should be filtered, got %v", banned, tokens)
		}
	}
	if !strings.Contains(joined, "consciousness") {
		t.Fatalf("expected non-generic semantic token to remain, got %v", tokens)
	}
}

func TestShortenForHeading_DoesNotClipMidSentence(t *testing.T) {
	text := "This heading has no sentence boundary and should remain fully intact even when long enough to trigger truncation logic"
	got := shortenForHeading(text, 40)
	if got != text {
		t.Fatalf("expected unchanged heading when no sentence boundary exists, got %q", got)
	}
}

func TestGenerateProgressReport_WritesInProgressFile(t *testing.T) {
	dir := t.TempDir()
	s := newTestSynthesizer(dir)

	path := s.GenerateProgressReport(nil, nil, nil, nil, nil, "test")
	if path == "" {
		t.Fatal("GenerateProgressReport returned empty path")
	}
	if !strings.Contains(filepath.Base(path), "in_progress") {
		t.Fatalf("expected in_progress filename, got %s", path)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("progress report file does not exist at %s", path)
	}
}

func TestGenerateReport_FilePathIsInsideOutputDir(t *testing.T) {
	dir := t.TempDir()
	s := newTestSynthesizer(dir)

	path := s.GenerateReport(nil, nil, nil, nil, nil, "test")
	rel, err := filepath.Rel(dir, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		t.Errorf("report path %q is not under output dir %q", path, dir)
	}
}

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

func TestStoreFindings_RequiresQdrant(t *testing.T) {
	t.Skip("requires a live Qdrant instance — run manually with QDRANT_URL set")
}

func TestStoreFindings_EmptySlice(t *testing.T) {
	s := newTestSynthesizer(t.TempDir())
	if err := s.StoreFindings(nil, nil); err != nil {
		t.Errorf("StoreFindings with nil should return nil, got: %v", err)
	}
}

func TestMemberPointIDsForFinding_ClusterAnalysis(t *testing.T) {
	clusterMemberPointIDs := buildClusterMemberPointIDs([]models.Cluster{
		{ID: 1, VectorIDs: []uint64{101, 102, 103}},
	})

	memberPointIDs, include := memberPointIDsForFinding(models.Finding{
		Type:     "cluster_analysis",
		Clusters: []int{1},
	}, clusterMemberPointIDs)
	if !include {
		t.Fatal("expected member_point_ids to be included for cluster_analysis")
	}
	want := []string{"101", "102", "103"}
	if strings.Join(memberPointIDs, ",") != strings.Join(want, ",") {
		t.Fatalf("member_point_ids = %v, want %v", memberPointIDs, want)
	}
}

func TestMemberPointIDsForFinding_BridgeAnalysisUnion(t *testing.T) {
	clusterMemberPointIDs := buildClusterMemberPointIDs([]models.Cluster{
		{ID: 1, VectorIDs: []uint64{101, 102}},
		{ID: 2, VectorIDs: []uint64{102, 201}},
	})

	memberPointIDs, include := memberPointIDsForFinding(models.Finding{
		Type:     "bridge_analysis",
		Clusters: []int{1, 2},
	}, clusterMemberPointIDs)
	if !include {
		t.Fatal("expected member_point_ids to be included for bridge_analysis")
	}
	want := []string{"101", "102", "201"}
	if strings.Join(memberPointIDs, ",") != strings.Join(want, ",") {
		t.Fatalf("member_point_ids = %v, want %v", memberPointIDs, want)
	}
}

func TestMemberPointIDsForFinding_DensityAndCoherenceAnomalies(t *testing.T) {
	clusterMemberPointIDs := buildClusterMemberPointIDs([]models.Cluster{
		{ID: 7, VectorIDs: []uint64{7001, 7002}},
	})

	for _, findingType := range []string{"density_anomaly", "coherence_anomaly"} {
		memberPointIDs, include := memberPointIDsForFinding(models.Finding{
			Type:     findingType,
			Clusters: []int{7},
		}, clusterMemberPointIDs)
		if !include {
			t.Fatalf("expected member_point_ids to be included for %s", findingType)
		}
		want := []string{"7001", "7002"}
		if strings.Join(memberPointIDs, ",") != strings.Join(want, ",") {
			t.Fatalf("%s member_point_ids = %v, want %v", findingType, memberPointIDs, want)
		}
	}
}
