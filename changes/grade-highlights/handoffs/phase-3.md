# Phase 3: Phase 3: Per-game preview highlights and living docs

## Done

### Task 5: Surface per-game A-grade counts on `#pick-slate` cards and the `"Top plays in this game"` strip at the top of `#pick-detail` (`internal/web/pick.go` + `web/templates/partials.html`) with handler tests in `internal/web/pick_test.go`.

- Added per-game `AGradeCount` to `PickGameCardView` and rendered the `"N A-grade"` emerald pill on qualifying `#pick-slate` cards in `pickPageData` and `pickSlateData`.
- Added `Highlights` (`HighlightsView`) to `PickDetailView` and rendered the `"Top plays in this game"` strip (up to 3 A-grade legs and 1 same-game A-grade parlay combo) at the top of `#pick-detail` via a shared `highlights-strip` partial in `web/templates/partials.html`.
- Added `from=pick` handling in `addLeg` (`internal/web/slips.go`) so one-tap `+ Add` from `"Top plays in this game"` returns the refreshed `#pick-body` and `tray-updated, slip-updated` triggers, matching `addLegsBatch`.
- Added handler tests in `internal/web/pick_test.go` covering `#pick-slate` counts, `#pick-detail` `"Top plays in this game"` rendering, one-tap `+ Add`, bulk `Add all 2`, slot exhaustion, and empty/zero-count hiding.

### Task 6: Docs: update `docs/concepts/slips.md`, `docs/concepts/analyzer.md`, and `docs/reference/routes.md` to document the A-grade highlights and `POST /slips/{id}/legs/batch`, then run `good-vibes docs lint`.

- Updated `docs/concepts/slips.md` to document the hub's `"Top A-grade ideas"` strip, `#pick-slate` `"N A-grade"` pills, and `#pick-detail` `"Top plays in this game"` strip, plus added the `Overview` Mermaid diagram and front-matter budget/summary updates required by `good-vibes docs lint`.
- Updated `docs/concepts/analyzer.md` to document `LoadBoardSignals`, `board.SelectHighlights`, bounded 2–3 leg combo selection, and how highlights share the `ScoreLeg` / `ScoreSlip` engine using cached SQLite data only.
- Updated `docs/reference/routes.md` to document `POST /slips/{id}/legs/batch`, `from=pick` on `POST /slips/{id}/legs`, and the pick-board A-grade slate pills and detail highlight strip.
- Ran `good-vibes docs lint` and confirmed `0 error(s)` across all modified docs.

## Deviations

### Task 5

- Extracted `{{define "highlights-strip"}}` in `web/templates/partials.html` to share rendering between `#slip-panel` and `#pick-detail` instead of duplicating the markup.
- Placed `"Top plays in this game"` immediately below the sticky header in `#pick-detail` rather than inside the sticky container so mobile viewports can scroll past it without the sticky header covering the screen.
- Added a `from=pick` branch to `addLeg` in `internal/web/slips.go` because `from=board` returns a single `pick-line-group` partial for inline row buttons, whereas `#pick-detail` highlight buttons need `#pick-body` refreshed.

### Task 6

- Added a front-matter `budget` override, `Overview` Mermaid diagram, and tightened `summary` in `docs/concepts/slips.md`, and trimmed a few non-essential code spans in `docs/concepts/analyzer.md`, so both existing concept docs pass `good-vibes docs lint` without errors.

## Open issues

None.

## Next-phase notes

### Task 5

Task 6 should update `docs/` to document the hub and pick-board A-grade highlight strips and per-game A-grade counts, then run `good-vibes docs lint`.

### Task 6

All Phase 3 tasks are complete.

## Files touched

- `docs/concepts/analyzer.md`
- `docs/concepts/slips.md`
- `docs/reference/routes.md`
- `internal/web/pick.go`
- `internal/web/pick_test.go`
- `internal/web/slips.go`
- `web/templates/partials.html`

## Run log

- Mode: 
- Checkpoints: task-5, task-6
- Verify: verify passed: test 800ms; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
