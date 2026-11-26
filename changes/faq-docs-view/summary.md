---
slug: faq-docs-view
run: 20261126-011534-faq-docs-view
outcome: done
generated: 2026-11-26T01:54:57Z
---

# Run summary: faq-docs-view

## Outcome

- Status: done
- Branch: faq-docs-view
- Close-out: written
- Close: pending
- Final verdict: claude@claude-sonnet-5-5 approved (issues: 0)

## Totals

- Tasks: 7 of 7
- Phases: 3
- Checkpoints: 8
- Duration: 35m46s
- Cost: $1.44
- Fresh input tokens: 831597
- Cache read tokens: 40032113
- Cache write tokens: 150202
- Output tokens: 224078

| Model | Tokens | Cost |
| --- | ---: | ---: |
| fireworks-ai/accounts/fireworks/models/deepseek-v4p1-flash-1017 | 39101010 | $0.73 |
| claude-sonnet-5-5 | 2136980 | $0.70 |

## Phases

### Phase 1: Primary Docs nav tab and unified hub

Tasks:

- [x] Add `TabDocs` (`Docs`, `/docs`) to `internal/web/page.go` and switch the bottom nav container in `web/templates/layout.html` from `grid-cols-3` to `grid-flow-col auto-cols-fr`, updating `internal/web/page_test.go` and `internal/web/layout_test.go`.
- [x] Implement `internal/web/docs.go` and `web/templates/docs.html` serving `GET /docs` (How Systems Work + FAQ with stable `#sys-*` anchors) and `GET /docs/glossary` (structured glossary with `#term-*` anchors and `?q=` filtering), wired into `internal/web/render.go` and `internal/web/slips.go` with handler and anchor-uniqueness tests.
- [x] Docs: document the new `/docs` and `/docs/glossary` routes in `docs/reference/routes.md`.

Phase totals: 3 checkpoints, 6m26s, $0.12, 4195593 input / 40918 output / 4033982 cache read / 0 cache write tokens

Checkpoints: task-1, task-2, task-3 Review fixes: 0 Integration checks: 0

#### Done

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

### Phase 2: Integrate Grading Guide and Eval Results into Docs

Tasks:

- [x] Register `GET /docs/grading` and `GET /docs/evals` alongside `GET /analyzer/guide` and `GET /analyzer/results`, set `page.Tab = TabDocs` on both, and add the shared Docs sub-navigation bar (forwarding sanitized `?from=`) to `web/templates/analyzer_guide.html` and `web/templates/analyzer_results.html` with tests in `internal/web/analyzer_test.go` and `internal/web/eval_results_test.go`.
- [x] Docs: update `docs/reference/routes.md` for `/docs/grading` and `/docs/evals`.

Phase totals: 2 checkpoints, 5m12s, $0.14, 7428487 input / 36674 output / 7254290 cache read / 0 cache write tokens

Checkpoints: task-4, task-5 Review fixes: 0 Integration checks: 0

#### Done

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

### Phase 3: Reusable `?` help bubbles and two-step annotation popovers

Tasks:

- [x] Add a reusable `help-bubble` partial and wire it into leg grades, slip grades, analyzer warnings, pick-board signal badges, `nfelo` model chips, implied probability / reference odds, and live hit chance in `web/templates/partials.html` (plus outside-click popover close in `web/static/app.js`), keeping `?` bubbles outside `<summary>` elements and adding tests that verify every rendered `?` bubble anchor exists in `/docs`, `/docs/glossary`, or `/docs/grading`.
- [x] Docs: update `docs/concepts/analyzer.md` and `docs/concepts/slips.md` to describe the Docs hub and two-step `?` help-bubble pattern.

Phase totals: 3 checkpoints, 23m12s, $1.15, 28875753 input / 140631 output / 28268241 cache read / 150202 cache write tokens

Checkpoints: task-6, task-7, final Review fixes: 0 Integration checks: 0

#### Done

### Task 6: Add a reusable `help-bubble` partial and wire it into leg grades, slip grades, analyzer warnings, pick-board signal badges, `nfelo` model chips, implied probability / reference odds, and live hit chance in `web/templates/partials.html` (plus outside-click popover close in `web/static/app.js`), keeping `?` bubbles outside `<summary>` elements and adding tests that verify every rendered `?` bubble anchor exists in `/docs`, `/docs/glossary`, or `/docs/grading`.

- Added a reusable `help-bubble` partial in `web/templates/partials.html`: a compact circular `?` anchor with an
  accessible label, a stable `data-help-bubble` hook, and the docs target in `href`.
- Replaced the grade breakdown's "See full rubric" link, the warning cards' "Why?" links, and the legacy inline
  `/analyzer/guide` links on the slip panel with `?` bubbles that point at `/docs/grading#leg-grades`,
  `#slip-grade`, and `#rule-<id>`.
- Paired the slip's live hit chance, leg implied chance, payout combined/reference implied chance, and `nfelo` model
  chip with bubbles to `/docs#sys-live`, `/docs/glossary#term-implied-prob`, `#term-reference-odds`, and `#term-nfelo`.
  The pick-detail header carries one badge bubble to `/docs/grading#suggestions`, since the signal badges themselves sit
  inside buttons where a nested anchor is invalid.
- Added `data-help-popover` to every grade `<details>` and an outside-click close in `web/static/app.js`, so only one
  breakdown stays open at a time on a phone.
- Added `internal/web/help_bubble_test.go`, which renders the slip panel, tray, payout, live panel, and pick detail, then
  asserts every rendered bubble targets one of `/docs`, `/docs/glossary`, or `/docs/grading` and that the fragment
  exists on that page. It also checks that the leg, slip, model, implied, reference, live, badge, and rule bubbles all
  appear.
- Rewrote the analyzer doc's banner section to describe the `?` bubbles instead of the removed "See full rubric" and
  "Why?" links.

### Task 7: Docs: update `docs/concepts/analyzer.md` and `docs/concepts/slips.md` to describe the Docs hub and two-step `?` help-bubble pattern.

- Rewrote `docs/concepts/analyzer.md`'s closing guide section as "Docs hub and help bubbles": the four Docs screens
  (Overview, Glossary, Grading & Rules, Eval Results), their aliases, the two-step annotation pattern, the
  `<summary>` nesting rule, and outside-click close.
- Added an annotation-to-target table covering leg and slip grades, warning cards, pick-board badges, the `nfelo` chip,
  implied chance, reference odds, and live hit chance, plus a note that every bubble forwards `?from=` and Docs renders
  a `safeReturnPath`-checked back link.
- Pointed the analyzer banner's inline-annotation paragraph at that section instead of repeating the pattern.
- Updated `docs/concepts/slips.md` to describe the Docs tab, the leg row's grade pill and its `?` bubbles (implied,
  `nfelo`), the payout Combined/Reference bubbles, the pick-detail badge bubble, the live-chance bubble, and the split
  between analyzer grades and graded results.
- Added `docs`/`help-bubbles` tags to both docs and budget overrides with reasons for the added conceptual copy.

## Final gate

- Rounds used: 2; verify failures: 1
- Final verify: passed
- Open issues: none

## Ship

- Sync: none
- Recipe: ask
