## Done

- Added `Store.FinalSeasons` in `internal/db/record.go` to query seasons with `FINAL` games, newest first.
- Added `?source=slips|market` parsing (returning `400 Bad Request` on invalid values), `EvalCache.GetOrComputeMode` lookup, `FINAL`-game default season resolution, `Source`/`IsMarket`/`SlipsURL`/`Sources` view state, and `WithSharp`/`WithL5Fallback` coverage fields to `internal/web/eval_results.go`, preserving `?source=` on all season and rubric filter URLs.
- Updated `web/templates/analyzer_results.html` with phone-first `User slips` / `Market-wide` source switcher pills, market-mode empty-state copy, hidden `Slip Calibration (A-F)` and `Slip Factor Attribution` sections (with a link note to `User slips`), and `nflverse L5 fallback` + `Sharp benchmarks` tiles in `Pre-Kickoff Data Coverage`, then ran `go generate ./...`.
- Added HTTP tests in `internal/web/eval_results_test.go` covering invalid `?source=` handling, empty market state, `FINAL`-game default season resolution, hidden slip-only sections, `?source=` preservation on filter URLs, and `InvalidateMarket` cache eviction.

## Deviations

None.

## Open issues

None.

## Next-phase notes

`GET /analyzer/results?source=market` is wired end-to-end and verified with `go generate ./...` and `internal/web` HTTP tests.

## Commit

Add market-wide source switcher to /analyzer/results

Users can now toggle between settled user slips and deduplicated market-wide pre-kickoff legs in the Grading Results UI without leaving the page. Market mode hides slip-only sections, defaults to seasons with final games, and surfaces sharp-benchmark and L5-fallback coverage counts.
