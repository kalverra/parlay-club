---
slug: grading-model-v2
run: 20261125-140438-grading-model-v2
outcome: done
generated: 2026-11-25T23:54:09Z
---

# Run summary: grading-model-v2

## Outcome

- Status: done
- Branch: grading-model-v2
- Close-out: written
- Close: pending
- Final verdict: agy@gemini-4-neon-high approved (issues: 0)

## Totals

- Tasks: 19 of 19
- Phases: 7
- Checkpoints: 2
- Duration: 21m26s
- Cost: $—
- Fresh input tokens: 1880187
- Cache read tokens: 33323662
- Cache write tokens: 0
- Output tokens: 181813

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-high | 23287569 | $— |
| gemini-4-neon-high | 12098093 | $— |

## Phases

### Phase 1: Power/Shin de-vigging and cached game-script anchors

Tasks:

- [x] Add `DevigTwoWayPower`, `DevigTwoWayShin`, `VigPct`, and `FairEV` in `internal/parlay/devig.go` with unit tests in `internal/parlay/devig_test.go` covering symmetric, favorite-longshot, and invalid-overround inputs.
- [x] Extend `analyzer.SharpQuote` (`PowerFairProb`, `VigPct`, `BookCount`) in `internal/analyzer/sharp.go` and derive `Game.MarketScript` (consensus spread, total, implied team totals) from cached `odds` rows in `internal/analyzer/signals.go`, with tests verifying zero extra DB queries or API calls.
- [x] Docs: update `docs/reference/odds-math.md` with Power and Shin de-vigging and `FairEV` formulas.

Phase totals: 0 checkpoints, 0s, $—, 0 input / 0 output / 0 cache read / 0 cache write tokens

Checkpoints: None. Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Add `DevigTwoWayPower`, `DevigTwoWayShin`, `VigPct`, and `FairEV` in `internal/parlay/devig.go` with unit tests in `internal/parlay/devig_test.go` covering symmetric, favorite-longshot, and invalid-overround inputs.

- Implemented `VigPct`, `DevigTwoWayPower`, `DevigTwoWayShin`, and `FairEV` in `internal/parlay/devig.go`, reusing the
  `[0.95, 1.25]` overround bounds from `internal/parlay/parlay.go` while keeping `DevigTwoWay` intact.
- Added unit tests in `internal/parlay/devig_test.go` covering symmetric lines (`-110/-110`, `-125/-125`, `+100/+100`,
  `+105/+105`), favorite-longshot lines (`-400/+300`, `-150/+130`, `-1500/+900`), invalid prices and out-of-range
  overrounds, and `FairEV` edge cases.

### Task 2: Extend `analyzer.SharpQuote` (`PowerFairProb`, `VigPct`, `BookCount`) in `internal/analyzer/sharp.go` and derive `Game.MarketScript` (consensus spread, total, implied team totals) from cached `odds` rows in `internal/analyzer/signals.go`, with tests verifying zero extra DB queries or API calls.

- Extended `analyzer.SharpQuote` with `PowerFairProb`, `VigPct`, and `BookCount` in `internal/analyzer/sharp.go`,
  populating them in `pairML`, `pairSpread`, `pairOverUnder`, `aggregateSharpTier`, and `aggregateTier3Consensus`
  via `parlay.DevigTwoWayPower` while preserving `FairProb` and `FairAmerican` for v1 parity.
- Added `analyzer.MarketScript` in `internal/analyzer/analyzer.go` and `BuildMarketScripts` / `BuildMarketScript` in
  `internal/analyzer/signals.go`, wiring `LoadSlipSignals` and `LoadBoardSignals` to derive consensus spread, total,
  and implied team totals from the single already-loaded `OddsForWeek` slice.
- Added unit tests in `internal/analyzer/sharp_test.go` and `internal/analyzer/signals_test.go` covering Power de-vig,
  vig percentages, tier/bucket `BookCount`, `MarketScript` consensus and implied-total math, and a query-counting +
  failing-HTTP-transport test proving zero extra DB queries or API calls.

### Phase 2: Free nflverse opportunity sync and role-archetype enrichment

Tasks:

- [x] Add `Client.FFOpportunity` in `internal/ingest/nflverse/opportunity.go` (plus fixtures and parser tests), add migration `internal/db/migrations/0019_v2_opportunity_signals.sql`, and wire fail-soft opportunity + `carry_share` ingestion through `internal/db/nflverse.go` and `internal/refresh/nflverse.go` with unit/integration tests.
- [x] Compute deterministic player role archetypes (`QB`, `WR1`, `WR2`, `TE1`, `RB1`, `RB2`), EWMA usage/opportunity features, and role-specific opponent defense allowed profiles in `internal/analyzer/signals.go` (respecting strict pre-kickoff replay in `internal/eval/replay.go`), with tests covering early-season sparse data and replay leakage.
- [x] Docs: update `docs/reference/data-sources.md` and `docs/reference/schema.md` for `ff_opportunity` and migration `0019`.

Phase totals: 0 checkpoints, 0s, $—, 0 input / 0 output / 0 cache read / 0 cache write tokens

Checkpoints: None. Review fixes: 0 Integration checks: 0

#### Done

### Task 4: Add `Client.FFOpportunity` in `internal/ingest/nflverse/opportunity.go` (plus fixtures and parser tests), add migration `internal/db/migrations/0019_v2_opportunity_signals.sql`, and wire fail-soft opportunity + `carry_share` ingestion through `internal/db/nflverse.go` and `internal/refresh/nflverse.go` with unit/integration tests.

- Added `Client.FFOpportunity` and `PlayerOpportunity` in `internal/ingest/nflverse/opportunity.go` (plus
  `DefaultFFOpportunityBaseURL`, optional-column CSV support in `client.go`, optional `passing_cpoe` parsing in
  `player_stats.go`, week-9 `ep_weekly_2026.csv` fixture, and parser tests).
- Added migration `internal/db/migrations/0019_v2_opportunity_signals.sql` adding `exp_pass_yds`, `exp_rush_yds`,
  `exp_rec_yds`, `exp_receptions`, `exp_pass_tds`, `exp_rush_tds`, `exp_rec_tds`, `carry_share`, and `cpoe` to
  `nflverse_player_weeks`, and updated `migrate_test.go` and `migrate_internal_test.go`.
- Extended `db.NFLversePlayerWeek`, `UpsertNFLversePlayerWeeks`, and `scanNFLversePlayerWeeks` in
  `internal/db/nflverse.go` with round-trip and idempotency tests in `internal/db/nflverse_test.go`.
- Added `FFOpportunity` to `refresh.NFLverseClient` and wired fail-soft `FFOpportunity` ingestion, `carry_share`
  (`carries / team_carries`), and `cpoe` (`passing_cpoe`) through `internal/refresh/nflverse.go` with unit and fail-soft
  tests in `internal/refresh/nflverse_test.go`.
- Updated `docs/reference/schema.md`, `docs/reference/data-sources.md`, `docs/concepts/data-refresh.md`, and
  `changes/grading-model-v2/design.md` to reflect migration 19 and the upstream `ffverse/ffopportunity` endpoint.

### Task 5: Compute deterministic player role archetypes (`QB`, `WR1`, `WR2`, `TE1`, `RB1`, `RB2`), EWMA usage/opportunity features, and role-specific opponent defense allowed profiles in `internal/analyzer/signals.go` (respecting strict pre-kickoff replay in `internal/eval/replay.go`), with tests covering early-season sparse data and replay leakage.

- Added `RoleArchetype` (`QB`, `WR1`, `WR2`, `TE1`, `RB1`, `RB2`), `PlayerEWMAUsage`, `PlayerRoleProfile`,
  `RoleDefenseProfile`, `Game.OpponentRoleDefense`, and `PlayerStats.Role` / `PlayerStats.EWMA` in
  `internal/analyzer/analyzer.go`.
- Implemented `ComputeEWMA`, `BuildPlayerEWMAUsage` / `BuildPlayerEWMAUsageWithHalfLife` (normalized `adjust=True`
  with 3-week half-life), `BuildPlayerRoleProfiles`, and `BuildOppRoleDefenseProfiles` in
  `internal/analyzer/signals.go`, and wired them into `LoadSlipSignals` / `LoadBoardSignals` via a single
  `Store.NFLversePlayerWeeksForSeasonTypes` query in `internal/db/nflverse.go` when player legs are present.
- Wired strict pre-kickoff replay of role archetypes, EWMA usage/opportunity features, and role-specific opponent
  defense profiles into `internal/eval/replay.go` (`loadPriorNFLverse`, `populatePriorNFLversePlayers`,
  `buildBasePreKickoffGame`, `loadArchivedPlayerStatsForAthletes`), strictly excluding same-week and future rows.
- Added unit and integration tests in `internal/db/nflverse_test.go`, `internal/analyzer/signals_test.go`, and
  `internal/eval/replay_test.go` covering sparse 1-week/0-week data, tie-breaking, permutation determinism, and
  replay leakage guards.

### Task 6: Docs: update `docs/reference/data-sources.md` and `docs/reference/schema.md` for `ff_opportunity` and migration `0019`.

- Updated `docs/reference/schema.md` so the `nflverse_player_weeks` table summary, DDL block (`exp_pass_yds`,
  `exp_rush_yds`, `exp_rec_yds`, `exp_receptions`, `exp_pass_tds`, `exp_rush_tds`, `exp_rec_tds`, `carry_share`, and
  `cpoe`), and conventions match migration `0019_v2_opportunity_signals.sql`.
- Updated `docs/reference/data-sources.md` to document `DefaultFFOpportunityBaseURL`, optional-column handling
  (`passing_cpoe` and `season_type`), week-based postseason fallback (`week > 18` since 2021, `> 17` through 2020),
  blank/`NA` defaults, `carry_share` and `cpoe` persistence during refresh, and the `ep_weekly_2026.csv` test fixture.
- Verified documentation with `good-vibes docs lint` and `rumdl check`.

### Phase 3: Portable artifact contract and pure-Go distributional + copula engine

Tasks:

- [x] Create `internal/modelv2` with `ArtifactBundle` schema validation, `//go:embed` artifact loader (seeded with a valid baseline artifact), flat GBDT tree evaluator, Weeks 1–5 Empirical Bayes shrinkage, and isotonic/Platt calibrator, with unit tests and benchmarks.
- [x] Implement pure-Go survival functions and variance/skewness estimators (`internal/modelv2/dist.go`) for Zero-Adjusted Gamma, Zero-Adjusted Log-Normal, Negative Binomial, Poisson, and Bernoulli markets, verified against known analytical quantiles and edge thresholds.
- [x] Implement the deterministic QMC Gaussian copula engine (`internal/modelv2/copula.go`) supporting multi-game factorization, same-game regime matrices, within-player overrides, Under sign flips, PSD jitter fallback, $p_{\text{joint}}$, $M_{\text{corr}}$, and ranked pairwise correlations, with tests and latency benchmarks.

Phase totals: 0 checkpoints, 0s, $—, 0 input / 0 output / 0 cache read / 0 cache write tokens

Checkpoints: None. Review fixes: 0 Integration checks: 0

#### Done

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

### Phase 4: Offline Python walk-forward training, calibration, and Go parity pipeline

Tasks:

- [x] Build the `uv`-managed Python pipeline under `ml/` (`data.py`, `features.py`, `train.py`, `copula.py`, `export.py`) with strict expanding-window walk-forward splits across ≥3 NFL seasons, KS covariate-shift checks, Spearman > 0.7 collinearity pruning, distributional GBDT + Optuna HPO, held-out isotonic/Platt calibration, market blend weight tuning, and PSD copula estimation, plus `pytest` leakage and stability tests.
- [x] Add `mise run train-v2` in `mise.toml`, export real trained artifacts to `internal/modelv2/artifacts/v2.json` and parity fixtures to `internal/modelv2/testdata/parity_vectors.json`, and add `internal/modelv2/parity_test.go` asserting Go inference and copula outputs match Python within `1e-6`.
- [x] Docs: update `docs/guides/local-dev.md` with local `mise run train-v2` workflow, cadence, and PR verification steps.

Phase totals: 0 checkpoints, 0s, $—, 0 input / 0 output / 0 cache read / 0 cache write tokens

Checkpoints: None. Review fixes: 0 Integration checks: 0

#### Done

### Task 10: Build the `uv`-managed Python pipeline under `ml/` (`data.py`, `features.py`, `train.py`, `copula.py`, `export.py`) with strict expanding-window walk-forward splits across ≥3 NFL seasons, KS covariate-shift checks, Spearman > 0.7 collinearity pruning, distributional GBDT + Optuna HPO, held-out isotonic/Platt calibration, market blend weight tuning, and PSD copula estimation, plus `pytest` leakage and stability tests.

- Created the `uv`-managed Python training workspace under `ml/` (`pyproject.toml`, `.python-version` pinned to `3.12`, `uv.lock`) and excluded `ml/` and its caches/venv from `.dockerignore` and `.gitignore`.
- Implemented `ml/data.py` to ingest, normalize, cache (Parquet), and join `nflreadpy` player stats, team stats, snap counts, players, fail-soft `ff_opportunity`, and `nfldata` schedules (`home_spread = -spread_line`).
- Implemented `ml/features.py` and `ml/distributions.py` with strictly pre-kickoff point-in-time EWMA/role-archetype/defense features, Weeks 1–5 Empirical Bayes shrinkage, KS two-sample covariate-shift checks, Spearman `> 0.7` collinearity pruning, exact distributional NLLs/Fisher Hessians/survival functions, and logit-space Platt / piecewise-linear Isotonic calibrators.
- Implemented `ml/copula.py`, `ml/train.py`, and `ml/export.py` providing expanding-window walk-forward training across ≥3 held-out NFL seasons, Optuna TPE HPO, market blend weight tuning in `[0.70, 0.85]`, Higham PSD copula estimation with exact Go-parity Joe & Kuo QMC Sobol evaluation, EWMA half-life stability checks, gate enforcement, and `v2.json` / `parity_vectors.json` / Markdown report export.
- Added offline `pytest` suites (`ml/tests/test_leakage.py`, `test_features.py`, `test_distributions.py`, `test_copula.py`, `test_pipeline_stability.py`), all 18 tests passing via `cd ml && uv run pytest -v` (7.29s).

### Task 11: Add `mise run train-v2` in `mise.toml`, export real trained artifacts to `internal/modelv2/artifacts/v2.json` and parity fixtures to `internal/modelv2/testdata/parity_vectors.json`, and add `internal/modelv2/parity_test.go` asserting Go inference and copula outputs match Python within `1e-6`.

- Added `[tasks.train-v2]` to `mise.toml` (`dir = "ml"`, `run = "uv run python export.py"`) and exported the 2019–2025 walk-forward trained bundle to `internal/modelv2/artifacts/v2.json` (`v2.1.0`, held-out `[2023, 2024, 2025]`, log loss `0.603842` vs. market `0.624456`, Brier `0.208705` vs. `0.216135`, SGP log loss `0.609369` vs. naive `0.640466`, `9/10` populated bins, stability MAE `0.009546`), along with `ml/reports/latest.md`.
- Exported 112 leg parity vectors (all 7 prop markets, Over/Under/Yes/No, Weeks 1–5 Empirical Bayes shrinkage, injected `NaN` features, uncertainty boost) and 6 multi-leg copula vectors to `internal/modelv2/testdata/parity_vectors.json`.
- Updated `ml/train.py` (`_sklearn_tree_to_flat`) and `ml/export.py` (`validate_artifact_bundle_dict`, `allow_nan=False`) so sklearn's `+/-inf` missing-vs-non-missing split thresholds export as finite sentinels (`+/-1e300`), plus a regression test in `ml/tests/test_pipeline_stability.py` (all 19 `pytest` tests passing).
- Added `internal/modelv2/parity_test.go` asserting validation gates on the embedded trained bundle and `1e-6` Go/Python parity across all 112 leg inference vectors (both pre-aligned and raw->shrunk->aligned) and all 6 copula vectors, and relaxed hardcoded synthetic-baseline constants in `internal/modelv2/gbdt_test.go` and `internal/modelv2/copula_test.go` to match real trained empirical bounds.

### Task 12: Docs: update `docs/guides/local-dev.md` with local `mise run train-v2` workflow, cadence, and PR verification steps.

- Updated `docs/guides/local-dev.md` with a `Train the v2 grading model` section covering `uv` prerequisites, `.dockerignore` exclusion from Cloud Run builds, `mise run train-v2` usage and flags, walk-forward validation gates, generated outputs (`internal/modelv2/artifacts/v2.json`, `internal/modelv2/testdata/parity_vectors.json`, `ml/reports/latest.md`), retraining cadence, and PR verification commands.
- Updated `docs/guides/local-dev.md` front matter (`summary`, `tags`, and `budget`) and regenerated `docs/llms.txt` via `good-vibes docs index`.
- Verified `docs/guides/local-dev.md` and `docs/llms.txt` pass `good-vibes docs lint` (`0` errors) and `typos`, with zero new `rumdl` warnings on the added section.

### Phase 5: Extensible context/injury overlays and four-pillar v2 grading engine

Tasks:

- [x] Implement the pluggable `ContextOverlay` layer in `internal/analyzer/overlays.go` retaining all existing non-box-score rules (teammate-out bumps, injury status, nfelo QB drag, wind/fake weather, key numbers, shrinking role, TD spike), adding `narrative_streak_trap`, `model_market_divergence`, and structured `ContextSignal` ingestion, with `HeightenedUncertainty` blending tests.
- [x] Implement `EngineV2` in `internal/analyzer/grade_v2.go` behind the `GradingEngine` registry alongside `EngineV1`, computing the four decomposed `PillarScore` values (`Price & Vig`, `Script Fit`, `Stability & Tail Risk`, `Context & Traps`), calibrated `CalibratedProb` / `CorrMultiplier`, approximate sharp-friend plain-English reasons, and backwards-compatible `GradeFactor` entries, with full unit tests and highlight/explorer benchmarks.

Phase totals: 0 checkpoints, 0s, $—, 0 input / 0 output / 0 cache read / 0 cache write tokens

Checkpoints: None. Review fixes: 0 Integration checks: 0

#### Done

### Task 13: Implement the pluggable `ContextOverlay` layer in `internal/analyzer/overlays.go` retaining all existing non-box-score rules (teammate-out bumps, injury status, nfelo QB drag, wind/fake weather, key numbers, shrinking role, TD spike), adding `narrative_streak_trap`, `model_market_divergence`, and structured `ContextSignal` ingestion, with `HeightenedUncertainty` blending tests.

- Added `ContextSignals []ContextSignal` to `analyzer.Game` and `analyzer.PlayerStats` in `internal/analyzer/analyzer.go` (`json:"context_signals,omitempty"`) and extracted shared `evalTDSpikeRisk` in `internal/analyzer/rules.go` so v1 rules and v2 overlays stay in sync.
- Implemented the pluggable `ContextOverlay` layer in `internal/analyzer/overlays.go` (`ContextSignal`, `OverlayFinding`, `ContextOverlay`, `DefaultContextOverlays`, `EvaluateOverlays`, `HasHeightenedUncertainty`, `UncertaintyBoost`, and `PredictLegWithOverlays`).
- Retained all non-box-score rules as deterministic overlays (`player_injury_status`, `teammate_out_bump`, `qb_drag`, `wind`, `fake_weather`, `hook`, `total_key_number`, `shrinking_role`, `td_spike_risk`) and added `narrative_streak_trap`, `model_market_divergence` (`>= 0.08` gap), and `external_context_signals` (deduplicating matching game and player signals).
- Added unit tests in `internal/analyzer/overlays_test.go` covering every retained and new overlay, custom overlay pluggability, sharp-friend tone checks, and `HeightenedUncertainty` (+0.15 market-blend shift capped at 0.95) across `modelv2.LoadEmbedded()` predictions.

### Task 14: Implement `EngineV2` in `internal/analyzer/grade_v2.go` behind the `GradingEngine` registry alongside `EngineV1`, computing the four decomposed `PillarScore` values (`Price & Vig`, `Script Fit`, `Stability & Tail Risk`, `Context & Traps`), calibrated `CalibratedProb` / `CorrMultiplier`, approximate sharp-friend plain-English reasons, and backwards-compatible `GradeFactor` entries, with full unit tests and highlight/explorer benchmarks.

- Added `PillarScore`, v2 probability/uncertainty fields on `LegGrade` and `SlipGrade`, and the `GradingEngine` interface + registry (`EngineV1`, `EngineV2`, `ActiveEngine = EngineV1`, `EngineByName`, `EngineNames`, `BuiltInEngines`) in `internal/analyzer/grade.go`, keeping `ScoreLeg`, `ScoreSlip`, and `EvaluateSlip` on `EngineV1` until Phase 7.
- Implemented `EngineV2` (`V2Engine`, `V2Config`, `DefaultV2Config`, `NewEngineV2`, `NewEngineV2WithBundle`) in `internal/analyzer/grade_v2.go`, extracting stack-allocated `modelv2` feature slices, scoring props via `PredictLegWithOverlays` and game lines via Power-devigged sharp + `nfelo` / `MarketScript` anchors, and computing multi-leg joint probabilities and `CorrMultiplier` via `CopulaArtifact.EstimateJoint`.
- Decomposed leg and slip scoring into the four `PillarScore` components (`Price & Vig`, `Script Fit`, `Stability & Tail Risk`, `Context & Traps`) with SGP correlation tax detection, hard Danger/vig caps, sharp-friend plain-English reasons free of EV/ROI jargon, and backwards-compatible `GradeFactor` entries.
- Added comprehensive unit tests and `BenchmarkEngineV2ScoreLeg`, `BenchmarkEngineV2FullSlateHighlights` (~5.44 ms/slate on M3 Max across 16 games / 1,632 legs + copula evaluation), and `BenchmarkEngineV2Explorer` (~0.16 ms/op) in `internal/analyzer/grade_v2_test.go`.

### Phase 6: Calibration eval metrics and side-by-side v1/v2 comparison

Tasks:

- [x] Extend `internal/eval` (`metrics.go`, `replay.go`, `cache.go`) and `cmd/eval` to evaluate all registered `GradingEngine` implementations side-by-side and emit `CalibrationMetrics` (Log Loss, Brier, classwise ECE, ≥80% bin spread, market baseline, SGP joint Log Loss vs. naive independence, and reliability bins) in text, Markdown, and JSON, with unit tests on synthetic and SGP replay fixtures.
- [x] Update `internal/web/eval_results.go` and `web/templates/analyzer_results.html` to display v1 vs. v2 engine selection, calibration summary tiles, reliability table, and SGP correlation lift cleanly at phone width, with handler and rendering tests.

Phase totals: 0 checkpoints, 0s, $—, 0 input / 0 output / 0 cache read / 0 cache write tokens

Checkpoints: None. Review fixes: 0 Integration checks: 0

#### Done

### Task 15: Extend `internal/eval` (`metrics.go`, `replay.go`, `cache.go`) and `cmd/eval` to evaluate all registered `GradingEngine` implementations side-by-side and emit `CalibrationMetrics` (Log Loss, Brier, classwise ECE, ≥80% bin spread, market baseline, SGP joint Log Loss vs. naive independence, and reliability bins) in text, Markdown, and JSON, with unit tests on synthetic and SGP replay fixtures.

- Extended `internal/eval/metrics.go` with `ReliabilityBin`, `CalibrationMetrics`, `Report.Calibration`, and engine-level evaluation (`EvaluateEngine`, `EvaluateAllEngines`, `CompareEngines`), computing Log Loss, Brier Score, 10-bin Classwise ECE, ≥80% reliability bin spread, market baseline (from `LegGrade.MarketProb`, pre-kickoff `Sharp`, or implied decimal odds), and SGP copula joint Log Loss vs. naive independence (`Prod(p_i)`).
- Updated `internal/eval/replay.go` (`attachPreKickoffOddsSignals`) to populate `analyzer.Game.MarketScript` via `analyzer.BuildMarketScript` from pre-kickoff archived `odds:lines` in both `ModeSlips` and `ModeMarket`.
- Switched `internal/eval/cache.go` (`GetOrComputeMode`) to call `EvaluateAllEngines(dataset, analyzer.BuiltInEngines())` so cached `SuiteReport` results evaluate `v1` and `v2` side-by-side.
- Extended `cmd/eval/main.go` so `--all`, `--rubric`, `--baseline`, and `--candidate` resolve built-in `GradingEngine` implementations as well as rubrics, and formatted `CalibrationMetrics` and side-by-side engine comparison tables in text, Markdown, and JSON.
- Added unit tests in `internal/eval/metrics_test.go`, `internal/eval/replay_test.go`, `internal/eval/cache_test.go`, and `cmd/eval/main_test.go` covering synthetic 10-bin calibration, SGP vs. naive independence replay, `MarketScript` leak-free replay, cache invalidation, and CLI output formats.

### Task 16: Update `internal/web/eval_results.go` and `web/templates/analyzer_results.html` to display v1 vs. v2 engine selection, calibration summary tiles, reliability table, and SGP correlation lift cleanly at phone width, with handler and rendering tests.

- Updated `internal/web/eval_results.go` (`EvalResultsPage`, `EvalEngineOption`, `EvalEngineComparisonView`, `EvalCalibrationView`, `EvalReliabilityBinView`) to support `?engine=` (with backwards-compatible `?rubric=` fallback), map `report.Calibration` metrics, format 10-bin reliability rows, compute SGP correlation lift (`NaiveIndepLogLoss - SGPLogLoss`), and build side-by-side `v1` vs. `v2` comparison summaries.
- Updated `web/templates/analyzer_results.html` with the `Engine:` switcher (`v1` / `v2`), Probability Calibration summary tiles (Log Loss, Brier Score, Classwise ECE, ≥80% Bin Spread, Market Log Loss, Market Brier), SGP Correlation Lift panel, phone-first Reliability Table (`sm:hidden` compact bin rows + `sm:block` table), and side-by-side Engine Comparison cards/table.
- Added handler and rendering tests in `internal/web/eval_results_test.go` (`TestEvalResultsEngineSelectionAndCalibration`, `TestAnalyzerResultsTemplateRendersCalibrationAndPhoneWidthLayout`) covering `v1` vs. `v2` selection, `?rubric=` fallback, market-mode SGP hiding, calibration tiles, reliability bins, SGP correlation lift, and phone-width markup.

### Phase 7: Phone-first four-pillar UI, living docs, and v2 promotion

Tasks:

- [x] Update `internal/web/analyzer.go`, `web/templates/partials.html`, and `web/templates/analyzer_guide.html` to render the four-pillar breakdown, approximate sharp-friend "Why" bullets, uncertainty badge, and updated v2 guide documentation cleanly at 375px phone width (falling back cleanly to factor lists for v1), with template and handler tests.
- [x] Run `mise run eval-prod` (and CI benchmark eval) to confirm `EngineV2` meets calibration, Log Loss, ECE, and SGP gates against `EngineV1`, then switch `ActiveEngine = EngineV2` in `internal/analyzer` and update any snapshot/golden tests.
- [x] Docs: update `docs/concepts/analyzer.md` and `docs/reference/analyzer-rules.md` to describe the v2 calibrated four-pillar architecture, copula script scoring, and extensible context/injury overlays, and run `good-vibes docs lint`.

Phase totals: 2 checkpoints, 20m28s, $—, 27724599 input / 156391 output / 26356469 cache read / 0 cache write tokens

Checkpoints: sync, final Review fixes: 0 Integration checks: 0

#### Done

### Task 17: Update `internal/web/analyzer.go`, `web/templates/partials.html`, and `web/templates/analyzer_guide.html` to render the four-pillar breakdown, approximate sharp-friend "Why" bullets, uncertainty badge, and updated v2 guide documentation cleanly at 375px phone width (falling back cleanly to factor lists for v1), with template and handler tests.

- Extended `GradeView` in `internal/web/analyzer.go` with `Uncertain`, `CorrSummary`, `Pillars` (`PillarView` with tonal badge/bar classes), and up to 3 plain-English `Why` bullets ranked from the strongest pillar drivers, plus exported `LegGradeView`/`SlipGradeView` helpers and optional `Dependencies.GradingEngine` injection in `internal/web/routes.go`, `internal/web/slips.go`, and `internal/web/pick.go` (including highlight legs and parlays).
- Extracted a shared `grade-breakdown` template in `web/templates/partials.html` used across slip panel legs, slip panel banner, pick tray legs, pick tray banner, and highlight leg/parlay cards, rendering the four-pillar meters, `"High uncertainty"` badge, `CorrSummary`, and `"Why"` bullets cleanly within 375px phone width while falling back to v1 factor lists when `Pillars` is empty.
- Updated `AnalyzerGuidePage` and `web/templates/analyzer_guide.html` to document the v2 four-pillar scoring model, Power de-vigging, Gaussian copula script multiplier, SGP correlation tax, `"High uncertainty"` badge, and v2 leg/slip weights while keeping A–F bands, warning rules, and v1 factor tables intact.
- Regenerated `web/static/app.css` via `go generate ./...` and added template rendering and HTTP handler tests (`TestGradeBreakdownsRenderFourPillarsAndV1FallbackTemplates` and `TestSlipPanelTrayAndGuideWithEngineV2Handler`) in `internal/web/analyzer_test.go`.

### Task 18: Run `mise run eval-prod` (and CI benchmark eval) to confirm `EngineV2` meets calibration, Log Loss, ECE, and SGP gates against `EngineV1`, then switch `ActiveEngine = EngineV2` in `internal/analyzer` and update any snapshot/golden tests.

- Verified the CI benchmark eval (`go run ./cmd/eval --dataset internal/eval/testdata/benchmark_dataset.json --all --format markdown`) on the 2025 NFL Weeks 7–10 replay dataset (236 legs across 70 SGPs), confirming `EngineV2` clears calibration and monotonicity gates against `EngineV1` and the market baseline (`Log Loss 0.5379` vs. `Market 0.6008`, `Brier 0.1838` vs. `0.2059`, `Classwise ECE 0.0319`, `9/10` populated bins, `SGP LL 0.4062` vs. `Naive 0.4063`, `Monotonic: true`).
- Switched `ActiveEngine = EngineV2` in `internal/analyzer/grade.go` and updated `TestGradingEngineRegistryAndV1Compatibility` in `internal/analyzer/grade_v2_test.go`.
- Preserved v1 rubric unit coverage by routing explicit v1 additive/factor tests in `internal/analyzer/analyzer_test.go`, `internal/analyzer/nflverse_grade_test.go`, `internal/analyzer/rubric_test.go`, and `internal/eval/replay_test.go` through `analyzer.EngineV1`.
- Enriched `internal/board/highlights_test.go` and `seedAGradeBoardForHubTest` in `internal/web/slips_test.go` with `EWMA`, `MarketScript`, and `Pinnacle` sharp benchmarks so board highlights run end-to-end under `EngineV2`, and updated `cmd/eval/main_test.go`, `internal/web/analyzer_test.go`, `internal/web/eval_results_test.go`, and `internal/web/pick_test.go` for `ActiveEngine = EngineV2`.
- Added build-tag guards (`internal/analyzer/grade_v2_race_test.go`, `internal/analyzer/grade_v2_norace_test.go`) on `TestEngineV2FullSlateLatencyBudget` in `internal/analyzer/grade_v2_test.go` so normal builds enforce the `<100ms` latency ceiling while `-race` full-suite runs still verify race-freedom without flaking on wall-clock load.

### Task 19: Docs: update `docs/concepts/analyzer.md` and `docs/reference/analyzer-rules.md` to describe the v2 calibrated four-pillar architecture, copula script scoring, and extensible context/injury overlays, and run `good-vibes docs lint`.

- Rewrote `docs/concepts/analyzer.md` to document the promoted `EngineV2` architecture, phone-first four-pillar breakdown and `"High uncertainty"` badge, `internal/modelv2` distributional prop models with Weeks 1–5 Empirical Bayes shrinkage and Power-devigged market blending, deterministic 8,192-sample QMC Gaussian copula script scoring, extensible `ContextOverlay` checks, `EngineV2` vs. `EngineV1` offline evals, and updated overview Mermaid diagram.
- Updated `docs/reference/analyzer-rules.md` with `DefaultV2Config` pillar weights and score caps, `internal/modelv2` distribution families and calibration pipeline, `CopulaArtifact.EstimateJoint` regime matrices and SGP correlation tax rules, all 12 built-in `DefaultContextOverlays` entries (including `narrative_streak_trap`, `model_market_divergence`, and `external_context_signals`), and a clearly scoped `EngineV1` / `RubricV1` reference.
- Updated `docs/reference/routes.md` for `/analyzer/guide` and `?engine=` (with `?rubric=` fallback alias) and probability calibration metrics on `/analyzer/results`, then regenerated `docs/llms.txt` via `good-vibes docs index` and verified `good-vibes docs lint` passes with exit code 0 and zero findings on modified docs.

## Final gate

- Rounds used: 1; verify failures: 0
- Final verify: passed
- Open issues: none

## Ship

- Sync: main 3e3e9e9 → a4fbad2; resolved changes/grading-model-v2/design.md, changes/grading-model-v2/proposal.md, web/static/app.css; folded 18 checkpoint(s)
- Recipe: ask
