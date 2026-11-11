# Phase 1 handoff: Replay a coherent Sunday morning

## Done

`internal/sandbox` now holds the fail-closed replay path for the sandbox and the named week 9 morning registry.

- `transport.go` defines `Transport`, an `http.RoundTripper` that answers only requests present in a preloaded
  `Registry`. Matching uses stage, uppercase method, lowercase host, escaped path, and sorted query values. Credential
  parameters (`apiKey`, `token`, `access_token`, in any case or `-`/`_` spelling) are dropped before matching and from
  error text. A miss returns `ErrNoResponse` naming the method, credential-free URL, and stage. A nil request or URL is
  an error. There is no base transport and no fallback, so the package cannot reach a provider. Each call gets a fresh
  `io.NopCloser` over the saved bytes, and `Stage`/`SetStage` are guarded, so the transport is safe for the concurrent
  refresh sources.
- `registry.go` defines `Stage`, the `Morning` stage, `Captured`/`Derived` provenance, `Fixture`, `Registry`,
  `NewRegistry`, and `Week9`. `NewRegistry` validates every fixture (name, stage, method, URL, file, provenance,
  derived-note, status) and reads every file up front, so a missing body, duplicate request, or incomplete fixture fails
  construction. It fills a content type from the file extension when the header omits one, returns copied headers, and
  registers 15 morning responses from an `fs.FS` rooted at the repository.
- The morning set covers the full scheduled week 9 slate (ESPN scoreboard, 14 Sunday/Monday games), the Parlay API event
  list, the 3-credit game-line board, nfelo projections, all 10 outdoor forecasts needed by the 48-hour refresh window,
  and the featured game's lazy prop board.
- Parlay API responses carry saved `X-Requests-Remaining`, `X-Requests-Used`, and `X-Requests-Last` headers. Remaining is
  620 after events, 617 after lines, and 614 after the featured props, all above `refresh.OddsLinesFloor` and
  `refresh.OddsPropsFloor`.
- `sandbox_test.go` drives the real `espn`, `odds`, `weather`, `nfelo`, and `refresh` constants through the replay client:
  it asserts the parsed slate, event matching (including the featured event ID), every matched price, all 10 forecasts,
  week 9 projections for every game, the featured prop markets, the saved credit balances, and that an unknown week, an
  unregistered The Odds API host, an unknown stage, a missing fixture file, and a duplicate request all fail closed.
- `docs/reference/sandbox-replay.md` documents the transport, the request table, provenance rules, credit headers, how to
  add a response, and the test coverage. `docs/llms.txt` indexes it. `internal/ingest/weather/testdata/README.md` lists
  the nine new derived forecasts and explains why they exist.

Verification run in this worktree: `go test -short -race ./internal/sandbox/ ./internal/ingest/... ./internal/refresh/`,
`golangci-lint run ./internal/sandbox/...` (0 issues), `good-vibes docs lint` (0 errors; 11 pre-existing stale-source
warnings in unrelated docs), and `good-vibes check local-sunday-sandbox` (passes).

## Deviations

- The design said the registry would reference existing source fixtures. Only Lumen Field had a captured week 9
  forecast, so nine new Open-Meteo-shaped snapshots now live under `internal/ingest/weather/testdata/` for the remaining
  outdoor kickoffs. Each is marked `Derived` with a note, and the directory README says so. The phase task explicitly
  allowed "any missing morning forecasts", so this fills the gap without pretending they are captures.
- The event list and line board are real The Odds API captures, not Parlay API captures. The two providers use the same
  wire shape, so the files are registered at `odds.DefaultBaseURL` (Parlay API's host) and their notes record the true
  origin. This keeps one client shape and one set of parsers, but the source host in the table is the replayed host, not
  the capture host.
- The featured prop board is a hand-built fixture. It is registered as `Derived` with a note and is the only props
  response in the morning set, matching the lazy-load rule.
- `Fixture` supports a per-fixture `Status` and `Header`, but the transport does not simulate transport-level failures,
  redirects, or rate-limit status codes. Nothing in the morning path needs them; a stage that wants a failure response
  can register a non-200 status.

## Open issues

- Only the `morning` stage is populated. `Transport.SetStage` and the per-fixture `Stage` field are ready for phase 4's
  `early-loss`, `all-clear`, and `sunday-complete` snapshots, but those fixtures do not exist yet, and there are no
  same-week live or final ESPN captures in the registry. A later stage therefore fails closed rather than showing stale
  morning data, which is the intended behavior until phase 4 adds the files.
- `Registry` is immutable after construction and rejects a second response for the same stage/method/host/path/query. A
  later stage can reuse the same URL with a different body by giving it a different `Stage`; it must not register two
  bodies for one stage.
- `weatherURL` and `week9PropMarkets` in `registry.go` duplicate request details owned by the weather and odds clients
  (hourly fields, units, window, prop market list). If either client changes its request, the registry must change in
  the same commit or the replay stops matching. The tests are the tripwire.
- The replay transport must stay the `*http.Client` transport in phase 2. Wrapping it in `archive.Transport` would put
  replayed calls into `api_responses` and break the "no archive writes" rule; the test suite does not yet assert that
  because the server wiring lands next.
- Saved credit headers are static per response. A second identical line or props call in one process sees the same
  balance rather than a decremented one. Both balances stay above the floors, so the normal guards still run, but a
  repeated-call test would see non-monotonic credits.
- The registry loads fixture paths relative to the repository root. Phase 2 must pass an `fs.FS` rooted at the repo (or
  embed the fixtures) before startup; a built binary run outside the checkout will fail to preload.
- The morning clock used by the tests is `2026-11-08T13:00:00Z`. Any scenario clock phase 2 picks must keep all ten
  outdoor kickoffs after `Now()` and inside the 48-hour weather lookahead, or the refresh will request forecasts the
  registry does not have.

## Next-phase notes

- Phase 2 should construct one `odds.NewClient("", syntheticKey, replayClient)`-style client with a non-empty key and no
  fallback base URL, plus the ordinary ESPN, weather, and nfelo clients over the same replay `*http.Client`. Phase 1
  proves those constructors need no changes.
- Call `sandbox.Week9(os.DirFS(repoRoot))` once at startup, before any ingest. Construction reads all bodies, so a broken
  fixture fails boot rather than a refresh.
- `refresh.Service.RunOnce` can fan out the same way `Kick` does; the transport is concurrency-safe and returns a fresh
  body per call. Only the synchronous wrapper and clock seams remain.
- When phase 4 adds live/final snapshots, put them in new stages and keep `morning` untouched. That preserves phase 1's
  tests and makes reset-to-morning the guaranteed baseline.
- Keep `docs/reference/sandbox-replay.md` as the single table of registered requests. Adding a stage means adding rows
  and a provenance note, then rerunning `go test ./internal/sandbox`.

## Files touched

- `docs/llms.txt`
- `docs/reference/sandbox-replay.md`
- `internal/ingest/weather/testdata/README.md`
- `internal/ingest/weather/testdata/forecast_arrowhead.json`
- `internal/ingest/weather/testdata/forecast_bank_of_america.json`
- `internal/ingest/weather/testdata/forecast_gillette.json`
- `internal/ingest/weather/testdata/forecast_hard_rock.json`
- `internal/ingest/weather/testdata/forecast_levis.json`
- `internal/ingest/weather/testdata/forecast_lincoln_financial.json`
- `internal/ingest/weather/testdata/forecast_northwest.json`
- `internal/ingest/weather/testdata/forecast_sofi.json`
- `internal/ingest/weather/testdata/forecast_soldier.json`
- `internal/sandbox/registry.go`
- `internal/sandbox/sandbox_test.go`
- `internal/sandbox/transport.go`

## Run log

- Mode: 
- Checkpoints: task-1
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
