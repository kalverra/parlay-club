# Phase 4: Prop quick-add, player autocomplete, and Research tab

## Done

- Expanded game card prop quick-add (`GET /games/{id}/props`):
  - Fetches game prop odds on demand via `SyncGameProps` using HTMX `hx-trigger="toggle once"`.
  - Displays quick-add pills for team totals (`TEAM_TOTAL`) and player props (passing, rushing, receiving, anytime TD, kicker points) grouped by player.
  - Links game cards to `/research?game_id={id}` and player prop headers to `/research?player_id={espnID}`.
  - Quick-add button posts `POST /slips/{id}/legs` with `quick=1` and selection metadata prefilled, returning the "Added {selection}" confirmation.
- Leg form player autocomplete and leg resolution:
  - Populates player `<datalist>` (`WeekPlayerNames`) on `GET /slips/{id}/legs/new` and `GET /legs/{id}/edit` with all distinct athlete names across the week's team rosters and prop lines.
  - In `legFromForm`, automatically resolves `player_id` (ESPN athlete ID) and `team` via `ResolvePlayerMatch` when a player market leg is submitted.
  - Adds "Research &rarr;" link to leg rows on the slip view when a leg has an associated player ID or game ID.
- Research tab and partials in `internal/web/research.go` and `web/templates/research.html`:
  - `GET /research`: Full page with week game dropdown and player autocomplete selector, supporting deep links via `?game_id=`, `?player_id=`, `?leg_id=`, `?market=`, and `?line=`. Supports HTMX partial delegation targeting `#game-research` or `#player-research`.
  - `GET /research/games/{id}`: On-demand sync of game injuries, rosters, and props (`SyncGameResearch`). Renders key injuries per team filtered to positions (`QB`, `RB`, `WR`, `TE`, `LT`, `OT`, `C`, `CB`, `S`), sportsbook prop player matching status, and manual match resolution form.
  - `GET /research/players/{espnID}`: On-demand sync of player gamelog (`SyncPlayerResearch`). Renders last-5 hit bars (green over line, red under), L5 median and season mean callouts, season games played, and interactive market switcher tabs.
  - `POST /research/matches`: Saves manual match (`manual = 1`) via `SaveManualPlayerMatch`, clears conflicting non-manual matches for that game, backfills odds and leg rows, and re-renders the updated game research partial.
- Navigation and responsive layout:
  - Added Research tab to the mobile bottom navigation bar and desktop top navigation.
  - At `lg` width, responsive layout links and displays research panels cleanly beside the slip.
- Tests:
  - Handler tests in `internal/web/research_test.go` (`TestResearchPage`, `TestResearchGameHandler`, `TestResearchPlayerHandler`, `TestResearchMatchesHandler`, `TestResearchLinksOnSlipLegs`).
  - Game props handler test in `internal/web/games_test.go` (`TestGamePropsHandler`).
  - Autocomplete datalist test in `internal/web/slips_test.go` (`TestLegFormPlayerAutocompleteDatalist`).
  - Layout test in `internal/web/layout_test.go` covering navigation links.
- Documentation:
  - Updated `docs/concepts/slips.md` with prop quick-add, autocomplete, and research deep links.
  - Updated `docs/reference/routes.md` documenting `GET /games/{id}/props`, `GET /research`, `GET /research/games/{id}`, `GET /research/players/{espnID}`, and `POST /research/matches`.
  - Regenerated `docs/llms.txt`.
  - Pruned completed research tab and route entries from `plan/product.md` and `plan/architecture.md`.

## Deviations

- None.

## Open issues

- None.

## Next-phase notes

- All phases of the `props-research` change are complete.
- The next step is to run the close skill to write `changes/props-research/closeout.md` and prepare for `good-vibes done`.

## Files touched

- `cmd/server/main.go`
- `docs/concepts/slips.md`
- `docs/llms.txt`
- `docs/reference/routes.md`
- `internal/db/odds.go`
- `internal/db/players.go`
- `internal/db/players_test.go`
- `internal/web/games.go`
- `internal/web/games_test.go`
- `internal/web/layout_test.go`
- `internal/web/render.go`
- `internal/web/research.go`
- `internal/web/research_test.go`
- `internal/web/routes.go`
- `internal/web/slips.go`
- `internal/web/slips_test.go`
- `plan/architecture.md`
- `plan/product.md`
- `web/templates/games.html`
- `web/templates/partials.html`
- `web/templates/research.html`

## Run log

- Mode: 
- Checkpoints: task-7, task-8, task-9
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
