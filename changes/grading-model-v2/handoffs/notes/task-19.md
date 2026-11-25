# Task 19

## Done

- Rewrote `docs/concepts/analyzer.md` to document the promoted `EngineV2` architecture, phone-first four-pillar breakdown and `"High uncertainty"` badge, `internal/modelv2` distributional prop models with Weeks 1–5 Empirical Bayes shrinkage and Power-devigged market blending, deterministic 8,192-sample QMC Gaussian copula script scoring, extensible `ContextOverlay` checks, `EngineV2` vs. `EngineV1` offline evals, and updated overview Mermaid diagram.
- Updated `docs/reference/analyzer-rules.md` with `DefaultV2Config` pillar weights and score caps, `internal/modelv2` distribution families and calibration pipeline, `CopulaArtifact.EstimateJoint` regime matrices and SGP correlation tax rules, all 12 built-in `DefaultContextOverlays` entries (including `narrative_streak_trap`, `model_market_divergence`, and `external_context_signals`), and a clearly scoped `EngineV1` / `RubricV1` reference.
- Updated `docs/reference/routes.md` for `/analyzer/guide` and `?engine=` (with `?rubric=` fallback alias) and probability calibration metrics on `/analyzer/results`, then regenerated `docs/llms.txt` via `good-vibes docs index` and verified `good-vibes docs lint` passes with exit code 0 and zero findings on modified docs.

## Deviations

- Also updated `docs/reference/routes.md` and regenerated `docs/llms.txt` alongside the two docs named in the task, following the Phase 6/7 handoff notes and `gv-docs` map sync rule so `?engine=` on `GET /analyzer/results` and updated doc summaries stay aligned.

## Open issues

None.

## Next-phase notes

- Phase 7 is complete and `good-vibes docs lint` passes with exit code 0 across all living docs.

## Commit

Update analyzer living docs for v2 grading model

Living docs still described the v1 additive rubric as the active engine. Updating the analyzer concept, rules reference, and routes reference keeps documentation aligned with EngineV2, the four-pillar UI, Gaussian copula script scoring, and extensible overlays.
