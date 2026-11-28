# Phase 1: Phase 1: Overall, hit chance, and payout value in the grade dropdown

## Done

### Task 1: Extend `GradeView`, `LegGradeView`, and `SlipGradeView` in `internal/web/analyzer.go` to compute `HitChance` (from `CalibratedProb`) and `PayoutLabel`/`PayoutToneClass` (from the `Price & Vig` pillar, free of `+EV %` copy), render the 3-column **Overall / Hit chance / Payout value** summary strip in `grade-breakdown` (`web/templates/partials.html`), and cover v2 legs, v2 multi-leg slips, and v1 fallback in `internal/web/analyzer_test.go` and `internal/web/slips_test.go`.

- Extended `GradeView`, `LegGradeView`, `SlipGradeView`, and `buildGradeView` in `internal/web/analyzer.go` to populate `HitChance` from `CalibratedProb` via `formatHitChance` and `PayoutLabel`/`PayoutToneClass` from the `Price & Vig` pillar via `payoutValueFromPillars` without `+EV %` copy.
- Added the 3-column `data-grade-summary` strip (**Overall / Hit chance / Payout value**) to `grade-breakdown` in `web/templates/partials.html`.
- Updated `internal/web/analyzer_test.go` and `internal/web/slips_test.go` to cover v2 legs, v2 multi-leg slips, v1 fallback, and absence of `+EV` copy.

### Task 2: Docs: update `docs/concepts/analyzer.md` to document the Overall, Hit chance, and Payout value summary strip inside the grade breakdown popover.

- Updated `docs/concepts/analyzer.md` under "What the banner shows" to document the 3-column **Overall / Hit chance / Payout value** summary strip inside the `grade-breakdown` popover, including `CalibratedProb` formatting (`~N%`, `<1%`, `>99%`), `Price & Vig` payout labels (`Strong value`, `Fair value`, `Slight juice`, `Steep juice`) without `+EV %` copy, and `EngineV1` fallback behavior.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 1

- `GradeView` now carries `HitChance`, `PayoutLabel`, and `PayoutToneClass` automatically for any `LegGradeView` call, so pre-pick grade pills added in Phase 2/3 will inherit the 3-column summary strip in `grade-breakdown`.

### Task 2

- Phase 2 and Phase 3 will add grouping modes and pre-pick grade pills on the pick board; `docs/concepts/analyzer.md` and `docs/concepts/slips.md` may need follow-up updates when those surfaces land.

## Files touched

- `docs/concepts/analyzer.md`
- `internal/web/analyzer.go`
- `internal/web/analyzer_test.go`
- `internal/web/slips_test.go`
- `web/templates/partials.html`

## Run log

- Mode: 
- Checkpoints: task-1, task-2
- Verify: verify passed: test 500ms; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
