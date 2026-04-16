# Changelog

All notable changes to this project will be documented here.

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
Versioning: [Semantic Versioning](https://semver.org/spec/v2.0.0.html)

---

## [Unreleased]

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
