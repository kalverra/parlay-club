# Task 9 Notes

## Done

- Wired `GET /record?season=` in `internal/web/record.go` and `internal/web/slips.go` (removing the old static `pageRoutes` shell handler in `internal/web/routes.go`), defaulting to the latest season with `GRADED` slips (or the active group's latest season / latest week / current year) and rejecting invalid `?season=` values with `400 Bad Request`.
- Replaced `web/templates/record.html` with the phone-first Record Book view (season selector, empty states, 2x2/4-col summary cards for W-L-P record, Units P/L, win/loss streaks, and leaderboard with mobile cards below `sm:` and a table at `sm:+`, Parlay Terrorist badge, and explicit subtext distinguishing active-leg Units P/L from logged-wager Dollar P/L) and rebuilt `web/static/app.css`.
- Added `internal/web/record_test.go` covering empty states, invalid season handling, default season selection, `?season=` switching, leaderboard/Parlay Terrorist/Dollar P/L rendering, and group isolation, and updated `internal/web/layout_test.go`.
- Verified `sandbox` week 9 stages (`AllClear` and `SundayComplete`) end-to-end with `TestSandboxRecordBookReflectsGradedStages` in `cmd/server/sandbox_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Task 10 can update `docs/reference/routes.md` and `docs/concepts/` for `GET /record?season=` and the Record Book view.
