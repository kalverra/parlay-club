---
slug: deploy-readiness
run: 20261113-173347-deploy-readiness
outcome: done
generated: 2026-11-13T18:12:47Z
---

# Run summary: deploy-readiness

## Outcome

- Status: done
- Branch: deploy-readiness
- Final verdict: agy@gemini-4-hydrogen-high approved (issues: 0)

## Totals

- Tasks: 10 of 10
- Phases: 4
- Checkpoints: 10
- Duration: 31m21s
- Cost: $0.49
- Fresh input tokens: 637413
- Cache read tokens: 17687490
- Cache write tokens: 0
- Output tokens: 160666

| Model | Tokens | Cost |
| --- | ---: | ---: |
| fireworks-ai/accounts/fireworks/models/deepseek-v4p1-flash-1017 | 18485569 | $0.49 |

## Phases

### Phase 1: Drain SSE streams and refresh workers on shutdown

Tasks:

- [x] Add `Stream.Shutdown()` in `internal/live/stream.go` so open SSE streams exit immediately on server stop without sending `bye`, and test SSE + normal HTTP shutdown together.
- [x] Add lifecycle cancellation and `Service.Shutdown(ctx)` in `internal/refresh/refresh.go` (and matching `sandboxRuntime.Shutdown(ctx)` in `cmd/server/sandbox.go`), wire both into `serve` in `cmd/server/main.go` before `database.Close()`, and test cancellation and timeout behavior.
- [x] Docs: update `docs/concepts/live-updates.md` and `docs/concepts/data-refresh.md` for shutdown behavior.

Phase totals: 3 checkpoints, 17m28s, $0.25, 11714424 input / 78907 output / 11431792 cache read / 0 cache write tokens

Checkpoints: task-1, task-2, task-3 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Add `Stream.Shutdown()` in `internal/live/stream.go` so open SSE streams exit immediately on server stop without sending `bye`, and test SSE + normal HTTP shutdown together.

- Added `Stream.done` plus `Stream.Shutdown()` in `internal/live/stream.go`. `Shutdown` closes the channel once via
  `sync.Once`, and `ServeSlip` selects on it and returns without `event: bye`.
- Gave `serve` and `listenAndServe` in `cmd/server/main.go` an optional `beforeShutdown func()` hook, called after the
  serve context cancels and before `http.Server.Shutdown`. `run` passes `stream.Shutdown`.
- Added `TestStreamShutdownEndsOpenStreamsWithoutBye` (two open streams, idempotent double shutdown, empty body) and
  `TestStreamServesAfterShutdownWithoutHanging` (a stream opened after shutdown returns a `200` header in under
  100 ms).
- Added `TestServeDrainsOpenStreamAndInflightRequestsOnShutdown` in `cmd/server/main_test.go`: an open SSE stream and a
  blocked slow request drain together, the stream body stays empty (no `bye`), and the drain still waits for the slow
  handler before `serve` returns.
- Documented the shutdown behavior in `docs/concepts/live-updates.md` under a new "Shutdown" subsection.

### Task 2: Add lifecycle cancellation and `Service.Shutdown(ctx)` in `internal/refresh/refresh.go` (and matching `sandboxRuntime.Shutdown(ctx)` in `cmd/server/sandbox.go`), wire both into `serve` in `cmd/server/main.go` before `database.Close()`, and test cancellation and timeout behavior.

- Added lifecycle cancellation to `refresh.Service`. `New` builds a `stopCtx`/`stopCancel` pair, `KickContext` refuses new
  passes once it is cancelled, and every source's per-source timeout context also cancels on the stop via
  `context.AfterFunc` (unregistered when the source returns). `Shutdown(ctx)` cancels, waits on the service's
  `WaitGroup`, and returns `ctx.Err()` when the caller's deadline expires first. A shutdown-cancelled source logs no
  outage and leaves no failure backoff.
- Gave `sandboxRuntime` a `stopCtx`/`kickWg` pair and `Shutdown(ctx)`: `Kick` refuses new passes after a stop, its
  serialized pass runs `RunOnce` on `stopCtx` so it aborts on shutdown, and `Shutdown` waits for that pass and then drains
  the refresher it owns.
- Replaced `serve`'s single hook with a `shutdownHooks{beforeHTTP, afterHTTP}` value. `beforeHTTP` (`stream.Shutdown`)
  runs first, `afterHTTP` (`refresher.Shutdown` / `runtime.Shutdown`) runs with the same 8-second `shutdownCtx` right
  after `http.Server.Shutdown`, and it still runs when the HTTP drain expired. `run` wires `drain` per mode and
  `database.Close()` still happens in `run`'s defer, after `serve` returns.
- Tests: `TestShutdownCancelsAnInFlightSync`, `TestKickAfterShutdownStartsNoWork`, and
  `TestShutdownReturnsTheDeadlineWhenASourceIgnoresIt` in `internal/refresh/shutdown_test.go`;
  `TestSandboxShutdownCancelsPageLoadKick` and `TestSandboxShutdownReportsTheDrainDeadline` in `cmd/server`;
  `TestServeDrainsOpenStreamAndInflightRequestsOnShutdown` now runs a real `refresh.Service` whose slow fetch must stay
  alive until the HTTP drain ends, and `TestServeReportsTheRefreshDrainError` proves a drain timeout fails the serve.
- Documented the drain in `docs/concepts/data-refresh.md` ("Shutdown" plus guardrails) and corrected the shutdown order in
  `docs/concepts/persistence.md`.

### Task 3: Docs: update `docs/concepts/live-updates.md` and `docs/concepts/data-refresh.md` for shutdown behavior.

- `docs/concepts/live-updates.md` now has a full "Shutdown" section: `Stream.Shutdown` closes the `done` channel once,
  every open `ServeSlip` returns without `bye`, `EventSource` reconnects on its own, a second `Shutdown` is safe, and a
  stream opened after it returns closes at once. It also explains why streams end before `http.Server.Shutdown` (that
  call waits on handlers without cancelling their request contexts) and links the 8 second budget to Persistence and
  Data refresh.
- `docs/concepts/data-refresh.md` "Shutdown" now gives the order end streams -> drain HTTP -> drain refresh -> close the
  database, says the stop check and goroutine start are one step so no pass slips past the wait, treats a cancelled
  source as a stop rather than a failure, notes the refresh drain still runs when the HTTP drain expires, and records
  that `sandboxRuntime.Shutdown` drains the page-load pass while stage and reset syncs are covered by the HTTP drain.
- Added `cmd/server/main.go` to the `live-updates` front-matter sources, regenerated `docs/llms.txt`, and ran
  `good-vibes docs lint` (0 errors; the 11 stale warnings are pre-existing in unrelated docs).

### Phase 2: Harden startup validation, OIDC discovery, and health checks

Tasks:

- [x] Validate `BASE_URL` scheme (`http`/`https`) and non-empty host in `internal/config/config.go` with table tests in `internal/config/config_test.go`, and update `docs/reference/config.md`.
- [x] Add bounded retry with backoff and per-attempt timeout to `auth.NewGoogleProvider` in `internal/auth/provider.go` with transient-failure and cancellation tests in `internal/auth/provider_test.go`.
- [x] Update `/healthz` in `cmd/server/main.go` to ping SQLite (`200 ok` / `503 unavailable`), update `cmd/server/main_test.go`, and update `docs/reference/routes.md`.

Phase totals: 3 checkpoints, 5m14s, $0.08, 1905024 input / 27639 output / 1791192 cache read / 0 cache write tokens

Checkpoints: task-4, task-5, task-6 Review fixes: 0 Integration checks: 0

#### Done

### Task 4: Validate `BASE_URL` scheme (`http`/`https`) and non-empty host in `internal/config/config.go` with table tests in `internal/config/config_test.go`, and update `docs/reference/config.md`.

- `internal/config/config.go` now has an unexported `validateBaseURL(raw string) error`. It parses with `url.Parse` and
  requires an `http` or `https` scheme plus a non-empty host, rejecting with
  `BASE_URL must be an http or https URL with a host, got %q`.
- `config.Load` runs the check whenever the resolved `cfg.BaseURL` is non-empty, so dev defaults and explicit values are
  validated and a missing prod value still reports only `BASE_URL is required in prod`.
- `internal/config/config_test.go` adds `TestLoadValidatesBaseURL`, a table test that runs every case in both `dev` and
  `prod`: accepts `http` and `https` with a host, trims trailing slashes, allows a path, and rejects `ftp://`, a bare
  hostname, a scheme with no host, and an unparsable URL.
- `docs/reference/config.md` describes the rule in the `BASE_URL` row and prose, adds the rejected-value row, and bumps
  `last_verified` to 2026-11-13.

### Task 5: Add bounded retry with backoff and per-attempt timeout to `auth.NewGoogleProvider` in `internal/auth/provider.go` with transient-failure and cancellation tests in `internal/auth/provider_test.go`.

- `internal/auth/provider.go` adds `discoverIssuer(ctx, issuer)`, which calls `oidc.NewProvider` up to three times with
  a 200ms then 500ms backoff and a 5-second deadline per attempt derived from the startup context. `NewGoogleProvider`
  keeps its signature, still wraps failures as `discover identity provider: %w`, and returns the context error wrapped
  when startup is cancelled, so `errors.Is(err, context.Canceled)` and `context.DeadlineExceeded` work.
- `internal/auth/provider_test.go` adds `TestGoogleProviderRetriesTransientDiscoveryFailures` (503 twice, then the third
  attempt succeeds and the provider works), `TestGoogleProviderFailsAfterExhaustingDiscoveryAttempts` (always 503,
  exactly three discovery calls, error names the provider and the 503),
  `TestGoogleProviderStopsDiscoveryWhenCancelled` (the context is cancelled during the first failing attempt; exactly
  one call and `context.Canceled` comes back in under 200ms), and `TestGoogleProviderBoundsAHungDiscoveryAttempt` (a
  handler that never replies; a 100ms parent deadline ends the attempt and startup returns `context.DeadlineExceeded`
  in under a second).
- Test helpers: `discoveryDocument` now builds the metadata both servers publish, and `flakyOIDCServer` answers
  discovery with 503 until a failure budget runs out while counting calls atomically.
- `docs/decisions/google-oauth.md` now describes the bounded retry in its consequences and implementation notes, scopes
  the lazy-discovery follow-up to longer outages, and bumps `last_verified` to 2026-11-13.

### Task 6: Update `/healthz` in `cmd/server/main.go` to ping SQLite (`200 ok` / `503 unavailable`), update `cmd/server/main_test.go`, and update `docs/reference/routes.md`.

- `cmd/server/main.go` replaces the static `healthz` with `newHealthzHandler(database *sql.DB, logger zerolog.Logger)`.
  Each probe pings SQLite under a `healthzPingTimeout` of 2 seconds derived from `r.Context()`, answers `200 ok` when
  the ping succeeds, and logs the error and answers `503 unavailable` with `text/plain; charset=utf-8` when it fails.
  The body never carries the SQLite error.
- `newMux` now takes the health handler next to the static and screenshot handlers and registers it with `mux.Handle` on
  `GET /healthz`, so `run` builds it once with the real database.
- `cmd/server/main_test.go` rewrites `TestHealthz`: an open handle answers `200 ok`, and after `database.Close()` the
  same mux answers `503 unavailable` with a generic body. Every other `newMux` call site, in `main_test.go` and
  `cmd/server/sandbox_test.go`, passes `http.NotFoundHandler()` for the new argument.
- `docs/reference/routes.md` describes the probe rule in the public-routes row and in prose, bumps `last_verified` to
  2026-11-13, and `docs/decisions/foundation.md` no longer claims `/healthz` never queries the database.

### Phase 3: Build group invite links from configured base URL

Tasks:

- [x] Thread `cfg.BaseURL` into `groups.NewHandlers` (`internal/groups/handlers.go`, `cmd/server/main.go`) so `inviteURL` uses `BASE_URL` when set and falls back to `r.Host` when empty, with proxy-Host tests in `internal/groups/handlers_test.go`, and update `docs/concepts/groups.md` and `docs/runbooks/custom-domain.md`.

Phase totals: 1 checkpoints, 2m19s, $0.04, 929602 input / 8180 output / 860872 cache read / 0 cache write tokens

Checkpoints: task-7 Review fixes: 0 Integration checks: 0

#### Done

### Task 7: Thread `cfg.BaseURL` into `groups.NewHandlers` (`internal/groups/handlers.go`, `cmd/server/main.go`) so `inviteURL` uses `BASE_URL` when set and falls back to `r.Host` when empty, with proxy-Host tests in `internal/groups/handlers_test.go`, and update `docs/concepts/groups.md` and `docs/runbooks/custom-domain.md`.

- `groups.NewHandlers` takes a `baseURL string` last, trims trailing slashes, and stores it on `Handlers`. `inviteURL` is
  now a method: a configured base URL builds `{baseURL}/join/{token}`, and an empty one keeps the old
  `r.Host` / `X-Forwarded-Proto` fallback, including the relative `/join/{token}` when `Host` is empty.
- `cmd/server/main.go` passes `cfg.BaseURL`, and `cmd/server/sandbox_test.go` passes `""`, so sandbox links keep
  following the request.
- `internal/groups/handlers_test.go` adds a `newJoinFixtureWithBaseURL` helper and two proxy-Host tests: a configured
  base URL with a trailing slash wins over a rewritten `.run.app` Host, and an empty base URL falls back to that Host
  with `X-Forwarded-Proto: https`.
- `docs/concepts/groups.md` explains the `BASE_URL` rule and fallback in Invites, adds a trade-off bullet, and bumps
  `last_verified`. `docs/runbooks/custom-domain.md` says `BASE_URL` also drives invite links, adds the
  `handlers.go`/`main.go` sources, and adds a verify step for the pasted link.

### Phase 4: Align GCP secrets, build caching, and deploy preflight

Tasks:

- [x] Add `github-token` secret and deployer viewer bindings (`secretmanager.viewer`, `storage.bucketViewer`) to `deploy/setup.sh`, and add BuildKit cache mounts to `Dockerfile`.
- [x] Update `.github/workflows/deploy.yml` with pre-auth variable checks, post-auth GCP/secret preflight, Buildx GHA layer caching, `GITHUB_REPO` env var, and conditional `GITHUB_TOKEN` secret wiring.
- [x] Docs: update `docs/guides/gcp-setup.md` and `docs/runbooks/deploy.md` to document the six secrets, deployer viewer permissions, pre-flight checks, and build caching.

Phase totals: 3 checkpoints, 6m20s, $0.13, 3775853 input / 45940 output / 3603634 cache read / 0 cache write tokens

Checkpoints: task-8, task-9, task-10 Review fixes: 0 Integration checks: 0

#### Done

### Task 8: Add `github-token` secret and deployer viewer bindings (`secretmanager.viewer`, `storage.bucketViewer`) to `deploy/setup.sh`, and add BuildKit cache mounts to `Dockerfile`.

- `deploy/setup.sh` adds `github-token` to `SECRETS`, so Secret Manager creates it empty and grants `parlay-runtime`
  `roles/secretmanager.secretAccessor` on it like the rest.
- The script grants `parlay-deployer` `roles/secretmanager.viewer` on the project (metadata only, no secret values) and
  `roles/storage.bucketViewer` on `gs://$BUCKET`, so the deploy pre-flight can list secret versions and describe the
  bucket after a re-run.
- The `Next:` block marks `session-key`, the Google pair, `odds-api-key`, and `the-odds-api-key` as the required secrets,
  and calls `github-token` optional: adding a version turns on GitHub issues from bug reports, and leaving it empty keeps
  reports in SQLite.
- `Dockerfile` adds BuildKit cache mounts: `/go/pkg/mod` on `go mod download` and both `/go/pkg/mod` and
  `/root/.cache/go-build` on `go build`. The existing `# syntax=docker/dockerfile:1` line already enables them.

### Task 9: Update `.github/workflows/deploy.yml` with pre-auth variable checks, post-auth GCP/secret preflight, Buildx GHA layer caching, `GITHUB_REPO` env var, and conditional `GITHUB_TOKEN` secret wiring.

- `deploy.yml` gains a pre-auth **Check the deployment variables** step. It fails before any GCP call when one of
  `GCP_PROJECT`, `GCP_REGION`, `WIF_PROVIDER`, `DEPLOY_SA`, `RUNTIME_SA`, `LITESTREAM_BUCKET`, or `AR_REPO` is empty,
  and rejects a `BASE_URL` that is not `http`/`https` with a host.
- A post-auth **Pre-flight the GCP project** step checks the Artifact Registry repo, the Litestream bucket, and an
  `ENABLED` version for each of `session-key`, `google-client-id`, `google-client-secret`, `odds-api-key`, and
  `the-odds-api-key`. It collects every failure before exiting. `github-token` is checked separately and writes
  `MOUNT_GITHUB_TOKEN=true` to `GITHUB_ENV` only when it has an `ENABLED` version; otherwise it logs a notice.
- The build path is now `docker/setup-buildx-action` plus `docker/build-push-action` with `cache-from: type=gha` and
  `cache-to: type=gha,mode=max`. The image tag is resolved once into both `GITHUB_ENV` (for the deploy step) and a step
  output (for the build action).
- `gcloud run deploy` passes `GITHUB_REPO` in `--update-env-vars` and appends `GITHUB_TOKEN=github-token:latest` to
  `--set-secrets` only when `MOUNT_GITHUB_TOKEN` is `true`.

### Task 10: Docs: update `docs/guides/gcp-setup.md` and `docs/runbooks/deploy.md` to document the six secrets, deployer viewer permissions, pre-flight checks, and build caching.

- `docs/guides/gcp-setup.md` now lists six secrets (`github-token` marked optional), the deployer's
  `secretmanager.viewer` and `storage.bucketViewer` grants, and a note to re-run `deploy/setup.sh` on an existing
  project. Step 3 is "Add a version to each required secret" and explains the five required versions plus how to add the
  optional `github-token` for GitHub issue filing.
- The variables step adds the optional `ISSUES_REPO` variable, says the workflow passes it as `GITHUB_REPO`, and notes
  the seven required variables are checked before any GCP call. The first-deploy step describes the new pre-flight.
- Both troubleshooting tables add rows for missing variables, a bad `BASE_URL`, unreadable secrets (missing
  `secretmanager.viewer`), missing bucket or Artifact Registry repo, missing secret versions, and bug reports that stay in
  SQLite because `github-token` has no version.
- `docs/runbooks/deploy.md` rewrites "What the workflow does" into nine steps covering the variable check, the GCP
  pre-flight, Buildx with the GitHub Actions layer cache, and the conditional `GITHUB_TOKEN` mount. It adds a "The build
  cache" section (warm versus cold builds, 7-day/10 GB eviction, no Artifact Registry usage) and a spend bullet saying
  the cache does not count against the registry free tier.
- Front matter: both docs bump `last_verified` to 2026-11-13; the deploy runbook adds `deploy/setup.sh` and `Dockerfile`
  to its sources. `rumdl` and `good-vibes docs lint` are clean and `good-vibes docs index` ran.

## Final gate

- Rounds used: 0; verify failures: 0
- Final verify: no commands configured
- Open issues: none
