# Phase 3 handoff: Add repeatable people and picks

## Done

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

## Deviations

- `/dev/sandbox/stage` is not mounted yet. The design's route sentence lists it, but stage selection and manifests are
  phase 4; phase 3 only names the `morning` stage.
- The controller lives in `cmd/server/sandbox.go` as `sandboxRuntime` instead of a new type in `internal/sandbox`. That
  keeps `internal/sandbox` free of `auth`, `groups`, and HTTP dependencies while still giving one owner to reset, the
  clock, the replay stage, and the state row.
- On the reset path the stage row is saved before the destructive clear, so the app and data agree on the morning while
  a rebuild runs. First boot still writes the row only after success. The design did not fix this ordering; the
  difference matters when a rebuild fails.
- Persona switching takes the same mutex as reset. A switch waits for an in-flight rebuild instead of racing the user
  and group recreation the design only asked to serialize for reset and stage work.
- `Seed()` is in memory only, so it is empty after an Air restart. The controls page resolves personas and groups from
  the database instead, and persona switching still works. Symbolic IDs are stable, but phase 4 must resolve them to
  current rows after a restart.
- Phone-width verification is structural, following the existing suite: the page and app tabs are parsed for the viewport
  meta tag and one-column/`sm` classes. There is no rendered screenshot or browser framework.
- `main_test.go` call sites pass an extra `nil` for the variadic sandbox mount argument, so existing non-sandbox tests
  need no other changes.

## Open issues

- There is no stage operation or `/dev/sandbox/stage` route yet. `sandboxRuntime` can only rebuild `morning`.
- After a restart, the in-memory seed is empty and there is no helper that maps a symbolic `LegID` back to a leg row
  across resets. Phase 4 needs one, or a manifest that finds legs by owner plus game plus market before grading them.
- `StateStore.Load` still accepts any non-empty stage and startup does not validate it against the registry. Phase 4
  should reject an unknown saved stage when more than one stage exists.
- `Kick` runs `RunOnce` with `context.Background` and holds the reset mutex for the pass, so a reset can wait up to the
  ingest timeout behind a page-load refresh. That protects the data, but it makes a page-load-triggered reset feel slow
  on a cold pass.
- `RunOnce` joins a scoreboard failure and continues with the previous slate. On a cleared database there is no
  previous week, so the seed step fails and reset returns 500; the next reset retries from the transactional clear.
- Only the featured game's prop board is preloaded. Props for another game will fail closed when someone opens it,
  which is the intended visible error but means not every app surface is explored by the morning scenario.
- The outsider's group stays their own even after accepting the shared invite, so switching back to Dex after a join
  hides the shared slip again. Reset restores the one-group view. This is deliberate for the control, but it is not the
  ordinary member experience.

## Next-phase notes

- Add a stage operation behind the existing `sandboxRuntime.mu`. Set the scenario clock and `transport.SetStage`
  before saving the row and running `RunOnce`, so refresh, slips, and the page all read the same time and data.
- Build each stage as "rebuild morning, then apply manifest". Lock the shared slip through `slips.Service.LockIn`, grade
  named legs through `SetLegResult`, and never write result columns or infer outcomes from scores.
- Resolve symbolic personas and legs to current rows through the store, not through an old in-memory seed. Keep the
  `PersonaID` and `LegID` names stable so manifests and tests survive a restart.
- Publish the normal refresh and slip signals, plus the explicit final week/slip signal after a reset. Publish only
  after the writes commit; let the acting browser refresh through HTMX and other clients refetch after SSE.
- Add the same-week live and final ESPN fixtures as new registry stages, mark derived snapshots in provenance, and teach
  `StateStore.Load` or startup to reject a stage the registry does not know.
- Test direct and sequential stage selection, unknown stages changing nothing (rows, clock, and cookies), all games final
  in `sunday-complete`, and a page-load `Kick` racing a stage change so the chosen stage wins.
- Keep auth cookies and SSE timers on real time, and keep replayed calls out of the response archive and above the
  credit floors.

## Files touched

- `cmd/server/main.go`
- `cmd/server/main_test.go`
- `cmd/server/sandbox.go`
- `cmd/server/sandbox_test.go`
- `docs/concepts/access.md`
- `docs/concepts/persistence.md`
- `docs/guides/local-dev.md`
- `docs/llms.txt`
- `docs/reference/routes.md`
- `docs/reference/sandbox-replay.md`
- `internal/refresh/refresh.go`
- `internal/refresh/run_test.go`
- `internal/sandbox/reset.go`
- `internal/sandbox/seed.go`
- `internal/web/render.go`
- `internal/web/render_test.go`
- `web/static/app.css`
- `web/templates/sandbox.html`

## Run log

- Mode: 
- Checkpoints: task-3, task-4
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
