---
slug: local-sunday-sandbox
run: 20261111-015148-local-sunday-sandbox
outcome: done
generated: 2026-11-11T02:51:24Z
---

# Run summary: local-sunday-sandbox

## Outcome

- Status: done
- Branch: local-sunday-sandbox
- Final verdict: claude@claude-opus-5-5 approved (issues: 0)

## Totals

- Tasks: 5 of 5
- Phases: 4
- Checkpoints: 6
- Duration: 56m33s
- Cost: $1.79
- Input tokens: 70469889
- Output tokens: 338272
- Cache read tokens: 67183782

## Phases

### Phase 1: Replay a coherent Sunday morning

Tasks:

- [x] Add the fail-closed replay transport and named week 9 response registry, including credit headers and any missing morning forecasts; drive real ingest clients through the scheduled slate, lines, weather, projections, and featured props, with tests for successful parsing and unknown/missing responses. Document captured versus derived provenance.

Phase totals: 1 checkpoints, 7m55s, $0.33, 6688709 input / 70134 output / 5985014 cache tokens

Checkpoints: task-1 Review fixes: 0 Integration checks: 0

#### Done

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

### Phase 2: Launch the isolated offline app

Tasks:

- [x] Add synchronous refresh and scenario-clock seams through refresh, slips, and web, then wire `SUNDAY_SANDBOX=1`, the mise convenience task, fixed isolated database, forced dev auth, synchronous morning bootstrap, and stage persistence. Test no-network startup, production rejection, normal dev preservation, and restart behavior; update local development, configuration, persistence, and data-refresh docs.

Phase totals: 1 checkpoints, 13m20s, $0.32, 20291557 input / 65803 output / 19887140 cache tokens

Checkpoints: task-2 Review fixes: 0 Integration checks: 0

#### Done

The sandbox now boots as the normal server with a replay transport, one scenario clock, and an isolated database.

- `internal/refresh` has `RunOnce(ctx) error`, the synchronous counterpart to `Kick`. It runs the same scoreboard-first
  pass, waits for every source it starts, and joins failures. `Kick` and `KickContext` stay asynchronous. Both paths now
  share `refreshSync`, so due checks, single-flight, and failure backoff behave the same way.
- `internal/sandbox/scenario.go` adds `Scenario`, a mutex-guarded stage and time with `Now`, `Stage`, `State`, and `Set`,
  plus `MorningTime()` (`2026-11-08T13:00:00Z`). `internal/sandbox/state.go` adds `StateStore` over the isolated
  database: `Ensure` creates the one-row `sandbox_state` table, `Load` reports `ErrStateNotFound` on first boot, and
  `Save` upserts the row. No production migration knows about the table.
- `cmd/server/sandbox.go` wires the sandbox: `sandbox.Week9` reads every fixture up front, one `*http.Client` carries the
  fail-closed transport into the ordinary ESPN, weather, Parlay API, and nfelo clients, and the Parlay client gets a
  synthetic key with no fallback provider. `newSandboxRuntime` bootstraps a fresh database with `RunOnce`, then saves the
  `morning` row. A later boot restores the saved stage and clock and does not rebuild or refetch. `authSettings` forces
  the dev sign-in form and never constructs Google while sandbox mode is on. `repoRoot` walks up from the working
  directory to the checkout so the fixture `fs.FS` works under Air and `mise run dev`.
- `cmd/server/main.go` branches on `cfg.Sandbox`: sandbox skips history import and the archived live clients, and passes
  `runtime.scenario.Now` to `slips.New` and `web.Dependencies.Now`. Normal dev keeps the existing clients, history
  import, and default wall clock.
- `internal/config` parses `SUNDAY_SANDBOX` as a boolean, rejects it in prod, and replaces `DB_PATH` with
  `./data/sunday-sandbox.db` whenever it is on.
- `internal/web` adds optional `Dependencies.Now` with a `time.Now` default. The game picker, export card, and nfelo
  freshness check read it, so they agree with refresh and slips. Auth cookies and SSE timers stay on real time.
- `mise.toml` adds `mise run sandbox`, which runs `SUNDAY_SANDBOX=1 ./scripts/dev.sh`.
- Tests cover offline bootstrap of the full slate, no archive writes, the replay transport on every client, restart
  restore without a refetch, forced dev auth with real-looking Google values, config path/prod rules, `RunOnce` waiting
  and joining failures, the three web clock reads, and a sentinel normal-dev file that sandbox startup never touches.
- Docs updated: `docs/guides/local-dev.md`, `docs/reference/config.md`, `docs/concepts/persistence.md`,
  `docs/concepts/data-refresh.md`, and `docs/reference/sandbox-replay.md`; `docs/llms.txt` indexes the reference.

Verified in this worktree: `go build ./...`, `go test -short -race ./...`, `golangci-lint run ./...` (0 issues),
`good-vibes docs lint` (0 errors; 9 pre-existing stale-source warnings in unrelated docs), and
`good-vibes check local-sunday-sandbox` (passes).

### Phase 3: Add repeatable people and picks

Tasks:

- [x] Implement transactional sandbox reset and real-service seeding of shared personas, an outsider, an editable slip, and named legs. Add the reset controls with post-commit notifications, then test repeatability, normal-database isolation, cleanup of cache/backoff state, and current-session recovery.
- [x] Add the development-only controls page and cookie-backed persona switching through real sign-in and group scope. Test member perspectives, activity ownership, outsider isolation and invite joining, CSRF/auth guards, and phone layout; update local development, access, and route docs.

Phase totals: 2 checkpoints, 21m33s, $0.73, 28789404 input / 133372 output / 27421902 cache tokens

Checkpoints: task-3, task-4 Review fixes: 0 Integration checks: 0

#### Done

The sandbox now has one repeatable reset, real seeded people, a real slip with named picks, and a development-only
controls page that switches personas through the normal session and group cookies.

- `internal/sandbox/reset.go` adds `Clear`, one transaction that deletes every disposable domain, cache, backoff, and
  archive row children-first. It keeps the schema, the migration version, and the sandbox-only `sandbox_state` row.
- `internal/sandbox/seed.go` builds the people, group, slip, and legs through the same code a person uses:
  `auth.SignIn` with `dev:{email}` subjects, `Create` and `JoinByToken` for the shared group, `CreateSlip`, and
  `slips.Service.AddLeg`. Three shared personas (Ada, Ben, Cleo) get two picks each; Dex stays in their own group as the
  outsider. `PersonaID` and `LegID` keep stages and tests off reset-created numeric IDs. `PersonaGroup` reads live
  membership, so it works after a restart that did not rebuild the seed.
- `cmd/server/sandbox.go` is now the sandbox controller the phase-2 handoff asked for. `sandboxRuntime` owns the
  refresher, slip service, scenario clock, replay stage, state row, cookies, renderer, and the last seed. One mutex
  serializes `Reset` and `Kick`; a page-load pass that begins first finishes before a reset, so its writes cannot land
  after the fresh morning.
- Reset sets the morning clock and replay stage, saves the stage row, clears the database, calls `ClearBackoff`, runs
  `RunOnce`, creates the people and shared membership, creates the slip and picks, syncs the featured prop board, then
  publishes `week:<id>` and `slip:<id>` after every write commits. A first boot still saves the stage only after a
  successful build, so a failed first run retries.
- `MountRoutes` adds `GET /dev/sandbox`, `POST /dev/sandbox/reset`, and `POST /dev/sandbox/persona` inside the ordinary
  `RequireUser`, group scope, and HTMX CSRF stack. `main.go` passes `runtime.Kick` into `newMux` and mounts the
  routes only when `cfg.Sandbox` is set.
- The controls page shows the active stage, scenario time, signed-in persona, active group, all four personas with
  their groups, the shared invite, and the reset button. It is phone-first: one column at phone width, two at `sm`, and
  full-width actions. The persona switch calls `auth.SignIn` and sets both real cookies; it does not inject a user or a
  permission. Reset reissues the same browser's session and group when the signed-in email matches a seeded persona;
  otherwise it clears the stale cookies and sends the browser to `/login`.
- `internal/refresh` exports `ClearBackoff`, and reset calls it after clearing `sync_state` so an old failure pause
  cannot skip the fresh pass.
- The renderer knows the `sandbox` page, and `web/static/app.css` was regenerated with the new responsive classes.
- Tests cover repeatability across two resets (people, groups, memberships, slip, legs, results, activity, games, sync
  state), a sentinel normal-dev file that sandbox startup and resets never touch, failure-backoff cleanup, post-commit
  week and slip signals, current-session recovery after reset, member perspectives on the shared slip, actor
  attribution on activity and grading, outsider 404s plus forged group cookies, the real invite join, CSRF and auth
  guards, unknown personas, phone-width layout, and control-route absence outside sandbox.
- Docs updated: `docs/guides/local-dev.md`, `docs/concepts/access.md`, `docs/concepts/persistence.md`,
  `docs/reference/routes.md`, `docs/reference/sandbox-replay.md`, and `docs/llms.txt`.

Verified in this worktree: `go build ./...`, `go test -short -race -count=1 ./...`, `golangci-lint run ./...`
(0 issues), `good-vibes docs lint` (0 errors; 8 pre-existing stale-source warnings in unrelated docs), and
`good-vibes check local-sunday-sandbox` (passes).

### Phase 4: Stage Sunday outcomes

Tasks:

- [x] Add coherent same-week live/final responses and deterministic early-loss, all-clear, and Sunday-complete controls using real refresh, locking, and manual grading. Test direct and sequential selection, scores/statuses, slip results, remaining pending legs, reset, SSE refetch signals, transition races, and production route absence; update local development, route, and live-update docs.

Phase totals: 2 checkpoints, 13m44s, $0.41, 14700219 input / 68963 output / 13889726 cache tokens

Checkpoints: task-5, final Review fixes: 0 Integration checks: 0

#### Done

The sandbox now has four selectable situations: `morning`, `early-loss`, `all-clear`, and `sunday-complete`. Each one is
built through the real refresh and slip services, and every stage starts by rebuilding the morning baseline, so any
button works from any prior state.

- `internal/sandbox/scenario.go` names the stages, fixes their clocks, validates saved names, and gives the runtime one
  `Scenario` value for stage and time.
- `internal/sandbox/plan.go` turns each stage into a `StagePlan`: its clock, whether the shared slip locks, and the named
  scripted manual grades. Early loss grades Ada's Bengals moneyline `LOST`. All clear and Sunday complete grade all six
  seeded picks `WON`. Morning locks and grades nothing.
- `internal/sandbox/registry.go` registers one same-week ESPN scoreboard per stage (`scoreboard_week9_live.json`,
  `scoreboard_week9_final_sunday.json`, `scoreboard_week9_complete.json`) and reuses the morning lines, projections,
  props, and forecasts under each stage. The registry now holds 60 fixtures: 15 morning responses plus 15 per later
  stage. All three stage scoreboards are marked `Derived` with notes.
- `internal/sandbox/state.go` rejects an unknown stage on `Load` and `Save`, so a stale or hand-edited state row cannot
  restore a stage the registry does not know.
- `cmd/server/sandbox.go` adds `Stage`, which runs under the existing reset mutex. It rebuilds morning, sets the scenario
  clock and replay stage, saves the stage row, runs `RunOnce`, then locks every shared member through
  `slips.Service.LockIn` and records each manifest grade through `SetLegResult`. It never writes result columns or infers
  an outcome from a score. `Reset` is now `Stage(morning)`, and both paths publish the week and slip signals after every
  write commits.
- `POST /dev/sandbox/stage` mounts only in sandbox mode, accepts the four names, answers `HX-Refresh` with reissued real
  session and group cookies, and answers 422 without touching rows, clock, or cookies for an unknown or empty stage.
  The controls page renders all four stage buttons and labels the results "scripted manual" grades.
- Tests cover direct selection from a fresh morning, sequential selection, early loss with five legs still pending and
  the slip settled `LOST`, all clear with every Sunday game final and one Monday game scheduled, Sunday complete with all
  14 games final and scored, reset back to an open slip with every pick pending, committed week/slip signals, the HTTP
  stage route and rendered pages, unknown stages changing nothing, a page-load kick racing a stage change, and the
  control routes staying 404 outside sandbox mode.
- Docs updated: `docs/guides/local-dev.md`, `docs/reference/routes.md`, `docs/concepts/live-updates.md`,
  `docs/reference/sandbox-replay.md`, `docs/llms.txt`, and the ESPN fixture README.

Verified in this worktree: `go generate ./...`, `go test -short -race -count=1 ./...`, `golangci-lint run ./...`
(0 issues), `good-vibes docs lint` (0 errors; 7 pre-existing stale-source warnings in unrelated docs), and
`good-vibes check local-sunday-sandbox` (passes).

## Final gate

- Rounds used: 1; verify failures: 0
- Final verify: no commands configured
- Open issues: none
