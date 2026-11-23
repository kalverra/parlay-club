# Task 3

## Done

- Exported `DistinctLegPlayerIDs`, `NFLverseSeasonTypes`, and `MergeInjuryFeeds` in `internal/analyzer/signals.go`, sharing `mergeTeamInjuryFeeds` with `MergeGameInjuries`.
- Updated `internal/eval/replay.go` to call `analyzer.DistinctLegPlayerIDs`, `analyzer.NFLverseSeasonTypes`, `analyzer.MergeInjuryFeeds`, `db.Archive*Key`, `store.LatestAPIResponseBefore`, and `store.SlipsWithLegsForWeek`, removing the duplicate local helpers.
- Added unit tests for `DistinctLegPlayerIDs`, `NFLverseSeasonTypes`, and `MergeInjuryFeeds` in `internal/analyzer/signals_test.go`.

## Deviations

- `NFLverseSeasonTypes` returns `[]int` rather than `[]string` as `design.md` originally wrote; updated `design.md` to match the existing `nflverseSeasonTypes` signature and `db.Store` methods.
- `latestArchivedBodyBefore` adjusts `kickoff` via `beforeKickoff(kickoff)` when calling `LatestAPIResponseBefore` (which uses `fetched_at <= before` at RFC3339 1-second resolution) and still checks `resp.FetchedAt.Before(kickoff)` to preserve the strict `fetched_at < kickoff` leakage guard.

## Open issues

None.

## Next-phase notes

Task 4 can now scope `eval.LoadDataset` to referenced games in `rawLegs` and replace `hasArchivedOddsBefore`'s full JSON decompression/parsing with a metadata-only `LatestAPIResponseBefore` check.

## Commit

Share analyzer signal helpers and archive reads with eval

Centralizes player ID extraction, postseason season-type expansion, and injury feed merging in `internal/analyzer`, and switches `internal/eval/replay.go` to those helpers alongside canonical `db.Archive*Key` builders, `LatestAPIResponseBefore`, and batched `SlipsWithLegsForWeek` queries. This removes duplicate logic and avoids N+1 slip/leg queries and full `ListAPIResponses` scans during dataset replay.
