# Phase 3: Phase 3: Render logos and portraits on research and standalone props

## Done

### Task 6: Render team logos and player portraits on `research-game` (matchup header, injury team headers, injury rows, matched players), `research-player` (header portrait and team logo), and `game-props` player headers, with handler/render tests.

- Wired `HeadshotURL` and `Initials` onto `GamePlayerPropsView` in [internal/web/games.go](../../../internal/web/games.go) and rendered `player-portrait` in `game-props` player headers in [web/templates/partials.html](../../../web/templates/partials.html).
- Wired `HomeLogo`/`AwayLogo` onto `GameResearchView`, `LogoURL` onto `TeamInjuriesView` (falling back to game team logos when missing from `injuries_json`), and `HeadshotURL`/`Initials` onto `InjuryView` and `PropMatchView` in [internal/web/research.go](../../../internal/web/research.go).
- Wired `HeadshotURL`, `Initials`, and `TeamLogo` (via `Store.TeamLogos`) onto `PlayerResearchView` in [internal/web/research.go](../../../internal/web/research.go), and rendered matchup logos, injury team/player media, matched player portraits, and player research header portrait/logo in [web/templates/partials.html](../../../web/templates/partials.html).
- Regenerated Tailwind CSS (`go generate ./web`) and added handler/render tests in [internal/web/games_test.go](../../../internal/web/games_test.go) and [internal/web/research_test.go](../../../internal/web/research_test.go).

### Task 7: Docs: update `docs/concepts/slips.md` to describe team logos and player portraits across the pick board, slip legs, and research views.

- Updated [docs/concepts/slips.md](../../../docs/concepts/slips.md) to document team logos and player portraits across slip legs, the pick board (slate cards, detail header, player prop cards, and standalone `GET /games/{id}/props`), and research views (`research-game` and `research-player`), including `app.js` image fallback behavior.
- Added `web/templates/media.html` to the `sources` front matter of [docs/concepts/slips.md](../../../docs/concepts/slips.md) and regenerated `docs/llms.txt` with `good-vibes docs index`.
- Verified formatting and doc checks with `rumdl check docs/concepts/slips.md`, `good-vibes docs lint`, and `good-vibes check logos-and-portraits` (0 errors).

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 6

- All research and standalone props views now use the `media.html` helpers (`team-logo`, `player-portrait`, `matchup-logos`).
- Next task can verify `app.js` fallback behavior, phone-width layout, and living documentation updates.

## Files touched

- `docs/concepts/slips.md`
- `internal/web/games.go`
- `internal/web/games_test.go`
- `internal/web/research.go`
- `internal/web/research_test.go`
- `web/static/app.css`
- `web/templates/partials.html`

## Run log

- Mode: 
- Checkpoints: task-6, task-7
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
