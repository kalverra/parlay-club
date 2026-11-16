# Task 5: Add HTMX `hx-sync` / search debounce guards

## Done

- Added `hx-sync="this:drop"` and `hx-disinherit` (`hx-include hx-sync` on `#pick-slate`, `hx-sync` on `#pick-detail` and `#slip-panel`) in `web/templates/partials.html` so duplicate `board-updated` refreshes do not stack while child controls remain responsive.
- Added `hx-sync="this:replace"` on the `#pick-slate` search `<form>` (`hx-trigger="input changed delay:250ms, submit"`) in `web/templates/partials.html` so rapid typing or submit replaces any in-flight `/pick/results` request.
- Added `triggerBoardUpdated` (150ms debounce via `boardUpdateTimer`, cleared when the tab becomes hidden) in `web/static/app.js` for SSE `board-updated` events and `refreshLive()` so simultaneous `visibilitychange` + `focus` or burst SSE signals collapse into a single `board-updated` trigger.
- Added assertions in `TestPickBoardRendersOpenSlateAndDetail` in `internal/web/pick_test.go` covering the `hx-sync` and `hx-disinherit` attributes.

## Deviations

None.

## Open issues

None.

## Next-phase notes

None.
