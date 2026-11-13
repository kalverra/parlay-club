# Task 4 notes

## Done

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

## Deviations

None. The helper, message, and "validate whenever non-empty" rule match the design.

## Open issues

- Validation runs on the already-trimmed `cfg.BaseURL`, not the raw env value. `BASE_URL=https://` therefore reports
  `got "https:"` after the trailing-slash trim. Keeping the message on the resolved value matches what the rest of the
  app uses; only the quoted value looks odd.
- `url.Parse` accepts userinfo and paths, and only the scheme and host are checked. That is what the design asked for;
  OAuth matching still relies on the Google client registration.

## Next-phase notes

- The deploy workflow's pre-auth `BASE_URL` check (same phase, deploy-pipeline task) should mirror this rule: `http` or
  `https` scheme and a non-empty host, evaluated only when `BASE_URL` is set. Keep it in sync with
  `validateBaseURL` so the workflow cannot pass a value startup rejects.
- The OIDC retry task in this phase does not touch `internal/config`; `config.Load` still runs once, before
  `auth.NewGoogleProvider`, on the signal context.
