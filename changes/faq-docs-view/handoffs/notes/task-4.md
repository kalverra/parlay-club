# Task 4 notes

## Done

- Registered `GET /docs/grading` and `GET /docs/evals` on the same handlers as `GET /analyzer/guide` and
  `GET /analyzer/results`, and moved both pages from `page.Tab = ""` to `pageWithTab(..., TabDocs)`.
- Added `Nav` to `AnalyzerGuidePage` and `EvalResultsPage`, and pointed the switcher's grading and eval tabs at
  `/docs/grading` and `/docs/evals` while keeping `?from=` forwarding.
- Extracted the switcher markup into a `docs-nav` partial in `web/templates/partials.html` and used it from
  `docs.html`, `analyzer_guide.html`, and `analyzer_results.html`.
- Added tests in `internal/web/analyzer_test.go` and `internal/web/eval_results_test.go` covering both URL pairs, the
  Docs tab, the current switcher pill, guide anchors and rules, results filters (`source`, `season`, `engine`), safe
  `?from=` forwarding, stripped unsafe `?from=`, and the alias's `400` on an unreadable season.
- Updated `docs/reference/routes.md` and `docs/concepts/analyzer.md` for the aliases and `TabDocs`.

## Deviations

- The hub's grading links (`Full grading rubric`, `Badge definitions`, and the uncertainty FAQ) now point at
  `/docs/grading#...` instead of `/analyzer/guide#...`. The task does not name them, but leaving them on the legacy path
  while the switcher marks `/docs/grading` current splits the Docs section across two URL namespaces.
- Fixed the stale bottom-bar sentence in `docs/concepts/slips.md` that Task 3 left for Phase 3. The bar has read
  "Groups, This week, Record Book, and Docs" since Task 1, so the sentence was already wrong.

## Open issues

- The guide's local "Grading results" pill and the results page's "How grading works" button still target
  `/analyzer/results` and `/analyzer/guide`. Both aliases render identically with the Docs tab, so nothing breaks;
  Phase 3 can repoint them when it rewrites the templates for help bubbles.
- Guide and eval pages now render two `aria-current="page"` anchors (bottom Docs tab plus the current switcher pill),
  matching the other docs pages. Tests that count occurrences must expect both.

## Next-phase notes

- Phase 3 help bubbles should deep-link to `/docs/grading?from=<current>#leg-grades`, `#slip-grade`, `#suggestions`,
  or `#rule-<id>`, and to `/docs/glossary?from=<current>#term-<slug>`. Those anchors are unchanged and covered by tests.
- `web/templates/partials.html` still links leg grades, slip grades, and warning cards to `/analyzer/guide`. Those links
  keep working, but moving them to `/docs/grading` is the remaining half of the move.
- Keep the `docs-nav` partial as the one switcher definition; adding a fifth docs section means editing `docsNav` in
  `internal/web/docs.go`, not the templates.

## Commit

Serve grading and evals inside Docs

The Docs tab was pointing at pages that still rendered with no tab and no way back into the section, so the move to a
Docs home was half done. Sharing the handlers keeps every existing bookmark and anchor alive while the section switcher
gives the grading guide and eval results one consistent home under `/docs`.
