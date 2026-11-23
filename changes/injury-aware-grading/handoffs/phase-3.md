# Phase 3: Phase 3: Add deterministic teammate-injury adjustments to `ScoreLeg`

## Done

### Task 7: Update `scoreFormAndUsage` in `internal/analyzer/grade.go` to soften `L5 median trap` from `-15` to `-5` and award `teammate_out_bump` (`+5`) when a same-team, same-position (`WR`/`TE`/`RB`) teammate is `Out` on an Over/Anytime TD prop, and update `GradeRubric()` in `internal/analyzer/docs.go`, with unit tests in `internal/analyzer/analyzer_test.go` and `internal/analyzer/nflverse_grade_test.go`.

- Updated `scoreFormAndUsage` in `internal/analyzer/grade.go` to accept `game Game`, soften `L5 median trap` from `-15` to `-5` when an eligible same-team, same-position (`WR`/`TE`/`RB`) teammate is `Out`, and award `teammate_out_bump` (`+5`) on qualifying Over and Anytime TD Yes props.
- Added `outSamePositionTeammate`, `isSkillVolumePosition`, and `isEligibleTeammateOutLeg` helpers in `internal/analyzer/grade.go`, ignoring `Questionable`, `Doubtful`, `Injured Reserve`, non-skill positions, self-injuries, and Under props.
- Updated `GradeRubric()` in `internal/analyzer/docs.go` to document the softened `L5 median trap` (`-15 (-5 w/ teammate Out)`) and `teammate_out_bump` (`+5`).
- Added unit tests in `internal/analyzer/analyzer_test.go` (`TestScoreLegTeammateOut`) and `internal/analyzer/nflverse_grade_test.go` (`TestScoreLeg` and `TestRuleDocsCoverageAndGradeRubric`).

### Task 8: Docs: update `docs/concepts/analyzer.md` and `docs/reference/analyzer-rules.md` to document the roster+summary injury merge and the teammate-out `ScoreLeg` adjustments.

- Updated `docs/concepts/analyzer.md` to document the `LoadSlipSignals` roster+summary injury merge (`MergeGameInjuries` / `espn.MergeTeamInjuries`), `PlayerStats.Position`, `teammate_out_bump` (`+5`), and the softened `median_trap` (`-15`, or `-5` with a same-position `Out` teammate).
- Trimmed excess inline code spans in `docs/concepts/analyzer.md` so `good-vibes docs lint` passes with 0 errors on changed docs.
- Updated `docs/reference/analyzer-rules.md` to document the roster+summary injury merge under `How rules see a slip`, update `median_trap` (`-15` / `-5` w/ teammate Out), and add `teammate_out_bump` (`+5`) to the `ScoreLeg` rubric table.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 7

`docs/reference/analyzer-rules.md` and `docs/concepts/analyzer.md` still list `-15` for `median_trap` and do not yet document `teammate_out_bump`; update them in the docs task.

### Task 8

All Phase 3 tasks are complete and `good-vibes docs lint` reports 0 errors.

## Files touched

- `docs/concepts/analyzer.md`
- `docs/reference/analyzer-rules.md`
- `internal/analyzer/analyzer_test.go`
- `internal/analyzer/docs.go`
- `internal/analyzer/grade.go`
- `internal/analyzer/nflverse_grade_test.go`

## Run log

- Mode: 
- Checkpoints: task-7, task-8
- Verify: verify passed: test 1s; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
