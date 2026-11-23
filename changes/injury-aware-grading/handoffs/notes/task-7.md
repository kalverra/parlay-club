## Done

- Updated `scoreFormAndUsage` in `internal/analyzer/grade.go` to accept `game Game`, soften `L5 median trap` from `-15` to `-5` when an eligible same-team, same-position (`WR`/`TE`/`RB`) teammate is `Out`, and award `teammate_out_bump` (`+5`) on qualifying Over and Anytime TD Yes props.
- Added `outSamePositionTeammate`, `isSkillVolumePosition`, and `isEligibleTeammateOutLeg` helpers in `internal/analyzer/grade.go`, ignoring `Questionable`, `Doubtful`, `Injured Reserve`, non-skill positions, self-injuries, and Under props.
- Updated `GradeRubric()` in `internal/analyzer/docs.go` to document the softened `L5 median trap` (`-15 (-5 w/ teammate Out)`) and `teammate_out_bump` (`+5`).
- Added unit tests in `internal/analyzer/analyzer_test.go` (`TestScoreLegTeammateOut`) and `internal/analyzer/nflverse_grade_test.go` (`TestScoreLeg` and `TestRuleDocsCoverageAndGradeRubric`).

## Deviations

None.

## Open issues

None.

## Next-phase notes

`docs/reference/analyzer-rules.md` and `docs/concepts/analyzer.md` still list `-15` for `median_trap` and do not yet document `teammate_out_bump`; update them in the docs task.
