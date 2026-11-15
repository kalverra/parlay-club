# Task 4 Notes

## Done

- Extended `analyzer.Game` with `HomeEPA`/`AwayEPA` (`*TeamDefenseProfile`, plus `TeamEPA`/`OpponentEPA` helpers) and `analyzer.PlayerStats` with `Usage` (`*PlayerUsage` and `PlayerWeekUsage`).
- Implemented the four new deterministic `nflverse` analyzer rules in `internal/analyzer/rules.go`: `tough_matchup`, `shrinking_role`, `pace_clash`, and `td_spike_risk`.
- Added deterministic `ScoreLeg` and `ScoreSlip` in `internal/analyzer/grade.go` returning `A`–`F` grades, `GradeFactor` breakdowns, category caps (`Form & usage`, `Matchup & environment`, `Line & price`), slip correlation/drag adjustments, and a sharp-friend slip summary.
- Added `RuleDocs()`, `GradeRubric()`, and `RuleIDs()` in `internal/analyzer/docs.go` and `internal/analyzer/analyzer.go` to keep the upcoming `/analyzer/guide` page locked to live engine thresholds.
- Added boundary, grade-band, fail-soft, determinism, and 1:1 rule-coverage tests in `internal/analyzer/nflverse_grade_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `TeamDefenseProfile` expects league ranks `1..32` (`1` = toughest defense / highest offense / fastest pace / highest pass rate; `0` = unset) and `PlayerWeekUsage` expects `0..1` fractions for `OffensePct`, `TargetShare`, and `AirYardsShare`.
- `RuleIDs()` and `RuleDocs()` are 1:1 tested; any future rule added to `rules` in `internal/analyzer/analyzer.go` must also be added to `RuleDocs()` in `internal/analyzer/docs.go`.
