# Vectoreologist Roadmap

## Current Status (2026-04)

### Completed
- Pure Go PCA + DBSCAN clustering (replaces Python subprocess entirely — no umap-learn, hdbscan, or Python required)
- Redis vector workspace (`--redis-url`) for streaming extraction on large collections
- `scripts/start-redis.sh` Docker setup for Redis
- `docker-compose.yml` for Qdrant + Redis
- Core `vectoreologist` CLI pipeline is working end-to-end:
  - Qdrant extraction (batched)
  - PCA + DBSCAN clustering (pure Go, in-process)
  - Bridge/moat detection
  - Anomaly detection
  - DeepSeek reasoning integration (`deepseek-reasoner` and `deepseek-chat`)
  - Markdown + JSON report generation
  - Findings upsert to `vectoreology_findings`
- Sampling and execution modes:
  - `random`, `stratified`, `diverse` sampling
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
   - Optional streaming output mode for reasoning progress
   - Better per-phase timing + throughput metrics in CLI output

5. **Operational Hardening**
   - Retry/backoff around network-bound operations (DeepSeek/Qdrant)
   - Optional write-disable mode for findings storage (`--no-store`)
   - Additional tests for incremental stamping edge cases

## Backlog

- Semantic label quality tuning and prompt iteration
- Better moat explanation heuristics beyond centroid distance threshold
- Temporal sampling strategy implementation (currently falls back)
- Additional report diff/comparison tooling across runs

## Definition of Done for Upcoming Lens Work

- New filters/export actions are discoverable in footer help
- Behavior is covered by unit tests in `internal/lens/*_test.go`
- `make test` remains green
- No regressions in existing keybindings or JSON report compatibility
