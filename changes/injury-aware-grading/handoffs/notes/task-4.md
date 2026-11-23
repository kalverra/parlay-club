# Task 4

## Done

- Added `espn.IsKeyInjuryPosition` in `internal/ingest/espn/summary.go` and reused it in `ParseInjuries` to normalize and check key injury positions (`QB`, `RB`, `WR`, `TE`, `LT`, `OT`, `C`, `CB`, `S`).
- Added `espn.MergeTeamInjuries` in `internal/ingest/espn/summary.go` to keep summary injuries first and append key-position fallback injuries not already present by `AthleteID` or case-insensitive `FullName`.
- Added `TestIsKeyInjuryPosition` and `TestMergeTeamInjuries` in `internal/ingest/espn/summary_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

`espn.IsKeyInjuryPosition` and `espn.MergeTeamInjuries` are ready for `analyzer.MergeGameInjuries` in `internal/analyzer/signals.go`.

## Commit

Add ESPN injury position check and team merge helper

ESPN game summaries cap injuries at five players per team, so fallback roster injuries need to be merged without duplicating summary entries or introducing non-key positions.
