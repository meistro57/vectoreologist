package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/meistro57/vectoreologist/internal/anomaly"
	"github.com/meistro57/vectoreologist/internal/excavator"
	"github.com/meistro57/vectoreologist/internal/models"
	"github.com/meistro57/vectoreologist/internal/reasoner"
	"github.com/meistro57/vectoreologist/internal/synthesis"
	"github.com/meistro57/vectoreologist/internal/topology"
)

// version is set at build time via:
//
//	go build -ldflags "-X main.version=$(git describe --tags --always)" ./cmd/vectoreologist
var version = "dev"

// loadDotEnv reads a .env file and sets any variables not already in the environment.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // no .env file is fine
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		val = strings.Trim(val, `"'`)
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}

func main() {
	loadDotEnv(".env")
	showVersion := flag.Bool("version", false, "Print version and exit")
	collection  := flag.String("collection",   "",                          "Qdrant collection name (required)")
	sampleSize  := flag.Int("sample",          5000,                        "Number of vectors to sample")
	outputPath  := flag.String("output",       "./findings",                "Output directory for reports")
	qdrantURL   := flag.String("qdrant-url",   "",                          "Qdrant URL (default: QDRANT_URL env or http://localhost:6333)")
	deepseekKey   := flag.String("deepseek-key",   "",                             "DeepSeek API key (default: DEEPSEEK_API_KEY env)")
	deepseekURL   := flag.String("deepseek-url",   "https://api.deepseek.com/v1",  "DeepSeek API base URL")
	deepseekModel := flag.String("deepseek-model", "deepseek-reasoner",            "Model: deepseek-reasoner (full R1 thinking) or deepseek-chat (fast)")
	flag.Parse()

	if *showVersion {
		fmt.Println("vectoreologist", version)
		return
	}

	if *collection == "" {
		fmt.Fprintln(os.Stderr, "Error: --collection is required")
		flag.Usage()
		os.Exit(1)
	}

	// Resolve Qdrant URL
	qdrant := *qdrantURL
	if qdrant == "" {
		qdrant = os.Getenv("QDRANT_URL")
	}
	if qdrant == "" {
		qdrant = "http://localhost:6333"
	}

	// Resolve DeepSeek API key
	dsKey := *deepseekKey
	if dsKey == "" {
		dsKey = os.Getenv("DEEPSEEK_API_KEY")
	}

	fmt.Printf("🏺 Vectoreologist - Excavating %s from %s\n\n", *collection, qdrant)

	// Phase 1: Vector Excavation
	fmt.Println("📡 Phase 1: Vector Excavation")
	exc := excavator.New(qdrant)
	vectors, metadata, err := exc.Extract(*collection, *sampleSize)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal: extraction failed: %v\n", err)
		os.Exit(1)
	}
	sampler := excavator.NewSampler(excavator.Random, time.Now().Unix())
	vectors, metadata = sampler.Sample(vectors, metadata, *sampleSize)
	fmt.Printf("   ✓ Extracted %d vectors with metadata\n\n", len(vectors))

	// Phase 2: Topology Analysis
	fmt.Println("🗺️  Phase 2: Topology Analysis")
	topo := topology.New()
	clusters := topo.AnalyzeClusters(vectors, metadata)
	bridges  := topo.FindBridges(clusters)
	moats    := topo.FindMoats(clusters)
	fmt.Printf("   ✓ Identified %d concept clusters\n", len(clusters))
	fmt.Printf("   ✓ Found %d domain bridges\n", len(bridges))
	fmt.Printf("   ✓ Detected %d knowledge moats\n\n", len(moats))

	// Phase 3: Anomaly Detection
	fmt.Println("⚠️  Phase 3: Anomaly Detection")
	det              := anomaly.New()
	clusterAnomalies := det.DetectClusterAnomalies(clusters)
	orphans          := det.DetectOrphans(clusters, bridges)
	contradictions   := det.DetectContradictions(clusters, metadata)
	anomalies        := append(clusterAnomalies, append(orphans, contradictions...)...)
	fmt.Printf("   ✓ Found %d cluster anomalies\n", len(clusterAnomalies))
	fmt.Printf("   ✓ Found %d orphaned clusters\n", len(orphans))
	fmt.Printf("   ✓ Found %d source contradictions\n\n", len(contradictions))

	// Phase 4: DeepSeek R1 Reasoning
	fmt.Println("🧠 Phase 4: DeepSeek R1 Reasoning")
	var reasonedFindings []models.Finding
	if dsKey != "" {
		r := reasoner.New2(*deepseekURL, dsKey, *deepseekModel)
		reasonedFindings = r.ReasonAboutTopology(clusters, bridges, moats)
	} else {
		fmt.Println("   ⚠ No DeepSeek API key — skipping reasoning phase")
	}
	allFindings := append(reasonedFindings, anomalies...)
	fmt.Printf("   ✓ Generated %d reasoning chains\n", len(reasonedFindings))
	fmt.Printf("   ✓ Total findings: %d\n\n", len(allFindings))

	// Phase 5: Synthesis & Storage
	fmt.Println("📝 Phase 5: Synthesis & Storage")
	synth      := synthesis.New(qdrant, *outputPath)
	reportPath := synth.GenerateReport(allFindings, clusters, bridges, moats)
	fmt.Printf("   ✓ Report written to %s\n", reportPath)
	if err := synth.StoreFindings(allFindings); err != nil {
		fmt.Fprintf(os.Stderr, "   ⚠ Failed to store findings: %v\n", err)
	} else {
		fmt.Println("   ✓ Findings stored in vectoreology_findings collection")
	}

	fmt.Println()
	fmt.Println("✨ Excavation Complete")
	fmt.Println()
	fmt.Println("Key Insights:")
	fmt.Printf("  • %d semantic concepts discovered\n", len(clusters))
	fmt.Printf("  • %d domain connections mapped\n", len(bridges))
	fmt.Printf("  • %d knowledge gaps identified\n", len(moats))
	fmt.Printf("  • %d anomalies flagged for investigation\n", len(anomalies))
	fmt.Println()
	fmt.Printf("Read full analysis: %s\n", reportPath)
}
