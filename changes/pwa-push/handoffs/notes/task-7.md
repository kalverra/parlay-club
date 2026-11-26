# Task 7 handoff

## Done

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

- The task asks to add the events table, but task 6 had already written it in `notifications.md`. Rewriting it would have
  churned accurate prose, so this task verified it against the code and added the missing seam cross-link instead.
- `slips.md` had no `Publisher` prose to sit beside; the seam was only implied by "publishes `slip:{id}`" and described
  in `live-updates.md`. The new paragraph covers both seams together, which is the closest reading of the task.

## Open issues

- Reminders are still undescribed in `notifications.md`. Phase 4 owns that section, the `push_log` reminder kind, and
  the task lifecycle.

## Next-phase notes

- Extend `notifications.md` with the reminder section beside `## Slip events`, and update the `push_log` paragraph in
  Sending to name the `reminder` kind.
- Keep the events table in `notifications.md` as the one place for message text and recipients; `slips.md` should only
  say when an event rises and what it carries.

## Commit

Document slip events and the notifier seam

The slips concept doc explained live publishing but not the notifier that rides beside it, so the two seams now sit
together with the event fields and the conditions that raise each one. The notifications page links back to that seam
so a reader can move from the message table to the code path that produces it.
