# Phase 1: Phase 1: Primary Docs nav tab and unified hub

## Done

### Task 1: Add `TabDocs` (`Docs`, `/docs`) to `internal/web/page.go` and switch the bottom nav container in `web/templates/layout.html` from `grid-cols-3` to `grid-flow-col auto-cols-fr`, updating `internal/web/page_test.go` and `internal/web/layout_test.go`.

- Added `TabDocs Tab = "docs"` to the tab constants and a `{Tab: TabDocs, Label: "Docs", Href: "/docs"}` entry at the
  end of `navTabs` in `internal/web/page.go`.
- Replaced `grid-cols-3` with `grid-flow-col auto-cols-fr` on the bottom `<nav>` container in
  `web/templates/layout.html`, so the bar splits width evenly for any number of registered tabs.
- Extended `internal/web/page_test.go` to expect four tabs, a `docs` case, and the new label list.
- Extended `TestLayoutRendersBottomTabs` in `internal/web/layout_test.go` to assert the `Docs` link, the flexible grid
  classes, and that `grid-cols-3` is gone.
- Regenerated `web/static/app.css` so the new utility classes ship.

### Task 2: Implement `internal/web/docs.go` and `web/templates/docs.html` serving `GET /docs` (How Systems Work + FAQ with stable `#sys-*` anchors) and `GET /docs/glossary` (structured glossary with `#term-*` anchors and `?q=` filtering), wired into `internal/web/render.go` and `internal/web/slips.go` with handler and anchor-uniqueness tests.

- Added `internal/web/docs.go` with the `DocsPage` shell, static hub content (quick jumps, seven `#sys-*` system
  sections, and seven FAQ items), and a 15-term `GlossaryEntry` slice with stable `#term-<slug>` anchors.
- Registered `GET /docs` and `GET /docs/glossary` in `slipHandlers.Routes` and the `docs` page in `render.go`. Both
  routes set `TabDocs` and carry a `safeReturnPath` back link that the section switcher forwards.
- Added `web/templates/docs.html`, which renders both screens from one file and gives the glossary a server-side `?q=`
  search form, a match count, and an empty state.
- Added `internal/web/docs_test.go` (unique slugs and section IDs, required anchors, filter behavior) and
  `internal/web/docs_handlers_test.go` (status, titles, tab highlight, anchor resolution, unique IDs, safe back link,
  escaped query).
- Extended the render test fixture for the new `docs` page entry and regenerated `web/static/app.css`.

### Task 3: Docs: document the new `/docs` and `/docs/glossary` routes in `docs/reference/routes.md`.

- Added `GET /docs` and `GET /docs/glossary` to the Shell and tabs route table in `docs/reference/routes.md`.
- Added a "Docs hub and glossary" section covering the shared section switcher, the `?from=` back link, the seven
  `#sys-*` anchors, the in-memory `?q=` glossary filter, `#term-<slug>` anchors, the match count, and the empty state.
- Updated the bottom-bar prose from three destinations to four, adding Docs, and added `docs` and `glossary` to the doc's
  tags.

## Deviations

### Task 1

None. The `/docs` route does not exist yet, so the tab points at a 404 until the next task registers the handler. That
is the order the design asks for.

### Task 2

- The design's switcher names four sections, but `/docs/grading` and `/docs/evals` are Phase 2. "Grading & Rules" and
  "Eval Results" point at the existing `/analyzer/guide` and `/analyzer/results` until Phase 2 repoints them.
- `pageFiles` has one `docs` entry mapped to `templates/docs.html`, and the file holds both a hub and a glossary block
  picked by data. This keeps the task's single-template shape instead of adding a second file.

## Open issues

### Task 1

- `/docs` returns 404 until `internal/web/docs.go` lands; do not ship this phase alone.

### Task 2

- The analyzer pages still render with no tab and no docs sub-nav; Phase 2 owns that.
- Docs pages render two `aria-current="page"` anchors: the bottom Docs tab and the current sub-nav section. Tests that
  count occurrences on docs routes must expect both.

### Task 3

- `docs/concepts/slips.md` still says the bottom bar keeps "Groups, This week, and Record Book". The Phase 3 docs task
  that rewrites that file for the Docs hub should correct the sentence.

## Next-phase notes

### Task 1

- Build the docs hub at `GET /docs` and set `page.Tab = TabDocs` on every docs route so this tab highlights.
- Phase 2 aliases (`/analyzer/guide`, `/analyzer/results`) also need `page.Tab = TabDocs`.

### Task 2

- Repoint `docsNav` grading and eval links to `/docs/grading` and `/docs/evals` when those aliases land, keeping the
  `?from=` forwarding.
- Leave `#sys-*` and `#term-*` anchors and slugs untouched; Phase 3 help bubbles link to them and the table-driven
  anchor tests rely on the current names.
- The next Phase 1 task still needs to add both routes to `docs/reference/routes.md`.

### Task 3

- Phase 2 must add `/docs/grading` and `/docs/evals` rows and move the switcher description off the temporary
  `/analyzer/guide` and `/analyzer/results` targets.
- Keep the `#sys-*` and `#term-*` anchor names in the doc in sync with `internal/web/docs.go`; Phase 3 help bubbles
  deep-link to them.

## Files touched

- `docs/reference/routes.md`
- `internal/web/docs.go`
- `internal/web/docs_handlers_test.go`
- `internal/web/docs_test.go`
- `internal/web/layout_test.go`
- `internal/web/page.go`
- `internal/web/page_test.go`
- `internal/web/render.go`
- `internal/web/render_test.go`
- `internal/web/slips.go`
- `web/static/app.css`
- `web/templates/docs.html`
- `web/templates/layout.html`

## Run log

- Mode: 
- Checkpoints: task-1, task-2, task-3
- Verify: verify passed: test 7.3s; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
