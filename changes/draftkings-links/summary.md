---
slug: draftkings-links
run: 20261109-185300-draftkings-links
outcome: done
generated: 2026-11-09T19:15:16Z
---

# Run summary: draftkings-links

## Outcome

- Status: done
- Branch: draftkings-links
- Final verdict: agy@claude-sonnet-5-5-high approved (issues: 0)

## Totals

- Tasks: 5 of 5
- Phases: 2
- Checkpoints: 6
- Duration: 20m53s
- Cost: $0.87
- Input tokens: 10655226
- Output tokens: 109357
- Cache read tokens: 8363058

## Phases

### Phase 1: Add The Odds API prop endpoint and prop merging

Tasks:

- [x] Add `TheOddsAPIBaseURL` to `internal/ingest/odds/client.go`, add `EventSid` to `odds.Prop`, and implement `Client.EventProps(ctx, eventID)` in `internal/ingest/odds/props.go` to fetch and parse The Odds API's nested `/events/{eventID}/odds` prop response, with unit tests against `testdata/odds_prop_links.json` and edge cases in `internal/ingest/odds/props_test.go`
- [x] Add `MergeProps(primary, secondary []Prop) []Prop` in `internal/ingest/odds/props.go` to backfill missing deep links (`Link`, `Sid`, `EventSid`) and missing selections while keeping newly linked lines consistent, with unit tests in `internal/ingest/odds/props_test.go`

Phase totals: 2 checkpoints, 4m33s, $0.24, 1509582 input / 26614 output / 831820 cache tokens

Checkpoints: task-1, task-2 Review fixes: 0 Integration checks: 0

#### Done

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

### Phase 2: Wire dual-provider game-line enrichment, fallback, and config

Tasks:

- [x] Add `FallbackLines`, `TheOddsAPISource` (`"odds:theoddsapi"`), `TheOddsAPIEnrichFloor` (`100`), and `TheOddsAPIFallbackFloor` (`25`) to `internal/refresh`, and update `syncOdds` (`internal/refresh/sources.go`) with `mergeOdds` to enrich Parlay API lines with The Odds API deep links and missing quotes, fall back when primary fails or pauses, and degrade gracefully, with unit tests in `internal/refresh/refresh_test.go`
- [x] Add `TheOddsAPIKey` (`THE_ODDS_API_KEY`) to `internal/config/config.go`, wire `FallbackLines` in `cmd/server/main.go`, add `the-odds-api-key` to `deploy/setup.sh` and `.github/workflows/deploy.yml`, and add config tests in `internal/config/config_test.go`
- [x] Docs: update `docs/reference/data-sources.md`, `docs/concepts/data-refresh.md`, `docs/concepts/export.md`, `docs/reference/config.md`, `docs/guides/gcp-setup.md`, and `plan/data-sources.md` for The Odds API enrichment and fallback, `THE_ODDS_API_KEY`, `odds:theoddsapi` credit tracking, tiered credit floors, and DraftKings deep-link export

Phase totals: 4 checkpoints, 16m20s, $0.63, 9145644 input / 82743 output / 7531238 cache tokens

Checkpoints: task-3, task-4, task-5, final Review fixes: 0 Integration checks: 0

#### Done

- `internal/refresh` now has `TheOddsAPISource` (`odds:theoddsapi`), `TheOddsAPIEnrichFloor` (`100`),
  `TheOddsAPIFallbackFloor` (`25`), and `Deps.FallbackLines` / `Service.fallbackLines`.
- `fetchOdds` runs one provider's path: free `/events`, record the reported balance, match its events to the week's
  games, buy `/odds` only above that provider's floor, and convert each line to `db.Odds` with the provider's own event
  map. A nil client or `odds.ErrNoAPIKey` is "provider disabled", not a failure.
- `syncOdds` runs Parlay API with `OddsLinesFloor` (`50`) and The Odds API under `archive.WithKey` with the enrichment
  floor (`100`) when the primary bought lines, otherwise the fallback floor (`25`). It records the primary balance
  under `odds:lines` and The Odds API balance under `odds:theoddsapi`.
- Error handling: one failed provider is logged as a warning and the other provider's rows are committed; if every
  configured provider fails, `syncOdds` returns the joined error so `odds:lines` backs off; if none is configured it
  returns `odds.ErrNoAPIKey`.
- Event matching stays per provider. `games.odds_event_id` takes the primary's match; a fallback-only match lands only
  when the game has no primary match and no stored event ID, so a transient primary outage does not churn it.
- `mergeOdds` keys rows by `(GameID, Book, MarketType, Team, PlayerName, Side)`. It backfills `EventSid` per
  `(GameID, Book)`, adopts a secondary link, sid, and event sid onto an unlinked primary row, and adopts the secondary
  line and price when `db.SameLine` is false. A linked primary row keeps its line, price, and link; its sid backfills
  only on a matching line. Secondary-only book or market rows are appended.
- `config.Config.TheOddsAPIKey` reads `THE_ODDS_API_KEY`. `cmd/server/main.go` passes
  `odds.NewClient(odds.TheOddsAPIBaseURL, cfg.TheOddsAPIKey, ingestClient)` as `FallbackLines`.
- `deploy/setup.sh` creates `the-odds-api-key` and names it in the post-setup instructions. The deploy workflow mounts
  `THE_ODDS_API_KEY=the-odds-api-key:latest` beside `ODDS_API_KEY`.
- Docs now describe the enrichment/fallback flow, both credit keys and floors, `THE_ODDS_API_KEY`, the `odds:theoddsapi`
  archive label, DraftKings deep-link export, and the five Cloud Run secrets. `docs/llms.txt` was regenerated.
- Tests cover merge precedence, the enrichment path, per-provider archive keys, the 100-credit enrichment floor, the
  25-credit fallback floor, one-sided provider failure (disabled and 503 in both directions), every-provider failure, and
  every-provider-disabled. Config tests cover the new key in dev and prod.
- `go test ./...`, `go vet` on the touched packages, `good-vibes docs lint` (0 errors), and
  `good-vibes check draftkings-links` pass.

## Final gate

- Rounds used: 1; verify failures: 0
- Final verify: no commands configured
- Open issues: none
