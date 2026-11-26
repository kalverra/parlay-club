# Task 3 handoff

## Done

- Redesigned `GET /research` into a dual-mode explorer: one debounced search box, game/player target
  pickers, and phone-first chip rows for bet type, grade, signals, and sort.
- Added `GET /research/explore` as the HTMX partial and set `Hx-Push-Url` to the matching `/research?...`
  URL, so filters and targets survive refreshes and shared links.
- Rendered candidate bet cards from `board.Explore` over cached week odds, with headshot/team media,
  selection text, grade, odds plus implied %, the top reason, signal pills, matchup, kickoff, and links
  into the player or game sheet.
- Added the explicit `POST /research/games/{id}/props` button, shown only when a targeted game has cached
  lines but no props, and made it the only research path that calls `SyncGameProps`.
- Added integration tests for cached browsing, chip filtering and odds sorting, `Hx-Push-Url`, the props
  button, preserved `?game_id=`/`?player_id=`/`?player=` views, and zero refresher calls on every research
  GET.

## Deviations

- The HTMX swap target is `#research-explore` (search form, chip bars, and cards together) rather than the
  design's `#research-results`. One response keeps facet counts, current-chip states, and the card list in
  sync. The design prose was corrected.
- Research game and player sheets no longer call `SyncGameResearch`/`SyncPlayerResearch`. The design's
  zero-credit guardrail and the task's zero-refresher requirement win over the old auto-sync, so the sheets
  read cached SQLite only. `SyncGameProps` is now the sole refresher call in the research handlers.
- Grade `C+` was undefined in the design. It filters to A, B, and C rows ("C or better").
- The candidate cards link to the existing player/game sheets for now. Their tap target becomes
  `/research/bet` when Phase 3 adds that route.
- Registered the new `research-explore` partial in `internal/web/render.go`; the task did not name that file
  but `RenderPartial` only serves registered names.

## Open issues

- After an HTMX search or chip change, an already-open sheet below the explorer is not replaced, so it can
  describe the previously targeted game or player. Clicking the new "Open game sheet" or "Open player
  sheet" link reloads the full page correctly. Phase 3 may want an out-of-band sheet reset.
- The player picker still renders every rostered athlete for the week. `hx-preserve` avoids re-rendering
  the control, but each partial response still carries the full option list.
- `board.Explore`'s text search matches team abbreviations, not full team names. The game picker covers
  that case.
- The targeted game sheet still writes auto-resolved player matches through `UpsertPlayerMatches`. That is
  an existing SQLite cache write, not an external API call.
- The Task 2 note about five bottom tabs crowding at 375px still applies.

## Next-phase notes

- `ResearchCardView` now carries `MarketType`, `Side`, `Team`, `Line`, and `OddsValue` alongside the display
  fields, so `/research/bet` links can be built from a card without re-reading `board.Row`.
- Build per-bet links with `db.SameSelection` identity and keep the explorer's `q`, `cat`, `grade`,
  `signal`, and `sort` in a `from` URL so the back link restores the list.
- Keep `GET /research/bet` read-only: the zero-refresher tests use a recording refresher and will catch any
  accidental `Sync*` call.
- The explicit props button's form posts the current explorer filters, so a refreshed partial comes back on
  the same view.

## Commit

Add research explorer with cached bet search

Browsing the week's slate previously required an open slip and a single-game drilldown. The explorer makes
targeted and open search fast while reading only cached SQLite odds, and the explicit props button keeps
paid fetches under user control.
