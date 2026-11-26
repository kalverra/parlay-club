# Task 2 notes

## Done

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

## Deviations

- The design's switcher names four sections, but `/docs/grading` and `/docs/evals` are Phase 2. "Grading & Rules" and
  "Eval Results" point at the existing `/analyzer/guide` and `/analyzer/results` until Phase 2 repoints them.
- `pageFiles` has one `docs` entry mapped to `templates/docs.html`, and the file holds both a hub and a glossary block
  picked by data. This keeps the task's single-template shape instead of adding a second file.

## Open issues

- The analyzer pages still render with no tab and no docs sub-nav; Phase 2 owns that.
- Docs pages render two `aria-current="page"` anchors: the bottom Docs tab and the current sub-nav section. Tests that
  count occurrences on docs routes must expect both.

## Next-phase notes

- Repoint `docsNav` grading and eval links to `/docs/grading` and `/docs/evals` when those aliases land, keeping the
  `?from=` forwarding.
- Leave `#sys-*` and `#term-*` anchors and slugs untouched; Phase 3 help bubbles link to them and the table-driven
  anchor tests rely on the current names.
- The next Phase 1 task still needs to add both routes to `docs/reference/routes.md`.

## Commit

Add Docs hub and searchable glossary

People learn the app from scattered screens and have no plain-language reference, so this adds one Docs destination
with system explainers, common questions, and a searchable glossary. The copy is static Go data, so the routes stay
deterministic and free.
