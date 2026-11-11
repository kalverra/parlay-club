# Phase 2 handoff: Launch the isolated offline app

## Done

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

## Deviations

- There is no sandbox controller yet. The design gives it reset/stage serialization and ownership of page-load kicks, but
  those controls are phase 3 work. Phase 2 still passes `refresher.Kick` straight to `newMux`. Startup is safe because
  the morning bootstrap finishes before the listener opens; a page-load kick after startup is not yet serialized against
  a reset.
- The design's phase-1 open issue asked for an `fs.FS` rooted at the repository. `repoRoot` discovers that root by
  walking up from the process working directory to `go.mod` instead of adding a new environment setting. This keeps the
  task thin and works for `mise run dev` and Air; a binary started outside the checkout fails with a clear error.
- The first boot creates `sandbox_state` before the ingest but writes the stage row only after `RunOnce` succeeds. A
  failed bootstrap therefore leaves the domain rows it managed to write but no saved state, so the next boot retries
  rather than restoring a half-built morning. A successful reset in phase 3 remains the clean recovery path.
- The restart test stores an `all-clear` stage row directly because phase 4 has no fixtures for it yet. That proves the
  stage string and clock round-trip through SQLite without claiming the stage is playable.

## Open issues

- `newMux` still receives the raw `refresher.Kick`. Phase 3 should route page-load kicks through the controller so a
  reset cannot race a background pass, and mount the control routes there.
- A crash during the first `RunOnce` can leave a partial slate with no stage row, so the next boot retries the morning
  against rows that are already there. Refresh treats those rows as current and will not necessarily rewrite them. The
  phase 3 reset is the intended way out.
- `StateStore.Load` accepts any non-empty stage and `newSandboxRuntime` passes it straight to `transport.SetStage`. A
  corrupt row makes every replay miss with a clear `ErrNoResponse`, but startup does not validate the stage against the
  registry. Phase 4 should add that check when more than one stage exists.
- The scenario clock is set once at boot. Any later stage change must call `Scenario.Set` and `StateStore.Save` before
  running refresh, or the data and the page will disagree.
- `repoRoot` requires the working directory to be inside the checkout. That is fine for the documented dev workflow,
  but a packaged binary still cannot serve the sandbox.
- Saved credit headers stay static per response, as in phase 1. A repeated line or props call sees the same balance.
  Both values stay above the floors, so the normal guards still run.

## Next-phase notes

- Let the controller own `refresher`, `scenario`, `transport`, and `state`. Add reset/stage operations under one mutex,
  set the clock and save the row before refresh, and pass the controller's serialized kick into `newMux`.
- Reset needs a small hook into `refresh` to clear the in-memory failure backoff. `clearBackoff` is private today; expose
  one narrow method rather than reaching into the map.
- Clear domain, sync, and archive rows transactionally, keep `sandbox_state` and the migrations, then run `RunOnce` and
  rebuild personas and the slip. The existing `sandboxFixtures(t)` helper in `cmd/server` is reusable for tests.
- Personas should go through `auth.SignIn` and the real invite/membership operations; the slip through `Store.CreateSlip`
  and `slips.Service.AddLeg`. Keep the `morning` stage and first-boot path unchanged so phase 1's replay tests stay a
  stable baseline.
- `web.Dependencies.Now` is already the seam every new sandbox screen should use. Do not switch cookies or SSE timers.

## Files touched

- `cmd/server/main.go`
- `cmd/server/sandbox.go`
- `cmd/server/sandbox_test.go`
- `docs/concepts/data-refresh.md`
- `docs/concepts/persistence.md`
- `docs/guides/local-dev.md`
- `docs/llms.txt`
- `docs/reference/config.md`
- `docs/reference/sandbox-replay.md`
- `internal/config/config.go`
- `internal/config/config_test.go`
- `internal/refresh/refresh.go`
- `internal/refresh/run_test.go`
- `internal/sandbox/registry.go`
- `internal/sandbox/scenario.go`
- `internal/sandbox/state.go`
- `internal/sandbox/state_test.go`
- `internal/web/analyzer.go`
- `internal/web/analyzer_test.go`
- `internal/web/export.go`
- `internal/web/export_test.go`
- `internal/web/routes.go`
- `internal/web/slips.go`
- `internal/web/slips_test.go`
- `mise.toml`

## Run log

- Mode: 
- Checkpoints: task-2
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
