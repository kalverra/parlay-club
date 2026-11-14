## Done

- Updated [routes.md](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/grading-record-book/docs/reference/routes.md) to replace the `/record` placeholder with the `GET /record?season=` route specification, season fallback order, `400` validation behavior, and `sources` front matter (`internal/web/record.go`, `internal/record/record.go`, `web/templates/record.html`).
- Created [record-book.md](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/grading-record-book/docs/concepts/record-book.md) documenting season selection, group scoping, chronological win/loss streaks with `PUSH` resets, active-leg Units P/L, the member leaderboard (`MinDecidedLegs = 6` threshold, avg leg odds, Parlay Terrorist ties, and personal wager Dollar P/L), and verified that [slips.md](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/grading-record-book/docs/concepts/slips.md) and [data-refresh.md](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/grading-record-book/docs/concepts/data-refresh.md) already cover auto-grading and wagers.
- Ran `good-vibes docs index` (updating `docs/llms.txt`) and `good-vibes docs lint` (passed `ok`).

## Deviations

None.

## Open issues

None.

## Next-phase notes

- All Phase 4 tasks are complete and docs are indexed and linted.
