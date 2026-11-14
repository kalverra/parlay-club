# Phase 4: Phase 4: Record Book calculation and `/record` screen

## Done

### Task 8: Create `internal/record` and season query methods in `internal/db` for `GRADED` slips, streaks, leaderboard (`<6` leg threshold, avg leg odds, Parlay Terrorist badge), and wager dollar P/L, with unit tests covering ties, pushes, and group isolation

- Added season Record Book queries in `internal/db/record.go` (`SeasonsForGroup`, `GradedSeasonsForGroup`, `GradedSlipsForGroupSeason`, `GradedLegsForGroupSeason`, and `RecordUsersForGroupSeason`).
- Added unit tests in `internal/db/record_test.go` covering chronological week ordering, `GRADED`-only filtering, group/season isolation, and retaining users who left a group after authoring a leg on a graded slip.
- Created pure `internal/record` package (`internal/record/record.go`) computing group W-L-P record, net `UnitsPL`, chronological `LongestWin`/`LongestLoss` streaks with `PUSH` resets, member leaderboard (`MinDecidedLegs = 6` threshold, arithmetic-mean decimal leg odds converted to American, `ParlayTerrorist` tie handling, and wager `DollarPL` distinguishing `nil` unlogged from `$0.00`).
- Added unit tests in `internal/record/record_test.go` covering empty/ungraded filtering, out-of-order streak calculation with `PUSH` resets, `<6` decided legs threshold, `VOID` exclusion from average odds, multi-loser vs solo-loser Parlay Terrorist ties, and wager dollar P/L edge cases.

### Task 9: Wire `GET /record?season=` in `internal/web` and replace `web/templates/record.html` with the phone-first Record Book view, verify `sandbox` week 9 stages (`AllClear` / `SundayComplete`), and cover routes/templates with tests

- Wired `GET /record?season=` in `internal/web/record.go` and `internal/web/slips.go` (removing the old static `pageRoutes` shell handler in `internal/web/routes.go`), defaulting to the latest season with `GRADED` slips (or the active group's latest season / latest week / current year) and rejecting invalid `?season=` values with `400 Bad Request`.
- Replaced `web/templates/record.html` with the phone-first Record Book view (season selector, empty states, 2x2/4-col summary cards for W-L-P record, Units P/L, win/loss streaks, and leaderboard with mobile cards below `sm:` and a table at `sm:+`, Parlay Terrorist badge, and explicit subtext distinguishing active-leg Units P/L from logged-wager Dollar P/L) and rebuilt `web/static/app.css`.
- Added `internal/web/record_test.go` covering empty states, invalid season handling, default season selection, `?season=` switching, leaderboard/Parlay Terrorist/Dollar P/L rendering, and group isolation, and updated `internal/web/layout_test.go`.
- Verified `sandbox` week 9 stages (`AllClear` and `SundayComplete`) end-to-end with `TestSandboxRecordBookReflectsGradedStages` in `cmd/server/sandbox_test.go`.

### Task 10: Docs: update `docs/reference/routes.md` and `docs/` (`good-vibes docs lint`, `good-vibes docs index`) for the Record Book screen and grading/wager specs

- Updated [routes.md](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/grading-record-book/docs/reference/routes.md) to replace the `/record` placeholder with the `GET /record?season=` route specification, season fallback order, `400` validation behavior, and `sources` front matter (`internal/web/record.go`, `internal/record/record.go`, `web/templates/record.html`).
- Created [record-book.md](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/grading-record-book/docs/concepts/record-book.md) documenting season selection, group scoping, chronological win/loss streaks with `PUSH` resets, active-leg Units P/L, the member leaderboard (`MinDecidedLegs = 6` threshold, avg leg odds, Parlay Terrorist ties, and personal wager Dollar P/L), and verified that [slips.md](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/grading-record-book/docs/concepts/slips.md) and [data-refresh.md](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/grading-record-book/docs/concepts/data-refresh.md) already cover auto-grading and wagers.
- Ran `good-vibes docs index` (updating `docs/llms.txt`) and `good-vibes docs lint` (passed `ok`).

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 8

- `Store.SeasonsForGroup`, `Store.GradedSeasonsForGroup`, `Store.GradedSlipsForGroupSeason`, `Store.GradedLegsForGroupSeason`, `Store.RecordUsersForGroupSeason`, and `Store.WagersForGroupSeason` provide all inputs needed for `GET /record?season=` in Task 9 to call `record.Build(...)`.

### Task 9

- Task 10 can update `docs/reference/routes.md` and `docs/concepts/` for `GET /record?season=` and the Record Book view.

### Task 10

- All Phase 4 tasks are complete and docs are indexed and linted.

## Files touched

- `cmd/server/sandbox_test.go`
- `docs/concepts/record-book.md`
- `docs/llms.txt`
- `docs/reference/routes.md`
- `internal/db/record.go`
- `internal/db/record_test.go`
- `internal/record/record.go`
- `internal/record/record_test.go`
- `internal/web/layout_test.go`
- `internal/web/record.go`
- `internal/web/record_test.go`
- `internal/web/routes.go`
- `internal/web/slips.go`
- `web/static/app.css`
- `web/templates/record.html`

## Run log

- Mode: 
- Checkpoints: task-8, task-9, task-10
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
