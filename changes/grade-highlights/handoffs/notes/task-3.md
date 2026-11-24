# Task 3: Add `POST /slips/{id}/legs/batch`

## Done

- Added `Service.AddLegs` in `internal/slips/rules.go` to validate `OPEN` slip status, unlocked member state, duplicate selections (against existing member legs and within the batch), remaining `LegsPerMember` capacity, and kickoff/slate scope inside one SQLite transaction, publishing `slip:{id}` (`slip-updated`) when legs are inserted.
- Registered `POST /slips/{id}/legs/batch` in `slipHandlers.Routes` and implemented `addLegsBatch` plus `legsFromBatchForm` (sharing `legFromValues` with `legFromForm`) in `internal/web/slips.go`.
- Returned `#slip-panel` by default, `#pick-body` for `from=board`/`from=pick`, and `#pick-dock` for `from=dock`, with corresponding `Hx-Trigger` headers including `slip-updated`.
- Added handler tests in `internal/web/slips_test.go` covering atomic insertion, SSE `slip-updated` broadcast, duplicate skipping, over-capacity rejection, kickoff rollback, locked member refusal, and `from=board` partial rendering.

## Deviations

- All slip routes are registered in `slipHandlers.Routes` inside `internal/web/slips.go` rather than `internal/web/routes.go`, which only calls `slipRoutes.Routes(mux)`.
- Transactional validation and insertion (`Service.AddLegs`) were placed in `internal/slips/rules.go` to match the repository pattern where `internal/web` handlers stay thin and `internal/slips` owns rules and `s.notify`.

## Open issues

None.

## Next-phase notes

Combo highlight forms should submit indexed fields `legs[0][game_id]`, `legs[0][market_type]`, `legs[0][team]`, `legs[0][player_name]`, `legs[0][player_id]`, `legs[0][side]`, `legs[0][line]`, `legs[0][odds]`, etc. (and `from=board` when triggered from `#pick-detail`).

## Commit

Add batch leg insertion endpoint for parlay combos

Support one-tap adding of 2- and 3-leg A-grade parlay highlights from the hub and pick board without partial inserts or duplicate picks. Validating capacity, duplicates, and kickoff locks in a single transaction keeps the slip state consistent and broadcasts one update to connected clients.
