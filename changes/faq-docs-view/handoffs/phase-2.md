# Phase 2: Phase 2: Integrate Grading Guide and Eval Results into Docs

## Done

### Task 4: Register `GET /docs/grading` and `GET /docs/evals` alongside `GET /analyzer/guide` and `GET /analyzer/results`, set `page.Tab = TabDocs` on both, and add the shared Docs sub-navigation bar (forwarding sanitized `?from=`) to `web/templates/analyzer_guide.html` and `web/templates/analyzer_results.html` with tests in `internal/web/analyzer_test.go` and `internal/web/eval_results_test.go`.

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

### Task 5: Docs: update `docs/reference/routes.md` for `/docs/grading` and `/docs/evals`.

- Confirmed `docs/reference/routes.md` lists `GET /docs/grading` and `GET /docs/evals` in the shell overview, the
  analyzer sections, and the Docs section, and that every claim matches the registered handlers.
- Tightened the analyzer guide and results sections to say both paths set `page.Tab = TabDocs` and render the same docs
  switcher with their own section current, and to separate the inline "Grading results" / "How grading works" links
  (still on the legacy paths) from the switcher links to `/docs/grading` and `/docs/evals`.
- Noted that `/docs/grading` preserves the guide's anchors as well as `?from=`, and that the eval `?from=` value also
  rides on the switcher's link to `/docs/grading`.
- Replaced the loose `safeReturnPath` summary with the checks the function actually makes: one leading `/`, no `//`,
  no backslash or control character, and no `scheme:` before the first path slash.

## Deviations

### Task 4

- The hub's grading links (`Full grading rubric`, `Badge definitions`, and the uncertainty FAQ) now point at
  `/docs/grading#...` instead of `/analyzer/guide#...`. The task does not name them, but leaving them on the legacy path
  while the switcher marks `/docs/grading` current splits the Docs section across two URL namespaces.
- Fixed the stale bottom-bar sentence in `docs/concepts/slips.md` that Task 3 left for Phase 3. The bar has read
  "Groups, This week, Record Book, and Docs" since Task 1, so the sentence was already wrong.

### Task 5

None. Task 4 had already added the rows while updating the analyzer pages, so this task verified them against the code
and corrected the surrounding prose instead of duplicating the rows.

## Open issues

### Task 4

- The guide's local "Grading results" pill and the results page's "How grading works" button still target
  `/analyzer/results` and `/analyzer/guide`. Both aliases render identically with the Docs tab, so nothing breaks;
  Phase 3 can repoint them when it rewrites the templates for help bubbles.
- Guide and eval pages now render two `aria-current="page"` anchors (bottom Docs tab plus the current switcher pill),
  matching the other docs pages. Tests that count occurrences must expect both.

### Task 5

- The two routes appear in three tables (shell overview, analyzer section, Docs section). That is deliberate for a
  reference doc, but future pruning could fold the analyzer and Docs tables together.
- The guide's inline "Grading results" pill and the results page's "How grading works" button still target
  `/analyzer/results` and `/analyzer/guide`. The doc says so on purpose; Phase 3 may change the links.

## Next-phase notes

### Task 4

- Phase 3 help bubbles should deep-link to `/docs/grading?from=<current>#leg-grades`, `#slip-grade`, `#suggestions`,
  or `#rule-<id>`, and to `/docs/glossary?from=<current>#term-<slug>`. Those anchors are unchanged and covered by tests.
- `web/templates/partials.html` still links leg grades, slip grades, and warning cards to `/analyzer/guide`. Those links
  keep working, but moving them to `/docs/grading` is the remaining half of the move.
- Keep the `docs-nav` partial as the one switcher definition; adding a fifth docs section means editing `docsNav` in
  `internal/web/docs.go`, not the templates.

### Task 5

- If Phase 3 repoints those inline pills to `/docs/grading` and `/docs/evals`, update the two sentences in
  `docs/reference/routes.md` that describe them. Nothing else in the file depends on the legacy targets.
- Keep the `#pillars`, `#leg-grades`, `#slip-grade`, `#suggestions`, `#rules`, and `#rule-<id>` anchor names: the route
  doc and the Phase 3 help bubbles both cite them.

## Files touched

- `docs/concepts/analyzer.md`
- `docs/concepts/slips.md`
- `docs/reference/routes.md`
- `internal/web/analyzer.go`
- `internal/web/analyzer_test.go`
- `internal/web/docs.go`
- `internal/web/docs_handlers_test.go`
- `internal/web/eval_results.go`
- `internal/web/eval_results_test.go`
- `internal/web/slips.go`
- `web/templates/analyzer_guide.html`
- `web/templates/analyzer_results.html`
- `web/templates/docs.html`
- `web/templates/partials.html`

## Run log

- Mode: 
- Checkpoints: task-4, task-5
- Verify: verify passed: test 7.7s; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
