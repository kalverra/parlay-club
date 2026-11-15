# Task 1: Sunday pre-fill window and staleness rules

## Done

- Added `SundayPrefillSource = "prefill:sunday"` and `sundayPrefillStartHour = 6` in [refresh.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/first-sunday-visit-prefill/internal/refresh/refresh.go).
- Added `sundayPrefillWindow` and `sundayPrefillStale` in [rules.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/first-sunday-visit-prefill/internal/refresh/rules.go) using the injected `eastern` (`America/New_York`) location and `lastKickoffOn`.
- Added table-driven unit tests `TestSundayPrefillWindow` and `TestSundayPrefillStale` in [rules_test.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/first-sunday-visit-prefill/internal/refresh/rules_test.go), covering weekday/Saturday/early-Sunday exclusions, last-Sunday-kickoff cutoffs with remaining Monday games, empty/Sunday-less slates, and DST calendar-date boundaries.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `SundayPrefillSource`, `sundayPrefillWindow`, and `sundayPrefillStale` are ready for `Service.kick` and `Service.RunPrefill` in Task 2.
