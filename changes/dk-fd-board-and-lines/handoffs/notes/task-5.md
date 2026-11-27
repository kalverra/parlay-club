# Task 5

## Done

- Updated `docs/concepts/slips.md` (which owns the pick board and Research concept sections) to document `board.BuildDualBook` (DK ∩ FD pickable filtering), explicit `DK`/`FD` split-line badges ordered by `homeBook` instead of median averaging, and `board.LineKey` grouping for totals when DK and FD differ by half a point.
- Updated `docs/concepts/export.md` to document export card sportsbook ordering with the viewer's `homeBook` first and the DraftKings / FanDuel / No preference Profile selector.
- Updated `docs/reference/routes.md` for `POST /me` validation (`draftkings`, `fanduel`, or blank for No preference, returning 422 on other values).
- Verified `good-vibes docs lint` passes with 0 errors and ran `good-vibes docs index`.

## Deviations

- `docs/concepts/pick-board.md` does not exist in this repository; updated `docs/concepts/slips.md` (which documents the pick board and Research) and `docs/reference/routes.md` (`POST /me`) alongside `docs/concepts/export.md`.

## Open issues

None.

## Next-phase notes

- All Phase 2 tasks are complete; living docs now match the dual-book board, split-line UI, Profile selector, and export ordering.

## Commit

Document DK and FD dual-book board and export UX

Keep living concept and route docs aligned with the dual-book pickable filter, split-line display, and DraftKings/FanDuel home-book preference across the pick board, Research, Profile, and export card.
