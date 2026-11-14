# Task 1: ESPN BoxScore parser

## Done

- Added `BoxScore`, `PlayerBoxStats`, `PassingStats`, `RushingStats`, `ReceivingStats`, `KickingStats`, and `(*Client).BoxScore(ctx, eventID)` in `internal/ingest/espn/boxscore.go`, parsing `boxscore.players[]` by `keys[]` name and merging categories per player.
- Updated `parseMadeAttempts` in `internal/ingest/espn/gamelog.go` to split on both `/` (`2/2`) and `-` (`2-2`), and added `FieldGoalsMade*3 + ExtraPointsMade` fallback when `totalKickingPoints` is omitted.
- Added fixture and unit tests in `internal/ingest/espn/boxscore_test.go` covering `summary_week9_featured.json`, `summary_empty.json`, `summary.json`, reordered keys, kicking fallback, `OtherTDs`, and HTTP errors.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `BoxScore.HasStats` is `true` when `len(boxscore.players) > 0` and `false` for empty or pregame summaries; `grader` should return `NEEDS_GRADING` when `HasStats == false` and `VOID` (DNP) when `HasStats == true` and the player is absent from `BoxScore.Players`.
- Players who appear in any stat group (including `defensive`, `fumbles`, or `punting`) are present in `BoxScore.Players` with `0` for all unlogged categories.
