# Changelog

All notable changes to this project will be documented here.

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
Versioning: [Semantic Versioning](https://semver.org/spec/v2.0.0.html)

---

## [Unreleased]

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

[Unreleased]: https://github.com/meistro57/vectoreologist/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/meistro57/vectoreologist/releases/tag/v0.1.0
