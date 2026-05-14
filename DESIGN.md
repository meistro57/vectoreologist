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

Live console output can show model thinking, but report artifacts store final-facing conclusions only.

### Phase 4: Anomaly Detection
```
- Coherence anomalies (contradictory vectors in same cluster)
- Density anomalies (too tight or too loose)
- Orphan clusters (no bridges)
- Source contradictions (consensus across opposing sources)
```

### Phase 5: Synthesis
```
Findings + Topology + Metadata → Evidence-gated Markdown + Structured JSON + Qdrant Storage
```

Reports written to `findings/` with timestamped filenames.

Synthesis emits diagnostics-first sections:
- Executive Summary (`duplicate_heavy_clusters`, `oversampled_clusters`, `skipped_bridges`)
- Evidence-gated cluster and bridge interpretations
- Semantic Attractors and Recommendations
- JSON parity fields for Lens (`attractors`, `recommendations`, `review_items`, cluster/bridge review flags)

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
    AnomalyType    string
    Evidence       string
    PossibleCauses []string
    RequiresReview bool
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

## Executive Summary
- Total clusters: 23
- Total bridges: 15
- Total moats: 8
- Duplicate-heavy clusters: 2
- Source-oversampled clusters: 5
- Bridge interpretations skipped (insufficient evidence): 3

## Cluster Analysis
### Cluster 7: Consciousness and phenomenology across observer-centered metaphysics…
Machine Label: `surface / source_path`
Confidence: 0.89
Representative Evidence:
1. "..."
2. "..."
3. "..."
Analysis:
Final-facing interpretation text.
Conclusion:
Observer-centered metaphysical phenomenology
Review Flags:
- None

## Semantic Bridges
### Bridge: Cluster 7 ↔ Cluster 12
Shared Concept:
Measurement-consciousness coupling
Analysis:
Final-facing bridge interpretation.
Conclusion:
Measurement-consciousness coupling

## Semantic Attractors
1. **Consciousness**
   - Supporting clusters: 7, 12, 14

## Recommendations
- Rebalance overrepresented sources in flagged clusters.
```

## Success Metrics

Vectoreologist succeeds if it:
1. **Discovers latent structure** not visible in individual documents
2. **Surfaces high-coherence concept clusters** with explainable labels
3. **Reveals cross-domain bridges** invisible in text analysis
4. **Identifies knowledge gaps** via moat detection
5. **Generates actionable insights** about the embedding space structure
