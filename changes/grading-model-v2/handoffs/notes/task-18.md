# Task 18

## Done

- Verified the CI benchmark eval (`go run ./cmd/eval --dataset internal/eval/testdata/benchmark_dataset.json --all --format markdown`) on the 2025 NFL Weeks 7–10 replay dataset (236 legs across 70 SGPs), confirming `EngineV2` clears calibration and monotonicity gates against `EngineV1` and the market baseline (`Log Loss 0.5379` vs. `Market 0.6008`, `Brier 0.1838` vs. `0.2059`, `Classwise ECE 0.0319`, `9/10` populated bins, `SGP LL 0.4062` vs. `Naive 0.4063`, `Monotonic: true`).
- Switched `ActiveEngine = EngineV2` in `internal/analyzer/grade.go` and updated `TestGradingEngineRegistryAndV1Compatibility` in `internal/analyzer/grade_v2_test.go`.
- Preserved v1 rubric unit coverage by routing explicit v1 additive/factor tests in `internal/analyzer/analyzer_test.go`, `internal/analyzer/nflverse_grade_test.go`, `internal/analyzer/rubric_test.go`, and `internal/eval/replay_test.go` through `analyzer.EngineV1`.
- Enriched `internal/board/highlights_test.go` and `seedAGradeBoardForHubTest` in `internal/web/slips_test.go` with `EWMA`, `MarketScript`, and `Pinnacle` sharp benchmarks so board highlights run end-to-end under `EngineV2`, and updated `cmd/eval/main_test.go`, `internal/web/analyzer_test.go`, `internal/web/eval_results_test.go`, and `internal/web/pick_test.go` for `ActiveEngine = EngineV2`.
- Added build-tag guards (`internal/analyzer/grade_v2_race_test.go`, `internal/analyzer/grade_v2_norace_test.go`) on `TestEngineV2FullSlateLatencyBudget` in `internal/analyzer/grade_v2_test.go` so normal builds enforce the `<100ms` latency ceiling while `-race` full-suite runs still verify race-freedom without flaking on wall-clock load.

## Deviations

- Skipped `mise run eval-prod` because Google Application Default Credentials are not configured in this environment (`gcloud auth application-default print-access-token` fails without ADC, blocking `litestream restore` for `gs://parlay-club-parlay-litestream`); the user approved proceeding on the 2023–2025 walk-forward report (`ml/reports/latest.md`) and real-data CI benchmark instead.
- Replaced `internal/eval/testdata/benchmark_dataset.json` with real 2025 NFL Weeks 7–10 data (user approved replacing the synthetic v1 fixture rather than gating on its synthetic 95% hit rates).

## Open issues

- Run `gcloud auth application-default login && LITESTREAM_BUCKET=parlay-club-parlay-litestream mise run eval-prod` from an authenticated shell whenever you want an extra calibration double-check against live group slips.

## Next-phase notes

- Living docs (`docs/concepts/analyzer.md`, `docs/reference/analyzer-rules.md`, `docs/reference/routes.md`, etc.) still need updating in the remaining Phase 7 docs task to reflect `ActiveEngine = EngineV2`, the four-pillar UI, and `?engine=` on `/analyzer/results`.

## Commit

Promote EngineV2 to ActiveEngine

Activates the calibrated four-pillar v2 grading engine across live leg, slip, board highlight, and eval paths after confirming calibration, Log Loss, ECE, and SGP gates against real NFL data. Replaces the synthetic v1 benchmark dataset with real 2025 NFL Weeks 7–10 replay data and aligns v1/v2 and race test coverage.
