## Done

- Updated `docs/concepts/data-refresh.md` with the viewer-driven `KickLive` / `SyncLive` 20-second `liveFastAge`
  cadence, zero paid odds guarantee, `FINAL` auto-grading, and 10-second per-slip `ForceLiveRefresh` cooldown.
- Updated `docs/concepts/live-updates.md` with the `## Overview` Mermaid diagram and `WithLiveTick` 25-second SSE tick
  lifecycle (including sandbox disablement and stop on disconnect, `bye` deadline, or `Shutdown`).
- Updated `docs/reference/routes.md` with `POST /slips/{id}/live/refresh` and the `GET /slips/{id}/events` live tick
  behavior.
- Verified with `good-vibes docs lint` (0 errors).

## Deviations

None.

## Open issues

None.

## Next-phase notes

Phase 2 documentation is complete; Phase 3 can proceed with the dedicated Live view templates, game-time ordering, and
overview spotlight.

## Commit

Document fast live refresh, SSE ticks, and route

Keep the living documentation aligned with the Phase 2 viewer-driven live refresh cadence, SSE tick lifecycle, and
manual refresh endpoint before Phase 3 builds the dedicated Live view UI.
