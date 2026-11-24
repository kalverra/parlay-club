# Phase 2: Phase 2: Main screen highlights and batch-add endpoint

## Done

### Task 3: Add `POST /slips/{id}/legs/batch` in `internal/web/slips.go` and `internal/web/routes.go` for atomic combo insertion with capacity/duplicate checks and `slip-updated` broadcast, plus handler tests in `internal/web/slips_test.go`.

- Added `Service.AddLegs` in `internal/slips/rules.go` to validate `OPEN` slip status, unlocked member state, duplicate selections (against existing member legs and within the batch), remaining `LegsPerMember` capacity, and kickoff/slate scope inside one SQLite transaction, publishing `slip:{id}` (`slip-updated`) when legs are inserted.
- Registered `POST /slips/{id}/legs/batch` in `slipHandlers.Routes` and implemented `addLegsBatch` plus `legsFromBatchForm` (sharing `legFromValues` with `legFromForm`) in `internal/web/slips.go`.
- Returned `#slip-panel` by default, `#pick-body` for `from=board`/`from=pick`, and `#pick-dock` for `from=dock`, with corresponding `Hx-Trigger` headers including `slip-updated`.
- Added handler tests in `internal/web/slips_test.go` covering atomic insertion, SSE `slip-updated` broadcast, duplicate skipping, over-capacity rejection, kickoff rollback, locked member refusal, and `from=board` partial rendering.

### Task 4: Wire `LoadBoardSignals` and `board.SelectHighlights` into the hub render path (`internal/web/slips.go` + `web/templates/partials.html`) to render the phone-first `"Top A-grade ideas"` strip during the viewer's `pick` step, with handler tests in `internal/web/slips_test.go`.

- Added `HighlightsView`, `LegHighlightView`, `ParlayHighlightView`, `slipHighlights`, `highlightsView`, and `legHighlightView` in `internal/web/slips.go`, wiring `analyzer.LoadBoardSignals` and `board.SelectHighlights` into `slipPageData` when `slip.Status == db.SlipOpen` and `next.Step == slips.StepPick`.
- Rendered the phone-first `"Top A-grade ideas"` strip inside `#slip-panel` in `web/templates/partials.html` (below the step banner), showing up to 3 A-grade legs and up to 2 A-grade parlay combos with green `A` pills, inline `<details>` factor breakdowns, one-tap `+ Add` forms, and `"Add all N"` batch forms (or disabled `"Needs N open slots"` buttons when slots are insufficient), hidden when empty.
- Added handler tests `TestSlipHubRendersTopAGradeIdeasDuringPickStep` and `TestSlipHubHighlightsHideAndDisableWhenSlotsFullOrLocked` in `internal/web/slips_test.go`.

## Deviations

### Task 3

- All slip routes are registered in `slipHandlers.Routes` inside `internal/web/slips.go` rather than `internal/web/routes.go`, which only calls `slipRoutes.Routes(mux)`.
- Transactional validation and insertion (`Service.AddLegs`) were placed in `internal/slips/rules.go` to match the repository pattern where `internal/web` handlers stay thin and `internal/slips` owns rules and `s.notify`.

## Open issues

None.

## Next-phase notes

### Task 3

Combo highlight forms should submit indexed fields `legs[0][game_id]`, `legs[0][market_type]`, `legs[0][team]`, `legs[0][player_name]`, `legs[0][player_id]`, `legs[0][side]`, `legs[0][line]`, `legs[0][odds]`, etc. (and `from=board` when triggered from `#pick-detail`).

### Task 4

`HighlightsView`, `LegHighlightView`, `ParlayHighlightView`, `highlightsView`, and `legHighlightView` in `internal/web/slips.go` can be reused directly for `#pick-detail`'s `"Top plays in this game"` strip in Phase 3.

## Files touched

- `internal/slips/rules.go`
- `internal/web/slips.go`
- `internal/web/slips_test.go`
- `web/static/app.css`
- `web/templates/partials.html`

## Run log

- Mode: 
- Checkpoints: task-3, task-4
- Verify: verify passed: generate changed: web/static/app.css, lint 3.9s, test 8.8s; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
