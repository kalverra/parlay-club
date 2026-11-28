# Phase 1: Phase 1: Popularity sorting and collapsible pick-board groups

## Done

### Task 1: Add `SortPopular` as the default pick-board detail sort (with `SortGrade` second) in `internal/board/board.go` using cached `PlayerUsage`, market breadth, and line prominence signals, and update `internal/web/pick.go` and `internal/board/board_test.go` + `internal/web/pick_test.go`.

- Added `SortPopular` (`"popular"`) as the default pick-board detail sort in `internal/board/board.go` and reordered `DetailSorts` to `[SortPopular, SortGrade, SortPlayer, SortOdds, SortLine, SortMatchup]`.
- Implemented deterministic popularity scoring for game lines (`SPREAD` 400 > `TOTAL` 300 > `ML` 200 > `TEAM_TOTAL` 100) and player props (0–60 from cached `PlayerUsage`, up to 25 from game-wide prop market breadth, and up to 15 from primary line / implied probability prominence), with `Grade.Score` and alphabetical tie-breakers.
- Verified `internal/web/pick.go` routes, query parsing, and sort chips delegate cleanly to `board.DetailSorts` and `board.Detail` with `SortPopular` first and `SortGrade` second.
- Updated existing default-sort assertions and added unit/handler tests for popularity ranking in `internal/board/board_test.go` and `internal/web/pick_test.go`.

### Task 2: Convert `pick-detail` sections in `web/templates/partials.html` to collapsed-by-default `<details>` headers (auto-opened when searching) and preserve expanded section keys across `#pick-detail` HTMX swaps in `web/static/app.js`, with template tests in `internal/web/pick_test.go`.

- Converted `pick-detail` sections in `web/templates/partials.html` from `<section>` to native `<details class="group/section space-y-2" data-section="{{.Key}}" data-keep-open="{{.Key}}">` with `<summary>` headers containing the section badge, label, count, and rotating chevron.
- Set sections to render collapsed by default when `$.Search` is empty and auto-open (`open`) when `$.Search` is non-empty, and added `data-game="{{.GameID}}"` on `#pick-detail`.
- Added `snapshotPickDetailOpenState` and `restorePickDetailOpenState` in `web/static/app.js` across `htmx:beforeSwap` and `htmx:afterSwap` to preserve expanded `[data-keep-open]` section keys across `#pick-detail` swaps on the same game while clearing state when switching games.
- Added `TestPickDetailSectionsCollapseByDefaultAndAutoOpenWhenSearching` in `internal/web/pick_test.go` and regenerated `web/static/app.css` via `good-vibes verify generate`.

## Deviations

### Task 1

- `internal/web/pick.go` required no code edits because `pickQuery`, `knownSort`, `pickDetailView`, and `pickPageData` already derive sort validation, chip order, and default sort from `board.DetailSorts` and `board.Detail`.

## Open issues

### Task 1

- None.

## Next-phase notes

### Task 1

- Collapsible `<details>` sections in `web/templates/partials.html` and `data-keep-open` HTMX state preservation in `web/static/app.js` are next in Phase 1.

### Task 2

- Phase 2 can now build compact O/U pick rows and slip O/U badges on top of the collapsible `#pick-detail` sections.

## Files touched

- `internal/board/board.go`
- `internal/board/board_test.go`
- `internal/web/pick_test.go`
- `web/static/app.css`
- `web/static/app.js`
- `web/templates/partials.html`

## Run log

- Mode: 
- Checkpoints: task-1, task-2
- Verify: verify passed: generate reused, changed: web/static/app.css, lint 2.8s, test 8.3s; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
