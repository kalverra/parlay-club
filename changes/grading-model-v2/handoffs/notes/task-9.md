# Task 9

## Done

- Implemented the deterministic QMC Gaussian copula engine in `internal/modelv2/copula.go` (`CopulaLeg`,
  `CopulaGameContext`, `PairCorrelation`, `JointParlayEstimate`, `SelectRegime`, and `CopulaArtifact.EstimateJoint`).
- Added multi-game factorization (treating single-leg games and empty-`GameID` legs as independent so cross-game
  parlays preserve `CorrMultiplier == 1.0`), same-game regime selection (`heavy_favorite` when `|spread| >= 7`,
  `shootout` when `total >= 48`, else `neutral`), canonical node and proxy resolution, `WithinPlayer` overrides with
  same-market `1.0` handling and distinct-player collision attenuation, and directional Under/opponent sign flips.
- Implemented unit-diagonal-preserving PSD jitter fallback (`choleskyWithJitter` scaling off-diagonals by
  `1/(1+lambda)`), Acklam's `normInv` over `[1e-4, 1-1e-4]` clamped probabilities, and a precomputed `8,192 x 32`
  midpoint-shifted Joe & Kuo Sobol QMC normal sequence (with Halton extension past 32 legs in one game).
- Added ranked `PairCorrelations` sorted descending by `|Correlation|` and verified with unit tests and latency
  benchmarks across `copula_test.go` and `copula_internal_test.go` (2-leg SGP ~34 µs, 4-leg SGP ~65 µs, 8-leg SGP
  ~80 µs on M3 Max, well below the 0.5 ms target).
- Verified with `good-vibes verify lint`, `good-vibes verify test`, and `go test -v -race ./internal/modelv2/...`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Phase 4 (`ml/export.py` and `ml/parity`) can match Go's `p_joint` to machine precision on `parity_vectors.json` by
  evaluating the same Joe & Kuo (`new-joe-kuo-6.21201`) unscrambled Gray-code Sobol draws over `N = 8,192` with midpoint
  shift `+ 0.5/8192` and clamping marginal leg probabilities to `[1e-4, 1-1e-4]` before `normInv`.
- `CopulaArtifact.EstimateJoint` accepts `CopulaLeg` and `map[string]CopulaGameContext`; Phase 5
  (`internal/analyzer/grade_v2.go`) can pass `MarketScript` fields into `CopulaGameContext` and read `JointProb`,
  `IndependentProb`, `CorrMultiplier`, and sorted `PairCorrelations` directly for `Price & Vig` and `Script Fit`.
- `CopulaArtifact.Nodes` in `artifacts/v2.json` does not include standalone `ANYTIME_TD` or `TEAM_TOTAL` nodes;
  `copula.go` proxies those to attenuated yardage/TD nodes when cross-player pairs are evaluated while applying exact
  `WithinPlayer` entries (`REC_YDS:ANYTIME_TD`, `RUSH_YDS:ANYTIME_TD`, `RECEPTIONS:ANYTIME_TD`) for same-player pairs.

## Commit

Add deterministic QMC Gaussian copula engine

Provide reproducible same-game parlay dependency pricing, regime-aware correlation lookups, and ranked pairwise script
drivers in pure Go with sub-millisecond latency.
