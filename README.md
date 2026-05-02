# Vectoreologist

**vec·tor·e·ol·o·gist** | /ˌvɛk·tər·ɪˈɒl·ə·dʒɪst/ | *noun*  
> One who excavates meaning from the geometry of thought.

[![CI](https://github.com/meistro57/vectoreologist/actions/workflows/ci.yml/badge.svg)](https://github.com/meistro57/vectoreologist/actions/workflows/ci.yml)
[![Go 1.23+](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Python 3.10+](https://img.shields.io/badge/Python-3.10+-3776AB?logo=python&logoColor=white)](https://python.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Vectoreologist analyzes embedding topology in Qdrant collections using UMAP + HDBSCAN, detects anomalies, runs DeepSeek reasoning over the topology, writes timestamped markdown + JSON reports, stores findings in Qdrant, and includes a terminal lens for interactive exploration.

---

## Pipeline

```
  Qdrant gRPC                                              findings/
  collection                                              *.md  *.json
      │                                                       ▲
      ▼                                                       │
 ┌──────────┐    ┌──────────────┐    ┌──────────┐    ┌──────────────┐
 │ ⛏ PHASE 1 │───▶│  🗺 PHASE 2  │───▶│ 🔬 PHASE 3│───▶│  📋 PHASE 5  │
 │ Excavate │    │  Topology   │    │ Anomalies│    │  Synthesis  │
 └──────────┘    └──────────────┘    └──────────┘    └──────────────┘
                       │                                     ▲
                       │  PCA → UMAP → HDBSCAN               │
                       │  clusters, bridges, moats           │
                       ▼                                     │
                 ┌──────────────┐                            │
                 │  🧠 PHASE 4  │────────────────────────────┘
                 │  Reasoning  │
                 │ DeepSeek R1 │
                 │  chain-of-  │
                 │   thought   │
                 └──────────────┘
```

`internal/topology/cluster.py` is compiled into the binary via `go:embed` and spawned at runtime. It applies PCA pre-reduction before UMAP to stay memory-safe on large, high-dimensional collections.

---

## What it does

1. **Extracts vectors + metadata** from a Qdrant collection over gRPC
2. **Samples vectors** with `random`, `stratified`, or `diverse` strategy
3. **Maps topology** — PCA pre-reduction → UMAP → HDBSCAN (memory-efficient, no OOM on large collections)
4. **Finds structures**: clusters, semantic bridges, and moats
5. **Detects anomalies**: cluster anomalies, orphans, and source contradictions
6. **Reasons with DeepSeek** (R1 by default; chain-of-thought logged live)
7. **Synthesizes outputs** to `findings/vectoreology_<timestamp>.md` and `.json`
8. **Stores findings** in Qdrant collection `vectoreology_findings`
9. **Supports incremental runs** by stamping processed points and skipping them on later runs
10. **Supports watch mode** for repeated excavation on a schedule

---

## Architecture

```
vectoreologist (Go CLI)
  ├─ Phase 1: Excavation      Qdrant gRPC scroll, batched
  ├─ Phase 2: Topology        embedded cluster.py — PCA → UMAP → HDBSCAN
  ├─ Phase 3: Anomaly         low coherence, density outliers, orphans, contradictions
  ├─ Phase 4: Reasoning       DeepSeek R1 / chat — visible chain-of-thought
  └─ Phase 5: Synthesis       Markdown + JSON + Qdrant findings upsert

vectoreologist-lens (Bubble Tea TUI)
  └─ Explore JSON reports: clusters, bridges, anomalies, search, export
```

---

## Requirements

| Dependency | Version | Purpose |
|---|---|---|
| Go | 1.23+ | Build binaries |
| Python | 3.10+ | Run embedded clustering script |
| umap-learn | latest | Dimensionality reduction |
| hdbscan | latest | Clustering |
| numpy | latest | Numeric ops |
| scikit-learn | latest | PCA pre-reduction (installed with umap-learn) |
| Qdrant | running instance | Vector source + findings storage |
| DeepSeek API key | optional | Reasoning + semantic labels |

```bash
pip install umap-learn hdbscan numpy
# scikit-learn is pulled in automatically as a umap-learn dependency
```

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
```

If no DeepSeek key is provided, topology and anomaly phases still run and reasoning is skipped.

---

## Usage

```bash
# Full collection (sample=0 means all vectors)
./vectoreologist --collection kae_chunks

# Fixed sample size with diverse sampling
./vectoreologist --collection kae_chunks --sample 5000 --sample-strategy diverse

# Incremental mode: only process unstamped points
./vectoreologist --collection kae_chunks --incremental

# Generate semantic labels using DeepSeek
./vectoreologist --collection kae_chunks --semantic-labels --deepseek-key "$DEEPSEEK_API_KEY"

# Watch mode: rerun every 5 minutes
./vectoreologist --collection kae_chunks --watch 5m

# Fast reasoning model
./vectoreologist --collection kae_chunks --deepseek-model deepseek-chat

# Use a specific named vector from multi-vector collections
./vectoreologist --collection kae_chunks --vector-name summary_vec

# Combine all named vectors into a single averaged vector
./vectoreologist --collection kae_chunks --vector-combine

# Print version
./vectoreologist --version
```

Invalid values are rejected early (`--sample >= 0`, `--batch-size > 0`, `--min-cluster-size > 0`, `--min-samples > 0`).

### Flags

| Flag | Default | Description |
|---|---|---|
| `--collection` | _(required)_ | Qdrant collection name |
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
| `--sample-strategy` | `random` | Sampling strategy: `random`, `stratified`, `diverse` |
| `--semantic-labels` | `false` | Generate semantic cluster labels via DeepSeek |
| `--incremental` | `false` | Only extract points not stamped by prior runs |
| `--min-cluster-size` | `5` | Minimum HDBSCAN cluster size |
| `--min-samples` | `3` | HDBSCAN `min_samples` |
| `--version` | — | Print version and exit |

---

## Output

Each run emits:

- Console phase progress and summary
- Markdown report: `findings/vectoreology_<timestamp>.md`
- JSON report: `findings/vectoreology_<timestamp>.json`
- Qdrant findings upsert to collection `vectoreology_findings`
- Point stamping payload `vectoreology_last_run=<RFC3339>` on processed source points

---

## Memory & scale

The topology phase is the most resource-intensive step. The pipeline uses three layers of protection against OOM kills:

| Guard | What it does |
|---|---|
| **Go-side cap** | Downsamples to 8,000 vectors before passing anything to Python |
| **PCA pre-reduction** | Shrinks vectors to 50 dimensions before UMAP — cuts NN-graph memory ~30× for typical LLM embeddings (1536 → 50 dims) |
| **`low_memory=True`** | Switches UMAP to an algorithm that avoids materialising the full distance matrix |

For very large collections use `--sample` to control the extraction size and `--sample-strategy diverse` to ensure the sample covers the full vector space.

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
make all          # build CLI + lens
make build        # build CLI
make lens         # build lens
make run          # go run CLI
make excavate     # sample run on kae_chunks
make meta         # sample run on kae_meta_graph
make history      # sample run on marks_gpt_history
make forum        # sample run on qmu_forum
make watch        # watch kae_chunks every 5m
make watch-meta   # watch kae_meta_graph every 10m
make run-lens     # open findings/vectoreology_*.json in lens
make test         # go test ./...
make fmt          # go fmt ./...
make lint         # golangci-lint run
make clean        # remove built binaries and findings/
```

---

## CI / Releases

- CI runs `go vet` and `go test -count=1 -timeout=10m ./...`
- Release workflow builds cross-platform binaries on `v*` tags and publishes GitHub releases

See:
- `.github/workflows/ci.yml`
- `.github/workflows/release.yml`
