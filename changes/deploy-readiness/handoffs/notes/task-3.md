# Task 3 notes

## Done

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

## Deviations

None. Task 1 and task 2 had already added draft shutdown prose to both docs; this task checked it against the code and
completed it rather than duplicating it.

## Open issues

- None for the docs. `good-vibes check` still reports the pre-existing stale-source warnings in access, analyzer,
  export, gcp setup, local dev, and other docs; they are outside this change.

## Next-phase notes

- Phase 2 rewrites `GET /healthz` and passes a database handle into `newMux`. Keep the `shutdownHooks` value and the
  `serve` signature as they are; `beforeHTTP` and `afterHTTP` are the only shutdown seam now.
- `config.Load` and `auth.NewGoogleProvider` both run before `serve` on the same signal context, so a SIGTERM during
  startup already cancels OIDC discovery. No extra drain work is needed there.
- The docs now say a drain timeout exits the process non-zero (`context.DeadlineExceeded` from `Shutdown`). Keep that
  behavior if later tasks touch `serve`'s error handling.
