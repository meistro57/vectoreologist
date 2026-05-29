# Vectoreologist

**vec·tor·e·ol·o·gist** | /ˌvɛk·tər·ɪˈɒl·ə·dʒɪst/ | *noun*  
> One who excavates meaning from the geometry of thought.
<img width="656" height="661" alt="image" src="https://github.com/user-attachments/assets/d8ebf658-0df6-448a-af11-7e44c1a6dd22" />

[![CI](https://github.com/meistro57/vectoreologist/actions/workflows/ci.yml/badge.svg)](https://github.com/meistro57/vectoreologist/actions/workflows/ci.yml)
[![Go 1.23+](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Vectoreologist analyzes embedding topology in Qdrant collections using pure Go PCA + DBSCAN, detects anomalies, runs DeepSeek reasoning over the topology, classifies every cluster on a 3-axis taxonomy (topic / mode / epistemic posture), writes timestamped markdown + JSON reports, stores findings in Qdrant, and includes a terminal lens for interactive exploration.

## What's New (2026-05)

- **Stabilized analysis quality** with adaptive DBSCAN diagnostics/fallback signaling, stronger hybrid label promotion, and calibrated anomaly confidence banding in markdown/JSON outputs.
- **Reduced runtime and cost** with streamed DeepSeek responses, reasoner budget profiles/overrides, deterministic topology-fingerprint caching, and incremental in-progress report assembly.
- **Expanded operator controls** via new reasoner flags: `--reasoner-profile`, `--reasoner-max-clusters`, `--reasoner-max-bridges`, `--reasoner-max-moats`, and `--reasoner-cache`.

---

## Pipeline

```
  Qdrant gRPC                                              findings/
  collection                                              *.md  *.json
      │                                                       ▲
      ▼                                                       │
 ┌──────────┐  ┌──────────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────┐
 │ ⛏ PHASE 1│─▶│  🗺 PHASE 2  │─▶│⚠ PHASE 3 │─▶│🧠 PHASE 4│─▶│  📋 PHASE 5  │
 │ Excavate │  │  Topology   │  │ Anomalies│  │ Reasoning│  │  Synthesis  │
 └──────────┘  └──────────────┘  └──────────┘  └──────────┘  └──────────────┘
                     │                               │
                     │  PCA → DBSCAN (pure Go)       │ DeepSeek R1 + label
                     │  clusters, bridges, moats     │ promotion → PHASE 4.5
                                                     ▼
                                             ┌──────────────┐
                                             │ 🔖 PHASE 4.5 │
                                             │  Taxonomy   │
                                             │  Classifier │
                                             │  + Repair   │
                                             └──────────────┘
```

All topology analysis is implemented in Go (`internal/topology/pca.go`, `internal/topology/dbscan.go`). No Python subprocess is spawned. Taxonomy classification (`internal/taxonomy/`) is rule-based keyword scoring — no extra LLM calls.

---

## What it does

1. **Extracts vectors + metadata** from a Qdrant collection over gRPC
2. **Samples vectors** with `random`, `stratified`, `diverse`, or `temporal` strategy
3. **Maps topology** — PCA → DBSCAN (pure Go, parallel, no subprocess), with optional adaptive DBSCAN tuning and explicit fallback diagnostics
4. **Finds structures**: clusters, semantic bridges, and moats
5. **Detects anomalies** with calibrated confidence banding (percentile baseline + z-style tails)
6. **Reasons with DeepSeek** (R1 by default) using streamed output, configurable budget profiles, and per-subject limits
7. **Classifies knowledge** on 3 axes: `topic`, `mode`, and `epistemic_posture` — no extra LLM calls
8. **Repairs misleading labels**: detects when a cluster's source-based label contradicts its content and sets a `label_warning`
9. **Caches reasoner findings** by deterministic topology fingerprint for repeat-run speedups
10. **Writes in-progress artifacts** (`vectoreology_in_progress.md/.json`) while reasoning runs
11. **Synthesizes outputs** to `findings/vectoreology_<timestamp>.md` and `.json`
12. **Stores findings** in Qdrant collection `vectoreology_findings`
13. **Supports query mode** to filter a JSON report by taxonomy axes without re-running the pipeline
14. **Supports incremental runs** by stamping processed points and skipping them on later runs
15. **Supports watch mode** for repeated excavation on a schedule

---

## Architecture

```
vectoreologist (Go CLI)
  ├─ Phase 1:   Excavation    Qdrant gRPC scroll, batched
  ├─ Phase 2:   Topology      pca.go + dbscan.go — PCA covariance → DBSCAN (in-process)
  ├─ Phase 3:   Anomaly       coherence, density, orphans, contradictions
  ├─ Phase 4:   Reasoning     DeepSeek R1 / chat — visible chain-of-thought + label promotion
  ├─ Phase 4.5: Taxonomy      rule-based 3-axis classifier + label repair + taxonomy anomalies
  └─ Phase 5:   Synthesis     Markdown + JSON + Qdrant findings upsert

vectoreologist-lens (Bubble Tea TUI)
  └─ Explore JSON reports: clusters, bridges, anomalies, search, export
```

---

## Requirements

| Dependency | Version | Purpose |
|---|---|---|
| Go | 1.23+ | Build binaries |
| gonum.org/v1/gonum | v0.15.1 | PCA (EigenSym) + matrix ops |
| Qdrant | running instance | Vector source + findings storage |
| DeepSeek API key | optional | Reasoning + semantic labels |
| Redis | default on `localhost:6379` | Streaming workspace — keeps Go heap at O(batch_size); Docker: `./scripts/start-redis.sh` |

---

## Install

```bash
git clone https://github.com/meistro57/vectoreologist.git
cd vectoreologist
make deps
make all
```

Binaries:
- `./vectoreologist`
- `./vectoreologist-lens`

---

## Configuration

A local `.env` is loaded automatically before flags are parsed (without overriding already-set environment vars):

```bash
DEEPSEEK_API_KEY=your_key_here
QDRANT_URL=http://localhost:6333
CLUSTER_SEED=42
MOAT_THRESHOLD=0.5
FILTER_DEGENERATE=true
```

If no DeepSeek key is provided, topology and anomaly phases still run and reasoning is skipped.

---

## Usage

```bash
# Default run — uses meta_reflections, full collection, Redis at localhost:6379
./vectoreologist

# Different collection, full extraction
./vectoreologist --collection my_collection

# Fixed sample size with diverse sampling
./vectoreologist --collection my_collection --sample 5000 --sample-strategy diverse

# Recency-aware temporal sampling (uses timestamp metadata when available)
./vectoreologist --collection my_collection --sample 5000 --sample-strategy temporal

# Incremental mode: only process unstamped points
./vectoreologist --collection my_collection --incremental

# Generate semantic labels using DeepSeek
./vectoreologist --collection my_collection --semantic-labels --deepseek-key "$DEEPSEEK_API_KEY"

# Watch mode: rerun every 5 minutes
./vectoreologist --collection my_collection --watch 5m

# Fast reasoning model
./vectoreologist --collection my_collection --deepseek-model deepseek-chat

# Use a specific named vector from multi-vector collections
./vectoreologist --collection my_collection --vector-name summary_vec

# Combine all named vectors into a single averaged vector
./vectoreologist --collection my_collection --vector-combine

# Disable Redis workspace
./vectoreologist --collection my_collection --redis-url ""

# Tune topology controls (deterministic seed, moat threshold, degenerate filtering)
./vectoreologist --collection my_collection --cluster-seed 42 --moat-threshold 0.65 --filter-degenerate=true

# Enable adaptive DBSCAN parameter selection
./vectoreologist --collection my_collection --auto-tune-dbscan

# Reasoner budget controls (profiles: fast, balanced, deep)
./vectoreologist --collection my_collection --reasoner-profile fast
./vectoreologist --collection my_collection --reasoner-profile deep --reasoner-max-bridges 40

# Disable topology-fingerprint reasoner cache
./vectoreologist --collection my_collection --reasoner-cache=false

# Query an existing JSON report — no pipeline run
./vectoreologist --query-report findings/vectoreology_2026-05-04_10-00-00.json \
  --query-topic consciousness_philosophy --query-mismatch

# Print version
./vectoreologist --version
```

Invalid values are rejected early (`--sample >= 0`, `--batch-size > 0`, `--min-cluster-size > 0`, `--reasoner-max-* >= -1`).

### Flags

| Flag | Default | Description |
|---|---|---|
| `--collection` | `meta_reflections` | Qdrant collection name |
| `--sample` | `0` | Number of vectors to sample (`0` = entire collection) |
| `--batch-size` | `5000` | Vectors per batch during extraction |
| `--strict` | `false` | Fail immediately if any extraction batch errors |
| `--vector-name` | `""` | Named vector to extract when points contain multiple named vectors |
| `--vector-combine` | `false` | Average all named vectors element-wise instead of selecting one |
| `--output` | `./findings` | Report output directory |
| `--qdrant-url` | `QDRANT_URL` or `http://localhost:6333` | Qdrant server URL |
| `--deepseek-key` | `DEEPSEEK_API_KEY` | DeepSeek API key |
| `--deepseek-url` | `https://api.deepseek.com/v1` | DeepSeek API base URL |
| `--deepseek-model` | `deepseek-reasoner` | `deepseek-reasoner` (R1, full chains) or `deepseek-chat` (fast) |
| `--watch` | `""` | Re-run on an interval (for example `5m`, `1h`) |
| `--sample-strategy` | `random` | Sampling strategy: `random`, `stratified`, `diverse`, `temporal` |
| `--semantic-labels` | `false` | Generate semantic cluster labels via DeepSeek |
| `--incremental` | `false` | Only extract points not stamped by prior runs |
| `--min-cluster-size` | `5` | Minimum DBSCAN cluster size |
| `--min-samples` | `3` | (no-op; DBSCAN uses `--min-cluster-size` only) |
| `--epsilon` | `0.3` | DBSCAN neighbourhood radius (cosine distance) |
| `--cluster-seed` | `42` | RNG seed for deterministic topology sampling/link selection (`0` = random each run) |
| `--moat-threshold` | `0.5` | Max centroid similarity to classify a pair as a moat (raise for dense corpora) |
| `--filter-degenerate` | `true` | Exclude density/coherence ≈ `1.0` null-content clusters from bridge + moat analysis |
| `--auto-tune-dbscan` | `false` | Adapt DBSCAN `epsilon` and `minPts` from sampled pairwise distance statistics |
| `--redis-url` | `redis://localhost:6379` | Redis URL for vector workspace; empty string disables it |
| `--reasoner-profile` | `balanced` | Reasoning budget profile: `fast`, `balanced`, `deep` |
| `--reasoner-max-clusters` | `-1` | Override max clusters sent to reasoner (`-1` = profile default, `0` = all) |
| `--reasoner-max-bridges` | `-1` | Override max bridges sent to reasoner (`-1` = profile default, `0` = all) |
| `--reasoner-max-moats` | `-1` | Override max moats sent to reasoner (`-1` = profile default, `0` = all) |
| `--reasoner-cache` | `true` | Cache reasoner findings under `findings/.cache/reasoner/` by topology fingerprint |
| `--query-report` | `""` | Path to a JSON report to query (pipeline does not run) |
| `--query-topic` | `""` | Filter clusters by topic (e.g. `consciousness_philosophy`) |
| `--query-mode` | `""` | Filter clusters by mode (e.g. `scholarly_annotation`) |
| `--query-posture` | `""` | Filter clusters by epistemic posture (e.g. `doctrinal_assertion`) |
| `--query-mismatch` | `false` | Restrict to clusters where label and content disagree |
| `--version` | — | Print version and exit |

---

## Taxonomy

Every cluster is classified on three axes after R1 label promotion. No extra API calls — pure keyword scoring on text fragments.

### Mode (what the text is *doing*)

| Value | Meaning |
|---|---|
| `didactic_teaching` | Pedagogical explanation aimed at building understanding |
| `meta_descriptive_summary` | Describes what a document/section does rather than its content |
| `scholarly_annotation` | Citation-heavy academic reference style |
| `transformational_dialogue` | Q&A or conversation format |
| `functional_definition` | Formal/mathematical definition |
| `unknown` | No mode signal detected above threshold |

### Epistemic posture (how certain the claim is)

| Value | Meaning |
|---|---|
| `doctrinal_assertion` | States things as established facts |
| `descriptive_abstract` | Generalizes with hedging language |
| `externally_referenced` | Defers to an external authority or study |
| `experiential_reframing` | First-person or perspective-shift framing |
| `conditional_revelation` | Conditional or hypothetical logic ("if … then …") |
| `unknown` | No posture signal detected above threshold |

### Topic (10 domains)

`consciousness_philosophy` · `quantum_mechanics` · `mathematics` · `computer_science` · `philosophy` · `biology` · `history` · `theology` · `psychology` · `linguistics` · `general`

### Label repair

When the classifier's `topic` or `mode` contradicts the cluster label, `label_warning` is populated in both the JSON output and the markdown report. The legacy label is preserved in `source_family` for backward compatibility.

### Confidence

Per-axis confidence is `(best_score − runner_up_score) / best_score`. Overall confidence is the average of the three. A value near `1.0` means one axis clearly dominated; near `0.0` means the signals were ambiguous.

### Query examples

```bash
# All doctrinal assertions about consciousness
./vectoreologist --query-report findings/run.json \
  --query-topic consciousness_philosophy --query-posture doctrinal_assertion

# All meta-descriptive clusters regardless of topic
./vectoreologist --query-report findings/run.json --query-mode meta_descriptive_summary

# Clusters where label and content disagree
./vectoreologist --query-report findings/run.json --query-mismatch

# Consciousness content regardless of assigned label (catches mislabeled clusters)
./vectoreologist --query-report findings/run.json --query-topic consciousness_philosophy
```

### Adding new modes or postures

1. Add a constant to `internal/taxonomy/taxonomy.go`.
2. Add signal phrases to the corresponding table in `internal/taxonomy/classifier.go` (`modeSignals` or `postureSignals`).
3. Add test cases in `internal/taxonomy/classifier_test.go`.

---

## Output

Each run emits:

- Console phase progress and summary
- Incremental in-progress report updates while reasoning runs: `findings/vectoreology_in_progress.md` + `.json`
- Final markdown report: `findings/vectoreology_<timestamp>.md`
- Final JSON report: `findings/vectoreology_<timestamp>.json`
- Optional reasoner cache artifact: `findings/.cache/reasoner/<topology-fingerprint>.json`
- Qdrant findings upsert to collection `vectoreology_findings`
- Point stamping payload `vectoreology_last_run=<RFC3339>` on processed source points (numeric + UUID IDs both supported via namespace-aware stamping)

JSON reports now also include synthesis-level diagnostics and action fields consumed by Lens:

- `summary.duplicate_heavy_clusters`, `summary.oversampled_clusters`, `summary.skipped_bridges`
- top-level `topology.parameters` + `topology.rationale` DBSCAN diagnostics
- top-level `attractors[]`, `recommendations[]`, `review_items[]`
- cluster-level `suggested_label`, `representative_snippets`, `source_balance`, `review_flags`, `duplicate_heavy`, `oversampled`
- bridge-level `label_a`, `label_b`, `evidence_a`, `evidence_b`, `shared_concept`, `review_flags`, `skipped`

### Structured evidence report generation (mb_ collections)

The markdown generator now enforces evidence-gated interpretation and emits:

- `## Executive Summary` with duplicate-heavy, oversampling, and skipped-bridge counts
- Cluster sections that separate machine findings from interpretive findings
- Bridge sections that hard-fail interpretation when either side lacks snippets
- `## Semantic Attractors` and `## Recommendations`
- Attractor keyword filtering to suppress generic metric terms (for example: `high`, `density`, `coherence`, `material`)
- Cleaned final-facing wording (reasoning leakage stripped), heading-safe truncation (no mid-word/mid-sentence clipping), and analysis/conclusion de-duplication

Run it against mb_ collections with the standard CLI flow:

```bash
./vectoreologist --collection mb_your_collection --output ./findings
```

Baseline comparison file used during this refactor:

- `findings/vectoreology_2026-05-14_13-44-21.md`

---

## Memory & scale

Topology analysis is fully in-process — no Python subprocess, no OOM guards needed. The pipeline caps input at `MaxTopologyTotal = 20,000` vectors, then PCA reduces to 50 dimensions in-process before DBSCAN runs. Peak RAM for topology is approximately 120–150 MB.

Redis workspace is enabled by default (`--redis-url redis://localhost:6379`). Extraction streams batches directly to Redis; only `MaxTopologyTotal` vectors are loaded into Go RAM for topology. Run `./scripts/start-redis.sh` to start a local Redis container. Pass `--redis-url ""` to disable if Redis is unavailable.

Topology runs are deterministic by default (`--cluster-seed 42`) and can be randomized with `--cluster-seed 0`. Moat sensitivity is configurable with `--moat-threshold` (default `0.5`, raise for dense corpora). Degenerate null-content clusters (density/coherence ~1.0) are filtered from bridge/moat analysis by default (`--filter-degenerate=true`). Optional adaptive DBSCAN tuning is available via `--auto-tune-dbscan`, with parameter rationale exported in markdown/JSON diagnostics.

Reasoning cost is tunable with `--reasoner-profile` (`fast`, `balanced`, `deep`) and granular `--reasoner-max-*` overrides (`-1` profile default, `0` all). Cached findings can be reused across equivalent topologies via `--reasoner-cache`.

Use `--sample` to limit extraction size. Use `--sample-strategy diverse` for coverage-focused MaxMin sampling, or `--sample-strategy temporal` for recency-weighted time-window sampling when timestamps are present.

---

## Vectoreologist Lens

Open a generated JSON report:

```bash
./vectoreologist-lens findings/vectoreology_2026-04-15_14-30-00.json
```

### Keybindings

| Key | Action |
|---|---|
| `↑↓` / `jk` | Move selection |
| `JK` | Scroll detail panel |
| `tab` | Cycle views |
| `c` / `b` / `a` | Cluster / bridge / anomaly view |
| `/` | Enter search |
| `enter` | Jump to selected search result |
| `esc` | Exit search / menus |
| `f` | Toggle anomalies-only filter |
| `s` | Cycle sort field |
| `e` | Open export menu |
| `j` (in export menu) | Export selected item |
| `v` (in export menu) | Export visible list |
| `r` | Reload report file |
| `q` | Quit |

---

## Make targets

```bash
make all                               # build CLI + lens
make build                             # build CLI
make lens                              # build lens
make run                               # go run CLI
make run-collection COLLECTION=my_col  # full collection via Redis workspace (low heap)
make run-redis COLLECTION=my_col       # alias — same as run-collection
make run-watch COLLECTION=my_col       # watch mode, reruns every 5m
make redis-start                       # start Redis Docker container
make redis-stop                        # stop Redis Docker container
make run-lens                          # open findings/vectoreology_*.json in lens
make test                              # go test ./...
make fmt                               # go fmt ./...
make lint                              # golangci-lint run
make clean                             # remove built binaries and findings/
```

---

## CI / Releases

- CI runs `go vet` and `go test -count=1 -timeout=10m ./...`
- Release workflow builds cross-platform binaries on `v*` tags and publishes GitHub releases

See:
- `.github/workflows/ci.yml`
- `.github/workflows/release.yml`
