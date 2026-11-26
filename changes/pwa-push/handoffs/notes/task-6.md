# Task 6 handoff

## Done

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

## Deviations

- The notifier arrives through `slips.Service.SetNotifier` rather than the constructor. `newProdServices` builds the
  slip service before `cmd/server` builds the push service, so a setter is the one place both branches can wire it.
- The `push_log` reservation helper lives in `internal/db` beside the other push storage, not in `internal/push`. The
  design sketch put the row insert near the sender; this repo keeps SQL in the store.

## Open issues

- A slip event can add up to the 3 second send cap to the request that triggered it. That matches the design's
  accepted latency risk; the cap in `internal/push` is the dial.
- `AutoGradeLegs` raises `slip_graded` inside the page-load refresh pass, so a grade push can wait for the next visit.
  That is the same staleness the hub already has and is called out in the design risks.
- `push_log` reminder reservations are still unused. Phase 4 owns them.

## Next-phase notes

- Use `db.ReservePushLog(ctx, db.PushLogReminder, slipID, userID, key)` before each reminder send. It reports false
  when the row already exists, so a Cloud Tasks retry sends nothing.
- `push.Service.Notify` is the shape to copy for reminders: load rows, decide recipients, `SendToUsers`, and keep the
  message builders pure so text is testable without decrypting the push payload.
- `internal/push` test helpers now record the request `Path`, so a test can tell which member received a send.
- Keep the `push_log` reservation before the send, even for reminders. A missed nudge is cheaper than a doubled one.

## Commit

Send push notifications for slip events

Members now hear when a friend adds legs, when the slip locks, and when it
grades, so a group does not have to keep refreshing the hub. Grade pushes
reserve a push_log row first, so a later correction stays quiet.
