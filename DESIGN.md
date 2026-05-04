# Vectoreologist Design Document

## Philosophy

**Vectoreologist treats vector embeddings as archaeological artifacts.** Instead of reasoning over text, it reasons over the *topology of semantic space itself*.

The core insight: **Vector space has emergent structure that isn't visible in individual chunks.** Clusters, bridges, moats, and anomalies in vector topology reveal patterns that only emerge when you analyze embeddings in aggregate.

This reveals:
- **Consensus concepts** that cluster tightly despite diverse sources
- **Knowledge bridges** that connect seemingly unrelated domains
- **Information moats** where no semantic connection exists (revealing gaps)
- **Contradictions** where similar vectors carry opposing metadata
- **Orphaned concepts** isolated from the rest of the embedding space

## Architecture

### Phase 1: Excavation
```
Qdrant → Sample Strategy → Vectors + Metadata
```

**Sampling strategies:**
- `random`: Baseline sampling
- `stratified`: Proportional sampling across sources
- `diverse`: MaxMin/FarthestFirst for coverage
- `temporal`: Time-windowed sampling

### Phase 2: Topology Analysis
```
Vectors → PCA Reduction (50 dims) → L2 Normalise → DBSCAN Clustering → Graph Construction
```

All computation is pure Go — no Python subprocess, no external runtime.

**Key metrics:**
- **Cluster coherence**: How tightly vectors group (mean cosine similarity to centroid)
- **Cluster density**: Vector concentration in reduced space
- **Bridge strength**: Inter-cluster cosine similarity
- **Moat distance**: Semantic isolation measure

### Phase 3: Reasoning (DeepSeek R1)
```
For each cluster: "What concept does this represent?"
For each bridge: "Why are these domains connected?"
For each moat: "Why is there no connection?"
```

**Visible reasoning chains** stored with `<think>` blocks preserved.

### Phase 4: Anomaly Detection
```
- Coherence anomalies (contradictory vectors in same cluster)
- Density anomalies (too tight or too loose)
- Orphan clusters (no bridges)
- Source contradictions (consensus across opposing sources)
```

### Phase 5: Synthesis
```
Findings → Markdown Report + JSON Report + Qdrant Storage
```

Reports written to `findings/` with timestamped filenames.

## Data Structures

### Cluster
Represents an emergent semantic concept in vector space.
```go
type Cluster struct {
    ID        int
    Label     string
    VectorIDs []uint64
    Centroid  []float32
    Density   float64
    Size      int
    Coherence float64
}
```

### Bridge
Semantic connection between knowledge domains.
```go
type Bridge struct {
    ClusterA int
    ClusterB int
    Strength float64
    LinkType string // strong_semantic, moderate_bridge, weak_connection
}
```

### Moat
Isolation between domains (gaps in knowledge).
```go
type Moat struct {
    ClusterA    int
    ClusterB    int
    Distance    float64
    Explanation string
}
```

### Finding
DeepSeek R1 reasoning result.
```go
type Finding struct {
    Type           string
    Subject        string
    ReasoningChain string
    Confidence     float64
    IsAnomaly      bool
    Clusters       []int
}
```

## Future Extensions

### Continuous Excavation
Watch Qdrant for new vectors, trigger incremental analysis.

### Multi-Collection Comparison
Compare vector topology across collections to find convergent clusters.

### Temporal Topology
Track how vector clusters evolve over time.

### Semantic Label Propagation
Use cluster labels to annotate raw vectors in the source collection.

## Performance Considerations

- **Memory**: 5000 vectors × 1536 dims × 4 bytes ≈ 30 MB; PCA covariance matrix is d×d (not n×d), bounded regardless of collection size
- **Clustering**: DBSCAN with precomputed neighbour lists, parallel across all CPU cores; PCA via covariance matrix O(n·d²) — parallel, bounded by d×d not n×d
- **Cap**: `MaxTopologyTotal = 20,000` — input is random-sampled before PCA runs
- **Redis workspace**: enabled by default (`redis://localhost:6379`); keeps Go heap at O(batch_size) during extraction; only `MaxTopologyTotal` vectors are loaded into RAM for topology

## Dependencies

- `github.com/qdrant/go-client` — Qdrant gRPC interaction
- `gonum.org/v1/gonum` — PCA (EigenSym) and matrix operations
- `github.com/redis/go-redis/v9` — optional Redis vector workspace
- No Python required

## Example Workflow

```bash
# Default run — meta_reflections, full collection, Redis enabled
./vectoreologist

# Analyse a specific collection (Redis on by default)
./vectoreologist --collection my_collection

# Compare two collections
./vectoreologist --collection collection_a --output ./findings/a
./vectoreologist --collection collection_b --output ./findings/b
diff findings/a/vectoreology_*.md findings/b/vectoreology_*.md

# Watch mode — rerun every 10 minutes
make run-watch COLLECTION=my_collection WATCH=10m

# Disable Redis if unavailable
./vectoreologist --collection my_collection --redis-url ""
```

## Output Example

```markdown
# Vectoreology Report

**Generated:** 2025-04-14T22:30:00Z

## Topology Summary

- **Clusters:** 23
- **Bridges:** 15
- **Moats:** 8

## Cluster Analysis

### Cluster 7: Consciousness & Phenomenology

**Reasoning Chain:**
<think>
This cluster shows high coherence (0.92). The tight clustering suggests
these sources converge on similar concepts. The centroid is semantically
close to "awareness as fundamental" and "observer-created reality".
</think>

**Anomaly:** None
**Confidence:** 0.89

### Bridge: Cluster 7 ↔ Cluster 12

**Strength:** 0.74 (strong_semantic)

**Reasoning Chain:**
<think>
Cluster 7 bridges to Cluster 12 through shared vocabulary around
measurement and observation. The bridge strength is higher than expected,
suggesting the collection treats these as more unified than conventional
analysis does.
</think>

## Knowledge Moats

### Moat: Cluster 3 ⊥ Cluster 18

**Distance:** 0.91

**Reasoning Chain:**
<think>
Cluster 3 and Cluster 18 show near-complete isolation. No semantic
bridges detected. This could be an opportunity: are these domains
truly unrelated, or is there a missing bridge worth exploring?
</think>
```

## Success Metrics

Vectoreologist succeeds if it:
1. **Discovers latent structure** not visible in individual documents
2. **Surfaces high-coherence concept clusters** with explainable labels
3. **Reveals cross-domain bridges** invisible in text analysis
4. **Identifies knowledge gaps** via moat detection
5. **Generates actionable insights** about the embedding space structure
