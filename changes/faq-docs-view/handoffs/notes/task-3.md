# Task 3 notes

## Done

- Added `GET /docs` and `GET /docs/glossary` to the Shell and tabs route table in `docs/reference/routes.md`.
- Added a "Docs hub and glossary" section covering the shared section switcher, the `?from=` back link, the seven
  `#sys-*` anchors, the in-memory `?q=` glossary filter, `#term-<slug>` anchors, the match count, and the empty state.
- Updated the bottom-bar prose from three destinations to four, adding Docs, and added `docs` and `glossary` to the doc's
  tags.

## Deviations

None.

## Open issues

- `docs/concepts/slips.md` still says the bottom bar keeps "Groups, This week, and Record Book". The Phase 3 docs task
  that rewrites that file for the Docs hub should correct the sentence.

## Next-phase notes

- Phase 2 must add `/docs/grading` and `/docs/evals` rows and move the switcher description off the temporary
  `/analyzer/guide` and `/analyzer/results` targets.
- Keep the `#sys-*` and `#term-*` anchor names in the doc in sync with `internal/web/docs.go`; Phase 3 help bubbles
  deep-link to them.

## Commit

Document docs routes in the routes reference

The route reference claimed the bottom bar had three destinations and did not list the two new Docs screens, so it no
longer described every authenticated page. Recording the anchors, filtering, and back-link behavior gives Phase 2 and
the help bubbles a stable contract to link against.
