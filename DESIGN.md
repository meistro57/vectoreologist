# Vectoreologist Design Document

## Philosophy

**Vectoreologist treats vector embeddings as archaeological artifacts.** Instead of reasoning over text, it reasons over the *topology of semantic space itself*.

The core insight: **Vector space has emergent structure that isn't visible in individual chunks.** Clusters, bridges, moats, and anomalies in vector topology reveal patterns that only emerge when you analyze embeddings in aggregate.

## Why This Matters

Your existing KAE runs have produced 7,380 concept nodes from 40,404 chunks. But those nodes were extracted by reasoning over *text content*. Vectoreologist asks: **What if we reason over the vector space directly?**

This reveals:
- **Consensus concepts** that cluster tightly despite diverse sources
- **Knowledge bridges** that connect seemingly unrelated domains
- **Information moats** where no semantic connection exists (revealing gaps)
- **Contradictions** where similar vectors carry opposing metadata
- **Orphaned concepts** isolated from the knowledge graph

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
Vectors → PCA Reduction → DBSCAN Clustering → Graph Construction
```

**Key metrics:**
- **Cluster coherence**: How tightly vectors group
- **Cluster density**: Vector concentration
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
Findings → Markdown Report + Qdrant Storage
```

Reports stored in `findings/` with cross-references to KAE runs.

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

## Integration with KAE

Vectoreologist complements KAE by:
1. **Validating KAE concepts** — do they cluster in vector space?
2. **Finding missed concepts** — clusters KAE didn't detect in text
3. **Cross-run convergence** — compare vector topology across KAE runs
4. **Meta-attractor validation** — do meta-graph attractors form tight clusters?

## Future Extensions

### KAE Lens-Style TUI
Bubbletea interface for:
- Live cluster visualization
- Interactive bridge exploration
- Anomaly drilling

### Continuous Excavation
Watch Qdrant for new vectors, trigger incremental analysis.

### Multi-Collection Comparison
Compare vector topology across:
- `kae_chunks` vs `marks_gpt_history`
- `qmu_forum` vs `aisc_manual`

Find convergent clusters across collections.

### Temporal Topology
Track how vector clusters evolve over time.

## Performance Considerations

- **Memory**: 5000 vectors × 1536 dims × 4 bytes = ~30MB; PCA covariance matrix is d×d (not n×d), bounded regardless of collection size
- **Clustering**: DBSCAN with precomputed neighbour lists, parallel across all CPU cores; PCA via covariance matrix O(n·d²) — parallel, bounded by d×d not n×d
- **DeepSeek calls**: Rate-limited, async batch processing

## Dependencies

- `github.com/qdrant/go-client` - Qdrant interaction
- `github.com/spf13/cobra` - CLI framework
- `gonum.org/v1/gonum` - PCA (EigenSym) and matrix operations
- `github.com/redis/go-redis/v9` - optional Redis vector workspace
- No Python required

## Example Workflow

```bash
# Excavate KAE chunks, find concept clusters
make excavate

# Analyze meta-graph for attractor validation
make meta

# Compare your GPT history topology
make history

# Cross-reference findings
./vectoreologist --collection kae_chunks --sample 10000 --output ./kae_findings
./vectoreologist --collection marks_gpt_history --sample 2000 --output ./gpt_findings

# Compare reports to find convergent concepts
diff kae_findings/vectoreology_*.md gpt_findings/vectoreology_*.md
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
This cluster shows high coherence (0.92) with vectors from Seth Speaks, 
QHHT transcripts, and Hemi-Sync documentation. The tight clustering 
suggests these sources converge on similar concepts of consciousness 
as primary reality. The centroid is semantically close to "awareness 
as fundamental" and "observer-created reality".

Key pattern: All vectors reference non-local consciousness, suggesting 
this is a stable attractor concept across your knowledge base.
</think>

**Anomaly:** None
**Confidence:** 0.89

### Bridge: Cluster 7 ↔ Cluster 12

**Strength:** 0.74 (strong_semantic)

**Reasoning Chain:**
<think>
Cluster 7 (Consciousness) bridges to Cluster 12 (Quantum Physics) 
through the observer effect and measurement problem. This is a 
well-established connection in consciousness studies.

Interesting: The bridge strength is higher than expected, suggesting 
your knowledge base treats these as more unified than conventional 
physics does. May reflect Seth/Bashar integration of physics + 
consciousness frameworks.
</think>

## Knowledge Moats

### Moat: Cluster 3 ⊥ Cluster 18

**Distance:** 0.91

**Reasoning Chain:**
<think>
Cluster 3 (Structural Steel Detailing) and Cluster 18 (Metaphysical 
Frameworks) show near-complete isolation. No semantic bridges detected.

This is expected given domain separation, but noteworthy: your mental 
model keeps these entirely separate. No cross-pollination of concepts 
like "structural integrity" → "reality frameworks" or vice versa.

Could be an opportunity: applying engineering rigor to metaphysics, 
or consciousness principles to structural design.
</think>
```

## Success Metrics

Vectoreologist succeeds if it:
1. **Finds concepts KAE missed** by analyzing vector topology
2. **Validates KAE findings** with cluster coherence scores
3. **Reveals cross-domain bridges** invisible in text analysis
4. **Identifies knowledge gaps** via moat detection
5. **Generates actionable insights** about your knowledge structure
