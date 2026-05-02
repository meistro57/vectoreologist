package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
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

type config struct {
	collection     string
	sampleSize     int
	batchSize      int
	strict         bool
	vectorName     string
	vectorCombine  bool
	outputPath     string
	qdrantURL      string
	deepseekKey    string
	deepseekURL    string
	deepseekModel  string
	sampleStrategy string
	semanticLabels bool
	incremental    bool
	minClusterSize int
	minSamples     int
}

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

func validateConfig(cfg config) error {
	if cfg.sampleSize < 0 {
		return fmt.Errorf("--sample must be >= 0")
	}
	if cfg.batchSize <= 0 {
		return fmt.Errorf("--batch-size must be > 0")
	}
	if cfg.minClusterSize <= 0 {
		return fmt.Errorf("--min-cluster-size must be > 0")
	}
	if cfg.minSamples <= 0 {
		return fmt.Errorf("--min-samples must be > 0")
	}
	return nil
}

// runOnce executes the full excavation pipeline and returns the report path.
func runOnce(cfg config) (string, error) {
	if err := validateConfig(cfg); err != nil {
		return "", err
	}
	fmt.Printf("🏺 Vectoreologist - Excavating %s from %s\n\n", cfg.collection, cfg.qdrantURL)

	// Phase 1: Vector Excavation
	fmt.Println("📡 Phase 1: Vector Excavation")
	exc := excavator.New(cfg.qdrantURL, cfg.vectorName, cfg.vectorCombine)
	sampleStrat := excavator.SamplingStrategy(cfg.sampleStrategy)

	// Resolve sample size: 0 means "use entire collection".
	collSize, err := exc.CollectionSize(cfg.collection)
	if err != nil {
		fmt.Fprintf(os.Stderr, "   ⚠ Could not determine collection size: %v\n", err)
		if cfg.sampleSize == 0 {
			cfg.sampleSize = 5000 // fallback when we can't query size
			fmt.Fprintf(os.Stderr, "   ⚠ Falling back to --sample %d\n", cfg.sampleSize)
		}
	} else {
		fmt.Printf("   ✓ Collection size: %d vectors\n", collSize)
		if cfg.sampleSize == 0 || uint64(cfg.sampleSize) > collSize {
			cfg.sampleSize = int(collSize)
		}
	}

	// For diverse sampling, extract a larger pool so MaxMin has room to maximise spread.
	extractLimit := cfg.sampleSize
	if sampleStrat == excavator.Diverse {
		extractLimit = cfg.sampleSize * 3 / 2
	}

	// Clamp to collection size if known.
	if collSize > 0 && uint64(extractLimit) > collSize {
		extractLimit = int(collSize)
	}

	target := extractLimit
	fmt.Printf("   ✓ Target sample: %d vectors (extracting %d)\n", cfg.sampleSize, extractLimit)
	fmt.Printf("   ✓ Batch size: %d vectors\n", cfg.batchSize)

	totalBatches := (target + cfg.batchSize - 1) / cfg.batchSize
	onBatch := func(batchNum, fetched, tgt int) {
		pct := 0.0
		if tgt > 0 {
			pct = 100.0 * float64(fetched) / float64(tgt)
		}
		fmt.Printf("   → Batch %d/%d: Extracted %d vectors (%.1f%%)\n", batchNum, totalBatches, fetched, pct)
	}

	var vectors [][]float32
	var metadata []models.VectorMetadata
	if cfg.incremental {
		fmt.Println("   \u2139 Incremental mode: extracting only unstamped points")
		vectors, metadata, err = exc.ExtractIncremental(cfg.collection, extractLimit, cfg.batchSize, cfg.strict, onBatch)
	} else {
		vectors, metadata, err = exc.Extract(cfg.collection, extractLimit, cfg.batchSize, cfg.strict, onBatch)
	}
	if err != nil {
		return "", fmt.Errorf("extraction failed: %w", err)
	}
	sampler := excavator.NewSampler(sampleStrat, time.Now().Unix())
	vectors, metadata = sampler.Sample(vectors, metadata, cfg.sampleSize)
	fmt.Printf("   ✓ Total extracted: %d vectors with metadata\n\n", len(vectors))

	// Phase 2: Topology Analysis
	fmt.Println("🗺️  Phase 2: Topology Analysis")
	topo := topology.New()
	topo.SetHDBSCANParams(cfg.minClusterSize, cfg.minSamples)
	clusters := topo.AnalyzeClusters(vectors, metadata)

	// Optional: replace layer/source labels with DeepSeek-generated semantic names.
	if cfg.semanticLabels && cfg.deepseekKey != "" {
		fmt.Println("   🏷  Generating semantic labels…")
		labeler := reasoner.New2(cfg.deepseekURL, cfg.deepseekKey, cfg.deepseekModel)
		clusters = labeler.LabelClusters(clusters, metadata)
	}

	bridges := topo.FindBridges(clusters, vectors, metadata)
	moats := topo.FindMoats(clusters)
	fmt.Printf("   ✓ Identified %d concept clusters\n", len(clusters))
	fmt.Printf("   ✓ Found %d domain bridges\n", len(bridges))
	fmt.Printf("   ✓ Detected %d knowledge moats\n\n", len(moats))

	// Phase 3: Anomaly Detection
	fmt.Println("⚠️  Phase 3: Anomaly Detection")
	det := anomaly.New()
	clusterAnomalies := det.DetectClusterAnomalies(clusters)
	orphans := det.DetectOrphans(clusters, bridges)
	contradictions := det.DetectContradictions(clusters, metadata)
	anomalies := append(clusterAnomalies, append(orphans, contradictions...)...)
	fmt.Printf("   ✓ Found %d cluster anomalies\n", len(clusterAnomalies))
	fmt.Printf("   ✓ Found %d orphaned clusters\n", len(orphans))
	fmt.Printf("   ✓ Found %d source contradictions\n\n", len(contradictions))

	// Phase 4: DeepSeek R1 Reasoning
	fmt.Println("🧠 Phase 4: DeepSeek R1 Reasoning")
	var reasonedFindings []models.Finding
	if cfg.deepseekKey != "" {
		r := reasoner.New2(cfg.deepseekURL, cfg.deepseekKey, cfg.deepseekModel)
		reasonedFindings = r.ReasonAboutTopology(clusters, bridges, moats, metadata)
		clusters = reasoner.PromoteClusterLabels(reasonedFindings, clusters)
		bridges = reasoner.PromoteBridgeLabels(reasonedFindings, bridges)
	} else {
		fmt.Println("   ⚠ No DeepSeek API key — skipping reasoning phase")
	}
	allFindings := append(reasonedFindings, anomalies...)
	fmt.Printf("   ✓ Generated %d reasoning chains\n", len(reasonedFindings))
	fmt.Printf("   ✓ Total findings: %d\n\n", len(allFindings))

	// Phase 5: Synthesis & Storage
	fmt.Println("📝 Phase 5: Synthesis & Storage")
	synth := synthesis.New(cfg.qdrantURL, cfg.outputPath)
	reportPath := synth.GenerateReport(allFindings, clusters, bridges, moats, cfg.collection)
	fmt.Printf("   ✓ Report written to %s\n", reportPath)
	if err := synth.StoreFindings(allFindings, clusters); err != nil {
		fmt.Fprintf(os.Stderr, "   ⚠ Failed to store findings: %v\n", err)
	} else {
		fmt.Println("   ✓ Findings stored in vectoreology_findings collection")
	}

	fmt.Println()
	fmt.Println("✨ Excavation Complete")

	// Stamp analyzed points so --incremental skips them next time.
	if len(metadata) > 0 {
		runID := time.Now().UTC().Format(time.RFC3339)
		ids := make([]uint64, len(metadata))
		for i, m := range metadata {
			ids[i] = m.ID
		}
		fmt.Printf("   📌 Stamping %d points with run ID %s\n", len(ids), runID)
		if err := exc.StampPoints(cfg.collection, ids, runID); err != nil {
			fmt.Fprintf(os.Stderr, "   ⚠ Failed to stamp points: %v\n", err)
		} else {
			fmt.Printf("   ✓ %d points stamped\n", len(ids))
		}
	}
	fmt.Println()
	fmt.Println("Key Insights:")
	fmt.Printf("  • %d semantic concepts discovered\n", len(clusters))
	fmt.Printf("  • %d domain connections mapped\n", len(bridges))
	fmt.Printf("  • %d knowledge gaps identified\n", len(moats))
	fmt.Printf("  • %d anomalies flagged for investigation\n", len(anomalies))
	fmt.Println()
	fmt.Printf("Read full analysis: %s\n", reportPath)

	return reportPath, nil
}

func main() {
	loadDotEnv(".env")
	showVersion := flag.Bool("version", false, "Print version and exit")
	collection := flag.String("collection", "", "Qdrant collection name (required)")
	sampleSize := flag.Int("sample", 0, "Number of vectors to sample (0 = entire collection)")
	batchSize := flag.Int("batch-size", 5000, "Vectors per batch during extraction")
	strict := flag.Bool("strict", false, "Fail immediately if any batch errors (default: stop early and continue)")
	vectorName := flag.String("vector-name", "", "Named vector to extract when points contain multiple named vectors")
	vectorCombine := flag.Bool("vector-combine", false, "Average all named vectors element-wise instead of selecting one")
	outputPath := flag.String("output", "./findings", "Output directory for reports")
	qdrantURL := flag.String("qdrant-url", "", "Qdrant URL (default: QDRANT_URL env or http://localhost:6333)")
	deepseekKey := flag.String("deepseek-key", "", "DeepSeek API key (default: DEEPSEEK_API_KEY env)")
	deepseekURL := flag.String("deepseek-url", "https://api.deepseek.com/v1", "DeepSeek API base URL")
	deepseekModel := flag.String("deepseek-model", "deepseek-reasoner", "Model: deepseek-reasoner (full R1 thinking) or deepseek-chat (fast)")
	watchInterval := flag.String("watch", "", "Re-run on this interval (e.g. 5m, 1h). Stops on SIGINT/SIGTERM.")
	sampleStrategy := flag.String("sample-strategy", "random", "Sampling strategy: random, stratified, diverse")
	semanticLabels := flag.Bool("semantic-labels", false, "Generate semantic cluster labels via DeepSeek (requires --deepseek-key)")
	incremental := flag.Bool("incremental", false, "Only extract unstamped points (skip previously analyzed)")
	minClusterSize := flag.Int("min-cluster-size", 5, "Minimum HDBSCAN cluster size")
	minSamples := flag.Int("min-samples", 3, "HDBSCAN min_samples (smaller = less noise)")
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

	if *sampleSize > 0 && *batchSize > *sampleSize {
		*batchSize = *sampleSize
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

	cfg := config{
		collection:     *collection,
		sampleSize:     *sampleSize,
		batchSize:      *batchSize,
		strict:         *strict,
		vectorName:     *vectorName,
		vectorCombine:  *vectorCombine,
		outputPath:     *outputPath,
		qdrantURL:      qdrant,
		deepseekKey:    dsKey,
		deepseekURL:    *deepseekURL,
		deepseekModel:  *deepseekModel,
		sampleStrategy: *sampleStrategy,
		semanticLabels: *semanticLabels,
		incremental:    *incremental,
		minClusterSize: *minClusterSize,
		minSamples:     *minSamples,
	}
	if err := validateConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Watch mode
	if *watchInterval != "" {
		watchDur, err := time.ParseDuration(*watchInterval)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid --watch value %q: %v\n", *watchInterval, err)
			os.Exit(1)
		}
		if watchDur <= 0 {
			fmt.Fprintln(os.Stderr, "Error: --watch duration must be positive")
			os.Exit(1)
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		fmt.Printf("👁  Watch mode: running every %s (Ctrl-C to stop)\n\n", watchDur)

		for cycle := 1; ; cycle++ {
			fmt.Printf("━━━ cycle %d [%s] ━━━\n\n", cycle, time.Now().UTC().Format(time.RFC3339))
			start := time.Now()
			_, runErr := runOnce(cfg)
			elapsed := time.Since(start).Round(time.Second)
			if runErr != nil {
				fmt.Fprintf(os.Stderr, "⚠  cycle %d failed (%s): %v\n\n", cycle, elapsed, runErr)
			} else {
				fmt.Printf("\n⏱  cycle %d completed in %s\n\n", cycle, elapsed)
			}

			select {
			case <-ctx.Done():
				fmt.Println("✋ Watch mode stopped.")
				return
			case <-time.After(watchDur):
			}
		}
	}

	// Single run
	if _, err := runOnce(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Fatal: %v\n", err)
		os.Exit(1)
	}
}
