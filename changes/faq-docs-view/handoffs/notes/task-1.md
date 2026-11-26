# Task 1: Docs nav tab and flexible bottom bar

## Done

- Added `TabDocs Tab = "docs"` to the tab constants and a `{Tab: TabDocs, Label: "Docs", Href: "/docs"}` entry at the
  end of `navTabs` in `internal/web/page.go`.
- Replaced `grid-cols-3` with `grid-flow-col auto-cols-fr` on the bottom `<nav>` container in
  `web/templates/layout.html`, so the bar splits width evenly for any number of registered tabs.
- Extended `internal/web/page_test.go` to expect four tabs, a `docs` case, and the new label list.
- Extended `TestLayoutRendersBottomTabs` in `internal/web/layout_test.go` to assert the `Docs` link, the flexible grid
  classes, and that `grid-cols-3` is gone.
- Regenerated `web/static/app.css` so the new utility classes ship.

## Deviations

None. The `/docs` route does not exist yet, so the tab points at a 404 until the next task registers the handler. That
is the order the design asks for.

## Open issues

- `/docs` returns 404 until `internal/web/docs.go` lands; do not ship this phase alone.

## Next-phase notes

- Build the docs hub at `GET /docs` and set `page.Tab = TabDocs` on every docs route so this tab highlights.
- Phase 2 aliases (`/analyzer/guide`, `/analyzer/results`) also need `page.Tab = TabDocs`.

## Commit

Add Docs tab and flexible bottom navigation

The bottom bar had a fixed three-column grid, so a fourth destination would have looked wrong. A flexible track lets the
Docs tab land now and the sibling Research tab land later without another layout change.
