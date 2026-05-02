# AGENTS.md — Vectoreologist

Agent guidance for working in this repository.

---

## What this project is

Knowledge archaeology engine for vector space topology. It pulls vector embeddings from Qdrant, runs pure Go PCA + DBSCAN clustering, detects semantic anomalies, and uses DeepSeek R1 to reason visibly about what the vector space means. Output is timestamped markdown reports + findings stored back to Qdrant.

---

## Build & test

```bash
make build          # embeds git version via ldflags
make test           # go test ./...
go vet ./...        # must be clean before any commit
./vectoreologist --version
```

All tests run without external services (no Qdrant, no DeepSeek key needed).

---

## Package map

```
cmd/vectoreologist/main.go      CLI entry, flag wiring, .env loading, pipeline orchestration
internal/models/models.go       Shared types: VectorMetadata, Cluster, Bridge, Moat, Finding
internal/excavator/qdrant.go    Qdrant gRPC client — Extract(collection, limit)
internal/excavator/sampler.go   Sampling strategies: Random, Stratified, Diverse, Temporal
internal/topology/clusterer.go  PCA+DBSCAN pipeline, SetClusterParams, FindBridges, FindMoats
internal/topology/pca.go        pcaReduce(): parallel covariance-matrix PCA via gonum EigenSym
internal/topology/dbscan.go     runDBSCAN(), buildNeighborLists(), unitCosineDistance(), l2Normalise()
internal/anomaly/detector.go    DetectClusterAnomalies, DetectOrphans, DetectContradictions
internal/reasoner/deepseek.go   DeepSeek API client; extracts reasoning_content for R1
internal/synthesis/report.go    GenerateReport (markdown), StoreFindings (Qdrant stub)
internal/workspace/redis.go     Workspace: StoreBatch, LoadSample, TotalVectors, Delete; binary float32 encoding
```

---

## Key constraints & gotchas

### Qdrant client
- `qdrant.Config.Host` takes a **bare hostname** — the gRPC client appends `:6334` itself.
- Both `excavator.New()` and `synthesis.New()` call `hostname()` to strip `http://` from URLs.
- Max gRPC receive message size is set to **256 MB** — large collections need this.
- `ScrollPoints.Limit` is `*uint32`, not `uint32`.
- Point IDs can be numeric (`uint64`) or UUID strings. `GetNum()` returns 0 for UUID points; the code falls back to a 1-based sequential index in that case to keep IDs unique across the pipeline.

### Go clustering
- `pcaDims = 50` constant in `pca.go` — vectors are reduced to 50 dimensions before DBSCAN.
- PCA uses the covariance-matrix approach: only a d×d float64 matrix is allocated (never n×d), then `mat.EigenSym` from gonum decomposes it.
- L2-normalisation (`l2Normalise`) is applied to every vector before DBSCAN.
- DBSCAN default epsilon is `0.3` (cosine distance ≈ 70% similarity threshold), configurable with `--epsilon`.
- `buildNeighborLists` precomputes all pairwise neighbours in parallel using atomic counters.
- `MaxTopologyTotal = 20000` — input is random-sampled to this cap before PCA runs.

### Redis workspace
- Optional: enabled only when `--redis-url` is non-empty.
- Vectors are stored as binary float32 blobs; keys follow `veo:{runID}:{vec|meta|info}:{batchNum}`.
- TTL is 1 hour on all keys; `Delete()` is called at run end for explicit early cleanup.
- With Redis enabled, extraction streams batches directly to Redis; only `MaxTopologyTotal` vectors are loaded back into Go RAM for topology.

### DeepSeek reasoning
- Default model is `deepseek-reasoner` (R1). Each call can take up to 5 minutes — timeout is set accordingly.
- `callDeepSeek` returns a `deepSeekResponse{thinking, conclusion}` struct — `reasoning_content` is the R1 chain-of-thought.
- Reasoning is capped: **all clusters**, top **10 bridges** by strength, top **5 moats** by distance.
- `--deepseek-model deepseek-chat` for fast mode (no chain-of-thought).

### .env loading
- `loadDotEnv(".env")` runs before flag parsing in `main()`.
- Does not override vars already set in the environment.
- Strips single and double quotes from values.

### Versioning
- `var version = "dev"` in `main.go`, overridden at build time: `-ldflags "-X main.version=v1.2.3"`.
- `make build` does this automatically via `git describe --tags --always --dirty`.

---

## Testing approach

- **No mocks for Qdrant** — tests that need the client bypass the constructor using unexported struct fields or helper constructors (see `synthesis/report_test.go`'s `newTestSynthesizer`).
- **DeepSeek is mocked** with `httptest.NewServer` — never makes real API calls in tests.
- **Topology tests are pure Go** — no Python, no PATH manipulation; PCA tests verify output shape and cluster separability; DBSCAN tests cover cluster formation, noise, and symmetry; Redis integration tests skip gracefully when Redis is unavailable.
- Table-driven tests throughout; standard `testing` package only.

---

## CI / release

- **CI** (`ci.yml`): triggers on push/PR to main — `go vet` then `go test -count=1 -timeout=10m ./...`; no external services required beyond Go and Redis (workspace integration tests skip when Redis is absent).
- **Release** (`release.yml`): push a `v*` tag → builds 5 cross-platform binaries → GitHub release with CHANGELOG section as body.
- `CHANGELOG.md` uses Keep-a-Changelog format. Add an entry under `[Unreleased]` for any notable change; the release workflow extracts the matching `[x.y.z]` section automatically.

---

## What's stubbed / incomplete

- `synthesis.StoreFindings` — the Qdrant write path is a no-op stub (TODOs in place).
- `topology.AnalyzeClusters` cluster labels come from dominant `layer/source` metadata of member vectors — not semantically meaningful yet.
- `excavator.Sampler` strategies `Diverse` and `Temporal` fall back to random sampling.
- No streaming for DeepSeek responses — full response is buffered before printing.

---

## Style

- Standard library only in tests; no third-party test frameworks.
- Errors are surfaced with `fmt.Fprintf(os.Stderr, ...)` — no `log` package.
- Internal packages are not exported beyond the module; keep `models` as the only shared type package.
- Don't add docstrings, comments, or error handling for scenarios that can't happen.
