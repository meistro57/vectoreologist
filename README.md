# Vectoreologist

**vec·tor·e·ol·o·gist** | /ˌvɛk·tər·ɪˈɒl·ə·dʒɪst/ | *noun*
> One who excavates meaning from the geometry of thought.

---

Your vector database isn't just storage — it's a **fossilized map of how your AI thinks**. Vectoreologist is the tool that reads it.

It digs into your Qdrant collections, maps the hidden topology of your embeddings with UMAP + HDBSCAN, and unleashes DeepSeek R1 to reason — out loud, chain-of-thought and all — about what every cluster, bridge, and knowledge gap actually *means*. No black boxes. No vibes. Just visible reasoning about the structure of your semantic universe.

Then **Vectoreologist Lens** lets you navigate those findings interactively — scroll through clusters, jump between semantic bridges, inspect anomalies, and fuzzy-search your entire knowledge topology from a slick terminal UI.

[![CI](https://github.com/meistro57/vectoreologist/actions/workflows/ci.yml/badge.svg)](https://github.com/meistro57/vectoreologist/actions/workflows/ci.yml)

---

## What It Does

Vectoreologist applies knowledge archaeology to **vector embeddings themselves**, not the source text. It:

1. **Excavates** vectors + metadata from any Qdrant collection
2. **Maps topology** with real UMAP dimensionality reduction + HDBSCAN clustering — finds the clusters your data actually forms, not the ones you assumed
3. **Detects anomalies** — incoherent clusters, orphaned concepts, density outliers, source contradictions — the weird stuff worth investigating
4. **Reasons visibly** via DeepSeek R1: every cluster, every top bridge, every moat gets a full chain-of-thought + conclusion printed live to your terminal
5. **Synthesizes** everything into timestamped markdown **and JSON** reports, then stores findings back to Qdrant
6. **Explores interactively** with Vectoreologist Lens — a terminal UI for navigating clusters, bridges, anomalies, and reasoning chains

---

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│  vectoreologist  (Go CLI)                                │
├──────────────────────────────────────────────────────────┤
│  1. Excavation      Qdrant gRPC → vectors + metadata     │
│  2. Topology        UMAP + HDBSCAN (embedded Python)     │
│  3. Anomaly         coherence / density / orphan / moat  │
│  4. Reasoning       DeepSeek R1 — visible chains         │
│  5. Synthesis       findings/TIMESTAMP.{md,json}         │
└──────────────────────────────────────────────────────────┘
                              │
                              ▼
┌──────────────────────────────────────────────────────────┐
│  vectoreologist-lens  (Bubbletea TUI)                    │
├──────────────────────────────────────────────────────────┤
│  Clusters  │  Bridges  │  Anomalies  │  Search           │
│  ↑↓ navigate · b/a/c switch views · / search · q quit   │
└──────────────────────────────────────────────────────────┘
```

The Python clustering script is **embedded in the Go binary** — no separate installation step needed beyond the pip packages.

---

## Requirements

| Dependency | Version | Purpose |
|---|---|---|
| Go | 1.23+ | Build the binary |
| Python | 3.10+ | UMAP + HDBSCAN clustering |
| umap-learn | latest | Dimensionality reduction |
| hdbscan | latest | Density-based clustering |
| numpy | latest | Vector math |
| Qdrant | any | Vector database |
| DeepSeek API key | — | R1 reasoning (optional) |

```bash
pip install umap-learn hdbscan numpy
```

---

## Install

```bash
git clone https://github.com/meistro57/vectoreologist.git
cd vectoreologist
make deps
make all        # builds both vectoreologist and vectoreologist-lens
```

---

## Configuration

Create a `.env` file (loaded automatically at startup):

```bash
DEEPSEEK_API_KEY=your_key_here
QDRANT_URL=http://localhost:6333
```

Or pass everything as flags.

---

## Usage

```bash
# Full pipeline — default settings
./vectoreologist --collection kae_chunks --sample 5000

# Specify endpoints explicitly
./vectoreologist \
  --collection kae_chunks \
  --sample 5000 \
  --output ./findings \
  --qdrant-url http://localhost:6333 \
  --deepseek-key $DEEPSEEK_API_KEY

# Fast mode — DeepSeek V3 instead of R1 (no chain-of-thought)
./vectoreologist --collection kae_chunks --deepseek-model deepseek-chat

# Print version
./vectoreologist --version
```

### All flags

| Flag | Default | Description |
|---|---|---|
| `--collection` | _(required)_ | Qdrant collection name |
| `--sample` | `5000` | Max vectors to sample |
| `--output` | `./findings` | Report output directory |
| `--qdrant-url` | `http://localhost:6333` | Qdrant server URL |
| `--deepseek-key` | `$DEEPSEEK_API_KEY` | DeepSeek API key |
| `--deepseek-url` | `https://api.deepseek.com/v1` | DeepSeek API base URL |
| `--deepseek-model` | `deepseek-reasoner` | `deepseek-reasoner` (R1, full chains) or `deepseek-chat` (fast) |
| `--version` | — | Print version and exit |

---

## Output

**Console** — live progress with R1 thinking chains printed as they arrive:

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
   Let me analyze this cluster carefully...
   ---

   ✓ reasoning complete (32/32)

📝 Phase 5: Synthesis & Storage
   ✓ Report written to findings/vectoreology_2026-04-14_21-27-41.md
   ✓ JSON written to findings/vectoreology_2026-04-14_21-27-41.json
   ✓ Findings stored in vectoreology_findings collection
```

**Files written** to `findings/vectoreology_TIMESTAMP.{md,json}`:
- Markdown: cluster analysis with full R1 `**Thinking:**` / `**Conclusion:**` blocks, top bridges, moats, anomalies
- JSON: structured findings with reasoning attached to each cluster/bridge/moat — consumed by Vectoreologist Lens

---

## Vectoreologist Lens

Interactive TUI for exploring findings without leaving your terminal.

```bash
# Generate a report first
./vectoreologist --collection kae_chunks --sample 5000

# Then explore it
./vectoreologist-lens findings/vectoreology_*.json
```

**Keybindings:**

| Key | Action |
|---|---|
| `↑↓` / `jk` | Navigate list |
| `JK` | Scroll detail panel |
| `c` | Cluster view |
| `b` | Bridge view |
| `a` | Anomaly view |
| `/` | Fuzzy search |
| `f` | Toggle anomalies-only filter |
| `s` | Cycle sort (coherence / density / size / id) |
| `r` | Reload report from disk |
| `q` | Quit |

---

## Make shortcuts

```bash
make all            # build vectoreologist + vectoreologist-lens
make build          # CLI only
make lens           # TUI only
make install-lens   # copy vectoreologist-lens to /usr/local/bin
make excavate       # kae_chunks, sample 5000
make run-lens       # open latest findings with the lens
make test           # go test ./...
make fmt            # go fmt ./...
```

---

## CI / Releases

- **CI**: runs `go vet` + `go test ./...` on every push and PR (includes real UMAP+HDBSCAN tests)
- **Releases**: push a `v*` tag to build cross-platform binaries and create a GitHub release with changelog notes

```bash
git tag v0.2.0
git push origin v0.2.0
```
