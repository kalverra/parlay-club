# Task 6 notes

## Done

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

## Deviations

- The design named `[data-help-popover]` and `[data-leg-grade]` as separate popover selectors. The grade `<details>`
  carry both attributes, and the outside-click handler closes `details[data-help-popover][open]`, which covers leg and
  slip grades plus any later popover that adopts the marker.
- Pick-option badge bubbles were not added inside the option buttons because an `<a>` inside a `<button>` is invalid
  HTML and double-fires on mobile Safari. The design already allowed the detail header, so the badge explanation lives
  there.

## Open issues

- The guide's local "Grading results" pill still points at `/analyzer/results`, and the results page's "How grading
  works" button still points at `/analyzer/guide`. Both aliases render the Docs tab, so nothing breaks; this task did
  not name them.
- `web/templates/week.html` still shows a live chance without a bubble. The task scoped bubbles to
  `web/templates/partials.html`, so the week card follows the shared slip panel behavior but has no inline explainer of
  its own.

## Next-phase notes

- Keep the `help-bubble` define in `partials.html`: `RenderPartial` and every full page parse that file, so a new
  annotation only needs `{{template "help-bubble" (dict "Href" ... "Label" ...)}}`.
- Keep the anchors `#sys-live`, `#term-nfelo`, `#term-implied-prob`, `#term-reference-odds`, `#leg-grades`,
  `#slip-grade`, `#suggestions`, and `#rule-<id>` stable. `help_bubble_test.go` fails if a rendered bubble points at a
  missing fragment.
- The test requires each rendered screen in its table to show at least one bubble. If a future task removes grades or
  odds from a screen, update the table rather than deleting the anchor checks.

## Commit

Add reusable help bubbles to inline annotations

The grade, warning, model, odds, badge, and live-chance annotations each explained themselves differently, and several
still pointed at the legacy analyzer guide. One shared `?` bubble keeps the labels compact and sends every annotation to
the docs anchor that now owns the explanation.
