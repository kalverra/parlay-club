# Task 4

## Done

- Added [web/templates/media.html](../../../web/templates/media.html) defining `team-logo`, `player-portrait`, and `matchup-logos` with lazy loading, `no-referrer`, subtle dark-mode backing (`bg-slate-800/80` / `ring-slate-700/60`), and `[data-media]` / `data-fallback-img` / `[data-fallback-badge]` attributes.
- Wired `templates/media.html` into both `Renderer.parse` and `Renderer.RenderPartial` in [internal/web/render.go](../../../internal/web/render.go), and added the `dict` (`templateDict`) template helper for passing slot options.
- Added a capturing `error` listener on `document` in [web/static/app.js](../../../web/static/app.js) for `img[data-fallback-img]` that hides the failed `<img>` and reveals its sibling `[data-fallback-badge]` with `flex`.
- Regenerated [web/static/app.css](../../../web/static/app.css) via `go generate ./web` and added `TestMediaHelpersRenderAndFallback` in [internal/web/render_test.go](../../../internal/web/render_test.go).

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Templates can invoke the helpers via `dict`, e.g. `{{template "team-logo" (dict "URL" .AwayLogo "Fallback" .AwayAbbr "Size" "h-5 w-5")}}`, `{{template "player-portrait" (dict "URL" .HeadshotURL "Initials" .Initials "Size" "h-8 w-8")}}`, and `{{template "matchup-logos" (dict "AwayURL" .AwayLogo "HomeURL" .HomeLogo)}}`.
- `data-fallback-badge` uses `hidden` when `URL` is present and `flex` when `URL` is empty (instead of combining `hidden` and `flex` in the same class list, since Tailwind v4 places `.inline-flex` after `.hidden` in `app.css`).
