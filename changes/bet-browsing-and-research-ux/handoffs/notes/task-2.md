# Task 2

## Done

- Converted `pick-detail` sections in `web/templates/partials.html` from `<section>` to native `<details class="group/section space-y-2" data-section="{{.Key}}" data-keep-open="{{.Key}}">` with `<summary>` headers containing the section badge, label, count, and rotating chevron.
- Set sections to render collapsed by default when `$.Search` is empty and auto-open (`open`) when `$.Search` is non-empty, and added `data-game="{{.GameID}}"` on `#pick-detail`.
- Added `snapshotPickDetailOpenState` and `restorePickDetailOpenState` in `web/static/app.js` across `htmx:beforeSwap` and `htmx:afterSwap` to preserve expanded `[data-keep-open]` section keys across `#pick-detail` swaps on the same game while clearing state when switching games.
- Added `TestPickDetailSectionsCollapseByDefaultAndAutoOpenWhenSearching` in `internal/web/pick_test.go` and regenerated `web/static/app.css` via `good-vibes verify generate`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Phase 2 can now build compact O/U pick rows and slip O/U badges on top of the collapsible `#pick-detail` sections.

## Commit

Collapse pick-detail sections and preserve open state

Long game slates with many prop categories were noisy to scan on phone screens when every section rendered expanded. Native details headers keep groups compact by default, auto-open during search, and retain user-expanded sections across HTMX refreshes on the same game.
