# Vectoreologist

**vec·tor·e·ol·o·gist** | /ˌvɛk·tər·ɪˈɒl·ə·dʒɪst/ | *noun*  
> One who excavates meaning from the geometry of thought.

[![CI](https://github.com/meistro57/vectoreologist/actions/workflows/ci.yml/badge.svg)](https://github.com/meistro57/vectoreologist/actions/workflows/ci.yml)

Vectoreologist analyzes embedding topology in Qdrant collections using UMAP + HDBSCAN, detects anomalies, runs DeepSeek reasoning over the topology, writes timestamped markdown + JSON reports, stores findings in Qdrant, and includes a terminal lens for interactive exploration.

---

## What it does

1. **Extracts vectors + metadata** from a Qdrant collection over gRPC
2. **Samples vectors** with `random`, `stratified`, or `diverse` strategy
3. **Maps topology** with embedded Python (`umap-learn` + `hdbscan`)
4. **Finds structures**: clusters, semantic bridges, and moats
5. **Detects anomalies**: cluster anomalies, orphans, and source contradictions
6. **Reasons with DeepSeek** (R1 by default, chat model optional)
7. **Synthesizes outputs** to `findings/vectoreology_<timestamp>.md` and `.json`
8. **Stores findings** in Qdrant collection `vectoreology_findings`
9. **Supports incremental runs** by stamping processed points and skipping stamped points on later runs
10. **Supports watch mode** for repeated excavation on a schedule

---

## Architecture

```
vectoreologist (Go CLI)
  ├─ Phase 1: Excavation (Qdrant gRPC scroll, batched)
  ├─ Phase 2: Topology (embedded cluster.py: UMAP + HDBSCAN)
  ├─ Phase 3: Anomaly detection
  ├─ Phase 4: DeepSeek reasoning
  └─ Phase 5: Synthesis (Markdown + JSON + Qdrant findings upsert)

vectoreologist-lens (Bubble Tea TUI)
  └─ Explore JSON reports: clusters, bridges, anomalies, search, export
```

`internal/topology/cluster.py` is embedded into the binary and executed at runtime via `python3`.

---

## Requirements

| Dependency | Version | Purpose |
|---|---|---|
| Go | 1.23+ | Build binaries |
| Python | 3.10+ | Run embedded clustering script |
| umap-learn | latest | Dimensionality reduction |
| hdbscan | latest | Clustering |
| numpy | latest | Numeric ops |
| Qdrant | running instance | Vector source + findings storage |
| DeepSeek API key | optional | Reasoning + semantic labels |

```bash
pip install umap-learn hdbscan numpy
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

If no DeepSeek key is provided, topology/anomaly phases still run and reasoning is skipped.

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

# Version
./vectoreologist --version
```

### Flags

| Flag | Default | Description |
|---|---|---|
| `--collection` | _(required)_ | Qdrant collection name |
| `--sample` | `0` | Number of vectors to sample (`0` = entire collection) |
| `--batch-size` | `5000` | Extraction batch size |
| `--strict` | `false` | Fail immediately on batch errors |
| `--output` | `./findings` | Output directory |
| `--qdrant-url` | env `QDRANT_URL` or `http://localhost:6333` | Qdrant URL |
| `--deepseek-key` | env `DEEPSEEK_API_KEY` | DeepSeek API key |
| `--deepseek-url` | `https://api.deepseek.com/v1` | DeepSeek API base URL |
| `--deepseek-model` | `deepseek-reasoner` | Reasoner model (`deepseek-reasoner` or `deepseek-chat`) |
| `--watch` | unset | Repeat run interval (`5m`, `1h`, etc.) |
| `--sample-strategy` | `random` | `random`, `stratified`, `diverse` |
| `--semantic-labels` | `false` | Generate semantic cluster labels via DeepSeek |
| `--incremental` | `false` | Only extract unstamped points |
| `--version` | `false` | Print version and exit |

---

## Output

Each run emits:

- Console phase progress and summary
- Markdown report: `findings/vectoreology_<timestamp>.md`
- JSON report: `findings/vectoreology_<timestamp>.json`
- Qdrant findings upsert to collection `vectoreology_findings`
- Point stamping payload `vectoreology_last_run=<RFC3339>` on processed source points

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
