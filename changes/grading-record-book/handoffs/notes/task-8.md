# Task 8 Notes

## Done

- Added season Record Book queries in `internal/db/record.go` (`SeasonsForGroup`, `GradedSeasonsForGroup`, `GradedSlipsForGroupSeason`, `GradedLegsForGroupSeason`, and `RecordUsersForGroupSeason`).
- Added unit tests in `internal/db/record_test.go` covering chronological week ordering, `GRADED`-only filtering, group/season isolation, and retaining users who left a group after authoring a leg on a graded slip.
- Created pure `internal/record` package (`internal/record/record.go`) computing group W-L-P record, net `UnitsPL`, chronological `LongestWin`/`LongestLoss` streaks with `PUSH` resets, member leaderboard (`MinDecidedLegs = 6` threshold, arithmetic-mean decimal leg odds converted to American, `ParlayTerrorist` tie handling, and wager `DollarPL` distinguishing `nil` unlogged from `$0.00`).
- Added unit tests in `internal/record/record_test.go` covering empty/ungraded filtering, out-of-order streak calculation with `PUSH` resets, `<6` decided legs threshold, `VOID` exclusion from average odds, multi-loser vs solo-loser Parlay Terrorist ties, and wager dollar P/L edge cases.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `Store.SeasonsForGroup`, `Store.GradedSeasonsForGroup`, `Store.GradedSlipsForGroupSeason`, `Store.GradedLegsForGroupSeason`, `Store.RecordUsersForGroupSeason`, and `Store.WagersForGroupSeason` provide all inputs needed for `GET /record?season=` in Task 9 to call `record.Build(...)`.
