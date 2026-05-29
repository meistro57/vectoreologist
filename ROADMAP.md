# Vectoreologist Roadmap

## Current Status (2026-05)

### Completed
- Pure Go PCA + DBSCAN clustering (replaces Python subprocess entirely — no umap-learn, hdbscan, or Python required)
- Redis vector workspace (`--redis-url`) for streaming extraction on large collections
- `scripts/start-redis.sh` Docker setup for Redis
- `docker-compose.yml` for Qdrant + Redis
- Core `vectoreologist` CLI pipeline is working end-to-end:
  - Qdrant extraction (batched)
  - PCA + DBSCAN clustering (pure Go, in-process)
  - Adaptive DBSCAN tuning + fallback diagnostics
  - Bridge/moat detection
  - Anomaly detection with calibrated confidence banding
  - DeepSeek reasoning integration (`deepseek-reasoner` and `deepseek-chat`)
  - Streamed reasoning output in CLI
  - Reasoner budget profiles/overrides (`fast`/`balanced`/`deep` + `--reasoner-max-*`)
  - Deterministic topology-fingerprint cache for reasoner findings
  - Incremental in-progress markdown/JSON report assembly
  - Markdown + JSON report generation
  - Findings upsert to `vectoreology_findings`
- Sampling and execution modes:
  - `random`, `stratified`, `diverse`, `temporal` sampling
  - Diverse sampler now uses metadata-stratified candidate pools + MaxMin selection
  - Temporal sampler now uses timestamp/run-id time windows with recency weighting
  - `--incremental` mode with point stamping (`vectoreology_last_run`)
  - `--watch` mode for scheduled reruns
- `vectoreologist-lens` TUI is implemented:
  - Cluster / bridge / anomaly views
  - Search and jump-to-result
  - Sorting and anomalies-only filter
  - Reload report from disk
  - JSON export for selected item / visible list
- Test coverage exists across core packages and lens logic.

## Gaps vs Earlier Lens Plan

### Not Yet Implemented
- Adjustable numeric filters in Lens (coherence/density thresholds)
- Orphans-only filter in Lens
- Bridge view scoped to selected cluster as default navigation mode
- CSV export from Lens
- Clipboard copy for reasoning chains

## Next Priorities

1. **Lens Filter Expansion**
   - Add interactive threshold controls for coherence/density
   - Add orphans-only toggle
   - Add tests for threshold + orphan filtering interactions

2. **Lens Navigation Improvements**
   - Add “show bridges from selected cluster” mode
   - Enable bridge-to-cluster jump consistency across filtered/sorted lists

3. **Lens Export Improvements**
   - Add CSV export for visible list
   - Add clipboard copy action for selected reasoning text

4. **Reasoning UX / Performance**
   - Better per-phase timing + throughput metrics in CLI output
   - Configurable cadence for in-progress report writes (count- or time-based)

5. **Operational Hardening**
   - Retry/backoff around network-bound operations (DeepSeek/Qdrant)
   - Decide stream-parse failure behavior (hard-fail vs non-stream fallback)
   - Integration tests for cache-hit/no-API-call behavior and mixed stream/non-stream responses
   - Optional write-disable mode for findings storage (`--no-store`)
   - Additional tests for incremental stamping edge cases

## Backlog

- Semantic label quality tuning and prompt iteration
- Better moat explanation heuristics beyond centroid distance threshold
- Additional report diff/comparison tooling across runs
- Optional standalone ID-normalization audit command output

## Definition of Done for Upcoming Lens Work

- New filters/export actions are discoverable in footer help
- Behavior is covered by unit tests in `internal/lens/*_test.go`
- `make test` remains green
- No regressions in existing keybindings or JSON report compatibility
