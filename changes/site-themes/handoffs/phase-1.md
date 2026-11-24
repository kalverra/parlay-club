# Phase 1: Phase 1: Theme registry, cookie plumbing, and CSS token foundation

## Done

### Task 1: Add `internal/web/theme.go` (`ThemeID`, `Theme`, allowlist for `paper-turf`, `redzone`, `almanac`, `ThemeCookie`, `ThemeFromRequest`) and `Page.Theme` / `Page.ActiveTheme()` in `internal/web/page.go`; wire `ThemeFromRequest(r)` in `internal/web`, `internal/auth`, `internal/groups`, and `cmd/server/sandbox.go`, with unit tests covering default, valid, and invalid cookies plus `layout_test.go` SSR assertions.

- Added `internal/web/theme.go` with `ThemeCookie`, `ThemeID`, `Theme`, the three-theme allowlist (`paper-turf`, `redzone`, `almanac`), and `ThemeFromRequest`.
- Added `Page.Theme` and `Page.ActiveTheme()` to `internal/web/page.go`, defaulting to `paper-turf` when empty or unrecognized.
- Wired `ThemeFromRequest` into all full-page builders in `internal/web`, `internal/auth`, `internal/groups`, and `cmd/server/sandbox.go`.
- Wired `data-theme`, `color-scheme`, and `theme-color` attributes in `web/templates/layout.html` so SSR reflects the active theme.
- Added cookie/fallback unit tests in `internal/web/theme_test.go` and SSR layout assertions in `internal/web/layout_test.go`.

### Task 2: Define the semantic `@theme` tokens, three `[data-theme="..."]` palettes (`paper-turf`, `redzone`, `almanac`), and shared component/badge classes in `web/tailwind/input.css`; add Google Fonts links, `data-theme`/`color-scheme`/`theme-color` attributes, and the compact top-bar `[data-theme-cycle]` button in `web/templates/layout.html`; implement instant theme cycling and `pc_theme` cookie persistence in `web/static/app.js`; regenerate `web/static/app.css`.

- Defined semantic `@theme` color and font tokens, three `[data-theme="..."]` palettes (`paper-turf`, `redzone`, `almanac`), and shared component/badge classes (`.ui-card`, `.ui-slip-card`, `.ui-slip-divider`, `.ui-next-card`, `.ui-odds`, `.ui-stamp`, `.ui-heads-up`, `.badge-*`, `.grade-*`, `.sev-*`, `.bar-*`, `.trend-*`) in `web/tailwind/input.css`.
- Added Google Fonts `<link>` tags (`Barlow Condensed`, `IBM Plex Mono`, `Space Grotesk`) and the compact top-bar `[data-theme-cycle]` switcher button (icon-only on narrow phones, short label from `sm` up) in `web/templates/layout.html`.
- Implemented instant theme cycling (`paper-turf` -> `redzone` -> `almanac`) updating `data-theme`, `color-scheme`, `theme-color`, button labels, and `pc_theme` cookie persistence in `web/static/app.js`.
- Added `[data-theme-cycle]` assertions in `internal/web/layout_test.go` and regenerated `web/static/app.css`.

## Deviations

### Task 2

- Added `.grade-f` alongside `.grade-a`..`.grade-d` and `.grade-neutral` in `web/tailwind/input.css` because `internal/web/analyzer.go`'s `gradeClass` handles grade `"F"`.
- Placed `[data-theme-cycle]` right before `{{if .User}}` in `web/templates/layout.html` so it sits right before the bug-report button when signed in and also remains available on headered pages without a user.

## Open issues

None.

## Next-phase notes

### Task 1

`web/templates/layout.html` already renders `data-theme="{{.ActiveTheme.ID}}"`, `<meta name="color-scheme" content="{{.ActiveTheme.ColorScheme}}">`, and `<meta name="theme-color" content="{{.ActiveTheme.ThemeColor}}">`; Task 2 only needs to add the Google Fonts links, the `[data-theme-cycle]` top-bar button, `web/tailwind/input.css` tokens/palettes, `web/static/app.js` cycling, and regenerate `web/static/app.css`.

### Task 2

Phase 2 can replace the hardcoded Tailwind class strings in `internal/web/slips.go`, `internal/web/analyzer.go`, and `internal/web/research.go` with the semantic `.badge-*`, `.grade-*`, `.sev-*`, and `.bar-*` classes already defined in `web/tailwind/input.css`.

## Files touched

- `cmd/server/sandbox.go`
- `internal/auth/handlers.go`
- `internal/groups/handlers.go`
- `internal/web/layout_test.go`
- `internal/web/page.go`
- `internal/web/research.go`
- `internal/web/slips.go`
- `internal/web/theme.go`
- `internal/web/theme_test.go`
- `web/static/app.css`
- `web/static/app.js`
- `web/tailwind/input.css`
- `web/templates/layout.html`

## Run log

- Mode: 
- Checkpoints: task-1, task-2
- Verify: verify passed: generate reused, changed: web/static/app.css, lint 2.7s, test 4.8s; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
