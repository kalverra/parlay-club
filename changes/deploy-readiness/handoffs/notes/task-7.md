# Task 7 notes

## Done

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

None. The parameter, trim, precedence, fallback, and doc updates match the design.

## Open issues

- The tests exercise the rendered `/groups` page, not the `inviteURL` method directly, because the package test is
  `groups_test`. The relative-link branch (`r.Host == ""`) stays untested, as it was before this task.
- `BASE_URL` is trusted verbatim beyond the scheme/host check in `config.Load`, so a base URL with a path (say
  `https://example.com/app`) would produce `https://example.com/app/join/{token}`. The app is root-mounted, and the
  config docs already say so.

## Next-phase notes

- The deploy workflow's pre-auth `BASE_URL` check (same phase) still needs to match `validateBaseURL`; this task adds no
  new acceptance rule.
- Any later handler that builds absolute URLs should take the configured base URL rather than `X-Forwarded-Host`, for the
  same reason invite links do: Cloudflare's origin rule rewrites `Host` to the `.run.app` hostname.
