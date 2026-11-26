# Phase 3: Phase 3: Slip event notifications

## Done

### Task 6: Add the `slips.Notifier` seam and `Event`. Raise `legs_added` from `AddLeg` and `AddLegs`, `slip_locked` from the final `LockIn`, and `slip_graded` from `SetLegResult` and `AutoGradeLegs`, each only after commit. Implement `Notify` in `internal/push`: recipients, message text in the app's voice, tag and URL, and graded dedupe through `push_log`. Wire it in `cmd/server`, as an untyped nil when push is off. Filter recipients to active users. Test with a recording notifier and push service tests.

- `internal/slips` gained `EventKind`, `Event`, and the `Notifier` seam. `AddLeg` and `AddLegs` raise `legs_added`,
  the final `LockIn` raises `slip_locked`, and `SetLegResult` and `AutoGradeLegs` raise `slip_graded` only on a move
  into `GRADED`, each after commit and never on a rollback.
- `push.Service.Notify` loads the slip, builds the message in the app's voice, sends to active slip members only, and
  leaves the actor out of `legs_added` and `slip_locked`.
- `push_log` gained `db.ReservePushLog` with the `PushLogGraded` and `PushLogReminder` kind constants. A grade reserves
  its slip-wide row before sending, so a later manual correction stays quiet.
- `cmd/server` sets the notifier from the shared push service, passing an untyped nil when push is off.
- `SendToUsers` now gives each concurrent send its own copy of the payload. webpush-go grows its buffer from the
  caller's slice, so the shared slice raced across devices; the design's `-race` check caught it.
- Tests cover the event seam with a recording notifier, plus push recipients, inactive users, non-members, one grade
  push across two passes, and every message body. `docs/concepts/notifications.md` and `docs/concepts/slips.md` describe
  the events and the seam.

### Task 7: Docs: add the events table to `docs/concepts/notifications.md` and the Notifier seam beside `Publisher` in `docs/concepts/slips.md`.

- `docs/concepts/slips.md` "Editing and locking" now describes `Publisher` and `Notifier` side by side: both are optional
  seams called after the commit, with `Event`'s fields and the three raise conditions. The seam mention that was rule 10
  folded into this paragraph, so the numbered rules are back to 1–9.
- `docs/concepts/notifications.md` "Slip events" links the seam to `slips.md#editing-and-locking` and names
  `SetNotifier` as the wiring point. The Sending section no longer points at reminders as future work.
- The events table itself landed with task 6. This task checked every row, recipient rule, tag, URL, and grade dedupe
  against `internal/push/notify.go` and `internal/slips/rules.go`; no corrections were needed.
- The notifications summary now names slip events, and `docs/llms.txt` was regenerated with `good-vibes docs index`.
- Raised the `slips.md` line budget from 575 to 585, since the doc was already at 575 and the seam paragraph adds content.

## Deviations

### Task 6

- The notifier arrives through `slips.Service.SetNotifier` rather than the constructor. `newProdServices` builds the
  slip service before `cmd/server` builds the push service, so a setter is the one place both branches can wire it.
- The `push_log` reservation helper lives in `internal/db` beside the other push storage, not in `internal/push`. The
  design sketch put the row insert near the sender; this repo keeps SQL in the store.

### Task 7

- The task asks to add the events table, but task 6 had already written it in `notifications.md`. Rewriting it would have
  churned accurate prose, so this task verified it against the code and added the missing seam cross-link instead.
- `slips.md` had no `Publisher` prose to sit beside; the seam was only implied by "publishes `slip:{id}`" and described
  in `live-updates.md`. The new paragraph covers both seams together, which is the closest reading of the task.

## Open issues

### Task 6

- A slip event can add up to the 3 second send cap to the request that triggered it. That matches the design's
  accepted latency risk; the cap in `internal/push` is the dial.
- `AutoGradeLegs` raises `slip_graded` inside the page-load refresh pass, so a grade push can wait for the next visit.
  That is the same staleness the hub already has and is called out in the design risks.
- `push_log` reminder reservations are still unused. Phase 4 owns them.

### Task 7

- Reminders are still undescribed in `notifications.md`. Phase 4 owns that section, the `push_log` reminder kind, and
  the task lifecycle.

## Next-phase notes

### Task 6

- Use `db.ReservePushLog(ctx, db.PushLogReminder, slipID, userID, key)` before each reminder send. It reports false
  when the row already exists, so a Cloud Tasks retry sends nothing.
- `push.Service.Notify` is the shape to copy for reminders: load rows, decide recipients, `SendToUsers`, and keep the
  message builders pure so text is testable without decrypting the push payload.
- `internal/push` test helpers now record the request `Path`, so a test can tell which member received a send.
- Keep the `push_log` reservation before the send, even for reminders. A missed nudge is cheaper than a doubled one.

### Task 7

- Extend `notifications.md` with the reminder section beside `## Slip events`, and update the `push_log` paragraph in
  Sending to name the `reminder` kind.
- Keep the events table in `notifications.md` as the one place for message text and recipients; `slips.md` should only
  say when an event rises and what it carries.

## Files touched

- `cmd/server/main.go`
- `docs/concepts/notifications.md`
- `docs/concepts/slips.md`
- `docs/llms.txt`
- `internal/db/push.go`
- `internal/push/notify.go`
- `internal/push/notify_internal_test.go`
- `internal/push/notify_test.go`
- `internal/push/send.go`
- `internal/push/send_test.go`
- `internal/slips/events_test.go`
- `internal/slips/rules.go`

## Run log

- Mode: 
- Checkpoints: task-6, task-7
- Verify: verify passed: test 500ms; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
