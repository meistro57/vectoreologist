# Vectoreologist Quickstart

## 1. Prerequisites

**Go 1.23+**
```bash
go version
```

**Python 3.10+ with ML dependencies**
```bash
pip install umap-learn hdbscan numpy
```

**Qdrant running**
```bash
docker ps | grep qdrant
# If not running:
docker run -d -p 6333:6333 qdrant/qdrant
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

```bash
./vectoreologist --collection kae_chunks --sample 5000
```

Or via make shortcuts:

```bash
make excavate   # kae_chunks, 5000 vectors
make meta       # kae_meta_graph, 100 vectors
make history    # marks_gpt_history, 2000 vectors
make forum      # qmu_forum, 300 vectors
```

---

## 5. Understanding the Output

### Console

```
🏺 Vectoreologist - Excavating kae_chunks from http://localhost:6333

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
   reasoning 1/32: Cluster 1: surface / kae_chunks ...

   --- thinking: Cluster 1: surface / kae_chunks ---
   Let me work through what this cluster represents...
   The density of 0.73 and coherence of 0.91 suggest tight grouping...
   ---

   reasoning 2/32: Cluster 2: deep / kae_meta_graph ...
   ...
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

**Noise vectors**: HDBSCAN naturally excludes outliers that don't belong to any cluster — these are reported but not analysed further.

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
./vectoreologist --collection kae_chunks --deepseek-model deepseek-chat
```

### Full R1 deep dive on a small collection
```bash
./vectoreologist --collection kae_meta_graph --sample 100
```

### Compare two collections
```bash
./vectoreologist --collection kae_chunks --output ./findings/kae
./vectoreologist --collection marks_gpt_history --output ./findings/gpt
diff findings/kae/vectoreology_*.md findings/gpt/vectoreology_*.md
```

### Use a specific named vector
```bash
./vectoreologist --collection kae_chunks --vector-name summary_vec
```

### Combine all named vectors
```bash
./vectoreologist --collection kae_chunks --vector-combine
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
curl http://localhost:6333/collections/kae_chunks
```

### "No DeepSeek API key — skipping reasoning phase"
```bash
# Check .env is in the working directory
cat .env | grep DEEPSEEK_API_KEY

# Or pass it directly
./vectoreologist --collection kae_chunks --deepseek-key sk-...
```

### "missing dependency: No module named 'umap'" 
```bash
pip install umap-learn hdbscan numpy
```

### Phase 4 hangs / times out
Each R1 call has a 5-minute timeout. For large cluster counts use fast mode:
```bash
./vectoreologist --collection kae_chunks --deepseek-model deepseek-chat
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
./vectoreologist --collection kae_chunks --sample 5000 --batch-size 1000 --min-cluster-size 5 --min-samples 3
```

---

## 8. Next Steps

1. Read `DESIGN.md` for architecture details
2. Browse `findings/` for reports
3. Query the `vectoreology_findings` Qdrant collection via KAE Lens
4. Compare reports across collections to find convergence with KAE runs
5. Tune `--sample` up for deeper coverage, down for faster iteration
