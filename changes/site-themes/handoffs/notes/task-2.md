## Done

- Defined semantic `@theme` color and font tokens, three `[data-theme="..."]` palettes (`paper-turf`, `redzone`, `almanac`), and shared component/badge classes (`.ui-card`, `.ui-slip-card`, `.ui-slip-divider`, `.ui-next-card`, `.ui-odds`, `.ui-stamp`, `.ui-heads-up`, `.badge-*`, `.grade-*`, `.sev-*`, `.bar-*`, `.trend-*`) in `web/tailwind/input.css`.
- Added Google Fonts `<link>` tags (`Barlow Condensed`, `IBM Plex Mono`, `Space Grotesk`) and the compact top-bar `[data-theme-cycle]` switcher button (icon-only on narrow phones, short label from `sm` up) in `web/templates/layout.html`.
- Implemented instant theme cycling (`paper-turf` -> `redzone` -> `almanac`) updating `data-theme`, `color-scheme`, `theme-color`, button labels, and `pc_theme` cookie persistence in `web/static/app.js`.
- Added `[data-theme-cycle]` assertions in `internal/web/layout_test.go` and regenerated `web/static/app.css`.

## Deviations

- Added `.grade-f` alongside `.grade-a`..`.grade-d` and `.grade-neutral` in `web/tailwind/input.css` because `internal/web/analyzer.go`'s `gradeClass` handles grade `"F"`.
- Placed `[data-theme-cycle]` right before `{{if .User}}` in `web/templates/layout.html` so it sits right before the bug-report button when signed in and also remains available on headered pages without a user.

## Open issues

None.

## Next-phase notes

Phase 2 can replace the hardcoded Tailwind class strings in `internal/web/slips.go`, `internal/web/analyzer.go`, and `internal/web/research.go` with the semantic `.badge-*`, `.grade-*`, `.sev-*`, and `.bar-*` classes already defined in `web/tailwind/input.css`.

## Commit

Add theme CSS tokens, top-bar switcher, and cycling

Defines the semantic theme token layer and three selectable palettes so templates and Go view helpers can adopt theme-aware styling in upcoming phases. Adds the compact top-bar theme switcher and client-side cookie persistence for instant, flicker-free theme switching.
