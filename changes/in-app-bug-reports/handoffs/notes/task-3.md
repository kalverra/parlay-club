# Task 3: Docs: update `docs/reference/config.md` with GitHub issue reporting environment variables

## Done

- Updated `docs/reference/config.md` with `GITHUB_TOKEN` and `GITHUB_REPO` in the `## Variables` table, including
  defaults, whitespace trimming, and optional behavior in dev and prod.
- Updated `docs/reference/config.md` front matter (`summary`, `tags`, `last_verified`) and regenerated `docs/llms.txt`.
- Verified `good-vibes docs lint` (`ok`) and `good-vibes docs index`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Phase 2 wires `internal/bugs` into `cmd/server` and `internal/web` (`POST /bug-reports` and
  `GET /screenshots/{token}.jpg`).
- Remember to update `docs/reference/routes.md` and `docs/runbooks/deploy.md` in the later documentation tasks once the
  HTTP endpoints and deployment env vars are wired up.
