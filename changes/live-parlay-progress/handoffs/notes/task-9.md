## Done

- Added `Live SlipLiveView` to `SpotlightSlipCard` in `internal/web/home.go` and populated it in `buildGroupOverviewPage` via `evaluateSlipProgress` and `slipLiveView`.
- Rendered a compact single-row live progress bar and live hit percentage (`data-spotlight-live-progress`, `role="progressbar"`, status badge, truncated summary, and `Live <X>%`) inside spotlight cards in `web/templates/week.html`.
- Added unit assertions in `internal/web/home_overview_test.go` and overview handler/HTML/phone-width layout tests (`TestGroupWeekSpotlightLiveProgress`) in `internal/web/home_test.go` covering scheduled, live, busted, and cashed states.
- Regenerated `web/static/app.css` via `go generate ./...`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Week spotlight cards reuse `evaluateSlipProgress` and `slipLiveView` from `internal/web/slips.go`, so any future live progress adjustments apply to both the slip hub and group overview.

## Commit

Show live parlay progress in week spotlight cards

Group members checking the weekly overview need to see live parlay progress and hit chances without opening every active slip. Reusing the slip progress evaluator in spotlight cards keeps live status, bar tone, and hit percentages consistent across both screens.
