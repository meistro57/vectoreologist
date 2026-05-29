package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/meistro57/vectoreologist/internal/anomaly"
	"github.com/meistro57/vectoreologist/internal/excavator"
	"github.com/meistro57/vectoreologist/internal/models"
	"github.com/meistro57/vectoreologist/internal/reasoner"
	"github.com/meistro57/vectoreologist/internal/synthesis"
	"github.com/meistro57/vectoreologist/internal/taxonomy"
	"github.com/meistro57/vectoreologist/internal/topology"
	"github.com/meistro57/vectoreologist/internal/workspace"
)

// version is set at build time via:
//
//	go build -ldflags "-X main.version=$(git describe --tags --always)" ./cmd/vectoreologist
var version = "dev"

type config struct {
	collection          string
	sampleSize          int
	batchSize           int
	strict              bool
	vectorName          string
	vectorCombine       bool
	outputPath          string
	qdrantURL           string
	deepseekKey         string
	deepseekURL         string
	deepseekModel       string
	sampleStrategy      string
	semanticLabels      bool
	incremental         bool
	minClusterSize      int
	minSamples          int // no-op; kept for backwards CLI compatibility
	redisURL            string
	epsilon             float64
	clusterSeed         int64
	moatThreshold       float64
	filterDegenerate    bool
	autoTuneDBSCAN      bool
	reasonerProfile     string
	maxReasonerClusters int
	maxReasonerBridges  int
	maxReasonerMoats    int
	reasonerCache       bool
	// Query flags — when any is set, the pipeline does not run; instead a JSON
	// report file is read and filtered results are printed.
	queryReport   string
	queryTopic    string
	queryMode     string
	queryPosture  string
	queryMismatch bool
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
	if cfg.maxReasonerClusters < -1 {
		return fmt.Errorf("--reasoner-max-clusters must be >= -1")
	}
	if cfg.maxReasonerBridges < -1 {
		return fmt.Errorf("--reasoner-max-bridges must be >= -1")
	}
	if cfg.maxReasonerMoats < -1 {
		return fmt.Errorf("--reasoner-max-moats must be >= -1")
	}
	return nil
}

func budgetLimitLabel(limit int) string {
	if limit <= 0 {
		return "all"
	}
	return fmt.Sprintf("%d", limit)
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

	// Set up optional Redis workspace.
	var ws *workspace.Workspace
	runID := time.Now().UTC().Format("20060102T150405Z")
	if cfg.redisURL != "" {
		ws, err = workspace.New(cfg.redisURL, runID, time.Hour)
		if err != nil {
			return "", fmt.Errorf("redis workspace: %w", err)
		}
		defer ws.Delete()
		fmt.Printf("   ✓ Redis workspace enabled (run %s)\n", runID)
	}

	batchCounter := 0
	onBatch := func(batchNum, fetched, tgt int) {
		pct := 0.0
		if tgt > 0 {
			pct = 100.0 * float64(fetched) / float64(tgt)
		}
		fmt.Printf("   → Batch %d/%d: Extracted %d vectors (%.1f%%)\n", batchNum, totalBatches, fetched, pct)
	}

	var vectors [][]float32
	var metadata []models.VectorMetadata

	if ws != nil {
		// Stream-to-Redis path: use a batch callback that also stores to Redis.
		// We wrap the extraction with a Redis-streaming batch callback.
		batchStorer := func(batchNum, fetched, tgt int) {
			onBatch(batchNum, fetched, tgt)
		}
		if cfg.incremental {
			fmt.Println("   ℹ Incremental mode: extracting only unstamped points")
			vectors, metadata, err = exc.ExtractIncremental(cfg.collection, extractLimit, cfg.batchSize, cfg.strict, batchStorer)
		} else {
			vectors, metadata, err = exc.Extract(cfg.collection, extractLimit, cfg.batchSize, cfg.strict, batchStorer)
		}
		if err != nil {
			return "", fmt.Errorf("extraction failed: %w", err)
		}
		// Store all extracted vectors to Redis in batches.
		for start := 0; start < len(vectors); start += cfg.batchSize {
			end := start + cfg.batchSize
			if end > len(vectors) {
				end = len(vectors)
			}
			if storeErr := ws.StoreBatch(batchCounter, vectors[start:end], metadata[start:end]); storeErr != nil {
				fmt.Fprintf(os.Stderr, "   ⚠ Redis store batch %d: %v\n", batchCounter, storeErr)
			}
			batchCounter++
		}
	} else {
		if cfg.incremental {
			fmt.Println("   ℹ Incremental mode: extracting only unstamped points")
			vectors, metadata, err = exc.ExtractIncremental(cfg.collection, extractLimit, cfg.batchSize, cfg.strict, onBatch)
		} else {
			vectors, metadata, err = exc.Extract(cfg.collection, extractLimit, cfg.batchSize, cfg.strict, onBatch)
		}
		if err != nil {
			return "", fmt.Errorf("extraction failed: %w", err)
		}
	}

	sampler := excavator.NewSampler(sampleStrat, time.Now().Unix())
	vectors, metadata = sampler.Sample(vectors, metadata, cfg.sampleSize)
	idAudit := excavator.AuditIDNormalization(metadata)
	fmt.Printf("   ✓ Total extracted: %d vectors with metadata\n", len(vectors))
	fmt.Printf("   ✓ ID normalization audit: %s\n\n", idAudit.Summary())

	// Phase 2: Topology Analysis
	fmt.Println("🗺️  Phase 2: Topology Analysis")
	topo := topology.New()
	topo.SetClusterParams(cfg.minClusterSize, cfg.epsilon)
	topo.SetSeed(cfg.clusterSeed)
	topo.SetMoatThreshold(cfg.moatThreshold)
	topo.SetFilterDegenerate(cfg.filterDegenerate)
	topo.SetAutoTune(cfg.autoTuneDBSCAN)

	// When Redis workspace is enabled, load topology sample from Redis instead
	// of the in-memory slice.
	topoVecs := vectors
	topoMeta := metadata
	if ws != nil {
		topoVecs, topoMeta, err = ws.LoadSample(topology.MaxTopologyTotal)
		if err != nil {
			fmt.Fprintf(os.Stderr, "   ⚠ Redis load sample failed, falling back to in-memory: %v\n", err)
			topoVecs = vectors
			topoMeta = metadata
		}
	}

	clusters := topo.AnalyzeClusters(topoVecs, topoMeta)
	dbscanDiag := topo.LastDiagnostics()

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

	paramOrigin := "configured defaults"
	switch {
	case dbscanDiag.AutoTuned:
		paramOrigin = "auto_tuned"
	case dbscanDiag.FallbackUsed && cfg.autoTuneDBSCAN:
		paramOrigin = "auto_tune_fallback"
	}
	diagnosticsFinding := models.Finding{
		Type:    "topology_diagnostics",
		Subject: fmt.Sprintf("DBSCAN params (%s): eps=%.3f minPts=%d", paramOrigin, dbscanDiag.ChosenEps, dbscanDiag.ChosenMinPts),
		ReasoningChain: fmt.Sprintf(
			"requested_eps=%.3f requested_minPts=%d sampled_pairs=%d p10=%.3f p25=%.3f p50=%.3f p75=%.3f p90=%.3f reason=%s",
			dbscanDiag.RequestedEps,
			dbscanDiag.RequestedMinPts,
			dbscanDiag.SampledPairs,
			dbscanDiag.P10,
			dbscanDiag.P25,
			dbscanDiag.P50,
			dbscanDiag.P75,
			dbscanDiag.P90,
			dbscanDiag.Reason,
		),
		Confidence: func() float64 {
			if dbscanDiag.AutoTuned {
				return 0.8
			}
			if dbscanDiag.FallbackUsed && cfg.autoTuneDBSCAN {
				return 0.65
			}
			return 0.6
		}(),
	}

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

	synth := synthesis.New(cfg.qdrantURL, cfg.outputPath)

	// Phase 4: DeepSeek R1 Reasoning
	fmt.Println("🧠 Phase 4: DeepSeek R1 Reasoning")
	reasoningBudget, resolvedProfile := reasoner.ResolveBudget(cfg.reasonerProfile, cfg.maxReasonerClusters, cfg.maxReasonerBridges, cfg.maxReasonerMoats)
	fmt.Printf("   ✓ Reasoner budget: profile=%s clusters=%s bridges=%s moats=%s\n",
		resolvedProfile,
		budgetLimitLabel(reasoningBudget.MaxClusters),
		budgetLimitLabel(reasoningBudget.MaxBridges),
		budgetLimitLabel(reasoningBudget.MaxMoats),
	)
	var reasonedFindings []models.Finding
	if cfg.deepseekKey != "" {
		r := reasoner.New2(cfg.deepseekURL, cfg.deepseekKey, cfg.deepseekModel)
		r.SetBudget(reasoningBudget)
		fingerprint := reasoner.TopologyFingerprint(cfg.collection, cfg.clusterSeed, cfg.deepseekModel, reasoningBudget, clusters, bridges, moats)
		cacheDir := filepath.Join(cfg.outputPath, ".cache", "reasoner")
		if cfg.reasonerCache {
			cachedFindings, cacheHit, cacheErr := reasoner.LoadCachedFindings(cacheDir, fingerprint)
			if cacheErr != nil {
				fmt.Fprintf(os.Stderr, "   ⚠ Reasoner cache read failed: %v\n", cacheErr)
			} else if cacheHit {
				reasonedFindings = cachedFindings
				fmt.Printf("   ✓ Reasoner cache hit: %s\n", fingerprint[:12])
			} else {
				fmt.Printf("   ℹ Reasoner cache miss: %s\n", fingerprint[:12])
			}
		}
		if len(reasonedFindings) == 0 {
			streamedFindings := make([]models.Finding, 0)
			r.SetFindingHandler(func(f models.Finding, done, total int) {
				streamedFindings = append(streamedFindings, f)
				if total > 0 && (done%3 == 0 || done == total) {
					progressFindings := make([]models.Finding, 0, len(streamedFindings)+len(anomalies)+1)
					progressFindings = append(progressFindings, streamedFindings...)
					progressFindings = append(progressFindings, diagnosticsFinding)
					progressFindings = append(progressFindings, anomalies...)
					progressPath := synth.GenerateProgressReport(progressFindings, clusters, bridges, moats, metadata, cfg.collection)
					fmt.Printf("\n   ↳ In-progress report: %s (%d/%d)\n", progressPath, done, total)
				}
			})
			reasonedFindings = r.ReasonAboutTopology(clusters, bridges, moats, metadata)
			r.SetFindingHandler(nil)
			if cfg.reasonerCache && len(reasonedFindings) > 0 {
				if cacheErr := reasoner.SaveCachedFindings(cacheDir, fingerprint, reasonedFindings); cacheErr != nil {
					fmt.Fprintf(os.Stderr, "   ⚠ Reasoner cache write failed: %v\n", cacheErr)
				} else {
					fmt.Printf("   ✓ Reasoner cache stored: %s\n", fingerprint[:12])
				}
			}
		}
		clusters = reasoner.PromoteClusterLabels(reasonedFindings, clusters)
		bridges = reasoner.PromoteBridgeLabels(reasonedFindings, bridges)
	} else {
		fmt.Println("   ⚠ No DeepSeek API key — skipping reasoning phase")
	}
	// Phase 4.5: Taxonomy Classification
	fmt.Println("🔖 Phase 4.5: Taxonomy Classification")
	tClassifier := taxonomy.New()
	clusters = tClassifier.ClassifyClusters(clusters, metadata)
	taxonomyAnomalies := det.DetectTaxonomyAnomalies(clusters, metadata)
	mismatchCount := 0
	for _, f := range taxonomyAnomalies {
		if f.AnomalyType == taxonomy.AnomalyLabelMismatch {
			mismatchCount++
		}
	}
	fmt.Printf("   ✓ Taxonomy classification complete (%d clusters)\n", len(clusters))
	fmt.Printf("   ✓ %d label mismatches detected\n", mismatchCount)
	fmt.Printf("   ✓ %d taxonomy anomalies total\n\n", len(taxonomyAnomalies))

	allFindings := append(reasonedFindings, diagnosticsFinding)
	allFindings = append(allFindings, anomalies...)
	allFindings = append(allFindings, taxonomyAnomalies...)
	fmt.Printf("   ✓ Generated %d reasoning chains\n", len(reasonedFindings))
	fmt.Printf("   ✓ Total findings: %d\n\n", len(allFindings))

	// Phase 5: Synthesis & Storage
	fmt.Println("📝 Phase 5: Synthesis & Storage")
	reportPath := synth.GenerateReport(allFindings, clusters, bridges, moats, metadata, cfg.collection)
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
		fmt.Printf("   📌 Stamping %d points with run ID %s\n", len(metadata), runID)
		if err := exc.StampMetadataPoints(cfg.collection, metadata, runID); err != nil {
			fmt.Fprintf(os.Stderr, "   ⚠ Failed to stamp points: %v\n", err)
		} else {
			fmt.Printf("   ✓ %d points stamped\n", len(metadata))
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

// runQuery reads a JSON report, applies taxonomy filters, and prints matching clusters.
func runQuery(reportPath, topic, mode, posture string, mismatchOnly bool) {
	data, err := os.ReadFile(reportPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading report: %v\n", err)
		os.Exit(1)
	}
	var report synthesis.JSONReport
	if err := json.Unmarshal(data, &report); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing report: %v\n", err)
		os.Exit(1)
	}

	q := taxonomy.Query{
		Topic:            topic,
		Mode:             mode,
		EpistemicPosture: posture,
		LabelMismatch:    mismatchOnly,
	}

	var matched []synthesis.JSONCluster
	for _, jc := range report.Clusters {
		var cl models.Cluster
		cl.ID = jc.ID
		cl.Label = jc.Label
		if jc.Taxonomy != nil {
			cl.Taxonomy = &models.TaxonomyLabel{
				Topic:            jc.Taxonomy.Topic,
				Mode:             jc.Taxonomy.Mode,
				EpistemicPosture: jc.Taxonomy.EpistemicPosture,
				LabelWarning:     jc.Taxonomy.LabelWarning,
			}
		}
		if q.MatchesCluster(cl) {
			matched = append(matched, jc)
		}
	}

	out, _ := json.MarshalIndent(matched, "", "  ")
	fmt.Printf("%s\n", out)
	fmt.Fprintf(os.Stderr, "Matched %d / %d clusters\n", len(matched), len(report.Clusters))
}

func main() {
	loadDotEnv(".env")
	showVersion := flag.Bool("version", false, "Print version and exit")
	collection := flag.String("collection", "meta_reflections", "Qdrant collection name")
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
	sampleStrategy := flag.String("sample-strategy", "random", "Sampling strategy: random, stratified, diverse, temporal")
	semanticLabels := flag.Bool("semantic-labels", false, "Generate semantic cluster labels via DeepSeek (requires --deepseek-key)")
	incremental := flag.Bool("incremental", false, "Only extract unstamped points (skip previously analyzed)")
	minClusterSize := flag.Int("min-cluster-size", 5, "Minimum DBSCAN cluster size")
	minSamples := flag.Int("min-samples", 3, "(no-op; DBSCAN uses --min-cluster-size only)")
	redisURL := flag.String("redis-url", "redis://localhost:6379", "Redis URL for vector workspace (e.g. redis://localhost:6379); empty = disabled")
	epsilon := flag.Float64("epsilon", 0.3, "DBSCAN neighbourhood radius (cosine distance; 0.3 = 70% similarity threshold)")
	clusterSeed := flag.Int64("cluster-seed", 42, "RNG seed for deterministic clustering (0 = random each run)")
	moatThreshold := flag.Float64("moat-threshold", 0.5, "Max centroid similarity for a pair to qualify as a knowledge moat (raise for dense corpora)")
	filterDegenerate := flag.Bool("filter-degenerate", true, "Exclude density=1.0/coherence=1.0 null-content clusters from bridge and moat analysis")
	autoTuneDBSCAN := flag.Bool("auto-tune-dbscan", false, "Adapt DBSCAN epsilon/minPts from sampled pairwise distance statistics")
	reasonerProfile := flag.String("reasoner-profile", "balanced", "Reasoning budget profile: fast, balanced, deep")
	reasonerMaxClusters := flag.Int("reasoner-max-clusters", -1, "Override max clusters sent to reasoner (-1 = profile default, 0 = all)")
	reasonerMaxBridges := flag.Int("reasoner-max-bridges", -1, "Override max bridges sent to reasoner (-1 = profile default, 0 = all)")
	reasonerMaxMoats := flag.Int("reasoner-max-moats", -1, "Override max moats sent to reasoner (-1 = profile default, 0 = all)")
	reasonerCache := flag.Bool("reasoner-cache", true, "Cache reasoner findings by deterministic topology fingerprint")
	queryReport := flag.String("query-report", "", "Path to a JSON report to query instead of running the pipeline")
	queryTopic := flag.String("query-topic", "", "Filter clusters by topic (e.g. consciousness_philosophy)")
	queryMode := flag.String("query-mode", "", "Filter clusters by mode (e.g. scholarly_annotation)")
	queryPosture := flag.String("query-posture", "", "Filter clusters by epistemic posture (e.g. doctrinal_assertion)")
	queryMismatch := flag.Bool("query-mismatch", false, "Filter to clusters where label and content disagree")
	flag.Parse()

	if *showVersion {
		fmt.Println("vectoreologist", version)
		return
	}

	// Query mode: read a JSON report and filter clusters by taxonomy.
	if *queryReport != "" || *queryTopic != "" || *queryMode != "" || *queryPosture != "" || *queryMismatch {
		reportPath := *queryReport
		if reportPath == "" {
			fmt.Fprintln(os.Stderr, "Error: --query-report is required when using --query-* flags")
			os.Exit(1)
		}
		runQuery(reportPath, *queryTopic, *queryMode, *queryPosture, *queryMismatch)
		return
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
		collection:          *collection,
		sampleSize:          *sampleSize,
		batchSize:           *batchSize,
		strict:              *strict,
		vectorName:          *vectorName,
		vectorCombine:       *vectorCombine,
		outputPath:          *outputPath,
		qdrantURL:           qdrant,
		deepseekKey:         dsKey,
		deepseekURL:         *deepseekURL,
		deepseekModel:       *deepseekModel,
		sampleStrategy:      *sampleStrategy,
		semanticLabels:      *semanticLabels,
		incremental:         *incremental,
		minClusterSize:      *minClusterSize,
		minSamples:          *minSamples,
		redisURL:            *redisURL,
		epsilon:             *epsilon,
		clusterSeed:         *clusterSeed,
		moatThreshold:       *moatThreshold,
		filterDegenerate:    *filterDegenerate,
		autoTuneDBSCAN:      *autoTuneDBSCAN,
		reasonerProfile:     *reasonerProfile,
		maxReasonerClusters: *reasonerMaxClusters,
		maxReasonerBridges:  *reasonerMaxBridges,
		maxReasonerMoats:    *reasonerMaxMoats,
		reasonerCache:       *reasonerCache,
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
