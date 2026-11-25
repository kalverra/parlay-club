# Task 8

## Done

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

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `DistModelArtifact.PredictLeg` returns `LegPrediction` with zero heap allocations (`BenchmarkPredictLeg`: `0 allocs/op`)
  and calibrates on the Over/positive scale before complementing for `UNDER`/`NO`, keeping Over and Under calibrated
  probabilities summing to `1.0`.
- Phase 4 (`ml/export.py` and `ml/train.py`) should calibrate on the Over outcome indicator (`y = 1{Y > line}`) and
  use the same parameterization (`ZeroAdjGamma`: `pi0=sigmoid`, `mu=exp`, `k=exp`; `ZeroAdjLogNormal`: `pi0=sigmoid`,
  `mu=raw`, `sigma=exp`; `NegBin`: `mu=exp`, `r=exp`; `Poisson`: `lambda=exp`; `Bernoulli`: `p=sigmoid`) to stay within
  `1e-6` parity.
- Task 9 (`internal/modelv2/copula.go`) and Phase 5 (`internal/analyzer/overlays.go`, `grade_v2.go`) can consume
  `LegPrediction` (`CalibratedProb`, `Stats.CV`, `Stats.Skewness`, `MarketProb`, `BlendWeight`) directly.

## Commit

Add pure-Go distributional survival and moment engine

Provide analytical survival probabilities, dispersion/skewness risk metrics, and market-anchored calibrated leg
predictions across all five prop distribution families in pure Go with zero runtime heap allocations.
