---
slug: sunday-odds-prefill
run: 20261115-092719-sunday-odds-prefill
outcome: done
generated: 2026-11-15T10:20:45Z
---

# Run summary: sunday-odds-prefill

## Outcome

- Status: done
- Branch: sunday-odds-prefill
- Final verdict: agy@gemini-4-argon-high approved (issues: 0)

## Totals

- Tasks: 8 of 8
- Phases: 3
- Checkpoints: 9
- Duration: 42m52s
- Cost: $—
- Fresh input tokens: 2404414
- Cache read tokens: 66726468
- Cache write tokens: 0
- Output tokens: 401092

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-medium | 69531974 | $— |

## Phases

### Phase 1: Sunday pre-fill engine and authenticated endpoint

Tasks:

- [x] Add `RunPrefill` (scoreboard -> lines -> free summaries/rosters + bounded parallel Parlay-only bulk props for
      upcoming games with 15m idempotency), tighten free-source staleness windows (`scoreboard`, `summary`, `roster`,
      `weather`, `nfelo`, `gamelog`), promote `summary` and unique week-team `roster` syncs into `Kick()`, and split
      `Kick` from `RunOnce` so `Kick` omits `odds:lines` in `internal/refresh` (updating
      `TestRefreshArchivesEveryIngestCall` and adding unit/integration tests)
- [x] Add optional `PrefillToken` (`PREFILL_TOKEN`) to `internal/config` and mount `POST /internal/prefill` outside
      cookie/CSRF middleware in `cmd/server` with constant-time bearer auth, synchronous execution, and handler/config
      tests
- [x] Docs: amend the lazy-only sync guardrail in `AGENTS.md` and update `docs/concepts/data-refresh.md`,
      `docs/reference/config.md`, and `docs/reference/routes.md` for pre-fill and `Kick` scope

Phase totals: 3 checkpoints, 22m22s, $—, 40671102 input / 167799 output / 39597621 cache read / 0 cache write tokens

Checkpoints: task-1, task-2, task-3 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Add `RunPrefill` (scoreboard -> lines -> free summaries/rosters + bounded parallel Parlay-only bulk props for upcoming games with 15m idempotency), tighten free-source staleness windows (`scoreboard`, `summary`, `roster`, `weather`, `nfelo`, `gamelog`), promote `summary` and unique week-team `roster` syncs into `Kick()`, and split `Kick` from `RunOnce` so `Kick` omits `odds:lines` in `internal/refresh` (updating `TestRefreshArchivesEveryIngestCall` and adding unit/integration tests)

- Added `RunPrefill(ctx context.Context) (PrefillReport, error)` and `prefillProps` in `internal/refresh`: syncs
  scoreboard first, `odds:lines` (15-minute idempotency), free sources (`freeSourcesFor` with concurrency 4), and
  Parlay-only bulk props for upcoming games (`prefillConcurrency = 4`, 15-minute idempotency, skipping `FallbackProps`
  and kicked-off games, respecting `OddsPropsFloor`).
- Tightened free-source staleness windows in `internal/refresh/refresh.go` and `internal/refresh/rules.go`: `scoreboard`
  (5m live / 15m GD / 2h), `summary` (15m GD or <=4h to kickoff / 1h, skip `FINAL`), `roster` (2h GD / 6h), `weather`
  (15m <=6h / 30m <=48h), `nfelo` (1h GD / 4h), and `gamelog` (1h GD / 4h).
- Split `Kick()` from `RunOnce()`: `Kick()` uses `freeSourcesFor` (including `espn:summary:{gameID}` and deduped
  week-team `espn:roster:{teamID}` for non-final games) and omits `odds:lines`; `RunOnce()` retains `odds:lines` for
  sandbox and full-pass callers.
- Updated `TestRefreshArchivesEveryIngestCall` and `syncKeys` in `cmd/server/`, updated staleness rule tests in
  `internal/refresh/rules_test.go`, `nfelo_test.go`, `research_test.go`, and `sources_test.go`, and added
  `internal/refresh/prefill_test.go`.

### Task 2: Add optional `PrefillToken` (`PREFILL_TOKEN`) to `internal/config` and mount `POST /internal/prefill` outside cookie/CSRF middleware in `cmd/server` with constant-time bearer auth, synchronous execution, and handler/config tests

- Added optional `PrefillToken` (`PREFILL_TOKEN`, whitespace-trimmed) to `config.Config` and `config.Load` in
  `internal/config/config.go`, with tests in `internal/config/config_test.go`.
- Added `newPrefillHandler`, `validBearerToken` (using `subtle.ConstantTimeCompare`), and `prefillTimeout` (2 minutes,
  linked to `stopCtx`) in `cmd/server/main.go`.
- Mounted `POST /internal/prefill` on an outer `http.ServeMux` in `newMux` outside `handlers.CSRF`, `RequireUser`,
  `Scope`, and `kickOnPage`, returning `404` when `PREFILL_TOKEN` is empty, `401` on missing/invalid bearer token, `503`
  in sandbox mode, `200` JSON `PrefillReport` on success, and `500` JSON on `RunPrefill` error.
- Updated `newMux` callers in `cmd/server/main_test.go` and `cmd/server/sandbox_test.go` and added `TestPrefillEndpoint`
  covering `404`, `401`, `503`, synchronous `200` JSON without cookies/CSRF, `500`, and `stopCtx` cancellation.

### Task 3: Docs: amend the lazy-only sync guardrail in `AGENTS.md` and update `docs/concepts/data-refresh.md`, `docs/reference/config.md`, and `docs/reference/routes.md` for pre-fill and `Kick` scope

- Amended the free-tier guardrails in `AGENTS.md` to allow the Sunday morning pre-fill (`POST /internal/prefill`) and
  pick-time line/prop refreshes while scoping `Kick()` to free/unmetered sources only.
- Updated `docs/concepts/data-refresh.md` for `RunPrefill` (scoreboard -> `odds:lines` -> free sources -> Parlay-only
  bulk props with 15m idempotency), `Kick()` vs `RunOnce()` scope, promoted `espn:summary` and deduped week-team
  `espn:roster` syncs, and tightened free-source staleness windows.
- Updated `docs/reference/config.md` to document optional `PREFILL_TOKEN` and its fail-closed bearer auth behavior.
- Updated `docs/reference/routes.md` to document `POST /internal/prefill`, its outer-mux placement outside cookie/CSRF
  middleware, and its `401`/`404`/`503`/`200`/`500` responses.
- Verified documentation with `good-vibes docs lint` (`ok`).

### Phase 2: Pick-time on-demand refresh rules and UI

Tasks:

- [x] Implement `SyncPickLines`, `SyncPickGame`, `ForcePickRefresh`, pick-time staleness rules (`15m`/`2h` lines,
      `30m`/`2h` props), and the `2m` manual refresh cooldown in `internal/refresh` with unit and single-flight tests
- [x] Extend `web.Refresher`, trigger pick-time syncs on `OPEN` slips with upcoming games in
      `pickBoard`/`pickResults`/`pickDetail` (`internal/web/pick.go`), and add `POST /slips/{id}/pick/refresh` with the
      phone-first refresh button in `web/templates/partials.html` and handler/cooldown/locked-slip tests
- [x] Docs: update `docs/concepts/data-refresh.md`, `docs/concepts/slips.md`, `docs/reference/data-sources.md`, and
      `docs/reference/routes.md` for pick-time refreshes and the refresh route

Phase totals: 3 checkpoints, 16m29s, $—, 24887290 input / 194104 output / 23856900 cache read / 0 cache write tokens

Checkpoints: task-4, task-5, task-6 Review fixes: 0 Integration checks: 0

#### Done

### Task 4: Implement `SyncPickLines`, `SyncPickGame`, `ForcePickRefresh`, pick-time staleness rules (`15m`/`2h` lines, `30m`/`2h` props), and the `2m` manual refresh cooldown in `internal/refresh` with unit and single-flight tests

- Added `SyncPickLines`, `SyncPickGame`, and `ForcePickRefresh` to `refresh.Service` in `internal/refresh/refresh.go`
  (using `requestTimeout` and single-flight `refreshSync`, ignoring `odds.ErrNoAPIKey` when odds providers are
  unconfigured).
- Implemented `oddsPickLinesStale` (15m on game days before the day's last kickoff or within 2h of next kickoff; 2h
  midweek), tightened `oddsPropsStale` (30m on game days or within 6h of kickoff; 2h earlier in the week), and
  `pickCooldownStale` (2m manual refresh cooldown) in `internal/refresh/rules.go`.
- Updated `TestOddsPropsStaleRules` in `internal/refresh/rules_test.go` and
  `TestOpeningSameGameTwiceWithinCacheWindowSpendsCreditsOnce` in `internal/refresh/research_test.go` for the 30m
  game-day props window, and added `TestOddsPickLinesStaleRules` and `TestPickCooldownStaleRules`.
- Added unit and single-flight tests in `internal/refresh/pick_test.go` covering `SyncPickLines`, `SyncPickGame`,
  `ForcePickRefresh`, cooldown enforcement, and concurrent caller coalescing.

### Task 5: Extend `web.Refresher`, trigger pick-time syncs on `OPEN` slips with upcoming games in `pickBoard`/`pickResults`/`pickDetail` (`internal/web/pick.go`), and add `POST /slips/{id}/pick/refresh` with the phone-first refresh button in `web/templates/partials.html` and handler/cooldown/locked-slip tests

- Extended `web.Refresher` in `internal/web/routes.go` with `SyncPickLines`, `SyncPickGame`, and `ForcePickRefresh`, and
  updated test refresher stubs in `internal/web/games_test.go` and `internal/web/research_test.go`.
- Added pick-time sync gating (`syncPickIfOpen`, `syncPickScope`) in `internal/web/pick.go` so `pickBoard` syncs lines
  (or game lines + props when an upcoming game is selected) and `pickResults`/`pickDetail` sync game lines + props only
  when `slip.Status == db.SlipOpen`, `q.GameID != ""`, and the game is upcoming, while `pickSlate` remains read-only.
- Added `POST /slips/{id}/pick/refresh` (`pickRefresh` in `internal/web/pick.go`, registered in
  `internal/web/slips.go`), enforcing group scope, `db.SlipOpen` (returning 422 on locked slips), and upcoming-game
  checks before calling `ForcePickRefresh` and returning `#pick-body`.
- Added `CanRefresh` to `PickSlateView` and `PickDetailView` and rendered the phone-first (`min-h-12`) `Refresh odds`
  button in `web/templates/partials.html` (`pick-slate` and `pick-detail`) only for `OPEN` slips with upcoming games.
- Added handler, group-scope, locked-slip, repeated-search, and 2-minute cooldown tests in `internal/web/pick_test.go`
  (`TestPickBoardAndPartialsSyncRules`, `TestPickRefreshHandlerAndGuards`,
  `TestPickRefreshCooldownAndRepeatedSearchWithService`).

### Task 6: Docs: update `docs/concepts/data-refresh.md`, `docs/concepts/slips.md`, `docs/reference/data-sources.md`, and `docs/reference/routes.md` for pick-time refreshes and the refresh route

- Updated `docs/concepts/data-refresh.md` to document `SyncPickLines`, `SyncPickGame`, `ForcePickRefresh`, pick-time
  `OPEN`-slip gating, the `15m`/`2h` lines and `30m`/`2h` props staleness rules, and the `2m` manual refresh cooldown.
- Updated `docs/concepts/slips.md` to document pick-time sync gating on `OPEN` slips with upcoming games, the
  phone-first "Refresh odds" button (`POST /slips/{id}/pick/refresh`), and read-only behavior on locked slips and
  `/pick/slate`.
- Updated `docs/reference/data-sources.md` to document the `30m`/`2h` props window, pick-time `15m`/`2h` lines window,
  `2m` manual cooldown, and `RunPrefill` Parlay-only props sweep in the Parlay API and Budget sections.
- Updated `docs/reference/routes.md` to add `POST /slips/{id}/pick/refresh` (`422` when not `OPEN`, `2m` cooldown,
  `#pick-body` response) and pick-time sync rules for `/slips/{id}/pick`, `/pick/results`, `/pick/detail`, and
  `/pick/slate`.
- Removed retired `sources` and `last_verified` front-matter fields from the four updated docs so `good-vibes docs lint`
  passes with zero errors or warnings on all four files.

### Phase 3: Cloud Scheduler deploy wiring and setup docs

Tasks:

- [x] Enable `cloudscheduler.googleapis.com`, add the `prefill-token` secret, and print the Sunday 9:30 AM
      `America/New_York` scheduler setup command in `deploy/setup.sh`, plus mount optional `PREFILL_TOKEN` in
      `.github/workflows/deploy.yml` (`bash -n` and `deploy/blackout_test.sh` pass)
- [x] Docs: update `docs/guides/gcp-setup.md` and `docs/runbooks/deploy.md` for `prefill-token` and the Cloud Scheduler
      job, then run `good-vibes docs lint` and `good-vibes docs index`

Phase totals: 3 checkpoints, 4m0s, $—, 3572490 input / 39189 output / 3271947 cache read / 0 cache write tokens

Checkpoints: task-7, task-8, final Review fixes: 0 Integration checks: 0

#### Done

### Task 7: Enable `cloudscheduler.googleapis.com`, add the `prefill-token` secret, and print the Sunday 9:30 AM `America/New_York` scheduler setup command in `deploy/setup.sh`, plus mount optional `PREFILL_TOKEN` in `.github/workflows/deploy.yml` (`bash -n` and `deploy/blackout_test.sh` pass)

- Enabled `cloudscheduler.googleapis.com` in `deploy/setup.sh`.
- Added `prefill-token` to `SECRETS` in `deploy/setup.sh` (creating the empty secret and granting
  `roles/secretmanager.secretAccessor` to `parlay-runtime`), and documented it as optional alongside `github-token`.
- Added the Sunday 9:30 AM `America/New_York` (`30 9 * * 0`) `gcloud scheduler jobs create http` command targeting
  `<BASE_URL>/internal/prefill` to the `Next:` instructions printed by `deploy/setup.sh`.
- Updated `.github/workflows/deploy.yml` to check for an `ENABLED` version of `prefill-token` during GCP pre-flight
  (`MOUNT_PREFILL_TOKEN=true`) and mount `PREFILL_TOKEN=prefill-token:latest` in `gcloud run deploy` when present.
- Verified `bash -n deploy/setup.sh` and `deploy/blackout_test.sh` (`22 passed, 0 failed`).

### Task 8: Docs: update `docs/guides/gcp-setup.md` and `docs/runbooks/deploy.md` for `prefill-token` and the Cloud Scheduler job, then run `good-vibes docs lint` and `good-vibes docs index`

- Updated `docs/guides/gcp-setup.md` for `cloudscheduler.googleapis.com`, the optional `prefill-token` secret (seven
  secrets total), step 8's `gcloud scheduler jobs create http` command for Sunday 9:30 AM `America/New_York`
  (`30 9 * * 0`) with `--attempt-deadline=3m`, and troubleshooting for `POST /internal/prefill` returning `404`.
- Updated `docs/runbooks/deploy.md` for `prefill-token` in preconditions, `MOUNT_PREFILL_TOKEN` / `PREFILL_TOKEN`
  mounting in `.github/workflows/deploy.yml`, Cloud Scheduler free-tier coverage under Spend, and troubleshooting for
  `POST /internal/prefill` returning `404`.
- Removed retired `sources` and `last_verified` front-matter fields from `docs/guides/gcp-setup.md` and
  `docs/runbooks/deploy.md`.
- Ran `good-vibes docs lint` (0 errors/warnings on `docs/guides/gcp-setup.md` and `docs/runbooks/deploy.md`;
  pre-existing error on `docs/reference/config.md` left untouched) and `good-vibes docs index` (`wrote docs/llms.txt`).

## Final gate

- Rounds used: 1; verify failures: 0
- Final verify: no commands configured
- Open issues: none
