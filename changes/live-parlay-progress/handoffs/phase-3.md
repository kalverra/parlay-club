# Phase 3: Phase 3: Render live progress bars and hit chances in the UI

## Done

### Task 8: Wire `SlipLiveView` and `LegLiveView` in `internal/web/slips.go` and render the parlay live progress card and per-leg progress bars with live hit chances in `web/templates/partials.html`, with handler and HTML tests covering live, cashed, busted, scheduled, and SSE-refreshed states.

- Added `SlipLiveView` and `LegLiveView` to `internal/web/slips.go`, decoding `BoxScoreJSON` once per `LIVE`/`FINAL` game and evaluating legs and slips via `progress.EvaluateLeg` and `progress.EvaluateSlip`.
- Rendered the parlay live progress card (`data-slip-live-progress`, `role="progressbar"`, live vs. pre-game hit chance, and status summary) and per-leg progress strips (`data-leg-live-progress`, stat/clock readout, and live hit chance) in `web/templates/partials.html`, and regenerated `web/static/app.css`.
- Added `TestSlipHubLiveProgressStates` and `TestSlipPanelSSERefreshesLiveProgress` in `internal/web/slips_test.go` covering scheduled, live, cashed, busted, and SSE-refreshed `#slip-panel` states.

### Task 9: Wire compact live parlay progress bars and live hit chances into `SpotlightSlipCard` in `internal/web/home.go` and `web/templates/week.html`, with overview handler tests and phone-width layout checks.

- Added `Live SlipLiveView` to `SpotlightSlipCard` in `internal/web/home.go` and populated it in `buildGroupOverviewPage` via `evaluateSlipProgress` and `slipLiveView`.
- Rendered a compact single-row live progress bar and live hit percentage (`data-spotlight-live-progress`, `role="progressbar"`, status badge, truncated summary, and `Live <X>%`) inside spotlight cards in `web/templates/week.html`.
- Added unit assertions in `internal/web/home_overview_test.go` and overview handler/HTML/phone-width layout tests (`TestGroupWeekSpotlightLiveProgress`) in `internal/web/home_test.go` covering scheduled, live, busted, and cashed states.
- Regenerated `web/static/app.css` via `go generate ./...`.

### Task 10: Docs: update `docs/concepts/slips.md` to document live parlay and leg progress bars and live hit chance display.

- Updated `docs/concepts/slips.md` front matter (`summary` and `tags`) to include live progress tracking.
- Documented spotlight card compact live progress bars and `Live X%` display on `GET /groups/{id}/week`.
- Added a `## Live progress` section covering when progress bars appear, the slip hub parlay card (`#slip-panel`), per-leg progress strips, `internal/progress` deterministic hit-chance and early-settlement rules, and automatic `slip-updated` SSE refreshes.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 8

- `evaluateSlipProgress`, `slipLiveView`, `formatLiveChance`, and `progressBarClass` in `internal/web/slips.go` are ready to reuse or adapt for week spotlight cards in `internal/web/home.go` and `web/templates/week.html`.

### Task 9

- Week spotlight cards reuse `evaluateSlipProgress` and `slipLiveView` from `internal/web/slips.go`, so any future live progress adjustments apply to both the slip hub and group overview.

### Task 10

Phase 3 is complete; `docs/concepts/slips.md` now reflects the live parlay and leg progress UI and hit-chance behavior across the slip hub and group overview.

## Files touched

- `docs/concepts/slips.md`
- `internal/web/home.go`
- `internal/web/home_overview_test.go`
- `internal/web/home_test.go`
- `internal/web/slips.go`
- `internal/web/slips_test.go`
- `web/static/app.css`
- `web/templates/partials.html`
- `web/templates/week.html`

## Run log

- Mode: 
- Checkpoints: task-8, task-9, task-10
- Verify: verify passed: test 6.2s; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
