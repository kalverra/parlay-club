# Final gate note

## Fixes

- Updated `docs/reference/data-sources.md` (`Bulk props` and `Budget > Parlay API`) to document `prefill:gameday`, date-scoped automatic `RunPrefill` prop warming (with non-game-day manual fallback), and scoreboard-derived `odds:lines` staleness windows.
- Updated `docs/reference/routes.md` (`POST /internal/prefill`) to describe per-game-day pre-fill, date-scoped prop warming, and `prefill:gameday`.
- Updated `docs/guides/gcp-setup.md` (steps 3 and 8) and `docs/runbooks/deploy.md` (`Preconditions` and `Spend`) to replace `prefill:sunday` / Sunday-only wording with `prefill:gameday` and update the step-8 heading anchor, plus added a `budget` span override with reason to `docs/runbooks/deploy.md` so `good-vibes docs lint` passes with zero errors.

## Verification

- `good-vibes docs lint`: 0 errors (0 warnings on modified docs).
- `good-vibes check`: 0 errors.
- `good-vibes verify --exec -- go test -short -race ./...`: passed across all 29 packages.

## Commit

Update prefill docs for per-game-day sync

The reference, setup guide, and deploy runbook docs still referenced the removed `prefill:sunday` key and weekday-hardcoded `odds:lines` windows after the switch to scoreboard-derived game-day pre-fill. Aligning these living docs with `internal/refresh` prevents operators from configuring or troubleshooting against stale behavior.
