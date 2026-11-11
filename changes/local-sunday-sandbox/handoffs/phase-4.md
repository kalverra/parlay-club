# Phase 4 handoff: Stage Sunday outcomes

## Done

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

## Deviations

- The design asked to add same-week live and final ESPN snapshots. No same-week capture existed, so all three stage
  scoreboards are derived from the captured week 9 slate rather than new captures. The registry and both READMEs mark
  them `Derived` and say what changed, which keeps the provenance rule the design called for.
- `Reset` is no longer a separate implementation. It delegates to `Stage(morning)`, so reset and every stage share one
  lock, one rebuild, one publish path, and one failure shape. The design treated reset and stage selection as adjacent
  operations; making them the same operation removed duplicate logic.
- Every non-morning stage locks all shared members. The design said "locks the intended members"; the seed has exactly
  three shared members and a shared slip, so locking all of them is the intended behavior with no partial case.
- A stage saves its stage and clock before running its refresh and grades. If the refresh or a grade fails, the saved
  state can be ahead of a partially applied database. The route answers 500 and reset recovers. This mirrors the reset
  ordering from phase 3; the design did not define failure ordering for stages, and a whole-stage transaction is not
  practical around the real refresh.
- The transition race is covered by holding the runtime mutex while a page-load kick and a stage change queue, then
  releasing it and asserting the chosen stage wins. That tests the actual serialization guarantee rather than racing two
  live HTTP requests whose interleaving the test cannot control.

## Open issues

- `sandboxRuntime.Seed()` is still in memory only and is empty after an Air restart. Every stage rebuilds first, so stage
  grading always has a fresh seed; there is no path that re-grades the restored rows without a rebuild. The controls
  page already resolves personas and groups from the database.
- A failed stage can leave the state row and clock on the new stage while the data is partly rebuilt. Reset is the only
  recovery path. There is no per-stage rollback.
- Only the featured game's prop board is registered. Opening another game's props fails closed, which is the intended
  visible error but means not every props surface is explored by the staged scenarios.
- `Kick` holds the transition mutex for a whole ingest pass, so a reset or stage selected during a cold page-load
  refresh waits for that pass to finish.
- The three stage scoreboards are hand-derived. A change to the week 9 slate, the seeded games, or the ESPN fixture shape
  requires editing them; the stage tests fail visibly if the counts, statuses, or scores drift.

## Next-phase notes

- This was the last implementation phase. Run the full gate, write the close-out with the close skill, and let a person
  review it before `good-vibes done local-sunday-sandbox`.
- Keep the stage outcomes labeled as scripted manual grades. `grading-record-book` still owns automatic grading, and no
  sandbox path should grow score inference or direct result writes.
- To add a stage later: add its scoreboard fixture to `week9StageScoreboards`, its name and clock to
  `internal/sandbox/scenario.go`, its plan and grades to `PlanFor`, its control description to the sandbox page, its row
  to the routes and replay docs, and a direct plus sequential server test.
- Keep the stage order: set the scenario and replay stage, save the state row, run `RunOnce`, apply the plan, then
  publish. Moving any write ahead of the clock or transport makes the refresh and page read different instants.

## Files touched

- `cmd/server/sandbox.go`
- `cmd/server/sandbox_test.go`
- `docs/concepts/live-updates.md`
- `docs/guides/local-dev.md`
- `docs/llms.txt`
- `docs/reference/routes.md`
- `docs/reference/sandbox-replay.md`
- `internal/ingest/espn/testdata/README.md`
- `internal/ingest/espn/testdata/scoreboard_week9_complete.json`
- `internal/ingest/espn/testdata/scoreboard_week9_final_sunday.json`
- `internal/ingest/espn/testdata/scoreboard_week9_live.json`
- `internal/sandbox/plan.go`
- `internal/sandbox/registry.go`
- `internal/sandbox/sandbox_test.go`
- `internal/sandbox/scenario.go`
- `internal/sandbox/state.go`
- `internal/sandbox/state_test.go`
- `web/static/app.css`
- `web/templates/sandbox.html`

## Run log

- Mode: 
- Checkpoints: task-5
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
