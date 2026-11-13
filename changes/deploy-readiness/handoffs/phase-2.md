# Phase 2: Phase 2: Harden startup validation, OIDC discovery, and health checks

## Done

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

## Deviations

### Task 4

None. The helper, message, and "validate whenever non-empty" rule match the design.

### Task 5

- The design suggested "expose an unexported helper if tests inject delays". The tests live in `package auth_test`, so
  they cannot set unexported timings. The constants stay fixed and the tests use the real 200ms/500ms backoff, which
  costs about 0.7s per retry test. The per-attempt deadline is exercised through a short parent deadline instead of a
  shortened constant.

### Task 6

None. The handler signature, timeout, status codes, and body words match the design.

## Open issues

### Task 4

- Validation runs on the already-trimmed `cfg.BaseURL`, not the raw env value. `BASE_URL=https://` therefore reports
  `got "https:"` after the trailing-slash trim. Keeping the message on the resolved value matches what the rest of the
  app uses; only the quoted value looks odd.
- `url.Parse` accepts userinfo and paths, and only the scheme and host are checked. That is what the design asked for;
  OAuth matching still relies on the Google client registration.

### Task 5

- Every discovery error is retried, including permanent ones such as a 404 discovery path or a malformed document, so
  those startup failures now cost an extra ~700ms. Narrowing the retry to 5xx and network errors would need a typed
  check on the `go-oidc` error.
- `discoverAttemptTimeout` has no test that pins it to 5 seconds; `TestGoogleProviderBoundsAHungDiscoveryAttempt` proves
  a hung request obeys the caller's deadline, not the constant itself.
- A fully hung Google uses the whole budget: three 5-second attempts plus 700ms of backoff, about 15.7s before startup
  fails. That fits Cloud Run's startup window but is much longer than the old single-request failure.

### Task 6

- The test proves the failure path with a closed handle. Nothing exercises the 2-second deadline itself, such as a busy
  `MaxOpenConns(1)` connection; the timeout is only pinned by inspection.
- A closed database answers 503, but `database.Close()` in `run` runs only after `serve` returns, so the probe cannot
  observe that state during a normal shutdown. It is a safety net for a handle that fails early.
- The probe is a write-free `PingContext`, so it does not detect a database that answers but cannot write. Litestream
  replication health is still outside the probe.

## Next-phase notes

### Task 4

- The deploy workflow's pre-auth `BASE_URL` check (same phase, deploy-pipeline task) should mirror this rule: `http` or
  `https` scheme and a non-empty host, evaluated only when `BASE_URL` is set. Keep it in sync with
  `validateBaseURL` so the workflow cannot pass a value startup rejects.
- The OIDC retry task in this phase does not touch `internal/config`; `config.Load` still runs once, before
  `auth.NewGoogleProvider`, on the signal context.

### Task 5

- The `/healthz` task in this phase rewrites the handler and passes the database into `newMux`; it does not touch
  `internal/auth`.
- `NewGoogleProvider` still runs in `cmd/server` before `serve` on the signal context, so a SIGTERM during backoff
  returns promptly with a context error instead of waiting out the retry budget.
- The deploy workflow's pre-flight checks cover configuration and secrets only. They do not probe Google's discovery
  endpoint, and this retry makes that unnecessary at boot.

### Task 6

- The deploy workflow's `/healthz` check now fails on a revision whose database does not answer, not only on one that
  never booted. Keep the retry loop: the first probes can arrive before migrations finish.
- `newMux` has nine parameters now. A later task that needs another handler should group the public handlers instead of
  adding a tenth.
- `healthzPingTimeout` in `cmd/server/main.go` is the single knob if Cloud Run probes need more or less room.

## Files touched

- `cmd/server/main.go`
- `cmd/server/main_test.go`
- `cmd/server/sandbox_test.go`
- `docs/decisions/foundation.md`
- `docs/decisions/google-oauth.md`
- `docs/reference/config.md`
- `docs/reference/routes.md`
- `internal/auth/provider.go`
- `internal/auth/provider_test.go`
- `internal/config/config.go`
- `internal/config/config_test.go`

## Run log

- Mode: 
- Checkpoints: task-4, task-5, task-6
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
