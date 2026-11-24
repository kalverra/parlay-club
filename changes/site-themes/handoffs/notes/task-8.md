## Done

- Documented the `pc_theme` cookie allowlist (`paper-turf`, `redzone`, `almanac`), SSR `data-theme` / meta tag rendering, and client-side `[data-theme-cycle]` switching in `docs/reference/routes.md`.
- Documented the semantic theme token convention (`web/tailwind/input.css` page/chrome, surface, accent/status, and `.ui-*` / semantic badge classes) and the no-raw-palette rule in `docs/reference/routes.md`.
- Added `themes` to `docs/reference/routes.md` front-matter tags and verified `good-vibes docs lint` reports 0 errors and 0 warnings for `docs/reference/routes.md`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

All Phase 4 tasks are complete.

## Commit

Document pc_theme cookie and semantic theme tokens

Record the cookie-backed SSR theme contract and Tailwind semantic token conventions in the routes reference so future UI changes stay theme-safe without raw palette leakage.
