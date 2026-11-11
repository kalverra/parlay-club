# Phase 2 handoff: Build the pick board

## Done

The pick board is now the place picks happen, and the tray finishes the round trip from the hub.

- **`internal/board` (`board.go`).** `Build` turns one week's stored odds into one row per selection. It uses the
  viewer's home-book line and price when that book carries the pick, the median across books otherwise, fills in the
  player ID from the roster, labels the row ("Josh Allen Over 245.5 Pass Yds"), maps it to a category (lines, team
  totals, passing, rushing, receiving, TDs, kicking) and an Eastern-time window (Sunday morning before noon, early
  before 4, late before 7, night, Monday), and leaves out games that kicked off. `Search` folds accents, lowercases,
  and strips punctuation, matches row text, player, team, and matchup, filters by category, window, and game, applies
  one of four sorts (kickoff, player, category, odds shortest first), counts started games, and names games whose props
  were never fetched. A picked row carries the viewer's leg ID. `IsPropRow` moved here so the board and the props
  partial share one definition.
- **`Store.OddsForWeek` (`internal/db/odds.go`).** One ordered read of every price for a week's games, so a board
  request does not query per game.
- **Pick routes (`internal/web/pick.go`, `internal/web/slips.go`).**
  - `GET /slips/{id}/pick` renders the full page with a back link, no bottom bar, the slip's `data-live-url`, and the
    tray in a right column at `lg`.
  - `GET /slips/{id}/pick/results` answers the search, chip, sort, and one-game requests. The search input debounces at
    250 ms; chips and sorts are links that swap the same partial and push the full page URL. Chips only appear for
    categories and windows with rows.
  - `GET /slips/{id}/pick/swap` renders the "Swap which pick?" sheet from the viewer's unlocked legs.
  - `GET /slips/{id}/tray` renders the tray partial.
  - `?game=` shows one game and pairs both sides of each line in one row, with a link back to the whole week.
- **Add and remove.** An unpicked row posts `POST /slips/{id}/legs` with `quick=1&from=board`; a picked row sends
  `DELETE /legs/{id}?from=board`. Both answer with the row and fire `Hx-Trigger: tray-updated`, so the tray refreshes
  without a reload. A row at the limit offers "swap in" instead of add.
- **"Load them".** A game whose props were never fetched appears as a prompt when the search names its team, matchup,
  or a roster player on it. The button calls `GET /games/{id}/props?slip=`, which runs `SyncGameProps` for that one game
  and answers an empty 200 with `Hx-Trigger: board-updated`; the results partial then re-runs the current search.
  Opening, typing, filtering, sorting, and the tray call `SyncGameProps` zero times.
- **Hub and old Games route.** The hub's pick action points at `/slips/{id}/pick`. `GET /games` is a 303 to the active
  group's week (or `/` when no group is active). `GET /games/{id}/props` still renders the standalone prop partial when
  no `?slip=` is given, so old game and Research links keep working.
- **Tray.** The viewer's legs with remove buttons (or "Locked" once the game kicks off), progress ("1 of 2"), the
  analyzer warnings that name one of the viewer's legs, and one action: "Done" back to the hub, or "Review and lock in"
  when `NextStep` says `lock_in`. It refreshes on `tray-updated` and `slip-updated`.
- **Replace at the limit.** The swap sheet lists the viewer's legs that are not kickoff-locked; a choice sends
  `PUT /legs/{old}` with `from=board` and the new pick's fields. `UpdateLeg` enforces ownership, lock in, and kickoff on
  both the old and the new game, writes one "changed" activity entry, and the handler answers with the refreshed
  results partial and `tray-updated`. A kickoff-locked old leg and a locked-in member are still refused. When every leg
  has kicked off, the sheet says "Every one of your picks has kicked off."
- **One props builder.** `games.go` now keeps only the standalone props rendering; it groups `board.Build` rows into
  team totals and per-player pills, so the board and the props partial cannot drift.
- **`currentOdds` fix.** A week read exposed that the old helper kept only the newest prop timestamp across every game,
  so fetching one game's props later hid another game's props (and made the board offer "Load them" for a game whose
  props were stored). It now keeps the newest fetch per game and sync group. The old single-game callers behave the
  same.
- **Tests.** `internal/board/board_test.go` covers every category and window, the Monday-night UTC-Tuesday kickoff,
  each sort, accent-insensitive search, started games hidden and counted, the home-book price over the median, an
  unloaded game named from its roster, and picked rows carrying the leg ID. `internal/web/pick_test.go` covers the
  page, search/filter/sort, two games keeping their own props, the unloaded prompt and one `SyncGameProps` tap, zero
  syncs on open/search/filter/sort/tray, add and remove round trips, swap at the limit, kickoff and lock-in refusals,
  the tray warning filter, the one-game pairing, the tray's `Review and lock in`, and the `/games` redirect.
  `internal/web/current_odds_test.go` pins the per-game behavior. The sandbox test now asserts the redirect and that the
  board hides kicked-off games at the scenario clock.
- **Docs and client.** `slips.md` rewrote the Games tab as the pick board, the tray, and replace; `routes.md` added the
  board, results, swap, tray, redirect, and `?slip=` rows; `data-refresh.md` explains "Load them" and that the board
  spends no credits; `analyzer.md` describes the tray's filtered heads-ups. `app.js` opens and closes the swap sheet and
  refreshes the board on `board-updated`; the CSS was rebuilt for the board layout.

## Deviations

- **`currentOdds` had to change, not just move.** The design called for one `OddsForWeek` read filtered through the
  existing `currentOdds`. That helper assumed one game's rows, so across a week it kept only the most recently fetched
  game's props. The board now filters per game and sync group; the Approach section says so.
- **The design's helper names were stale.** `cardView` and `pickGameLine` no longer exist. Their replacement is
  `board.Build` plus `groupProps`/`gameLineView` for the standalone props partial. The Approach section is updated.
- **`board.Query` grew `Now`.** The design's query struct held only the search fields. `Search` needs an injected clock
  for the sandbox, past weeks, and deterministic tests, so `Query.Now` and `Build`'s `now` argument carry it. A zero
  `Now` falls back to `time.Now`.
- **`?slip=` is additive, not a replacement.** The design said `/games/{id}/props` takes `?slip=` instead of guessing
  the slip. It does, but the route also keeps its old no-`?slip=` partial so Research and old quick-add links still
  work; it never guesses a slip.
- **"Review and lock in" returns to the hub.** The tray's second action reads `NextStep`'s `lock_in` step but links to
  the hub, where the banner owns the one lock action. It does not post the lock itself, so a person can review the
  slip first.
- **The props partial is shared, but the old Games card is gone.** The design said the card helpers move behind
  `board.Build`. The board itself renders rows; only the standalone props partial keeps a small grouping helper.

## Open issues

- **Phase 3 must clear votes on replace.** Swapping at the limit goes through `UpdateLeg`. The design's vote rules say
  a changed pick loses its votes while an odds-only edit keeps them; that clearing must land in the same `UpdateLeg`
  transaction or a swapped-in pick will inherit the old pick's votes.
- **Chip counts do not follow the search.** `categoryChips` and `windowChips` count the whole week's rows, not the rows
  the current search matched, and an active chip stays visible at zero. That matches "chips with rows" but not a count
  that follows the query. If phase 4 wants the latter, change those two helpers.
- **Past-week tray actions.** `trayFrom` treats a past week as not kickoff-locked, so its remove buttons render even
  though the service refuses the write because every game kicked off. The board itself is empty for a past week. The
  phase 4 empty-state pass can decide whether the tray should say picks are closed instead.
- **The old Games template is dead.** `web/templates/games.html`, the `games` entry in `render.go`, and the
  `render_test.go` page list still exist but nothing routes to them. They are harmless; a later docs/template cleanup
  can delete them.
- **The board reads the whole week's odds per request.** It is one query plus in-memory grouping, but a full week with
  many props is the place to watch if latency shows. The design's cache-per-(week, fetch time) idea is still open.

## Next-phase notes

- Phase 3 adds votes on the hub. `slip-updated` already refreshes the tray and the hub panel; the pick results listen
  only to `board-updated`, so votes alone will not re-run a board search.
- The board's swap path posts `from=board` plus the current `q`, `cat`, `win`, `sort`, and `game`; `updateLeg` rebuilds
  the results with `pickQuery` from that form. Keep that contract when `UpdateLeg` changes for votes.
- `board.Build` and `board.Search` are pure and take the app clock; reuse them for phase 4's "no lines this week" and
  "member missing at the deadline" screens instead of adding a second row builder.
- `/games` now redirects, so the two-persona sandbox walk should start at the group list, open the hub, and use
  `/slips/{id}/pick`. `cmd/server/sandbox_test.go` already asserts the redirect and the kicked-off count.
- The phase 1 open issue about `loadLeg` is closed: it resolves the slip through `loadSlip`, so a leg in another of the
  viewer's groups switches the active group instead of 404ing, and the replace route inherits that.

## Files touched

- `cmd/server/sandbox_test.go`
- `docs/concepts/analyzer.md`
- `docs/concepts/data-refresh.md`
- `docs/concepts/slips.md`
- `docs/llms.txt`
- `docs/reference/routes.md`
- `internal/board/board.go`
- `internal/board/board_test.go`
- `internal/db/odds.go`
- `internal/web/games.go`
- `internal/web/games_test.go`
- `internal/web/home_test.go`
- `internal/web/layout_test.go`
- `internal/web/next_test.go`
- `internal/web/pick.go`
- `internal/web/pick_test.go`
- `internal/web/render.go`
- `internal/web/render_test.go`
- `internal/web/routes_test.go`
- `internal/web/slips.go`
- `web/static/app.css`
- `web/static/app.js`
- `web/templates/partials.html`
- `web/templates/pick.html`

## Run log

- Mode: 
- Checkpoints: task-3, task-4
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
