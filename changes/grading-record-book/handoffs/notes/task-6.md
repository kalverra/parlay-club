# Task 6 Notes

## Done

- Added `internal/db/wagers.go` (`Wager`, `UpsertWager`, `DeleteWager`, `WagerForUser`, `WagersForSlip`, `WagersForGroupSeason`) and DB tests in `internal/db/wagers_test.go`.
- Added `slips.Service.SetWager` and `slips.Service.DeleteWager` in `internal/slips/wagers.go` enforcing group membership, finite `Stake > 0`, valid American `Odds` (`<= -100` or `>= 100`), `BoostPct >= 0`, trimmed `Book <= 40` runes, and live `slip:{id}` notifications, covered by `internal/slips/wagers_test.go`.
- Added `PUT /slips/{id}/wagers/me` and `DELETE /slips/{id}/wagers/me` handlers, `WagerView`, and `wagerView` in `internal/web/slips.go`, registered `"slip-wager"` in `internal/web/render.go`, and added HTTP tests in `internal/web/wagers_test.go`.
- Added the `slip-wager` **"My wager"** HTMX partial in `web/templates/partials.html` and included it in `web/templates/slip.html` below `slip-payout`.

## Deviations

- Placed the **"My wager"** form as its own `slip-wager` partial in `web/templates/slip.html` (alongside `slip-payout`) rather than inside `slip-panel`, so live `slip-updated` panel refreshes do not wipe out in-progress wager inputs.

## Open issues

None.

## Next-phase notes

- `Store.WagersForGroupSeason(ctx, groupID, season)` is ready for `internal/record` in Phase 4.
