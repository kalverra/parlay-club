---
slug: parlay-api
run: 20261109-182237-parlay-api
outcome: done
generated: 2026-11-09T18:34:01Z
---

# Run summary: parlay-api

## Outcome

- Status: done
- Branch: parlay-api
- Final verdict: agy@gemini-4-argon-low approved (issues: 0)

## Totals

- Tasks: 4 of 4
- Phases: 1
- Checkpoints: 4
- Duration: 9m39s
- Cost: $0.37
- Input tokens: 5952684
- Output tokens: 66375
- Cache read tokens: 5072230

## Phases

### Phase 1: Switch odds client to Parlay API and add bulk props ingest

Tasks:

- [x] Update `DefaultBaseURL` in `internal/ingest/odds/client.go` to `https://parlay-api.com/v1/sports/americanfootball_nfl`, update `sourceForHost` in `internal/ingest/archive/labels.go` to recognize `parlay-api.com`, and harden `Client.Events` and `Client.Lines` (`internal/ingest/odds/events.go`, `internal/ingest/odds/lines.go`) to skip null `commence_time`, null `price`/`point` outcomes, and non-team `h2h` outcomes
- [x] Add `Prop` and `Client.Props(ctx, eventID)` in `internal/ingest/odds/props.go` with `/props` query building, `period == "FULL"` filtering, market alias mapping for all 9 prop markets, `ANYTIME_TD` (`YES`, nil line) and `TEAM_TOTAL` (`Team` set) normalization, and `over_link`/`under_link`/`over_sid`/`under_sid` extraction
- [x] Add `internal/ingest/odds/testdata/props_sample.json` and unit tests in `internal/ingest/odds/props_test.go`, `events_test.go`, `lines_test.go`, and `internal/ingest/archive/transport_test.go` covering Parlay API edge cases and host labelling
- [x] Docs: update `docs/reference/data-sources.md`, `docs/concepts/data-refresh.md`, `docs/concepts/export.md`, `docs/reference/config.md`, `plan/data-sources.md`, and `internal/ingest/odds/testdata/README.md` for Parlay API endpoints, 1,000-credit budget, 3-credit bulk `/props`, and FanDuel deep-link coverage

Phase totals: 4 checkpoints, 9m39s, $0.37, 5952684 input / 66375 output / 5072230 cache tokens

Checkpoints: task-1, task-2, task-3, task-4 Review fixes: 0 Integration checks: 0

#### Done

- `odds.DefaultBaseURL` now points at `https://parlay-api.com/v1/sports/americanfootball_nfl`. `ODDS_API_KEY` and
  `ODDS_BASE_URL` keep their names, so deployment secrets and local overrides need no change.
- `archive.sourceForHost` maps both `parlay-api.com` and `the-odds-api.com` to `SourceOdds`. An unlabelled Parlay API
  call archives under `odds`, keeps `apiKey=REDACTED`, and records `X-Requests-Remaining` / `X-Requests-Last`.
- `Client.Events` skips a wire event whose `commence_time` is null, missing, or blank instead of failing the whole
  list. A present but malformed timestamp is still an error.
- `parseLines` skips an outcome with a null or missing `price`, a spread or total without a `point`, and an `h2h`
  outcome whose name is neither team (such as a 3-way `Draw`). Valid bookmakers on the same event still produce rows,
  and FanDuel `link` / `sid` / `event_sid` survive unchanged.
- `internal/ingest/odds/props.go` adds `Prop` and `Client.Props(ctx, eventID)`. It calls `/props` with `eventId`,
  the nine canonical markets, `oddsFormat=american`, `includeLinks=true`, `includeSids=true`, and `limit=10000`, and
  rejects an empty event ID before sending.
- `parseProps` filters by requested event, accepts only `period` empty or `FULL`, reads `player` with a
  `player_name` fallback, and maps every canonical key plus the documented aliases. It expands priced sides into
  `OVER` / `UNDER` rows, normalizes `ANYTIME_TD` to one `Side: "YES"` row with a nil line, sets `Team` on
  `TEAM_TOTAL` and `PlayerName` on player props, and copies `over_link` / `under_link` / `over_sid` / `under_sid`.
  Unpriced sides, unknown markets, and rows with no subject or book are skipped.
- `props_sample.json` plus `props_test.go` cover the flat fixture: FanDuel links and sids, alias keys, a `1H` row, an
  `UNKNOWN` row, an unpriced DFS row, `player_anytime_td`, `team_totals`, and a row for another event. `events_test.go`
  and `lines_test.go` cover the new skips; `labels_internal_test.go` and `transport_test.go` cover host labelling,
  redaction, and credits.
- Living docs and the data plan now describe Parlay API, the 1,000-credit month, the 3-credit bulk `/props` call, and
  FanDuel-only deep links: `docs/reference/data-sources.md`, `docs/concepts/data-refresh.md`,
  `docs/concepts/export.md`, `docs/reference/config.md`, `docs/llms.txt`, and `plan/data-sources.md`.
- `go test ./...`, `golangci-lint run ./...`, `good-vibes docs lint`, and `good-vibes check parlay-api` pass.
  `good-vibes check` still lists ten unrelated docs-stale warnings that predate this change.

## Final gate

- Rounds used: 0; verify failures: 0
- Final verify: no commands configured
- Open issues: none
