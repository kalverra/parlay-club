# Phase 2: Phase 2: Render logos and portraits on the pick board and slip hub

## Done

### Task 4: Add shared `media.html` template helpers (`team-logo`, `player-portrait`, `matchup-logos`) wired in `internal/web/render.go`, plus the capturing `error` fallback in `web/static/app.js` and regenerated `web/static/app.css`.

- Added [web/templates/media.html](../../../web/templates/media.html) defining `team-logo`, `player-portrait`, and `matchup-logos` with lazy loading, `no-referrer`, subtle dark-mode backing (`bg-slate-800/80` / `ring-slate-700/60`), and `[data-media]` / `data-fallback-img` / `[data-fallback-badge]` attributes.
- Wired `templates/media.html` into both `Renderer.parse` and `Renderer.RenderPartial` in [internal/web/render.go](../../../internal/web/render.go), and added the `dict` (`templateDict`) template helper for passing slot options.
- Added a capturing `error` listener on `document` in [web/static/app.js](../../../web/static/app.js) for `img[data-fallback-img]` that hides the failed `<img>` and reveals its sibling `[data-fallback-badge]` with `flex`.
- Regenerated [web/static/app.css](../../../web/static/app.css) via `go generate ./web` and added `TestMediaHelpersRenderAndFallback` in [internal/web/render_test.go](../../../internal/web/render_test.go).

### Task 5: Render team logos and player portraits on `pick-slate` cards, `pick-detail` headers and `PickPlayerView` cards, and `slip-panel` legs (`LegView`) with graceful fallbacks for missing or unlinked legs, with handler/render tests.

- Threaded `AwayLogo` and `HomeLogo` onto `PickGameCardView` and `PickDetailView`, and `Initials`, `HeadshotURL`, and `TeamLogo` onto `PickPlayerView` in [internal/web/pick.go](../../../internal/web/pick.go).
- Extended `LegView` with `PlayerHeadshot`, `PlayerInitials`, `TeamLogo`, `MatchupAwayLogo`, and `MatchupHomeLogo`, and preloaded player portraits and team logos in `slipPageData` (`legMedia` / `applyLegMedia`) in [internal/web/slips.go](../../../internal/web/slips.go) using `RosterPlayersByIDs`, week games, and `TeamLogos` fallback.
- Updated `slip-panel`, `pick-slate`, and `pick-detail` templates in [web/templates/partials.html](../../../web/templates/partials.html) to render `matchup-logos`, `player-portrait`, and `team-logo` with graceful fallbacks and plain-text rendering for unlinked legs, and regenerated [web/static/app.css](../../../web/static/app.css).
- Added handler/render tests `TestPickBoardRendersLogosAndPortraits` in [internal/web/pick_test.go](../../../internal/web/pick_test.go) and `TestSlipPanelRendersLegLogosAndPortraits` in [internal/web/slips_test.go](../../../internal/web/slips_test.go).

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 4

- Templates can invoke the helpers via `dict`, e.g. `{{template "team-logo" (dict "URL" .AwayLogo "Fallback" .AwayAbbr "Size" "h-5 w-5")}}`, `{{template "player-portrait" (dict "URL" .HeadshotURL "Initials" .Initials "Size" "h-8 w-8")}}`, and `{{template "matchup-logos" (dict "AwayURL" .AwayLogo "HomeURL" .HomeLogo)}}`.
- `data-fallback-badge` uses `hidden` when `URL` is present and `flex` when `URL` is empty (instead of combining `hidden` and `flex` in the same class list, since Tailwind v4 places `.inline-flex` after `.hidden` in `app.css`).

### Task 5

- Next tasks can wire `GamePlayerPropsView` (`internal/web/games.go`) and research views (`GameResearchView`, `PlayerResearchView` in `internal/web/research.go`) using the same `media.html` helpers and `Initials` helper.

## Files touched

- `internal/web/pick.go`
- `internal/web/pick_test.go`
- `internal/web/render.go`
- `internal/web/render_test.go`
- `internal/web/slips.go`
- `internal/web/slips_test.go`
- `web/static/app.css`
- `web/static/app.js`
- `web/templates/media.html`
- `web/templates/partials.html`

## Run log

- Mode: 
- Checkpoints: task-4, task-5
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
