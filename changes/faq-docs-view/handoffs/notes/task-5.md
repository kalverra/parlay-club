# Task 5 notes

## Done

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

None. Task 4 had already added the rows while updating the analyzer pages, so this task verified them against the code
and corrected the surrounding prose instead of duplicating the rows.

## Open issues

- The two routes appear in three tables (shell overview, analyzer section, Docs section). That is deliberate for a
  reference doc, but future pruning could fold the analyzer and Docs tables together.
- The guide's inline "Grading results" pill and the results page's "How grading works" button still target
  `/analyzer/results` and `/analyzer/guide`. The doc says so on purpose; Phase 3 may change the links.

## Next-phase notes

- If Phase 3 repoints those inline pills to `/docs/grading` and `/docs/evals`, update the two sentences in
  `docs/reference/routes.md` that describe them. Nothing else in the file depends on the legacy targets.
- Keep the `#pillars`, `#leg-grades`, `#slip-grade`, `#suggestions`, `#rules`, and `#rule-<id>` anchor names: the route
  doc and the Phase 3 help bubbles both cite them.

## Commit

Document Docs routes for grading and evals

The route reference needs to say that the grading guide and eval results now live under Docs without breaking their
old analyzer URLs. Spelling out the shared handler, Docs tab, switcher, and back-link rules keeps an engineer from
treating either path as a redirect or losing the anchors.
