# Phase 3: Phase 3: Portable artifact contract and pure-Go distributional + copula engine

## Done

### Task 7: Create `internal/modelv2` with `ArtifactBundle` schema validation, `//go:embed` artifact loader (seeded with a valid baseline artifact), flat GBDT tree evaluator, Weeks 1–5 Empirical Bayes shrinkage, and isotonic/Platt calibrator, with unit tests and benchmarks.

- Created `internal/modelv2/artifact.go` with `ArtifactBundle`, `DistModelArtifact`, `CopulaArtifact`,
  `ValidationSummary`, `//go:embed artifacts/*.json` (`LoadEmbedded`, `LoadEmbeddedPath`, `ParseArtifactBundle`), and
  strict schema/PSD Cholesky matrix validation.
- Added `internal/modelv2/gbdt.go` implementing zero-allocation `FlatTree.Eval` and `GBDTModel.Eval` (handling `<`,
  `>=`, and `NaN`/out-of-bounds `Missing` branches) plus iterative DFS cycle and bounds validation.
- Added `internal/modelv2/shrink.go` with canonical `FeatureSchemaVersion = 1` feature constants, `RolePrior`,
  position/role fallback lookup (`LookupPrior`), Weeks 1–5 Empirical Bayes shrinkage (`EmpiricalBayesShrink`,
  `ShrinkFeatureMap`, `ShrinkFeatureSlice`), and `AlignFeatures`/`AlignFeaturesInto`.
- Added `internal/modelv2/calibrator.go` implementing validated piecewise-linear `isotonic` interpolation and
  logit-space `platt` calibration, and seeded `internal/modelv2/artifacts/v2.json` covering all 7 prop markets, role
  priors, and factor-generated PSD copula matrices.
- Added unit tests and benchmarks across `artifact_test.go`, `gbdt_test.go`, `shrink_test.go`, and `calibrator_test.go`,
  passing `good-vibes verify lint`, `good-vibes verify test`, and zero-allocation hot-path benchmarks.

### Task 8: Implement pure-Go survival functions and variance/skewness estimators (`internal/modelv2/dist.go`) for Zero-Adjusted Gamma, Zero-Adjusted Log-Normal, Negative Binomial, Poisson, and Bernoulli markets, verified against known analytical quantiles and edge thresholds.

- Implemented `internal/modelv2/dist.go` with `Distribution`, `DistStats`, `LegPrediction`, and pure-Go parameter
  evaluation (`DistModelArtifact.EvalDistribution`, `DistModelArtifact.PredictLeg`) across all five market families:
  `ZeroAdjGamma`, `ZeroAdjLogNormal`, `NegBin`, `Poisson` (with optional `r` NegBin override), and `Bernoulli`.
- Added numerically stable pure-Go survival evaluators using regularized incomplete gamma (`regIncGamma` with power
  series and modified Lentz continued fraction), `math.Erfc`, log-space `math.Lgamma` PMFs, and regularized incomplete
  beta (`regIncBeta`) for upper-tail and high-threshold `NegBin` queries.
- Implemented cancellation-free analytical moment estimators (`Mean`, `Variance`, `StdDev`, `CV`, `Skewness`) sharing
  a unified `zeroAdjustedStats` helper for zero-inflated continuous mixtures.
- Added `internal/modelv2/dist_test.go` verifying all five families against closed-form analytical quantiles (Exponential,
  Erlang-3, scaled chi-squared, Log-Normal quantiles, Geometric/NegBin-2, Poisson, Bernoulli), edge thresholds, market
  blending with `uncertaintyBoost`, Over/Under complementarity, and zero-allocation hot-path benchmarks.
- Verified with `go test -v -race -bench=. -benchmem ./internal/modelv2/...`, `good-vibes verify lint`, and
  `good-vibes verify test`.

### Task 9: Implement the deterministic QMC Gaussian copula engine (`internal/modelv2/copula.go`) supporting multi-game factorization, same-game regime matrices, within-player overrides, Under sign flips, PSD jitter fallback, $p_{\text{joint}}$, $M_{\text{corr}}$, and ranked pairwise correlations, with tests and latency benchmarks.

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

### Task 7

- Task 8 (`internal/modelv2/dist.go`) can rely on `DistModelArtifact.ParamModels` keys validated per family:
  `zero_adj_gamma` (`pi0`, `mu`, `k`), `zero_adj_lognormal` (`pi0`, `mu`, `sigma`), `negbin` (`mu`, `r`), `poisson`
  (`lambda`), and `bernoulli` (`p`).
- Task 9 (`internal/modelv2/copula.go`) can rely on `CopulaArtifact.DefaultMatrix` and `RegimeMatrices` already being
  validated as symmetric, unit-diagonal, PSD correlation matrices (`nodes` length >= 2), with `RegimeNeutral`,
  `RegimeHeavyFavorite`, and `RegimeShootout` seeded in `artifacts/v2.json`.
- Phase 4 (`ml/export.py`) must export Platt calibrator parameters fit in `logit(p)` space
  (`sigmoid(PlattA*logit(p) + PlattB)` with `PlattA > 0`) and flat GBDT trees using `val < threshold -> Left`,
  `val >= threshold -> Right`, `NaN -> Missing` to match Go inference within `1e-6`.

### Task 8

- `DistModelArtifact.PredictLeg` returns `LegPrediction` with zero heap allocations (`BenchmarkPredictLeg`: `0 allocs/op`)
  and calibrates on the Over/positive scale before complementing for `UNDER`/`NO`, keeping Over and Under calibrated
  probabilities summing to `1.0`.
- Phase 4 (`ml/export.py` and `ml/train.py`) should calibrate on the Over outcome indicator (`y = 1{Y > line}`) and
  use the same parameterization (`ZeroAdjGamma`: `pi0=sigmoid`, `mu=exp`, `k=exp`; `ZeroAdjLogNormal`: `pi0=sigmoid`,
  `mu=raw`, `sigma=exp`; `NegBin`: `mu=exp`, `r=exp`; `Poisson`: `lambda=exp`; `Bernoulli`: `p=sigmoid`) to stay within
  `1e-6` parity.
- Task 9 (`internal/modelv2/copula.go`) and Phase 5 (`internal/analyzer/overlays.go`, `grade_v2.go`) can consume
  `LegPrediction` (`CalibratedProb`, `Stats.CV`, `Stats.Skewness`, `MarketProb`, `BlendWeight`) directly.

### Task 9

- Phase 4 (`ml/export.py` and `ml/parity`) can match Go's `p_joint` to machine precision on `parity_vectors.json` by
  evaluating the same Joe & Kuo (`new-joe-kuo-6.21201`) unscrambled Gray-code Sobol draws over `N = 8,192` with midpoint
  shift `+ 0.5/8192` and clamping marginal leg probabilities to `[1e-4, 1-1e-4]` before `normInv`.
- `CopulaArtifact.EstimateJoint` accepts `CopulaLeg` and `map[string]CopulaGameContext`; Phase 5
  (`internal/analyzer/grade_v2.go`) can pass `MarketScript` fields into `CopulaGameContext` and read `JointProb`,
  `IndependentProb`, `CorrMultiplier`, and sorted `PairCorrelations` directly for `Price & Vig` and `Script Fit`.
- `CopulaArtifact.Nodes` in `artifacts/v2.json` does not include standalone `ANYTIME_TD` or `TEAM_TOTAL` nodes;
  `copula.go` proxies those to attenuated yardage/TD nodes when cross-player pairs are evaluated while applying exact
  `WithinPlayer` entries (`REC_YDS:ANYTIME_TD`, `RUSH_YDS:ANYTIME_TD`, `RECEPTIONS:ANYTIME_TD`) for same-player pairs.

## Files touched

- `internal/modelv2/artifact.go`
- `internal/modelv2/artifact_test.go`
- `internal/modelv2/artifacts/v2.json`
- `internal/modelv2/calibrator.go`
- `internal/modelv2/calibrator_test.go`
- `internal/modelv2/copula.go`
- `internal/modelv2/copula_internal_test.go`
- `internal/modelv2/copula_test.go`
- `internal/modelv2/dist.go`
- `internal/modelv2/dist_test.go`
- `internal/modelv2/gbdt.go`
- `internal/modelv2/gbdt_test.go`
- `internal/modelv2/shrink.go`
- `internal/modelv2/shrink_test.go`

## Run log

- Mode: 
- Checkpoints: task-7, task-8, task-9
- Verify: verify passed: generate 600ms, lint reused, changed: internal/modelv2/copula.go, internal/modelv2/copula_internal_test.go, internal/modelv2/copula_test.go, test reused; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
