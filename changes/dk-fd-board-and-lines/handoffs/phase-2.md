# Phase 2: Phase 2: Split-line phone UI and DK/FD-first profile and export UX

## Done

### Task 3: Render explicit DK vs. FD split lines (ordered with the viewer's `homeBook` first) in `web/templates/` and view builders (`internal/web/pick.go`, `internal/web/research.go`, `internal/web/slips.go`) whenever `LinesDiffer` is true, with handler tests in `internal/web/pick_test.go` and `internal/web/research_test.go`.

- Added `splitLineBadge` and `formatBookQuote` in `internal/web/pick.go` to render `DK` vs. `FD` split lines with the viewer's `homeBook` first whenever `LinesDiffer` is true.
- Wired `SplitLine` (and `SpreadSplit` / `TotalSplit` on slate cards) through `internal/web/pick.go` (`PickGameCardView`, `PickRowView`, `TrayLegView`), `internal/web/research.go` (`ResearchCardView`, `BetResearchView`), and `internal/web/slips.go` (`LegView`, `LegHighlightView`).
- Updated `web/templates/partials.html` (slate cards, `pick-option` buttons, `pick-tray`, `slip-panel` legs, `highlights-strip`, and `research-explore` cards) and `web/templates/research_bet.html` (bet summary and related cards) to render split badges cleanly at phone width, and regenerated `web/static/app.css` via `go generate ./...`.
- Added handler tests `TestPickBoardRendersSplitLinesWithHomeBookFirst` in `internal/web/pick_test.go` and `TestResearchExploreAndBetRenderSplitLinesWithHomeBookFirst` in `internal/web/research_test.go`, and verified `good-vibes verify generate`, `good-vibes verify lint`, and `go test -short ./internal/web/...` pass.

### Task 4: Replace the free-text `home_book` input in `web/templates/profile.html` with a DraftKings / FanDuel / No preference selector, validate `home_book` in `internal/web/slips.go`, and order the export card's books in `internal/web/export.go` with the viewer's `homeBook` first, with tests in `internal/web/slips_test.go` and `internal/web/export_test.go`.

- Replaced the free-text `home_book` input in `web/templates/profile.html` with a segmented DraftKings / FanDuel / No preference radio selector and updated `web/static/app.css` via `go generate ./...`.
- Added `validateHomeBook` and `normalizeSelectableHomeBook` in `internal/web/slips.go` so `POST /me` accepts `""`, `"draftkings"`, or `"fanduel"` (rejecting other values with `422`) and `GET /me` displays legacy unknown keys as `"No preference"`.
- Added `exportBooksForHomeBook` in `internal/web/export.go` and updated `exportView` to order the export card's sportsbooks with the viewer's `homeBook` first.
- Updated `TestProfilePageRendersTheHomeBookForm` and added `TestUpdateProfileValidatesHomeBookAndNormalizesLegacyKeys` in `internal/web/slips_test.go`, plus `TestExportCardOrdersBooksWithHomeBookFirst` in `internal/web/export_test.go`.
- Verified `good-vibes verify generate`, `good-vibes verify lint`, and `go test -short ./internal/web/...` pass.

### Task 5: Docs: update `docs/concepts/pick-board.md` (or board/research concept docs) and `docs/concepts/export.md` to document the DK ∩ FD pickable filter, split-line display, and DK/FD home-book UX.

- Updated `docs/concepts/slips.md` (which owns the pick board and Research concept sections) to document `board.BuildDualBook` (DK ∩ FD pickable filtering), explicit `DK`/`FD` split-line badges ordered by `homeBook` instead of median averaging, and `board.LineKey` grouping for totals when DK and FD differ by half a point.
- Updated `docs/concepts/export.md` to document export card sportsbook ordering with the viewer's `homeBook` first and the DraftKings / FanDuel / No preference Profile selector.
- Updated `docs/reference/routes.md` for `POST /me` validation (`draftkings`, `fanduel`, or blank for No preference, returning 422 on other values).
- Verified `good-vibes docs lint` passes with 0 errors and ran `good-vibes docs index`.

## Deviations

### Task 5

- `docs/concepts/pick-board.md` does not exist in this repository; updated `docs/concepts/slips.md` (which documents the pick board and Research) and `docs/reference/routes.md` (`POST /me`) alongside `docs/concepts/export.md`.

## Open issues

None.

## Next-phase notes

### Task 3

- Remaining Phase 2 tasks can build on `splitLineBadge` and `homeBook` ordering for the profile selector and export card.

### Task 4

- Remaining work can rely on `home_book` being `""`, `"draftkings"`, or `"fanduel"` for any newly saved profile, with legacy unknown keys normalized to `""` on display.

### Task 5

- All Phase 2 tasks are complete; living docs now match the dual-book board, split-line UI, Profile selector, and export ordering.

## Files touched

- `docs/concepts/export.md`
- `docs/concepts/slips.md`
- `docs/reference/routes.md`
- `internal/web/export.go`
- `internal/web/export_test.go`
- `internal/web/pick.go`
- `internal/web/pick_test.go`
- `internal/web/research.go`
- `internal/web/research_test.go`
- `internal/web/slips.go`
- `internal/web/slips_test.go`
- `web/static/app.css`
- `web/templates/partials.html`
- `web/templates/profile.html`
- `web/templates/research_bet.html`

## Run log

- Mode: 
- Checkpoints: task-3, task-4, task-5
- Verify: verify passed: test 400ms; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
