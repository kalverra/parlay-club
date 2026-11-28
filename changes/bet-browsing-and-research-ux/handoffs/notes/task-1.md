# Task 1

## Done

- Added `SortPopular` (`"popular"`) as the default pick-board detail sort in `internal/board/board.go` and reordered `DetailSorts` to `[SortPopular, SortGrade, SortPlayer, SortOdds, SortLine, SortMatchup]`.
- Implemented deterministic popularity scoring for game lines (`SPREAD` 400 > `TOTAL` 300 > `ML` 200 > `TEAM_TOTAL` 100) and player props (0–60 from cached `PlayerUsage`, up to 25 from game-wide prop market breadth, and up to 15 from primary line / implied probability prominence), with `Grade.Score` and alphabetical tie-breakers.
- Verified `internal/web/pick.go` routes, query parsing, and sort chips delegate cleanly to `board.DetailSorts` and `board.Detail` with `SortPopular` first and `SortGrade` second.
- Updated existing default-sort assertions and added unit/handler tests for popularity ranking in `internal/board/board_test.go` and `internal/web/pick_test.go`.

## Deviations

- `internal/web/pick.go` required no code edits because `pickQuery`, `knownSort`, `pickDetailView`, and `pickPageData` already derive sort validation, chip order, and default sort from `board.DetailSorts` and `board.Detail`.

## Open issues

- None.

## Next-phase notes

- Collapsible `<details>` sections in `web/templates/partials.html` and `data-keep-open` HTMX state preservation in `web/static/app.js` are next in Phase 1.

## Commit

Default pick-board detail sort to Popular

Make the pick board surface the most relevant game lines and featured players first instead of sorting alphabetically by player name. Using cached usage, market breadth, and line prominence keeps the ordering deterministic with zero extra API or database calls.
