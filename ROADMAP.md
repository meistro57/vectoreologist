# Vectoreologist Roadmap

## Current Status (2026-07)

Vectoreologist has moved beyond the earlier stabilisation plan. The core excavation pipeline is now mature enough that the next phase should focus on trust, operator workflow, repeatable evaluation, and lower-friction exploration rather than more one-off topology features.

### Completed Since the Earlier Plan

- Pure Go PCA + DBSCAN remains the default topology engine, with optional adaptive DBSCAN tuning and explicit fallback diagnostics.
- Analysis output now includes calibrated anomaly confidence banding and structured markdown/JSON diagnostics.
- Cluster labelling has advanced from source-dominant labels to hybrid metadata/exemplar labels with optional reasoner promotion.
- DeepSeek reasoning now supports streamed CLI output, configurable reasoning budgets, deterministic topology-fingerprint caching, and in-progress markdown/JSON snapshots.
- Sampling has been upgraded beyond random fallbacks: `diverse` uses metadata-stratified MaxMin selection, and `temporal` uses timestamp/run-id windows with recency weighting.
- Point ID handling is deterministic and auditable across numeric and UUID Qdrant IDs through explicit namespace normalization.
- Incremental runs, watch mode, taxonomy classification, taxonomy-aware query mode, and the `vectoreologist-lens` terminal explorer are available.

## Product Direction

The next version should make Vectoreologist feel less like a powerful batch excavation tool and more like a dependable analysis workbench. The main goal is to help operators answer three questions quickly:

1. **Can I trust this run?**
2. **What changed since the last run?**
3. **What should I inspect or act on first?**

## Priority Plan

### 1. Run Trust and Diagnostics

- Add a per-run diagnostics artifact containing phase timings, vector counts, sampled/dropped counts, DBSCAN tuning source, Redis usage, cache hits, reasoner call counts, and Qdrant write results.
- Add a concise terminal run summary that highlights warnings and skipped/degraded phases at the end of every run.
- Promote existing topology/anomaly diagnostics into a stable machine-readable schema so Lens, tests, and future dashboards can consume them consistently.
- Add a `--diagnostics-only` mode that validates inputs, connectivity, model configuration, and output paths without running the full excavation.

### 2. Report Diffing and Trend Analysis

- Add a report comparison command that accepts two JSON reports and emits added/removed/changed clusters, bridges, moats, anomalies, taxonomy shifts, and confidence deltas.
- Add stable cluster matching heuristics across runs using centroid similarity, exemplar overlap, taxonomy labels, and source signatures.
- Add trend-oriented markdown and JSON output for repeated watch/incremental runs.
- Surface "new since previous run" and "worsened since previous run" sections in reports and Lens.

### 3. Lens Workflow Improvements

- Add adjustable coherence/density/confidence threshold filters.
- Add orphans-only and review-required filters.
- Add bridge navigation scoped to the currently selected cluster.
- Add CSV export for visible rows and clipboard copy for selected reasoning/evidence text.
- Add a diagnostics view that explains run quality, cache use, skipped bridges, and evidence limitations without opening raw JSON.

### 4. Reasoner Reliability and Cost Control

- Decide and implement stream-parse fallback behaviour: either hard-fail with a precise diagnostic or retry once using non-stream mode.
- Add retry/backoff with jitter for transient DeepSeek and Qdrant failures, keeping non-retryable errors explicit.
- Add budget previews before reasoning starts: estimated subject counts, cache hits, and expected API calls.
- Add optional per-run reasoner cost telemetry when token usage is available from provider responses.

### 5. Evaluation Harness

- Build a small fixture corpus suite with known topology shapes: tight clusters, bridge-heavy graphs, duplicate-heavy inputs, orphan-heavy inputs, and temporal drift.
- Add golden tests for markdown and JSON reports generated from deterministic fixtures.
- Add benchmarks for PCA reduction, neighbour-list construction, DBSCAN, sampler strategies, report rendering, and report diffing.
- Track benchmark output in CI initially as non-blocking artefacts, then introduce regression thresholds once stable.

### 6. Operational Controls

- Add `--no-store` to generate reports without writing findings back to Qdrant.
- Add configurable Redis TTL and `--keep-workspace-on-failure` for post-mortem debugging.
- Add an in-memory fallback mode when Redis is unavailable, guarded by explicit caps and warnings.
- Add a standalone ID-normalization audit command for collections where reproducibility matters before the first full run.

## Suggested Sequencing

### Phase 1: Trust Baseline

1. Per-run diagnostics JSON artifact.
2. End-of-run terminal summary.
3. `--no-store` for safe dry reporting.
4. Golden JSON/markdown tests for one deterministic fixture.

### Phase 2: Compare Runs

1. JSON report diff command.
2. Stable cluster matching heuristics.
3. Trend sections in generated reports.
4. Lens support for changed/new/review-required items.

### Phase 3: Operator Polish

1. Lens threshold filters and scoped bridge navigation.
2. CSV export and clipboard copy.
3. Diagnostics-only command.
4. Retry/backoff and stream fallback policy.

### Phase 4: Scale and Governance

1. Benchmark suite and CI artefacts.
2. Redis TTL/workspace retention controls.
3. Reasoner cost telemetry.
4. Standalone ID audit command.

## Backlog

- Better moat explanation heuristics beyond centroid distance threshold.
- Optional HTML report export for non-terminal review.
- Optional static dashboard generated from report JSON.
- Pluggable reasoner providers behind the existing reasoner interface.
- Config file support for teams that repeatedly run the same excavation profile.
- Report redaction controls for sensitive metadata/snippets.

## Definition of Done for Upcoming Work

- New CLI flags are documented in `README.md` and covered by argument validation tests.
- New JSON structures are backward-compatible or explicitly versioned.
- Report-rendering changes include golden tests.
- Lens behaviours are covered by unit tests in `internal/lens/*_test.go`.
- `go vet ./...` and `make test` pass before commit.
- Notable user-facing changes are recorded in `CHANGELOG.md` under `[Unreleased]`.
