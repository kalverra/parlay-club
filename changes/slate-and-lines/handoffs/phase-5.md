# Phase 5: Games tab

## Done

- `/games` is now its own handler instead of a shell placeholder. It reads `LatestWeek`, `GamesForWeek`, and each game's
  `OddsForGame` rows, then renders one card per game in kickoff order.
- Each card shows the matchup (`MIA @ BUF`, plus full names), the kickoff as a `<time datetime="{UTC ISO}">` that
  `app.js` formats in the browser's zone, a status chip (Upcoming, Live, Final), the score for a live or final game, and
  the two spreads and the over and under.
- Prices and lines come from the viewer's `HomeBook` when that book prices the selection, and from the median across
  every book otherwise. Medians use `parlay.ReferencePrice` for prices and a sorted median for lines.
- Weather badges: 🏠 Indoor for a dome, 💨 wind mph (amber from 15, red from 20), and 🌧 at 0.1 in/hr of precipitation or
  more. An indoor game or a game with no forecast shows no weather.
- Odds rows from an older sync are dropped per game, so a card never mixes a stale price with the current board. This
  resolves the stale-odds open issue from phase 4; no prune was added.
- The "lines as of" note uses the newest current odds row on the board, or the `odds:lines` `sync_state.fetched_at` when
  that is newer. A board with neither says "No lines yet this week."
- Quick-add pills for both spreads, both moneylines, Over, Under, and Custom leg. A pill posts the normal
  `POST /slips/{id}/legs` route with `quick=1` and the game, market, team, side, line, and price prefilled, swaps an
  "Added {selection}" confirmation into that card, and keeps `Hx-Trigger: leg-saved`. Custom leg opens the bottom sheet
  with the game preselected and targets the same card.
- Quick add targets the newest slip of the week and shows the pills only while it is `OPEN`. With no slip the card links
  to `/slips/new`; with a locked slip it says picks cannot be added.
- The leg form gained a game picker. A current week lists only games that have not kicked off and requires a game; a past
  week lists every game and allows "No game". `?game_id=` preselects a game, and `?quick=1` makes the form target the
  game card. A kicked-off pick still answers 422 from the existing kickoff lock.
- The slip payout now shows **Reference** odds next to Combined. It is the product of each counting leg's reference
  price: one price per slip member's home book for the same game, market, player, side, and line, reduced with
  `parlay.ReferencePrice`. A leg no member book prices keeps its own price.
- Docs: `docs/reference/routes.md` documents the Games route, the stale-row filter, the note, quick add, and the new leg
  form parameters; `docs/concepts/slips.md` covers quick add, the game picker, reference odds, and the Games tab;
  `plan/product.md` no longer carries the Games tab or "lines as of" bullets; `docs/guides/local-dev.md` and
  `docs/llms.txt` follow.
- Tests: `internal/web/games_test.go` covers a full card, live and final cards, stale-row filtering, the home-book
  choice, all six quick-add markets, the custom-leg target, and a moneyline-only board. `internal/web/slips_test.go`
  covers the picker (upcoming games only, past-week history), the kickoff 422, and reference odds across two member
  books. `internal/web/layout_test.go` renders the Games page through the shell.
- Verification: `go test ./...`, `go test -count=1 ./...`, `go test -short -race ./internal/web/ ./internal/slips/`,
  `go vet ./...`, and `golangci-lint run ./...` pass. `good-vibes docs lint` and `good-vibes check` pass.

## Deviations

- The card summary shows spreads and totals; moneylines appear in the pills and on the slip. This matches the product
  spec ("spread and total") and keeps the collapsed card short on a phone.
- Server-rendered time text is the UTC fallback ("Mon 3:04 PM UTC"); the `<time datetime>` element is what makes it
  local. The game picker is a `<select>`, so its option labels cannot hold a `<time>` element and stay in UTC. The
  server never guesses a zone, per the product rule.
- A current week with no games available (none synced, or every game kicked off) makes the game picker optional, so a
  hand-entered leg is still possible. When games exist, the form marks the picker required.
- Quick add only uses the newest slip of the week, the same slip the home page opens. The design did not name which slip.
- Slip reference odds take the median across every member's home book, deduplicated by book, not "the viewer's home book
  else median". The slip belongs to the group, so one member's book cannot be the reference for everyone. The docs spell
  this out.
- The "lines as of" note is the max of the board's current rows and `odds:lines.fetched_at`, not just the game rows.
  This keeps the note honest when a game has no lines but the source was fetched.
- During close, a moneyline-only board was setting `HasLines` false and rendering "No lines yet." next to its moneyline
  pills. `cardView` now counts moneyline pills as lines, with a test.

## Open issues

- Odds rows are still never pruned. The Games tab and slip reference odds filter to the newest `FetchedAt` per game, so
  old rows cannot show, but the table grows without bound.
- The stale-row filter assumes one sync stamps every row for a game with the same `fetched_at`. A partial write with a
  newer timestamp would hide the rows that did not get it.
- The web handler enforces the game requirement only through the form's `required` attribute. `slips.Service.AddLeg`
  still accepts a nil game on a current week, so a crafted POST can create one. The pre-existing rules doc already says
  a leg needs a game outside a past week; the service is the place to make that true.
- The picker filters on kickoff alone. A postponed game whose kickoff moved stays visible on its card but drops out of the
  picker until the next scoreboard sync updates it. That is the same data path, so it should be brief.
- The board runs one `OddsForGame` query per game, 14 on a full slate. That is fine for one SQLite instance; batch it if
  the board ever spans more games.
- Card and picker tests use the real `time.Now()`. The kickoff cases use games 48 hours out and 1–3 hours back, so they
  are stable, but there is still no injectable clock in the web layer.

## Next-phase notes

- Props should extend the same card. `GameLineView` carries `Market`, `Team`, `Side`, `Line`, and `Odds`, and the card's
  Custom leg already opens the form with `game_id` and `quick=1`. Prop pills must carry a player name and should not go
  through `rowsFor`: it deliberately ignores rows with a `PlayerName`, which is what keeps game lines and props apart on
  a card.
- Reuse `currentOdds` for any new odds-backed view so a later sync cannot leak old prices. If a second source starts
  writing odds, make the cutoff per source key instead of "newest row on the game".
- `referenceOdds` in `internal/web/slips.go` reads `SlipMembers` and `OddsForGame` and reuses `currentOdds` and
  `candidatePrices`. The record book can reuse the same helpers if it needs group-reference prices.
- The Games tab needs no `Kick` call and no SSE. `Kick` is in the full-page middleware, and `refresh.LiveWindow` already
  owns the live deadline.
- `plan/architecture.md` lists only the routes that are still to come. New Games behavior belongs in
  `docs/reference/routes.md`, not that table.
- The row filter from this phase settles the phase-4 "filter by the latest sync time" option; a prune is still the
  cleaner long-term fix if `odds` writes become frequent.

## Files touched

- `docs/concepts/slips.md`
- `docs/guides/local-dev.md`
- `docs/llms.txt`
- `docs/reference/routes.md`
- `internal/web/games.go`
- `internal/web/games_test.go`
- `internal/web/layout_test.go`
- `internal/web/render.go`
- `internal/web/routes.go`
- `internal/web/slips.go`
- `internal/web/slips_test.go`
- `plan/architecture.md`
- `plan/product.md`
- `web/templates/games.html`
- `web/templates/partials.html`

## Run log

- Mode: 
- Checkpoints: task-13, task-14, task-15
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
