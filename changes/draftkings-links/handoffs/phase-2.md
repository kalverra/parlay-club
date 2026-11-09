# Phase 2 handoff: Wire dual-provider game-line enrichment, fallback, and config

## Done

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

## Deviations

- The design had `writeOdds` holding `primaryMatches` and applying the fallback-only guard. The implementation builds one
  merged match map in `syncOdds` and `writeOdds` writes it unchanged, so `writeOdds` stays provider-agnostic. Behavior is
  the same; the design's approach and risk wording were updated to match.
- The design sketched the two-provider sequence inline. A shared `fetchOdds` helper carries one provider's
  events-to-lines path, so both providers share the credit-floor, event-matching, and disabled handling; `syncOdds` only
  chooses each provider's floor and combines the results.
- `odds:theoddsapi` is recorded with `RecordSync` on each successful events and lines response, so its `fetched_at`
  advances even when the floor skips the paid call. The design only required storing the balance. This keeps the
  archive and sync table consistent and does not make it a scheduled source.
- A failed balance write makes `fetchOdds` count that provider as failed with the write error rather than returning
  partial rows. The design did not cover a failing `sync_state` write; failing closed keeps a stale balance from gating
  the next paid call.
- `syncOdds` publishes the primary's remaining balance with `odds:lines` (nil when the primary did not buy lines). The
  fallback balance always lives under `odds:theoddsapi`, never overwriting the primary's credit count.

## Open issues

- `games.odds_event_id` prefers the primary whenever it has a match. Only fallback-only games keep a fallback ID, and
  only when no ID existed. If Parlay API were permanently retired, existing primary IDs would stay until the game's
  rows change; nothing recomputes them today.
- While the primary is disabled or below its floor, `odds:lines` holds the primary's last-known (possibly nil) balance.
  The fallback's count is only under `odds:theoddsapi`.
- Props remain unwired. `Client.EventProps` and `MergeProps` from phase 1 still have no caller. The nine-market request
  string has never been validated against a live `THE_ODDS_API_KEY`; one rejected market fails the whole call with a
  4xx.
- The fallback client is built even when `THE_ODDS_API_KEY` is empty. Calls short-circuit on `ErrNoAPIKey`, but startup
  gives no warning that enrichment is off.
- `good-vibes docs lint` reports eight stale-doc warnings for docs outside this change. They existed before this phase
  and do not fail the lint.

## Next-phase notes

- Phase 2 is the last phase in `design.md`. Next is the close-out with the close skill; a person then reviews it and runs
  `good-vibes done draftkings-links`.
- Before closing, run one live sync with both keys set and confirm: `sync_state` gains `odds:theoddsapi`, DraftKings rows
  gain `link`, `sid`, and `event_sid`, and a locked slip with two DraftKings legs on one game renders the combined
  `/event/{eventSid}?outcomes=` URL through `export.Plan`.
- Keep `MergeProps` per event. When the props sync lands in a later change, mirror `mergeOdds` precedence: adopt the
  linked line and price when attaching a link, keep a linked primary line, and backfill a sid only on a matching line.

## Files touched

- `.github/workflows/deploy.yml`
- `cmd/server/main.go`
- `deploy/setup.sh`
- `docs/concepts/archive.md`
- `docs/concepts/data-refresh.md`
- `docs/concepts/export.md`
- `docs/guides/gcp-setup.md`
- `docs/llms.txt`
- `docs/reference/config.md`
- `docs/reference/data-sources.md`
- `docs/reference/schema.md`
- `docs/runbooks/google-oauth-setup.md`
- `internal/config/config.go`
- `internal/config/config_test.go`
- `internal/refresh/refresh.go`
- `internal/refresh/refresh_test.go`
- `internal/refresh/sources.go`
- `plan/data-sources.md`

## Run log

- Mode: 
- Checkpoints: task-3, task-4, task-5
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
