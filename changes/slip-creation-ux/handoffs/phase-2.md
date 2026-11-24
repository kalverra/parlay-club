# Phase 2: Phase 2: Unified one-off/recurring create-slip UI with game checklist and slate shortcuts

## Done

### Task 3: Update `GET /slips/new`, add `GET /slips/new/games`, and update `POST /slips` in `internal/web/slips.go` to restrict to `UpcomingWeeks`, remove past-week entry, handle `mode=one_off` (per-game selection) and `mode=recurring` (schedule creation + `EnsureWeekSlips`), and update `internal/web/slips_test.go`.

The worker wrote no notes.

### Task 4: Redesign the create-slip form in `web/templates/slip.html` and add mode-toggle and slate-shortcut checkbox helpers in `web/static/app.js` for phone-first usability (One-off/Recurring toggle, dynamic weekday shortcuts, game checklist with kickoffs, live selected count).

The worker wrote no notes.

### Task 5: Docs: update `docs/concepts/slips.md` to document upcoming-only slip creation, unified one-off vs. recurring creation, and custom per-game slip eligibility.

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
- Checkpoints: task-3, task-4, task-5
- Verify: verify passed: generate reused, lint reused, test 600ms, actionlint reused, deploy-blackout reused
- Rounds used: 1 of 2; verify failures: 0
