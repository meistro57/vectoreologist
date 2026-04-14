# Vectoreologist

Archaeological reasoning engine for vector space topology. Excavates vector embeddings from Qdrant, analyzes topological structure, and uses DeepSeek R1 to reason about emergent semantic patterns, knowledge domain bridges, and anomalies.

## What It Does

Vectoreologist applies knowledge archaeology principles to **vector embeddings themselves**, not the source text. It:

1. **Excavates** vectors + metadata from Qdrant collections
2. **Analyzes topology** using dimensionality reduction and clustering
3. **Reasons visibly** via DeepSeek R1 about what clusters *mean*
4. **Detects anomalies** (contradictions, orphaned concepts, density weirdnesses)
5. **Synthesizes findings** into living reports + stores back to Qdrant

## Architecture

```
┌─────────────────────────────────────────────────┐
│  Vectoreologist (Go CLI Agent)                  │
├─────────────────────────────────────────────────┤
│  1. Vector Excavation (Qdrant client)           │
│  2. Topology Analysis (UMAP + HDBSCAN)          │
│  3. DeepSeek R1 Reasoning (visible chains)      │
│  4. Anomaly Detection (topology weirdness)      │
│  5. Synthesis (reports + Qdrant storage)        │
└─────────────────────────────────────────────────┘
```

## Usage

```bash
# Run full excavation pipeline
./vectoreologist \
  --collection kae_chunks \
  --sample 5000 \
  --output ./findings

# Specify Qdrant and DeepSeek endpoints
./vectoreologist \
  --qdrant-url http://localhost:6333 \
  --deepseek-key $DEEPSEEK_API_KEY \
  --collection kae_meta_graph
```

## Output

- **Markdown reports** in `./findings/vectoreology_TIMESTAMP.md`
- **Findings stored** in `vectoreology_findings` Qdrant collection
- **Cross-references** with KAE runs for convergence analysis

## Build

```bash
go mod tidy
go build -o vectoreologist ./cmd/vectoreologist
```

## Environment

```bash
export DEEPSEEK_API_KEY=your_key_here
export QDRANT_URL=http://localhost:6333
```
