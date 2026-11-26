# Phase 3: Phase 3: Reusable `?` help bubbles and two-step annotation popovers

## Done

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

## Deviations

### Task 6

- The design named `[data-help-popover]` and `[data-leg-grade]` as separate popover selectors. The grade `<details>`
  carry both attributes, and the outside-click handler closes `details[data-help-popover][open]`, which covers leg and
  slip grades plus any later popover that adopts the marker.
- Pick-option badge bubbles were not added inside the option buttons because an `<a>` inside a `<button>` is invalid
  HTML and double-fires on mobile Safari. The design already allowed the detail header, so the badge explanation lives
  there.

### Task 7

- Both concept docs were at or near their line budgets, so the change raises `analyzer.md` to 270 lines (new budget
  block) and `slips.md` to 590. Splitting either doc or shaving the pattern was worse than an explicit override with a
  reason.

## Open issues

### Task 6

- The guide's local "Grading results" pill still points at `/analyzer/results`, and the results page's "How grading
  works" button still points at `/analyzer/guide`. Both aliases render the Docs tab, so nothing breaks; this task did
  not name them.
- `web/templates/week.html` still shows a live chance without a bubble. The task scoped bubbles to
  `web/templates/partials.html`, so the week card follows the shared slip panel behavior but has no inline explainer of
  its own.

### Task 7

- The guide's local "Grading results" pill and the results page's "How grading works" button still target the legacy
  `/analyzer/results` and `/analyzer/guide` aliases. The docs describe those aliases accurately, but a future pass could
  repoint the buttons and simplify the copy.

## Next-phase notes

### Task 6

- Keep the `help-bubble` define in `partials.html`: `RenderPartial` and every full page parse that file, so a new
  annotation only needs `{{template "help-bubble" (dict "Href" ... "Label" ...)}}`.
- Keep the anchors `#sys-live`, `#term-nfelo`, `#term-implied-prob`, `#term-reference-odds`, `#leg-grades`,
  `#slip-grade`, `#suggestions`, and `#rule-<id>` stable. `help_bubble_test.go` fails if a rendered bubble points at a
  missing fragment.
- The test requires each rendered screen in its table to show at least one bubble. If a future task removes grades or
  odds from a screen, update the table rather than deleting the anchor checks.

### Task 7

- The target table in `analyzer.md#docs-hub-and-help-bubbles` mirrors `internal/web/help_bubble_test.go`. When a bubble
  is added, renamed, or repointed, update the table and the test together.
- Keep the `/docs#sys-live`, `/docs/glossary#term-*`, and `/docs/grading#*` anchors stable; the concept docs now cite
  them as the public contract.

## Files touched

- `docs/concepts/analyzer.md`
- `docs/concepts/slips.md`
- `internal/web/analyzer_test.go`
- `internal/web/help_bubble_test.go`
- `web/static/app.css`
- `web/static/app.js`
- `web/templates/partials.html`

## Run log

- Mode: 
- Checkpoints: task-6, task-7
- Verify: verify passed: test 800ms; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
