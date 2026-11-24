## Done

- Added `internal/web/theme.go` with `ThemeCookie`, `ThemeID`, `Theme`, the three-theme allowlist (`paper-turf`, `redzone`, `almanac`), and `ThemeFromRequest`.
- Added `Page.Theme` and `Page.ActiveTheme()` to `internal/web/page.go`, defaulting to `paper-turf` when empty or unrecognized.
- Wired `ThemeFromRequest` into all full-page builders in `internal/web`, `internal/auth`, `internal/groups`, and `cmd/server/sandbox.go`.
- Wired `data-theme`, `color-scheme`, and `theme-color` attributes in `web/templates/layout.html` so SSR reflects the active theme.
- Added cookie/fallback unit tests in `internal/web/theme_test.go` and SSR layout assertions in `internal/web/layout_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

`web/templates/layout.html` already renders `data-theme="{{.ActiveTheme.ID}}"`, `<meta name="color-scheme" content="{{.ActiveTheme.ColorScheme}}">`, and `<meta name="theme-color" content="{{.ActiveTheme.ThemeColor}}">`; Task 2 only needs to add the Google Fonts links, the `[data-theme-cycle]` top-bar button, `web/tailwind/input.css` tokens/palettes, `web/static/app.js` cycling, and regenerate `web/static/app.css`.

## Commit

Add theme registry and SSR cookie plumbing

Support server-side rendering of the user's selected site theme from the `pc_theme` cookie so pages load with the right palette and browser chrome metadata without flicker. An allowlist of three themes falls back to `paper-turf` whenever the cookie is missing or invalid.
