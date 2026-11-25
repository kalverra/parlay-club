# Task 8

## Done

- Added `SlipLiveView` and `LegLiveView` to `internal/web/slips.go`, decoding `BoxScoreJSON` once per `LIVE`/`FINAL` game and evaluating legs and slips via `progress.EvaluateLeg` and `progress.EvaluateSlip`.
- Rendered the parlay live progress card (`data-slip-live-progress`, `role="progressbar"`, live vs. pre-game hit chance, and status summary) and per-leg progress strips (`data-leg-live-progress`, stat/clock readout, and live hit chance) in `web/templates/partials.html`, and regenerated `web/static/app.css`.
- Added `TestSlipHubLiveProgressStates` and `TestSlipPanelSSERefreshesLiveProgress` in `internal/web/slips_test.go` covering scheduled, live, cashed, busted, and SSE-refreshed `#slip-panel` states.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `evaluateSlipProgress`, `slipLiveView`, `formatLiveChance`, and `progressBarClass` in `internal/web/slips.go` are ready to reuse or adapt for week spotlight cards in `internal/web/home.go` and `web/templates/week.html`.
