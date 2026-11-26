# Task 7 handoff

## Done

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

- Both concept docs were at or near their line budgets, so the change raises `analyzer.md` to 270 lines (new budget
  block) and `slips.md` to 590. Splitting either doc or shaving the pattern was worse than an explicit override with a
  reason.

## Open issues

- The guide's local "Grading results" pill and the results page's "How grading works" button still target the legacy
  `/analyzer/results` and `/analyzer/guide` aliases. The docs describe those aliases accurately, but a future pass could
  repoint the buttons and simplify the copy.

## Next-phase notes

- The target table in `analyzer.md#docs-hub-and-help-bubbles` mirrors `internal/web/help_bubble_test.go`. When a bubble
  is added, renamed, or repointed, update the table and the test together.
- Keep the `/docs#sys-live`, `/docs/glossary#term-*`, and `/docs/grading#*` anchors stable; the concept docs now cite
  them as the public contract.

## Commit

Document the Docs hub and help bubbles

The concept docs now explain where every inline `?` bubble leads and how the Docs tab is organized, so the help pattern
is discoverable from the analyzer and slip surfaces rather than only from the templates.
