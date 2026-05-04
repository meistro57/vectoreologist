# Vectoreologist Quickstart

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
   reasoning 1/32: Cluster 1: surface / my_collection ...

   --- thinking: Cluster 1: surface / my_collection ---
   Let me work through what this cluster represents...
   The density of 0.73 and coherence of 0.91 suggest tight grouping...
   ---

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

**Phase 4 speed**: DeepSeek R1 (`deepseek-reasoner`) reasons about every cluster + the top 10 bridges + top 5 moats. Each call can take 20–90 seconds. Use `--deepseek-model deepseek-chat` for fast mode (no chain-of-thought, results in seconds).

### Markdown Report

Open `findings/vectoreology_*.md` to see:
- Each cluster: full R1 `**Thinking:**` block + `**Conclusion:**`
- Top semantic bridges: why the domains connect
- Knowledge moats: why the domains are isolated
- Anomaly section: coherence failures, density outliers, source contradictions

---

## 6. Common Workflows

### Fast pass (no reasoning)
```bash
./vectoreologist --collection my_collection --deepseek-model deepseek-chat
```

### Full R1 deep dive on a small collection
```bash
./vectoreologist --collection my_collection --sample 100
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
Each R1 call has a 5-minute timeout. For large cluster counts use fast mode:
```bash
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
./vectoreologist --collection my_collection --sample 5000 --batch-size 1000 --min-cluster-size 5
```

---

## 8. Next Steps

1. Read `DESIGN.md` for architecture details
2. Browse `findings/` for reports
3. Query the `vectoreology_findings` Qdrant collection directly
4. Tune `--sample` up for deeper coverage, down for faster iteration
5. Try `--sample-strategy diverse` to maximise vector-space coverage
