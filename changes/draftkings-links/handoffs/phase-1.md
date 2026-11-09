# Phase 1 handoff: Add The Odds API prop endpoint and prop merging

## Done

- `odds.TheOddsAPIBaseURL` is the The Odds API NFL root. `Events` and `Lines` parse its wire format unchanged.
- `outcomeResponse` gained `Description`, the field The Odds API uses to name the player or team a prop prices. Game
  lines leave it empty.
- `odds.Prop` gained `EventSid`, the bookmaker's event ID (`bookmakers[].sid`) needed for combined DraftKings links.
- `Client.EventProps(ctx, eventID)` requests `GET /events/{eventID}/odds` with `regions=us`, the nine prop markets,
  `oddsFormat=american`, `includeLinks=true`, and `includeSids=true`. It decodes the single nested `eventResponse`,
  maps market keys through `propMarketAliases`, and emits one `Prop` per priced outcome:
  - `ANYTIME_TD` keeps only `name == "Yes"`, sets `SideYES`, and leaves `Line` nil even when the wire sends a point.
  - `TEAM_TOTAL` sets `Team` from `description`; every other prop sets `PlayerName`.
  - Over/Under outcomes map through `totalSide`; unknown markets, unknown sides, empty subjects, missing prices, and
    missing points on non-TD markets are skipped.
  - Every row carries the book's `EventSid`, plus the outcome's `Link` and `Sid` when present.
  - A blank event ID returns an error before any request is sent.
- `MergeProps(primary, secondary)` keys rows by `(Book, MarketType, Team, PlayerName, Side)` and works per event:
  - Records each book's non-empty `EventSid` from `secondary` and backfills it onto every primary row for that book.
  - Appends selections present only in `secondary`.
  - A primary row with no link adopts `secondary`'s link, sid, and event sid; if the lines differ it also adopts the
    linked line and price so the stored number matches the deep link.
  - A primary row that already has a link keeps its line, price, and link. It backfills `Sid` only when the lines
    match, and backfills `EventSid` when empty.
  - One-sided boards return a copy, so the result never aliases the input slices.
- `props_test.go` covers the saved nested fixture (48 rows, DraftKings event sid `34118128`, FanDuel `35603243`), TD and
  team-total normalization, skip cases, blank event ID, a 503, line adoption (`268.5` -> `278.5`), sid backfill rules,
  secondary-only rows, and input-copy behavior.
- `go test ./...` and `golangci-lint run ./internal/ingest/odds/...` pass.

## Deviations

- The design named `EventSid` on `Prop` but not the wire change. `eventResponse`'s shared `outcomeResponse` also needed
  `Description` so nested prop outcomes can name their subject. That is the only extra struct field.
- The design's verification said the saved The Odds API fixture exercises `ANYTIME_TD` and `TEAM_TOTAL`. The fixture
  only contains `player_pass_yds`, so those normalizations are covered by a synthetic response instead. Behavior matches
  the design; only the fixture's coverage differs.
- `EventProps` builds the path from the existing `eventsPath` / `oddsPath` constants and escapes the event ID, rather
  than hardcoding `/events/{id}/odds`. Same URL for normal IDs.
- `parseEventProps` returns a nil slice when no rows survive. The design did not choose nil versus empty; this mirrors
  the existing `parseProps`.

## Open issues

- `EventProps` is not called by `internal/refresh` or anything else yet. Prop rows are still not written to SQLite;
  this phase only supplies the client seam and merge helper.
- The `propsMarkets` request string is shared with Parlay API. Only `player_pass_yds` appears in the saved The Odds API
  fixture. The other eight keys are assumed valid; if The Odds API rejects any one, the whole per-event call fails with
  a 4xx. A live check with `THE_ODDS_API_KEY` should confirm the full market list.
- `MergeProps` propagates `EventSid` by book only, so it assumes every row belongs to one event. Callers must merge one
  event's boards at a time and must not combine boards from different games.

## Next-phase notes

- `TheOddsAPIBaseURL` and `EventProps` are ready. Phase 2's game-line work can reuse the same client and base URL with
  no further ingest changes.
- Mirror the `MergeProps` precedence in `mergeOdds`: adopt the secondary line and price when attaching a link to an
  unlinked row, keep a linked primary line/price, and backfill `Sid` only when the lines match. The two merge functions
  should tell the same story for odds rows and prop rows.
- `MergeProps` is per event; do not reuse one call across a week's games.
- The test helpers `fixtureServer` and `apiKeyQuery` live in `internal/ingest/odds/client_test.go` and can serve any new
  fixture-based test.

## Files touched

- `internal/ingest/odds/client.go`
- `internal/ingest/odds/events.go`
- `internal/ingest/odds/props.go`
- `internal/ingest/odds/props_test.go`

## Run log

- Mode: 
- Checkpoints: task-1, task-2
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
