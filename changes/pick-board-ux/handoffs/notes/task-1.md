## Done

- Extended `GradeView`, `LegGradeView`, `SlipGradeView`, and `buildGradeView` in `internal/web/analyzer.go` to populate `HitChance` from `CalibratedProb` via `formatHitChance` and `PayoutLabel`/`PayoutToneClass` from the `Price & Vig` pillar via `payoutValueFromPillars` without `+EV %` copy.
- Added the 3-column `data-grade-summary` strip (**Overall / Hit chance / Payout value**) to `grade-breakdown` in `web/templates/partials.html`.
- Updated `internal/web/analyzer_test.go` and `internal/web/slips_test.go` to cover v2 legs, v2 multi-leg slips, v1 fallback, and absence of `+EV` copy.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `GradeView` now carries `HitChance`, `PayoutLabel`, and `PayoutToneClass` automatically for any `LegGradeView` call, so pre-pick grade pills added in Phase 2/3 will inherit the 3-column summary strip in `grade-breakdown`.
