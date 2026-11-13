# Task 5 notes

## Done

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

## Deviations

- The design suggested "expose an unexported helper if tests inject delays". The tests live in `package auth_test`, so
  they cannot set unexported timings. The constants stay fixed and the tests use the real 200ms/500ms backoff, which
  costs about 0.7s per retry test. The per-attempt deadline is exercised through a short parent deadline instead of a
  shortened constant.

## Open issues

- Every discovery error is retried, including permanent ones such as a 404 discovery path or a malformed document, so
  those startup failures now cost an extra ~700ms. Narrowing the retry to 5xx and network errors would need a typed
  check on the `go-oidc` error.
- `discoverAttemptTimeout` has no test that pins it to 5 seconds; `TestGoogleProviderBoundsAHungDiscoveryAttempt` proves
  a hung request obeys the caller's deadline, not the constant itself.
- A fully hung Google uses the whole budget: three 5-second attempts plus 700ms of backoff, about 15.7s before startup
  fails. That fits Cloud Run's startup window but is much longer than the old single-request failure.

## Next-phase notes

- The `/healthz` task in this phase rewrites the handler and passes the database into `newMux`; it does not touch
  `internal/auth`.
- `NewGoogleProvider` still runs in `cmd/server` before `serve` on the signal context, so a SIGTERM during backoff
  returns promptly with a context error instead of waiting out the retry budget.
- The deploy workflow's pre-flight checks cover configuration and secrets only. They do not probe Google's discovery
  endpoint, and this retry makes that unnecessary at boot.
