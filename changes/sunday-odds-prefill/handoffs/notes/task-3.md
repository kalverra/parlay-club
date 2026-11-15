# Task 3

## Done

- Amended the free-tier guardrails in `AGENTS.md` to allow the Sunday morning pre-fill (`POST /internal/prefill`) and pick-time line/prop refreshes while scoping `Kick()` to free/unmetered sources only.
- Updated `docs/concepts/data-refresh.md` for `RunPrefill` (scoreboard -> `odds:lines` -> free sources -> Parlay-only bulk props with 15m idempotency), `Kick()` vs `RunOnce()` scope, promoted `espn:summary` and deduped week-team `espn:roster` syncs, and tightened free-source staleness windows.
- Updated `docs/reference/config.md` to document optional `PREFILL_TOKEN` and its fail-closed bearer auth behavior.
- Updated `docs/reference/routes.md` to document `POST /internal/prefill`, its outer-mux placement outside cookie/CSRF middleware, and its `401`/`404`/`503`/`200`/`500` responses.
- Verified documentation with `good-vibes docs lint` (`ok`).

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Phase 2 will add pick-time on-demand refreshes (`SyncPickLines`, `SyncPickGame`, `ForcePickRefresh`, `POST /slips/{id}/pick/refresh`) and update `docs/concepts/data-refresh.md` and `docs/reference/routes.md` for those endpoints and staleness windows.
