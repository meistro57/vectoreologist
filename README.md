# Vectoreologist

**vec·tor·e·ol·o·gist** | /ˌvɛk·tər·ɪˈɒl·ə·dʒɪst/ | *noun*  
> One who excavates meaning from the geometry of thought.

[![CI](https://github.com/meistro57/vectoreologist/actions/workflows/ci.yml/badge.svg)](https://github.com/meistro57/vectoreologist/actions/workflows/ci.yml)
[![Go 1.23+](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Vectoreologist analyzes embedding topology in Qdrant collections using pure Go PCA + DBSCAN, detects anomalies, runs DeepSeek reasoning over the topology, writes timestamped markdown + JSON reports, stores findings in Qdrant, and includes a terminal lens for interactive exploration.

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
                       │  PCA → DBSCAN (pure Go)             │
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

All topology analysis is implemented in Go (`internal/topology/pca.go`, `internal/topology/dbscan.go`). No Python subprocess is spawned.

---

## What it does

1. **Extracts vectors + metadata** from a Qdrant collection over gRPC
2. **Samples vectors** with `random`, `stratified`, or `diverse` strategy
3. **Maps topology** — PCA → DBSCAN (pure Go, parallel, no subprocess)
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
  ├─ Phase 2: Topology        pca.go + dbscan.go — PCA covariance → DBSCAN (in-process)
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

# Print version
./vectoreologist --version
```

Invalid values are rejected early (`--sample >= 0`, `--batch-size > 0`, `--min-cluster-size > 0`).

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
| `--sample-strategy` | `random` | Sampling strategy: `random`, `stratified`, `diverse` |
| `--semantic-labels` | `false` | Generate semantic cluster labels via DeepSeek |
| `--incremental` | `false` | Only extract points not stamped by prior runs |
| `--min-cluster-size` | `5` | Minimum DBSCAN cluster size |
| `--min-samples` | `3` | (no-op; DBSCAN uses `--min-cluster-size` only) |
| `--epsilon` | `0.3` | DBSCAN neighbourhood radius (cosine distance) |
| `--redis-url` | `redis://localhost:6379` | Redis URL for vector workspace; empty string disables it |
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

Topology analysis is fully in-process — no Python subprocess, no OOM guards needed. The pipeline caps input at `MaxTopologyTotal = 20,000` vectors, then PCA reduces to 50 dimensions in-process before DBSCAN runs. Peak RAM for topology is approximately 120–150 MB.

Redis workspace is enabled by default (`--redis-url redis://localhost:6379`). Extraction streams batches directly to Redis; only `MaxTopologyTotal` vectors are loaded into Go RAM for topology. Run `./scripts/start-redis.sh` to start a local Redis container. Pass `--redis-url ""` to disable if Redis is unavailable.

Use `--sample` to limit extraction size and `--sample-strategy diverse` to maximise vector-space coverage.

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
