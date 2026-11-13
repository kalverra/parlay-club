# Phase 3: Phase 3: Build group invite links from configured base URL

## Done

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

## Deviations

### Task 7

None. The parameter, trim, precedence, fallback, and doc updates match the design.

## Open issues

### Task 7

- The tests exercise the rendered `/groups` page, not the `inviteURL` method directly, because the package test is
  `groups_test`. The relative-link branch (`r.Host == ""`) stays untested, as it was before this task.
- `BASE_URL` is trusted verbatim beyond the scheme/host check in `config.Load`, so a base URL with a path (say
  `https://example.com/app`) would produce `https://example.com/app/join/{token}`. The app is root-mounted, and the
  config docs already say so.

## Next-phase notes

### Task 7

- The deploy workflow's pre-auth `BASE_URL` check (same phase) still needs to match `validateBaseURL`; this task adds no
  new acceptance rule.
- Any later handler that builds absolute URLs should take the configured base URL rather than `X-Forwarded-Host`, for the
  same reason invite links do: Cloudflare's origin rule rewrites `Host` to the `.run.app` hostname.

## Files touched

- `cmd/server/main.go`
- `cmd/server/sandbox_test.go`
- `docs/concepts/groups.md`
- `docs/runbooks/custom-domain.md`
- `internal/groups/handlers.go`
- `internal/groups/handlers_test.go`

## Run log

- Mode: 
- Checkpoints: task-7
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
