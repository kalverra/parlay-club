# Phase 1: Phase 1: Custom slip game eligibility and upcoming-weeks storage

## Done

### Task 1: Add migration `0017_slip_games.sql`, `SlateScopeCustom`, `ErrSlipGamesRequired`, `Slip.GameIDs` hydration, `CreateSlip` custom-game persistence, and `Store.UpcomingWeeks(ctx, now)` in `internal/db/slips.go` and `internal/db/weeks.go`, with store tests.

The worker wrote no notes.

### Task 2: Add `slips.FilterGamesForSlip(games, slip)` in `internal/slips/next.go`, switch `NextStep`, `internal/web/pick.go`, and `internal/web/slips.go` leg validation/pickers to use it, and add `"N Games"` scope badge formatting, with unit and handler tests.

The worker wrote no notes.

## Deviations

None.

## Open issues

None.

## Next-phase notes

None.

## Files touched

None.

## Run log

- Mode: 
- Checkpoints: task-1, task-2
- Verify: verify passed: generate 800ms, lint 2.8s, test 400ms, actionlint 200ms, deploy-blackout 1.1s
- Rounds used: 1 of 2; verify failures: 0
