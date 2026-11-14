# Phase 3: Phase 3: Per-member wager logging

## Done

### Task 6: Add `internal/db/wagers.go`, `slips.Service.SetWager` / `DeleteWager`, `PUT` / `DELETE /slips/{id}/wagers/me` in `internal/web`, and the **"My wager"** form in `partials.html`, covered by DB, service, and HTTP tests

- Added `internal/db/wagers.go` (`Wager`, `UpsertWager`, `DeleteWager`, `WagerForUser`, `WagersForSlip`, `WagersForGroupSeason`) and DB tests in `internal/db/wagers_test.go`.
- Added `slips.Service.SetWager` and `slips.Service.DeleteWager` in `internal/slips/wagers.go` enforcing group membership, finite `Stake > 0`, valid American `Odds` (`<= -100` or `>= 100`), `BoostPct >= 0`, trimmed `Book <= 40` runes, and live `slip:{id}` notifications, covered by `internal/slips/wagers_test.go`.
- Added `PUT /slips/{id}/wagers/me` and `DELETE /slips/{id}/wagers/me` handlers, `WagerView`, and `wagerView` in `internal/web/slips.go`, registered `"slip-wager"` in `internal/web/render.go`, and added HTTP tests in `internal/web/wagers_test.go`.
- Added the `slip-wager` **"My wager"** HTMX partial in `web/templates/partials.html` and included it in `web/templates/slip.html` below `slip-payout`.

### Task 7: Docs: update `docs/reference/routes.md` and `docs/concepts/slips.md` for wager endpoints and behavior

- Updated `docs/reference/routes.md` (`last_verified: 2026-11-14`, `sources`, and `tags`) to document `PUT /slips/{id}/wagers/me` and `DELETE /slips/{id}/wagers/me`, including request fields, group-membership access, 422 validation rules, and the `slip-wager` partial response.
- Updated `docs/concepts/slips.md` (`sources`, `tags`, `## The hub`, typed errors table, and a new `## Wagers` section) to describe the **My wager** form (`slip-wager`), its placement outside `slip-panel`, validation/default rules, live `slip:{id}` publishing, and `ErrWager*` errors.
- Verified documentation front matter and links with `good-vibes docs lint` (`ok`).

## Deviations

### Task 6

- Placed the **"My wager"** form as its own `slip-wager` partial in `web/templates/slip.html` (alongside `slip-payout`) rather than inside `slip-panel`, so live `slip-updated` panel refreshes do not wipe out in-progress wager inputs.

## Open issues

None.

## Next-phase notes

### Task 6

- `Store.WagersForGroupSeason(ctx, groupID, season)` is ready for `internal/record` in Phase 4.

### Task 7

- No code changes were made in this task; all docs match the Phase 3 wager implementation in `internal/db/wagers.go`, `internal/slips/wagers.go`, `internal/web/slips.go`, `web/templates/slip.html`, and `web/templates/partials.html`.

## Files touched

- `docs/concepts/slips.md`
- `docs/reference/routes.md`
- `internal/db/wagers.go`
- `internal/db/wagers_test.go`
- `internal/slips/wagers.go`
- `internal/slips/wagers_test.go`
- `internal/web/render.go`
- `internal/web/slips.go`
- `internal/web/wagers_test.go`
- `web/templates/partials.html`
- `web/templates/slip.html`

## Run log

- Mode: 
- Checkpoints: task-6, task-7
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
