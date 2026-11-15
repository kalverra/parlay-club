# Task 6

## Done

- Updated `docs/concepts/data-refresh.md` to document `SyncPickLines`, `SyncPickGame`, `ForcePickRefresh`, pick-time
  `OPEN`-slip gating, the `15m`/`2h` lines and `30m`/`2h` props staleness rules, and the `2m` manual refresh cooldown.
- Updated `docs/concepts/slips.md` to document pick-time sync gating on `OPEN` slips with upcoming games, the
  phone-first "Refresh odds" button (`POST /slips/{id}/pick/refresh`), and read-only behavior on locked slips and
  `/pick/slate`.
- Updated `docs/reference/data-sources.md` to document the `30m`/`2h` props window, pick-time `15m`/`2h` lines window,
  `2m` manual cooldown, and `RunPrefill` Parlay-only props sweep in the Parlay API and Budget sections.
- Updated `docs/reference/routes.md` to add `POST /slips/{id}/pick/refresh` (`422` when not `OPEN`, `2m` cooldown,
  `#pick-body` response) and pick-time sync rules for `/slips/{id}/pick`, `/pick/results`, `/pick/detail`, and
  `/pick/slate`.
- Removed retired `sources` and `last_verified` front-matter fields from the four updated docs so `good-vibes docs lint`
  passes with zero errors or warnings on all four files.

## Deviations

None. (`docs/reference/config.md`, modified in an earlier task, still reports a retired front-matter error in
`good-vibes docs lint` and was left untouched as outside this task.)

## Open issues

None.

## Next-phase notes

- Phase 2 is complete.
