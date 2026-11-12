---
slug: logos-and-portraits
run: 20261112-193639-logos-and-portraits
outcome: done
generated: 2026-11-12T20:02:05Z
---

# Run summary: logos-and-portraits

## Outcome

- Status: done
- Branch: logos-and-portraits
- Final verdict: agy@gemini-4-hydrogen-high approved (issues: 0)

## Totals

- Tasks: 7 of 7
- Phases: 3
- Checkpoints: 7
- Duration: 23m56s
- Cost: $—
- Fresh input tokens: 1024014
- Cache read tokens: 16843820
- Cache write tokens: 0
- Output tokens: 161202

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-medium | 18029036 | $— |

## Phases

### Phase 1: Ingest, store, and wire logo and portrait metadata

Tasks:

- [x] Parse `team.logo` in `espn.Scoreboard`, `headshot.href` in `espn.Roster`, and injury `team.logo`/`athlete.headshot.href` in `espn.ParseInjuries` (normalizing to `https://` only), with fixture tests covering present and missing URLs.
- [x] Add `0010_logos_portraits.sql` (`games.home_logo`, `games.away_logo`, `roster_players.headshot_url`), update `db.Game`, `db.RosterPlayer`, `UpsertGame` idempotency, `RosterPlayersByIDs`/`TeamLogos`, `refresh` syncs, and `board` (`GameCard`, `Row`, `PlayerGroup`) with tests.
- [x] Docs: update `docs/reference/data-sources.md` and `docs/reference/schema.md` for the new logo and headshot fields and migration `0010`.

Phase totals: 3 checkpoints, 9m51s, $—, 6716238 input / 63950 output / 6277036 cache read / 0 cache write tokens

Checkpoints: task-1, task-2, task-3 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Parse `team.logo` in `espn.Scoreboard`, `headshot.href` in `espn.Roster`, and injury `team.logo`/`athlete.headshot.href` in `espn.ParseInjuries` (normalizing to `https://` only), with fixture tests covering present and missing URLs.

- Added `normalizeHTTPSURL` in `internal/ingest/espn/client.go` to trim whitespace and accept only `https://` URLs with a non-empty host.
- Extended `espn.Team` (`Logo`), `espn.RosterPlayer` (`HeadshotURL`), `espn.TeamInjuries` (`LogoURL`), and `espn.Injury` (`HeadshotURL`) and wired parsing from `team.logo`, `headshot.href`, and injury `team.logo`/`athlete.headshot.href`.
- Updated `scoreboard_test.go`, `roster_test.go`, and `summary_test.go` to verify real fixtures (`scoreboard_week.json`, `scoreboard_live.json`, `roster.json`, `roster_falcons.json`, `summary.json`) as well as missing and invalid/non-HTTPS URLs.

### Task 2: Add `0010_logos_portraits.sql` (`games.home_logo`, `games.away_logo`, `roster_players.headshot_url`), update `db.Game`, `db.RosterPlayer`, `UpsertGame` idempotency, `RosterPlayersByIDs`/`TeamLogos`, `refresh` syncs, and `board` (`GameCard`, `Row`, `PlayerGroup`) with tests.

- Added `internal/db/migrations/0011_logos_portraits.sql` adding `games.home_logo`, `games.away_logo`, and `roster_players.headshot_url` (with `NOT NULL DEFAULT ''`), and updated schema version assertions in `migrate_test.go` and `migrate_internal_test.go`.
- Updated `db.Game` (`HomeLogo`, `AwayLogo`), `gameColumns`, `scanGame`, and `UpsertGame` (including the `WHERE ... IS NOT ...` dirty check), and added `Store.TeamLogos` in `internal/db/games.go`.
- Updated `db.RosterPlayer` (`HeadshotURL`), `rosterColumns`, `scanRosterPlayer`, and `UpsertRosterPlayers`, and added `Store.RosterPlayersByIDs` in `internal/db/players.go`.
- Wired `HomeLogo`/`AwayLogo` in `internal/refresh/sources.go` (`slateGame`) and `HeadshotURL` in `internal/refresh/research.go` (`syncRoster`).
- Added `AwayLogo`/`HomeLogo` to `board.GameCard` and `HeadshotURL` to `board.Row` and `board.PlayerGroup` via `matchPlayer` in `internal/board/board.go`, with tests across `internal/db`, `internal/refresh`, and `internal/board`.

### Task 3: Docs: update `docs/reference/data-sources.md` and `docs/reference/schema.md` for the new logo and headshot fields and migration `0010`.

- Updated `docs/reference/data-sources.md` (`last_verified: 2026-11-12`) to document `team.logo` in ESPN Scoreboard, `LogoURL` and `HeadshotURL` in ESPN Summary injuries, `HeadshotURL` (`headshot.href`) in ESPN Roster, and `https://`-only URL normalization.
- Updated `docs/reference/schema.md` (`last_verified: 2026-11-12`) to document `0011_logos_portraits.sql`, the `games.home_logo` / `games.away_logo` and `roster_players.headshot_url` DDL columns, and their conventions.

### Phase 2: Render logos and portraits on the pick board and slip hub

Tasks:

- [x] Add shared `media.html` template helpers (`team-logo`, `player-portrait`, `matchup-logos`) wired in `internal/web/render.go`, plus the capturing `error` fallback in `web/static/app.js` and regenerated `web/static/app.css`.
- [x] Render team logos and player portraits on `pick-slate` cards, `pick-detail` headers and `PickPlayerView` cards, and `slip-panel` legs (`LegView`) with graceful fallbacks for missing or unlinked legs, with handler/render tests.

Phase totals: 2 checkpoints, 8m20s, $—, 5575798 input / 48626 output / 5283392 cache read / 0 cache write tokens

Checkpoints: task-4, task-5 Review fixes: 0 Integration checks: 0

#### Done

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

### Phase 3: Render logos and portraits on research and standalone props

Tasks:

- [x] Render team logos and player portraits on `research-game` (matchup header, injury team headers, injury rows, matched players), `research-player` (header portrait and team logo), and `game-props` player headers, with handler/render tests.
- [x] Docs: update `docs/concepts/slips.md` to describe team logos and player portraits across the pick board, slip legs, and research views.

Phase totals: 2 checkpoints, 5m45s, $—, 5575798 input / 48626 output / 5283392 cache read / 0 cache write tokens

Checkpoints: task-6, task-7 Review fixes: 0 Integration checks: 0

#### Done

### Task 6: Render team logos and player portraits on `research-game` (matchup header, injury team headers, injury rows, matched players), `research-player` (header portrait and team logo), and `game-props` player headers, with handler/render tests.

- Wired `HeadshotURL` and `Initials` onto `GamePlayerPropsView` in [internal/web/games.go](../../../internal/web/games.go) and rendered `player-portrait` in `game-props` player headers in [web/templates/partials.html](../../../web/templates/partials.html).
- Wired `HomeLogo`/`AwayLogo` onto `GameResearchView`, `LogoURL` onto `TeamInjuriesView` (falling back to game team logos when missing from `injuries_json`), and `HeadshotURL`/`Initials` onto `InjuryView` and `PropMatchView` in [internal/web/research.go](../../../internal/web/research.go).
- Wired `HeadshotURL`, `Initials`, and `TeamLogo` (via `Store.TeamLogos`) onto `PlayerResearchView` in [internal/web/research.go](../../../internal/web/research.go), and rendered matchup logos, injury team/player media, matched player portraits, and player research header portrait/logo in [web/templates/partials.html](../../../web/templates/partials.html).
- Regenerated Tailwind CSS (`go generate ./web`) and added handler/render tests in [internal/web/games_test.go](../../../internal/web/games_test.go) and [internal/web/research_test.go](../../../internal/web/research_test.go).

### Task 7: Docs: update `docs/concepts/slips.md` to describe team logos and player portraits across the pick board, slip legs, and research views.

- Updated [docs/concepts/slips.md](../../../docs/concepts/slips.md) to document team logos and player portraits across slip legs, the pick board (slate cards, detail header, player prop cards, and standalone `GET /games/{id}/props`), and research views (`research-game` and `research-player`), including `app.js` image fallback behavior.
- Added `web/templates/media.html` to the `sources` front matter of [docs/concepts/slips.md](../../../docs/concepts/slips.md) and regenerated `docs/llms.txt` with `good-vibes docs index`.
- Verified formatting and doc checks with `rumdl check docs/concepts/slips.md`, `good-vibes docs lint`, and `good-vibes check logos-and-portraits` (0 errors).

## Final gate

- Rounds used: 0; verify failures: 0
- Final verify: no commands configured
- Open issues: none
