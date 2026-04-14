# Vectoreologist Quickstart

## Setup

1. **Install Go 1.23+**
```bash
# Check version
go version
```

2. **Clone and setup**
```bash
cd ~/Vectoreologist
make deps
```

3. **Configure environment**
```bash
cp .env.example .env
# Edit .env with your DeepSeek API key
```

4. **Ensure Qdrant is running**
```bash
docker ps | grep qdrant
# If not running:
docker run -d -p 6333:6333 qdrant/qdrant
```

## First Excavation

```bash
# Build
make build

# Run on your KAE chunks (5000 vector sample)
./vectoreologist \
  --collection kae_chunks \
  --sample 5000 \
  --output ./findings

# Or use make shortcuts:
make excavate     # kae_chunks
make meta         # kae_meta_graph
make history      # marks_gpt_history
make forum        # qmu_forum
```

## Understanding Output

### Console Output
```
🏺 Vectoreologist - Excavating kae_chunks from http://localhost:6333

📡 Phase 1: Vector Excavation
   ✓ Extracted 5000 vectors with metadata

🗺️  Phase 2: Topology Analysis
   ✓ Identified 23 concept clusters
   ✓ Found 15 domain bridges
   ✓ Detected 8 knowledge moats

⚠️  Phase 3: Anomaly Detection
   ✓ Found 3 cluster anomalies
   ✓ Found 2 orphaned clusters
   ✓ Found 1 source contradictions

🧠 Phase 4: DeepSeek R1 Reasoning
   ✓ Generated 46 reasoning chains
   ✓ Total findings: 52

📝 Phase 5: Synthesis & Storage
   ✓ Report written to ./findings/vectoreology_2025-04-14_22-30-15.md
   ✓ Findings stored in vectoreology_findings collection

✨ Excavation Complete

Key Insights:
  • 23 semantic concepts discovered
  • 15 domain connections mapped
  • 8 knowledge gaps identified
  • 6 anomalies flagged for investigation

Read full analysis: ./findings/vectoreology_2025-04-14_22-30-15.md
```

### Markdown Report
Check `findings/vectoreology_*.md` for:
- Cluster analysis with visible reasoning chains
- Bridge explanations (why domains connect)
- Moat analysis (why domains are isolated)
- Anomaly details with investigation prompts

### Qdrant Storage
Findings are stored in `vectoreology_findings` collection for:
- Cross-referencing with KAE runs
- Building meta-analysis over time
- Query via Qdrant API or KAE Lens

## Common Workflows

### Compare Collections
```bash
# Excavate multiple collections
./vectoreologist --collection kae_chunks --output ./kae_findings
./vectoreologist --collection marks_gpt_history --output ./gpt_findings

# Compare topology
diff kae_findings/vectoreology_*.md gpt_findings/vectoreology_*.md
```

### Deep Dive on Specific Collection
```bash
# Use larger sample for comprehensive analysis
./vectoreologist --collection kae_meta_graph --sample 96
```

### Validate KAE Concepts
```bash
# Run on same collection as a KAE run
# Compare KAE concept nodes with Vectoreologist clusters
# Look for:
#   - KAE concepts that form tight clusters (validated)
#   - Clusters with no corresponding KAE concept (missed by KAE)
#   - KAE concepts spread across multiple clusters (over-generalized)
```

## Troubleshooting

### "Failed to connect to Qdrant"
```bash
# Check if Qdrant is running
curl http://localhost:6333/collections

# Restart if needed
docker restart $(docker ps -q --filter ancestor=qdrant/qdrant)
```

### "No vectors returned"
```bash
# Verify collection exists
curl http://localhost:6333/collections

# Check collection has vectors
curl http://localhost:6333/collections/kae_chunks
```

### "DeepSeek API error"
```bash
# Verify API key is set
echo $DEEPSEEK_API_KEY

# Or check .env file
cat .env | grep DEEPSEEK
```

### Build errors
```bash
# Clean and rebuild
make clean
make deps
make build
```

## Next Steps

1. **Read DESIGN.md** for architecture details
2. **Explore findings/** directory for reports
3. **Query vectoreology_findings** collection via KAE Lens
4. **Compare with KAE runs** to find convergence
5. **Iterate on sampling strategies** for better coverage

## Advanced Usage

### Custom sampling
```go
// Edit internal/excavator/sampler.go
sampler := excavator.NewSampler(excavator.Stratified, time.Now().Unix())
```

### Adjust clustering parameters
```go
// Edit internal/topology/clusterer.go
topo := topology.New()
topo.neighbors = 20  // More neighbors = looser clusters
topo.minDist = 0.05  // Lower = tighter UMAP projection
```

### Custom anomaly detection
```go
// Edit internal/anomaly/detector.go
det := anomaly.New()
det.coherenceThreshold = 0.4  // Lower = more anomalies flagged
```
