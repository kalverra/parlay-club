# Task 2

## Done

- Added `WeekID` and `ScopeBadge` to `SlipView` in `internal/web/slips.go`, populated from `slip.WeekID` and `scopeBadge(slip.SlateScope)` in `slipView`.
- Updated `web/templates/partials.html` to render `ScopeBadge` beside the slip title and a `+ New slip` link (`/slips/new?week=...`) in both single-slip weeks (metadata row) and multi-slip weeks (at the end of `[data-sibling-switcher]`).
- Removed the redundant top-bar `#slip-switcher` from `internal/web/page.go` (`SlipOption`, `Page.Slips`), `internal/web/slips.go` (`slipOptions`), `web/templates/layout.html`, and `web/static/app.js`, and regenerated `web/static/app.css`.
- Updated `internal/web/layout_test.go` and `internal/web/slips_test.go` to verify `#slip-switcher` is absent and `ScopeBadge` and `+ New slip` render in both single-slip and multi-slip views.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `SlipView.WeekID` and `ScopeBadge` are now available on `/slips/{id}` and `/slips/{id}/panel` partials.

## Commit

Show scope badge and new-slip link on slip header

Display each slip's slate scope and a direct link to create another slip for the week on the slip card, so single-slip weeks have a one-tap entry point to one-off slips. Remove the redundant top-bar slip select to keep the phone header uncluttered and rely on the card's sibling pill bar instead.
