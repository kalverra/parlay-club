# Design: Local Sunday Sandbox

## Context

`cmd/server` is the composition root. It owns one SQLite handle, `db.Store`, real ingest clients, `refresh.Service`,
`slips.Service`, auth cookies, the SSE hub, and the web handler. The ingest clients already accept an `*http.Client`;
refresh and slips already accept clocks. Those are the main seams. `internal/seed` is a history importer, not a scenario
builder: it has no games, lines, memberships, or sign-in identities.

The useful captured responses are split among the ingest packages. Week 9 has matching scheduled ESPN, Parlay API, and
nfelo data. Its live ESPN fixture belongs to another week, and there is no same-week all-final response. Weather,
game-log, and summary coverage is selective. The response archive stores runtime calls but has no replay transport.

Keep these invariants intact: one process and one open database; lazy refresh only; no real odds credit use in the
sandbox; manual grading remains the source of staged leg results; production auth and routes do not change; SSE uses real
wall-clock timers even though game logic uses the scenario clock.

**Read before starting:**

- `plan/architecture.md` **Stack**, **Repo layout**, **Routes**, and **Database access** — process, routing, and store
  conventions. The plan has no separate acceptance checks for this development-only change.
- `plan/product.md` **Grading** and **Screens** — grading belongs to the existing services, and controls must work at
  phone width. Auto-grading remains owned by `grading-record-book`.
- `plan/data-sources.md` **Sources and staleness** and **Fixtures and live tests** — cache keys, fixture provenance, and
  the distinction between captured and derived data.
- `docs/guides/local-dev.md`, `docs/reference/config.md`, `docs/reference/routes.md`, and
  `docs/concepts/{access,data-refresh,persistence,live-updates,archive}.md`.

## Approach

### Opt in without creating a second app

Add `SUNDAY_SANDBOX=1` as an explicit dev setting. `SUNDAY_SANDBOX=1 mise run dev` runs the normal Air/server loop; a
thin `mise run sandbox` task can set the same value. Config rejects the setting with `APP_ENV=prod`. Sandbox mode always
uses `./data/sunday-sandbox.db`, ignoring `DB_PATH`, and skips history import. Normal dev remains on `./data/parlay.db`.

In sandbox mode, skip Google provider construction, enable the existing dev login even when `.env` contains Google
values, and never pass real odds keys to a source client. Only sandbox mode constructs the controller or mounts
`/dev/sandbox`, `/dev/sandbox/reset`, `/dev/sandbox/stage`, and `/dev/sandbox/persona`. The controls use the normal
signed-in/group-scoped middleware and HTMX CSRF checks. The standalone, phone-first controls page shows the current
persona, group, stage, and scenario time; successful actions refresh the app view.

### Replay raw responses below the real clients

A new `internal/sandbox` package owns a fail-closed `http.RoundTripper`. It matches method, host/path, stage, and
meaningful normalized query values while ignoring credentials. It returns saved status, headers, and body bytes to the
ordinary ESPN, weather, Parlay API, and nfelo clients. An unknown request is an error naming the request and stage; the
transport has no network fallback. Replayed calls do not enter the external-response archive.

The scenario registry references the existing source fixtures through an `fs.FS` rooted at the repository and adds new
fixtures beside them where a capture is missing, avoiding copies of large responses. Add every forecast needed by the
initial refresh, and add same-week live and final ESPN snapshots when a later stage needs them, along with any narrow
research responses used by the scenario. Mark edited or derived snapshots as such in fixture provenance.
Use one primary Parlay API client with a synthetic non-empty key and no fallback provider. Replayed credit headers stay
above the existing floors, so the normal event, line, and lazy prop paths still enforce their rules.

Preload every registered response at startup. The morning reset must successfully exercise the full scheduled slate,
game lines, projections, and weather; only the featured game needs its lazy prop board preloaded. Later research calls
may use additional registered responses, but an unregistered game or player must fail visibly rather than invent data.

### Synchronize setup and keep one scenario clock

Add `refresh.Service.RunOnce(ctx) error` as the synchronous counterpart to `Kick`. It reuses the existing scoreboard-first
pass and source implementations, waits for the sources it starts, and returns joined failures. `Kick` remains
asynchronous. The sandbox controller serializes reset/stage operations and its page-load kicks, and reset clears failure
backoffs along with `sync_state`; an old background snapshot must not finish after a reset.

A mutex-protected scenario state supplies one `Now()` to refresh, slips, and web. Add optional `web.Dependencies.Now`,
defaulting to `time.Now`, and replace the picker, export, and nfelo-freshness wall-clock reads with it. Auth cookies and
SSE timers continue to use real time. Existing SQLite-created metadata timestamps may also remain wall-clock values;
repeatability covers domain state, not byte-identical database files.

The sandbox owns a one-row stage/time record in its isolated SQLite file, created only by sandbox setup rather than the
production migrations. A first boot builds `morning`; an Air restart restores the selected stage and clock without
resetting manual edits. Reset clears application, sync, and archive rows transactionally but retains migrations and the
sandbox metadata table. Keep this destructive SQL in `internal/sandbox`, not as a general production store API. A failed
rebuild reports an error and leaves reset available for recovery.

### Build real people, membership, and picks

Create personas through `auth.SignIn` with fixed `.test` emails and `dev:{email}` subjects, so they have the same user
records and signed cookies as ordinary dev sign-in. Create one shared group and add the named members through the real
invite/membership store operations. Leave the outsider in their own group. Build the shared slip with `Store.CreateSlip`,
add every pick through `slips.Service.AddLeg` before the scenario kickoff, and retain symbolic persona/leg IDs for later
stages instead of relying on numeric IDs. Morning remains editable with all legs pending.

The reset flow is: establish the morning clock, clear disposable state, run morning ingest synchronously, create people
and shared membership, create the slip and picks, sync the featured prop board, then publish the final week/slip change
signals. Setup records enough persona identity to reissue the current browser's real session/group cookie after reset.

Persona switching calls `auth.SignIn`, then sets both the real session cookie and the correct shared or outsider group
cookie. It does not inject a user into request context or add permissions. Activity and grading therefore name the actual
selected persona, and the outsider receives the normal 404 for shared-slip URLs and can still exercise the normal invite
join flow.

### Select independent, deterministic situations

The control names are `morning`, `early-loss`, `all-clear`, and `sunday-complete`. Morning is the explicit reset. Each
other target rebuilds that baseline before applying its manifest, so every button works regardless of the previous stage.
A stage manifest selects the scoreboard and fixed time, runs the real refresh, locks the intended members, and calls
`slips.Service.SetLegResult` for named legs. It does not infer grades from scores or write result columns directly.

| Target | Visible state |
| --- | --- |
| `morning` | Full scheduled week, open shared slip, pending picks, upcoming games selectable. |
| `early-loss` | Coherent live scoreboard, locked slip, one leg `LOST`, other legs still pending or won; normal rules immediately settle the slip `LOST`. |
| `all-clear` | The picked games have usable final/live context, every leg is `WON`, and normal rules settle the slip `WON`. |
| `sunday-complete` | Every Sunday and Monday game in the fixture is final with scores; the declared final outcomes remain visible and no upcoming-leg flow treats the week as Sunday morning. |

Refresh publishes its normal committed week changes and grading publishes normal slip changes. The controller publishes
an explicit final signal after a full reset because deleting and recreating equivalent rows need not look like a source
change. The acting browser receives an HTMX refresh; other connected clients refetch committed state after SSE signals.

Rejected alternatives: direct domain-row fixtures would bypass parsers and cache rules; a checked-in SQLite file would
complicate resets and hide provenance; generic archive replay would need a broad export/import feature this task does not
need; a separate mock HTTP server would add a process without improving fidelity.

## Verification

Standard gate: `go generate ./...`, `golangci-lint run ./...`, `go test -short -race ./...`, `good-vibes docs lint`, and
`good-vibes check local-sunday-sandbox`.

- Replay the morning through real clients and refresh, asserting the complete slate, matched prices, projections,
  forecasts, featured props, and scenario timestamps. Exercise an unknown URL and a missing fixture and prove neither
  can fall through to the network. Run with real-looking provider/Google settings and verify they are unused.
- Start with a sentinel normal dev database and prove sandbox startup and repeated resets never modify it. Reset twice
  and compare users, memberships, slips, legs, results, activity content, and sync state; no duplicates or stale cache
  freshness may survive. Reconstruct the server to prove stage/time survives an Air-style restart.
- Use `httptest`, cookie jars, and the real middleware rather than adding a browser framework. Switch among shared
  personas and the outsider; verify signed sessions, shared-slip visibility, actor attribution, outsider 404s, and an
  actual invite join. Check the controls and ordinary app pages at phone width.
- From a fresh morning, select each target directly and in sequence. Assert early loss settles while other legs remain
  unresolved, all-clear requires every leg won, Sunday completion marks every game final, and reset removes all manual
  edits. Confirm the picker/export/nfelo freshness use scenario time.
- Subscribe before reset/stage actions, assert week/slip signals and committed refetches, and race a page-load kick with
  reset to show an old response cannot overwrite the chosen stage. Unknown stage/persona requests change nothing.
- With otherwise-valid production config, assert sandbox startup is rejected and every control route is absent. Run the
  existing non-sandbox server tests to show default clocks, OAuth selection, history import, and live ingest are intact.

## Risks

- **Fixture drift or mismatched weeks:** ids and kickoff times may stop matching. Strict replay plus complete-scenario
  tests fail visibly; fixture provenance distinguishes real captures from derived live/final snapshots.
- **Accidental network or credit use:** this is the highest-impact failure. The replay transport has no fallback, sandbox
  ignores real credentials, and startup/prod tests guard both paths.
- **Reset races:** background or on-demand work could write after a reset. Transition serialization, synchronous setup,
  and the race test cover the supported paths; SQLite transactions prevent partially committed individual mutations.
- **Partial failed rebuild:** a new fixture error can leave a partial sandbox, though never the normal dev database. Show
  the error and make reset retry from the transactional clear; preloading minimizes this case.
- **Clock leakage:** using scenario time for cookies or SSE could expire sessions or hold streams open. Keep those on real
  time and retain the existing live-stream timer tests.
- **False confidence about grading:** staged outcomes could look auto-derived. Label them as scripted manual grades and
  keep `grading-record-book` behavior out of this change.

## TODO

Effort: medium (the change coordinates transport, lifecycle, auth, time, and live updates across the composition root,
but reuses the existing clients and domain services rather than adding production behavior.)

### Phase 1: Replay a coherent Sunday morning

- [x] Add the fail-closed replay transport and named week 9 response registry, including credit headers and any missing
  morning forecasts; drive real ingest clients through the scheduled slate, lines, weather, projections, and featured
  props, with tests for successful parsing and unknown/missing responses. Document captured versus derived provenance.

### Phase 2: Launch the isolated offline app

- [ ] Add synchronous refresh and scenario-clock seams through refresh, slips, and web, then wire `SUNDAY_SANDBOX=1`, the
  mise convenience task, fixed isolated database, forced dev auth, synchronous morning bootstrap, and stage persistence.
  Test no-network startup, production rejection, normal dev preservation, and restart behavior; update local development,
  configuration, persistence, and data-refresh docs.

### Phase 3: Add repeatable people and picks

- [ ] Implement transactional sandbox reset and real-service seeding of shared personas, an outsider, an editable slip,
  and named legs. Add the reset controls with post-commit notifications, then test repeatability, normal-database
  isolation, cleanup of cache/backoff state, and current-session recovery.
- [ ] Add the development-only controls page and cookie-backed persona switching through real sign-in and group scope.
  Test member perspectives, activity ownership, outsider isolation and invite joining, CSRF/auth guards, and phone
  layout; update local development, access, and route docs.

### Phase 4: Stage Sunday outcomes

- [ ] Add coherent same-week live/final responses and deterministic early-loss, all-clear, and Sunday-complete controls
  using real refresh, locking, and manual grading. Test direct and sequential selection, scores/statuses, slip results,
  remaining pending legs, reset, SSE refetch signals, transition races, and production route absence; update local
  development, route, and live-update docs.
