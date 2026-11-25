# Task 7

## Done

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

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Task 8 (`internal/modelv2/dist.go`) can rely on `DistModelArtifact.ParamModels` keys validated per family:
  `zero_adj_gamma` (`pi0`, `mu`, `k`), `zero_adj_lognormal` (`pi0`, `mu`, `sigma`), `negbin` (`mu`, `r`), `poisson`
  (`lambda`), and `bernoulli` (`p`).
- Task 9 (`internal/modelv2/copula.go`) can rely on `CopulaArtifact.DefaultMatrix` and `RegimeMatrices` already being
  validated as symmetric, unit-diagonal, PSD correlation matrices (`nodes` length >= 2), with `RegimeNeutral`,
  `RegimeHeavyFavorite`, and `RegimeShootout` seeded in `artifacts/v2.json`.
- Phase 4 (`ml/export.py`) must export Platt calibrator parameters fit in `logit(p)` space
  (`sigmoid(PlattA*logit(p) + PlattB)` with `PlattA > 0`) and flat GBDT trees using `val < threshold -> Left`,
  `val >= threshold -> Right`, `NaN -> Missing` to match Go inference within `1e-6`.

## Commit

Add modelv2 artifact loader, GBDT, shrinkage, and calibrator

Establish the pure-Go, zero-I/O v2 inference foundation with embedded schema and PSD validation so upcoming
distributional survival functions, copula evaluation, and offline-trained artifacts share a single verified contract.
