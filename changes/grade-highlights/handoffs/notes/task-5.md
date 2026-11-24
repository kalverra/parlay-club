## Done

- Added per-game `AGradeCount` to `PickGameCardView` and rendered the `"N A-grade"` emerald pill on qualifying `#pick-slate` cards in `pickPageData` and `pickSlateData`.
- Added `Highlights` (`HighlightsView`) to `PickDetailView` and rendered the `"Top plays in this game"` strip (up to 3 A-grade legs and 1 same-game A-grade parlay combo) at the top of `#pick-detail` via a shared `highlights-strip` partial in `web/templates/partials.html`.
- Added `from=pick` handling in `addLeg` (`internal/web/slips.go`) so one-tap `+ Add` from `"Top plays in this game"` returns the refreshed `#pick-body` and `tray-updated, slip-updated` triggers, matching `addLegsBatch`.
- Added handler tests in `internal/web/pick_test.go` covering `#pick-slate` counts, `#pick-detail` `"Top plays in this game"` rendering, one-tap `+ Add`, bulk `Add all 2`, slot exhaustion, and empty/zero-count hiding.

## Deviations

- Extracted `{{define "highlights-strip"}}` in `web/templates/partials.html` to share rendering between `#slip-panel` and `#pick-detail` instead of duplicating the markup.
- Placed `"Top plays in this game"` immediately below the sticky header in `#pick-detail` rather than inside the sticky container so mobile viewports can scroll past it without the sticky header covering the screen.
- Added a `from=pick` branch to `addLeg` in `internal/web/slips.go` because `from=board` returns a single `pick-line-group` partial for inline row buttons, whereas `#pick-detail` highlight buttons need `#pick-body` refreshed.

## Open issues

None.

## Next-phase notes

Task 6 should update `docs/` to document the hub and pick-board A-grade highlight strips and per-game A-grade counts, then run `good-vibes docs lint`.

## Commit

Surface A-grade counts and top plays on pick board

Show players where the strongest A-grade single legs and same-game parlays live while browsing the slate or drilling into a game. Using the shared analyzer scoring engine and cached SQLite data keeps pick-board suggestions consistent with slip grades without extra API calls.
