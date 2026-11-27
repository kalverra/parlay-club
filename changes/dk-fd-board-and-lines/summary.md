---
slug: dk-fd-board-and-lines
run: 20261127-223846-dk-fd-board-and-lines
outcome: done
generated: 2026-11-27T23:03:56Z
---

# Run summary: dk-fd-board-and-lines

## Outcome

- Status: done
- Branch: dk-fd-board-and-lines
- Close-out: written
- Close: pending
- Final verdict: agy@gemini-4-neon-medium approved (issues: 0)

## Totals

- Tasks: 5 of 5
- Phases: 2
- Checkpoints: 6
- Duration: 22m36s
- Cost: $—
- Fresh input tokens: 1736159
- Cache read tokens: 30249553
- Cache write tokens: 0
- Output tokens: 293064

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-medium | 25232988 | $— |
| gemini-4-neon-medium | 7045788 | $— |

## Phases

### Phase 1: Dual-book intersection and split-line model in `internal/board`

Tasks:

- [x] Add `BookQuote`, `Row.DK`, `Row.FD`, `Row.LinesDiffer`, and dual-book filtering (`draftkings` ∩ `fanduel`) in `internal/board/board.go` (never averaging differing lines into a median), keep an explicit all-book mode for `internal/eval/market.go`, and update `lineKey` pairing so split `TOTAL`/`TEAM_TOTAL` rows stay paired; add unit tests in `internal/board/board_test.go`.
- [x] Switch user-facing board callers in `internal/web/pick.go`, `internal/web/games.go`, `internal/web/research.go`, and `internal/web/slips.go` to the dual-book board builder and verify `internal/eval` and `internal/analyzer` tests stay green.

Phase totals: 2 checkpoints, 7m49s, $—, 2214382 input / 39764 output / 2002868 cache read / 0 cache write tokens

Checkpoints: task-1, task-2 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Add `BookQuote`, `Row.DK`, `Row.FD`, `Row.LinesDiffer`, and dual-book filtering (`draftkings` ∩ `fanduel`) in `internal/board/board.go` (never averaging differing lines into a median), keep an explicit all-book mode for `internal/eval/market.go`, and update `lineKey` pairing so split `TOTAL`/`TEAM_TOTAL` rows stay paired; add unit tests in `internal/board/board_test.go`.

- Added `BookQuote`, `BuildOptions`, `Row.DK`, `Row.FD`, and `Row.LinesDiffer` to `internal/board/board.go`.
- Added `BuildDualBook` and `BuildWithOptions` implementing `draftkings` ∩ `fanduel` filtering, selecting `homeBook` (or `fanduel` when unset) without averaging differing lines into a median.
- Updated `internal/eval/market.go` to call `board.BuildWithOptions` with `BuildOptions{DualBook: false}` so market-wide consensus evaluation remains explicitly all-book.
- Updated `board.LineKey` (and `lineKey` via it) so `TOTAL` keys on market alone and `TEAM_TOTAL` keys on `TEAM_TOTAL|<team>`, keeping split Over/Under rows paired in one `LineGroup`.
- Updated `TestLineKeyCanonicalPlayerPropsAndGameLines` and added dual-book intersection, split-line non-averaging, and split `TOTAL`/`TEAM_TOTAL` pairing unit tests in `internal/board/board_test.go`.

### Task 2: Switch user-facing board callers in `internal/web/pick.go`, `internal/web/games.go`, `internal/web/research.go`, and `internal/web/slips.go` to the dual-book board builder and verify `internal/eval` and `internal/analyzer` tests stay green.

- Switched all user-facing board callers in `internal/web/pick.go` (5 sites), `internal/web/research.go` (2 sites), `internal/web/games.go` (1 site), and `internal/web/slips.go` (1 site) from `board.Build` to `board.BuildDualBook`.
- Updated single-book test fixtures and dual-book line/price expectations in `internal/web/games_test.go`, `internal/web/pick_test.go`, `internal/web/research_test.go`, and `internal/web/slips_test.go` so pickable rows include both `draftkings` and `fanduel` quotes and reflect non-averaged dual-book lines.
- Verified `go test ./internal/eval/... ./internal/analyzer/... ./internal/board/...`, `go test -short ./...`, and `golangci-lint run ./...` all pass with 0 issues.

### Phase 2: Split-line phone UI and DK/FD-first profile and export UX

Tasks:

- [x] Render explicit DK vs. FD split lines (ordered with the viewer's `homeBook` first) in `web/templates/` and view builders (`internal/web/pick.go`, `internal/web/research.go`, `internal/web/slips.go`) whenever `LinesDiffer` is true, with handler tests in `internal/web/pick_test.go` and `internal/web/research_test.go`.
- [x] Replace the free-text `home_book` input in `web/templates/profile.html` with a DraftKings / FanDuel / No preference selector, validate `home_book` in `internal/web/slips.go`, and order the export card's books in `internal/web/export.go` with the viewer's `homeBook` first, with tests in `internal/web/slips_test.go` and `internal/web/export_test.go`.
- [x] Docs: update `docs/concepts/pick-board.md` (or board/research concept docs) and `docs/concepts/export.md` to document the DK ∩ FD pickable filter, split-line display, and DK/FD home-book UX.

Phase totals: 4 checkpoints, 14m6s, $—, 27670368 input / 243801 output / 26340447 cache read / 0 cache write tokens

Checkpoints: task-3, task-4, task-5, final Review fixes: 0 Integration checks: 0

#### Done

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

## Final gate

- Rounds used: 1; verify failures: 0
- Final verify: passed
- Open issues: none

## Ship

- Sync: none
- Recipe: ask
