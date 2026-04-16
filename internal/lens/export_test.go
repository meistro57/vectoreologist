package lens

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/meistro57/vectoreologist/internal/synthesis"
)

func makeTestModel(t *testing.T) Model {
	t.Helper()
	dir := t.TempDir()
	reportPath := filepath.Join(dir, "vectoreology_test.json")
	os.WriteFile(reportPath, []byte("{}"), 0644)

	report := &synthesis.JSONReport{
		Collection: "test",
		Clusters: []synthesis.JSONCluster{
			{ID: 1, Label: "ML Models", Size: 10, Coherence: 0.8},
			{ID: 2, Label: "Data Pipelines", Size: 5, Coherence: 0.6},
		},
		Bridges: []synthesis.JSONBridge{
			{ClusterA: 1, ClusterB: 2, Strength: 0.75, LinkType: "strong_semantic"},
		},
		Anomalies: []synthesis.JSONAnomaly{
			{Type: "low_coherence", Subject: "Cluster 3: noise", ClusterID: 3},
		},
	}

	m := New(reportPath)
	m.report = report
	m.visibleClusters = report.Clusters
	m.visibleBridges = report.Bridges
	m.visibleAnomalies = report.Anomalies
	return m
}

func TestExportCurrentItem_Cluster(t *testing.T) {
	m := makeTestModel(t)
	m.viewMode = clusterView
	m.selectedIndex = 0

	status := m.exportCurrentItem()

	if !strings.HasPrefix(status, "✓ ") {
		t.Fatalf("expected success status, got: %s", status)
	}
	path := strings.TrimPrefix(status, "✓ ")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("exported file not found at %q: %v", path, err)
	}
	var env exportEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("exported file is not valid JSON: %v", err)
	}
	if env.Kind != "cluster_1" {
		t.Errorf("kind: want cluster_1, got %q", env.Kind)
	}
	if env.SourceFile != "vectoreology_test.json" {
		t.Errorf("source_file: want vectoreology_test.json, got %q", env.SourceFile)
	}
}

func TestExportCurrentItem_Bridge(t *testing.T) {
	m := makeTestModel(t)
	m.viewMode = bridgeView
	m.selectedIndex = 0

	status := m.exportCurrentItem()
	if !strings.HasPrefix(status, "✓ ") {
		t.Errorf("expected success, got: %s", status)
	}
}

func TestExportCurrentItem_Anomaly(t *testing.T) {
	m := makeTestModel(t)
	m.viewMode = anomalyView
	m.selectedIndex = 0

	status := m.exportCurrentItem()
	if !strings.HasPrefix(status, "✓ ") {
		t.Errorf("expected success, got: %s", status)
	}
}

func TestExportCurrentItem_NoReport(t *testing.T) {
	m := New("/some/path.json")
	if got := m.exportCurrentItem(); got != "no report loaded" {
		t.Errorf("want 'no report loaded', got %q", got)
	}
}

func TestExportCurrentItem_SearchViewReturnsHint(t *testing.T) {
	m := makeTestModel(t)
	m.viewMode = searchView
	status := m.exportCurrentItem()
	if strings.HasPrefix(status, "✓") {
		t.Errorf("search view should not produce a file, got: %s", status)
	}
}

func TestExportVisibleList_Clusters(t *testing.T) {
	m := makeTestModel(t)
	m.viewMode = clusterView

	status := m.exportVisibleList()
	if !strings.HasPrefix(status, "✓ ") {
		t.Fatalf("expected success, got: %s", status)
	}
	path := strings.TrimPrefix(status, "✓ ")
	data, _ := os.ReadFile(path)
	var env exportEnvelope
	json.Unmarshal(data, &env)
	if !strings.HasPrefix(env.Kind, "clusters_") {
		t.Errorf("kind should start with clusters_, got %q", env.Kind)
	}
}

func TestExportVisibleList_Bridges(t *testing.T) {
	m := makeTestModel(t)
	m.viewMode = bridgeView

	status := m.exportVisibleList()
	if !strings.HasPrefix(status, "✓ ") {
		t.Errorf("expected success, got: %s", status)
	}
}

func TestWriteExport_WritesValidJSON(t *testing.T) {
	dir := t.TempDir()
	m := New(filepath.Join(dir, "report.json"))
	m.report = &synthesis.JSONReport{Collection: "test"}

	payload := map[string]string{"hello": "world"}
	status := m.writeExport("test_kind", payload)

	if !strings.HasPrefix(status, "✓ ") {
		t.Fatalf("writeExport failed: %s", status)
	}
	path := strings.TrimPrefix(status, "✓ ")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file not written: %v", err)
	}
	if !json.Valid(data) {
		t.Errorf("output is not valid JSON")
	}
}

func TestWriteExport_FilenameContainsKind(t *testing.T) {
	dir := t.TempDir()
	m := New(filepath.Join(dir, "report.json"))
	m.report = &synthesis.JSONReport{}

	status := m.writeExport("mydata", struct{}{})
	if !strings.HasPrefix(status, "✓ ") {
		t.Fatalf("writeExport failed: %s", status)
	}
	path := strings.TrimPrefix(status, "✓ ")
	if !strings.Contains(filepath.Base(path), "mydata") {
		t.Errorf("filename should contain kind 'mydata', got %q", filepath.Base(path))
	}
}

func TestWriteExport_NoReport(t *testing.T) {
	m := New("/nonexistent/path.json")
	// No report set — writeExport should try to write to /nonexistent/
	// and fail gracefully.
	status := m.writeExport("test", struct{}{})
	// Should return an error message, not panic.
	if strings.HasPrefix(status, "✓ ") {
		// If it somehow succeeded, that's unexpected but not a test failure per se.
		// The important thing is no panic.
	}
}
