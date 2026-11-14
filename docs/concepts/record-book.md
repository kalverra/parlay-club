---
id: record-book
title: Record Book
type: concept
summary: How a group's season Record Book aggregates graded slips, chronological win/loss streaks, active-leg Units P/L, member leaderboard rankings, the Parlay Terrorist badge, and personal wager Dollar P/L.
tags: [record-book, record, leaderboard, streaks, units, wagers, parlay-terrorist, grading, seasons]
sources: [internal/record/record.go, internal/db/record.go, internal/db/wagers.go, internal/web/record.go, web/templates/record.html]
last_verified: 2026-11-14
---

# Record Book

## Summary

The Record Book (`GET /record?season=`) turns a group's `GRADED` slips for one NFL season into a group summary and a
member leaderboard. All math lives in the pure `internal/record` package (`record.Build`), with no database, HTTP, or
clock dependencies.

## Season selection and group scope

Every query (`internal/db/record.go`) is scoped to the active group:

- `SeasonsForGroup` and `GradedSeasonsForGroup` list seasons with slips (newest first) for the season selector.
- When `?season=` is omitted, `GET /record` defaults to the latest season with `GRADED` slips, falling back to the
  latest season with any slip, the latest loaded week's season, or the current calendar year. An invalid or non-positive
  `?season=` returns `400 Bad Request`.
- `RecordUsersForGroupSeason` includes every current group member and any user who authored a leg on a `GRADED` slip in
  that group and season, so members who leave a group keep their historical stats on the leaderboard.

## Group totals and streaks

`record.Build` considers only `GRADED` slips with a `WON`, `LOST`, or `PUSH` result:

- **Group record and Units P/L:** Counts `Wins`, `Losses`, and `Pushes`, and sums `parlay.Units(slip.Result, parlay.CombinedOdds(activeLegs))`
  across all graded slips. Units P/L uses active leg odds (`VOID` and `PUSH` legs drop out), independent of personal
  wager odds.
- **Chronological streaks:** Slips are sorted by `(Season, SeasonType, WeekNum, CreatedAt, ID)`. `LongestWin` and
  `LongestLoss` track the longest consecutive runs of `WON` and `LOST` slips. A `PUSH` slip resets both running streak
  counters to `0`.

## Member leaderboard

Every user with at least one leg on a `GRADED` slip in the season gets a row:

- **Hit% and qualification (`MinDecidedLegs = 6`):** `HitPct` is `WON / (WON + LOST) * 100` once a member reaches 6
  decided legs. Below 6 decided legs, `HitPct` renders `"–"` and the member sorts after all qualified members. Ordering
  is qualified first by `HitPct` descending, then `WonLegs` descending, `DecidedLegs` descending, `Name` ascending
  (case-insensitive), and `UserID` ascending.
- **Avg leg odds:** Arithmetic mean of `parlay.AmericanToDecimal(leg.Odds)` over `WON`, `LOST`, and `PUSH` legs
  (excluding `VOID`, `PENDING`, and `NEEDS_GRADING`), converted back via `parlay.DecimalToAmerican`.
- **Parlay Terrorist:** Counts `GRADED` slips with `Result == LOST` where *exactly one* leg was `LOST`, credited to that
  leg's owner. Multi-loser slips do not count. Every member tied for the highest count (`> 0`) receives the Parlay
  Terrorist badge.
- **Personal Dollar P/L:** Sums net wager outcome across the member's logged wagers on `GRADED` slips (`WON` adds
  `parlay.Payout(stake, parlay.AmericanToDecimal(w.Odds), w.BoostPct) - stake`, `LOST` subtracts `stake`, and `PUSH`
  adds `0`). Members with zero logged wagers show `"–"`, distinct from `"$0.00"`.

On phones (below `sm:`), each member renders as a compact card; at `sm:` and wider, the leaderboard renders as a table.

## Related

- [Slips](slips.md): auto-grading, manual overrides, and personal wagers.
- [Groups](groups.md): how the active group scopes the Record Book.
- [Odds math](../reference/odds-math.md): `CombinedOdds`, `Payout`, and `Units` formulas.
- [Routes](../reference/routes.md#record-book): `GET /record?season=` behavior and status codes.
- [Database schema](../reference/schema.md): `slips`, `legs`, and `wagers` tables.
