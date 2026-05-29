# Vectoreologist Quickstart

## New in this release

- **Higher analysis quality**: adaptive DBSCAN diagnostics, improved hybrid label promotion, and calibrated anomaly confidence bands in report output.
- **Lower reasoning cost**: streamed DeepSeek output, reasoner budget profiles/overrides, deterministic topology-fingerprint cache, and in-progress report artifacts.
- **New controls**: `--reasoner-profile`, `--reasoner-max-clusters`, `--reasoner-max-bridges`, `--reasoner-max-moats`, `--reasoner-cache`.

## 1. Prerequisites

**Go 1.23+**
```bash
go version
```

**Qdrant running**
```bash
docker ps | grep qdrant
# If not running:
docker run -d -p 6333:6333 qdrant/qdrant
```

**Redis (enabled by default at `localhost:6379`)**
```bash
./scripts/start-redis.sh
# Pulls redis:7-alpine and starts the vectoreologist-redis container on port 6379.
# Safe to re-run — starts an existing stopped container rather than recreating it.
# Pass --redis-url "" to disable if Redis is unavailable.
```

---

## 2. Clone & Build

```bash
git clone https://github.com/meistro57/vectoreologist.git
cd vectoreologist
make deps
make build
./vectoreologist --version
```

---

## 3. Configure

Create a `.env` file — it's loaded automatically at startup:

```bash
cat > .env << 'EOF'
DEEPSEEK_API_KEY=your_key_here
QDRANT_URL=http://localhost:6333
CLUSTER_SEED=42
MOAT_THRESHOLD=0.5
FILTER_DEGENERATE=true
EOF
```

No DeepSeek key? The tool still runs — Phase 4 reasoning is skipped and you still get topology + anomaly findings.

---

## 4. First Excavation

Run with no flags to excavate the default collection (`meta_reflections`, full extraction, Redis enabled):

```bash
./vectoreologist
```

Or target a specific collection:

```bash
./vectoreologist --collection my_collection
```

Or via make:

```bash
make run-collection COLLECTION=my_collection
```

---

## 5. Understanding the Output

### Console

```
🏺 Vectoreologist - Excavating my_collection from http://localhost:6333

📡 Phase 1: Vector Excavation
   ✓ Extracted 5000 vectors with metadata

🗺️  Phase 2: Topology Analysis
   ℹ 1224/5000 vectors classified as noise
   ✓ Identified 22 concept clusters
   ✓ Found 205 domain bridges
   ✓ Detected 0 knowledge moats

⚠️  Phase 3: Anomaly Detection
   ✓ Found 11 cluster anomalies
   ✓ Found 0 orphaned clusters
   ✓ Found 0 source contradictions

🧠 Phase 4: DeepSeek R1 Reasoning
   ✓ Reasoner budget: profile=balanced clusters=all bridges=10 moats=5
   reasoning 1/32: Cluster 1: surface / my_collection ...
   --- stream: Cluster 1: surface / my_collection ---
   ...live model output...
   --- end stream ---
   ↳ In-progress report: findings/vectoreology_in_progress.md (3/32)
   ✓ reasoning complete (32/32)

📝 Phase 5: Synthesis & Storage
   ✓ Report written to findings/vectoreology_2026-04-14_21-27-41.md
   ✓ Findings stored in vectoreology_findings collection

✨ Excavation Complete

Key Insights:
  • 22 semantic concepts discovered
  • 205 domain connections mapped
  • 0 knowledge gaps identified
  • 11 anomalies flagged for investigation

Read full analysis: findings/vectoreology_2026-04-14_21-27-41.md
```

**Noise vectors**: DBSCAN naturally excludes outliers that don't belong to any cluster — these are reported but not analysed further.

**Phase 4 speed**: default `--reasoner-profile balanced` reasons about all clusters + top 10 bridges + top 5 moats. Use `--reasoner-profile fast` to cap work (`12/4/2`) or `--reasoner-profile deep` for broader bridge/moat coverage (`all/25/12`). Fine-tune with `--reasoner-max-*` (`-1` = profile default, `0` = all). Chain-of-thought/live output may appear in the console stream, but markdown/JSON store final-facing output only.

### Markdown Report

Open `findings/vectoreology_*.md` to see:
- Executive Summary with duplicate-heavy, source-oversampled, skipped-bridge counts, and DBSCAN parameter diagnostics
- Evidence-gated cluster and bridge sections (insufficient evidence is explicitly marked)
- Semantic Attractors and Recommendations sections
- Final-facing interpretation text (chain-of-thought is not written into report markdown)

---

## 6. Common Workflows

### Fast pass (no reasoning)
```bash
./vectoreologist --collection my_collection --deepseek-model deepseek-chat
```

### Full R1 deep dive on a small collection
```bash
./vectoreologist --collection my_collection --sample 100 --reasoner-profile deep
```

### Cost-controlled reasoning pass
```bash
./vectoreologist --collection my_collection --reasoner-profile fast
./vectoreologist --collection my_collection --reasoner-profile balanced --reasoner-max-bridges 6 --reasoner-max-moats 3
```

### Warm cache, then rerun quickly on unchanged topology
```bash
./vectoreologist --collection my_collection --reasoner-cache=true
./vectoreologist --collection my_collection --reasoner-cache=true
```

### Disable reasoner cache
```bash
./vectoreologist --collection my_collection --reasoner-cache=false
```

### Compare two collections
```bash
./vectoreologist --collection collection_a --output ./findings/a
./vectoreologist --collection collection_b --output ./findings/b
diff findings/a/vectoreology_*.md findings/b/vectoreology_*.md
```

### Use a specific named vector
```bash
./vectoreologist --collection my_collection --vector-name summary_vec
```

### Combine all named vectors
```bash
./vectoreologist --collection my_collection --vector-combine
```

### Large collection (Redis is on by default)
```bash
./scripts/start-redis.sh   # ensure Redis container is running
./vectoreologist --collection my_large_collection
# or via make:
make run-collection COLLECTION=my_large_collection
```

### Tune moat sensitivity for dense corpora
```bash
./vectoreologist --collection my_collection --moat-threshold 0.65
```

### Use temporal sampling for recency-aware runs
```bash
./vectoreologist --collection my_collection --sample 5000 --sample-strategy temporal
```

### Reproducible vs random topology runs
```bash
# deterministic (default)
./vectoreologist --collection my_collection --cluster-seed 42

# different subsamples/link sets each run
./vectoreologist --collection my_collection --cluster-seed 0
```

### Include null-content clusters in bridge/moat analysis
```bash
./vectoreologist --collection my_collection --filter-degenerate=false
```

### Watch mode
```bash
./vectoreologist --collection my_collection --watch 5m
# or via make:
make run-watch COLLECTION=my_collection WATCH=10m
```

---

## 7. Troubleshooting

### "Failed to connect to Qdrant"
```bash
curl http://localhost:6333/collections
docker restart $(docker ps -q --filter ancestor=qdrant/qdrant)
```

### "No vectors returned"
```bash
# Verify collection name and contents
curl http://localhost:6333/collections
curl http://localhost:6333/collections/my_collection
```

### "No DeepSeek API key — skipping reasoning phase"
```bash
# Check .env is in the working directory
cat .env | grep DEEPSEEK_API_KEY

# Or pass it directly
./vectoreologist --collection my_collection --deepseek-key sk-...
```

### "Redis connection refused"
Redis is enabled by default. Either start the container or disable Redis:
```bash
./scripts/start-redis.sh
# or to run without Redis:
./vectoreologist --collection my_collection --redis-url ""
```

### Phase 4 hangs / times out
Each R1 call has a 5-minute timeout. Reduce reasoner budget first, then switch model if needed:
```bash
./vectoreologist --collection my_collection --reasoner-profile fast
./vectoreologist --collection my_collection --deepseek-model deepseek-chat
```

### Build errors
```bash
make clean
make deps
make build
```

### "Error: --batch-size must be > 0" (or similar flag validation errors)
Use valid numeric bounds:
```bash
./vectoreologist --collection my_collection --sample 5000 --batch-size 1000 --min-cluster-size 5 --min-samples 3
```

---

## 8. Next Steps

1. Read `DESIGN.md` for architecture details
2. Browse `findings/` for reports
3. Query the `vectoreology_findings` Qdrant collection directly
4. Tune `--sample` up for deeper coverage, down for faster iteration
5. Try `--sample-strategy diverse` to maximise vector-space coverage
6. Try `--sample-strategy temporal` for recency-aware time-window sampling
7. Raise `--moat-threshold` for dense corpora where clusters share baseline vocabulary
8. Use `--cluster-seed 0` when you want stochastic topology runs
9. Tune reasoner spend with `--reasoner-profile` and `--reasoner-max-*`
10. Keep `--reasoner-cache=true` for repeat runs on stable topologies
