# Changelog

All notable changes to this project will be documented here.

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
Versioning: [Semantic Versioning](https://semver.org/spec/v2.0.0.html)

---

## [Unreleased]

### Added
- **Semantic cluster labels from R1 conclusions** — after DeepSeek R1 reasoning, the first
  sentence of each cluster's `**Conclusion:**` is parsed, stripped of markdown, and promoted
  into `cluster.Label` (capped at ~80 chars). The original `layer/source`-based label from
  HDBSCAN is preserved in a new `cluster.Source` field and carried through to JSON reports.
  Bridges receive the same treatment via a new `bridge.Label` field.
- **Text snippets in cluster prompts** — for collections where source URLs are absent or
  uninformative (e.g. `kae_lens_findings`), `ReasonAboutTopology` now fetches up to 5 text
  payload fragments from each cluster's member vectors and includes them in the R1 prompt so
  the model reasons about actual content rather than just topology metrics.

### Fixed
- **Bridge prompts now include actual content** — `buildBridgePrompt` previously passed only
  cluster IDs and a strength float to R1, producing generic placeholder reasoning chains. It
  now accepts the `byID` fragment map and injects text snippets from both sides of the bridge
  via the pre-computed `Bridge.SampleLinks` (top-5 cross-cluster vector pairs), mirroring how
  cluster prompts already worked. Bridge reasoning chains now reflect actual cross-tradition
  content rather than "Clusters N and M may share metaphysical vocabulary."
- **R1 temperature set to 0** — `callDeepSeek` hardcoded `temperature: 0.7` for all calls
  including DeepSeek-Reasoner (R1). R1 uses chain-of-thought internally and is a deterministic
  reasoning model; 0.7 added noise without improving output quality. Temperature is now 0.
- **Conclusion parser now picks the last `**Conclusion:**` block** — R1 sometimes emits a
  verbose summary block followed by a terse bolded one (e.g. "The cluster represents
  **Common Surface Knowledge and Noise**."). `ExtractConclusionLabel` previously took the
  first block and returned the wordy sentence; it now takes the last, so the promoted label
  is the concise concept name R1 intended.
- **R1 prompt dump on first cluster** — `ReasonAboutTopology` prints the full assembled
  prompt for the first cluster (between `--- R1 prompt ---` delimiters) so it is trivial
  to verify whether text snippets are reaching the model or not.

### Watch mode (`--watch <duration>`) — re-runs the full excavation pipeline on a configurable
  interval (e.g. `--watch 5m`, `--watch 1h`), writing a new timestamped report each cycle and
  printing a one-line elapsed-time summary per cycle to stdout. Stops cleanly on SIGINT/SIGTERM.
  The timer restarts after each cycle completes, so overlapping runs are impossible even when a
  DeepSeek R1 call exceeds the interval. Cycle errors are logged as warnings; the loop continues.
- `make watch` and `make watch-meta` convenience targets
- **Diverse sampling** (`--sample-strategy diverse`) — implements greedy MaxMin (Farthest-First)
  selection that maximises the minimum pairwise distance across the chosen vectors. When selected,
  extracts a 1.5× larger pool from Qdrant then downsamples to `--sample`, ensuring the topology
  analysis sees the full spread of the vector space rather than a random slice.
- **Semantic cluster labels** (`--semantic-labels`) — calls `deepseek-chat` once per cluster
  after topology analysis to replace the raw `layer/source` label with a concise 3–6 word
  semantic description derived from the cluster's text fragments. Labels feed into all subsequent
  reasoning, reports, and the Lens TUI.
- **Lens TUI export** (`e` key) — press `e` in any view to open the export menu; `j` exports
  the currently selected item (cluster, bridge, or anomaly) to a JSON file; `v` exports the
  entire visible list; `esc` closes the menu. Files are written to the same directory as the
  loaded report.

---

## [0.3.0] - 2026-04-15

### Fixed
- **`StoreFindings` implemented** — the `vectoreology_findings` Qdrant collection is now actually created and populated on every run; previously the function was a no-op stub that printed a false success message
  - Creates the collection on first run (1-dimensional cosine vector; full embeddings deferred until an embedding API is wired in)
  - Stores each finding with payload fields: `type`, `subject`, `reasoning_chain`, `confidence`, `is_anomaly`, `clusters`, `stored_at`
  - Uses millisecond-timestamp-based IDs so successive runs append rather than overwrite

---

## [0.2.0] - 2026-04-15

### Added
- **Vectoreologist Lens** — interactive Bubbletea TUI for exploring findings (`vectoreologist-lens`)
  - Cluster view with coherence/density/size stats and full DeepSeek reasoning chains
  - Bridge navigator showing semantic connections between clusters with strength bars
  - Anomaly inspector grouped by detection type
  - Fuzzy search (`/`) across all cluster labels and reasoning text
  - Filter by anomalies-only (`f`), sort by coherence/density/size/id (`s`)
  - Cyberpunk color theme (cyan highlights, magenta anomalies, blue bridges)
- **JSON export** — every run now writes a matching `vectoreology_TIMESTAMP.json` alongside the markdown report, containing enriched clusters (with reasoning + `is_anomaly`), bridges, moats, and anomalies
- `make lens` and `make all` build targets; `make install-lens` for system-wide install
- New dependencies: `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/lipgloss`

### Fixed
- **Qdrant client upgraded to v1.17.1** — dense vectors now read from `VectorOutput.GetDense().Data` (protobuf field 101) instead of the deprecated `.Data` field, which was silently nil on Qdrant ≥1.12 servers and caused NaN floats in the UMAP pipeline
- Vector extraction handles both unnamed and named vector collections gracefully

### Changed
- `synthesis.GenerateReport()` now accepts a `collection` parameter and calls `GenerateJSON()` internally
- `go.mod` upgraded: Go 1.24, grpc v1.78.0, protobuf v1.36.11

---

## [0.1.0] - 2026-04-14

### Added
- Initial release of Vectoreologist — knowledge archaeology engine for vector space topology
- Phase 1: Vector excavation from Qdrant collections via gRPC with configurable sample size
- Phase 2: Real topology analysis using UMAP dimensionality reduction + HDBSCAN clustering (via embedded Python script)
- Phase 3: Anomaly detection — low coherence clusters, density outliers, orphaned clusters, source contradictions
- Phase 4: DeepSeek R1 reasoning with visible chain-of-thought; `reasoning_content` logged to terminal and stored in reports
- Phase 5: Markdown report synthesis written to `./findings/` with timestamped filenames
- `.env` file support for `DEEPSEEK_API_KEY` and `QDRANT_URL`
- CLI flags: `--collection`, `--sample`, `--output`, `--qdrant-url`, `--deepseek-key`, `--deepseek-url`, `--deepseek-model`, `--version`
- `--deepseek-model` flag to switch between `deepseek-reasoner` (default, full R1) and `deepseek-chat` (fast)
- gRPC max receive message size raised to 256 MB to handle large collections
- GitHub Actions CI workflow (`ci.yml`): Go 1.23, Python 3.11, umap-learn + hdbscan, `go vet` + `go test ./...`
- GitHub Actions release workflow (`release.yml`): builds binaries for linux/darwin/windows on `v*` tags

### Fixed
- Qdrant client `Config.Host` now strips `http://` scheme so `http://localhost:6333` works correctly
- `excavator.ScrollPoints.Limit` type corrected to `*uint32`
- DeepSeek response parsing no longer panics on empty or malformed API responses

[Unreleased]: https://github.com/meistro57/vectoreologist/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/meistro57/vectoreologist/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/meistro57/vectoreologist/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/meistro57/vectoreologist/releases/tag/v0.1.0
