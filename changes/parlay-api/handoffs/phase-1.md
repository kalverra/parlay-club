# Phase 1 handoff: Switch odds client to Parlay API and add bulk props ingest

## Done

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

## Deviations

- The design named four living docs. `docs/concepts/archive.md` and `docs/guides/gcp-setup.md` also named The Odds API,
  the 500-credit budget, or only `the-odds-api.com` in the host list, so both were updated to match the new provider.
- Stale provider names in comments that this change made wrong were corrected without behavior changes:
  `odds/live_test.go`, `odds/lines.go`, `refresh/refresh.go`, `refresh/sources.go`, `db/sync.go`, and
  `export/export.go`.
- `parseProps` returns a nil slice when no rows survive. The design did not say nil versus empty, and no caller exists
  yet.
- The fixture is hand-built from the documented `/props` shape rather than a captured response, because the local run
  had no Parlay API key. `testdata/README.md` says so. The request shape is still asserted exactly by `propsQuery`.

## Open issues

- `Props` is not wired into `internal/refresh`; the `LinesClient` interface is still `Events` + `Lines`. Nothing writes
  prop rows to SQLite yet. That belongs to `props-research`, which also owns the `odds:props:{game}` sync key and the
  150-credit floor.
- Parlay API currently populates validated `link` / `sid` on FanDuel only. DraftKings legs will store NULL links and
  appear as manual `no link` entries until the provider adds DraftKings metadata. No schema or export change is needed.
- `mapTeamMarket` still returns an error for a spread outcome with an empty name. Only unpriced spreads, point-less
  spreads, and non-team moneylines are skipped. No observed Parlay API row has hit this, but a future one-sided feed
  could. Leaving it until a response shows it avoids guessing at a fallback.
- The live test (`TestLiveEvents`) has not been run against the real endpoint in this worktree; it skips without
  `ODDS_API_KEY`. Its only change is a comment.

## Next-phase notes

- Use `Client.Props` directly; do not add a second props client. Keep the flat `Prop` rows as the seam and let the
  refresh layer assign the game and write the `odds` rows.
- Expect the props fixture to be your shape reference: `player` may arrive as `player_name`, market keys may be aliases,
  and `period` may be `UNKNOWN` or a half/quarter even when the request asked for full-game markets.
- Any TD market row carries `line: 0.5` on the wire. Keep storing it as nil line with side `YES`, as `Prop` already
  does, so the slip UI's `ANYTIME_TD` path keeps working.
- Props and game lines share the same credit headers and archive source; the trust boundary is already covered by
  `Client.get` and `archive.Transport`.

## Files touched

- `docs/concepts/data-refresh.md`
- `docs/concepts/export.md`
- `docs/llms.txt`
- `docs/reference/config.md`
- `docs/reference/data-sources.md`
- `internal/ingest/archive/labels.go`
- `internal/ingest/archive/labels_internal_test.go`
- `internal/ingest/archive/transport_test.go`
- `internal/ingest/odds/client.go`
- `internal/ingest/odds/events.go`
- `internal/ingest/odds/events_test.go`
- `internal/ingest/odds/lines.go`
- `internal/ingest/odds/lines_internal_test.go`
- `internal/ingest/odds/lines_test.go`
- `internal/ingest/odds/props.go`
- `internal/ingest/odds/props_test.go`
- `internal/ingest/odds/testdata/README.md`
- `internal/ingest/odds/testdata/props_sample.json`
- `plan/data-sources.md`

## Run log

- Mode: 
- Checkpoints: task-1, task-2, task-3, task-4
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
