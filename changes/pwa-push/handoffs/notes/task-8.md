# Task 8 handoff

## Done

- `slips.FirstKickoff` returns the earliest kickoff among a slip's scoped games, whether passed or not, so reminders key to
  the fixed slip start rather than the rolling `NextStep.Deadline`.
- Migration `0021_reminder_tasks.sql` adds `reminder_tasks`; `internal/db` reads open slips, the stored task rows, and
  upserts one row per slip and start.
- `internal/reminders` adds `Due` (pure window, stale-start, status, and recipient rule), `Sync` (reconcile with no
  per-slip queries), the `Scheduler` seam with `ErrAlreadyExists`, `LocalTimers`, the fire path with `push_log` dedupe,
  and the two reminder messages.
- `REMINDER_TOKEN` is config, and `POST /internal/reminders?slip=&start=` sits beside prefill on the outer bearer-auth
  mux. It returns 404 with no token, 401 on a bad token, 400 on bad parameters, 200 on a stale or no-op fire, and 500 on
  a database error.
- `refresh.Service.SetAfterPass` runs `SyncNow` after a pass, `POST /slips` calls it after a create, and the sandbox
  controls page has a working **Send reminders now** button. Tests cover `FirstKickoff`, `Due`, `Sync`, the handler,
  dedupe, the refresh hook, and the sandbox button.

## Deviations

- `reminder_tasks` also stores `scheduled_at` and `updated_at`, not just the start. The extra columns make the row useful
  when reading the queue later and cost nothing.
- `Sync` reads open slips, the task map, and each week's games rather than literally one query. That is still a constant
  number of reads with no per-slip work; the design sentence now says that instead of "one SQLite query".
- `SetAfterPass` has no clock, so the wiring calls a small `SyncNow` wrapper. `Sync(ctx, now)` remains the tested seam.
- Local timers are wired in dev only when `REMINDER_TOKEN` is set, matching the design's "off when the token is unset".
  Sandbox never gets a scheduler; its button calls `SendNow`, which keeps the stale checks and skips the window.
- The sandbox button acts on the active group's newest open slip instead of a slip carried in the form, because a
  restored sandbox has no in-memory seed.

## Open issues

- Production has no scheduler yet: with a VAPID pair and token, the endpoint is mounted but `Sync` is a no-op. The next
  task owns `CloudTasks`, `REMINDER_QUEUE`, and the deploy setup.
- `docs/reference/schema.md` still says version 20 and that nothing writes `push_log`; `docs/reference/config.md`,
  `docs/concepts/notifications.md`, `docs/runbooks/push-setup.md`, and the glossary are also waiting on the Phase 4 docs
  task.
- A fire reserves `push_log` before the send, so a push-service failure is not retried. That is the intended trade:
  a missed nudge beats a doubled one.

## Next-phase notes

- Implement `Scheduler` with the Cloud Tasks REST API, return `reminders.ErrAlreadyExists` on a 409, and wire it in
  `cmd/server` when `REMINDER_QUEUE` and `REMINDER_TOKEN` are set. `Sync` and `Fire` need no changes.
- Keep `reminder_tasks` updated only after a successful enqueue or an `ErrAlreadyExists`, so a failed call retries.
- The docs task should add `reminder_tasks` to the schema DDL, name `reminder` in the `push_log` conventions, document
  `REMINDER_TOKEN` and `REMINDER_QUEUE`, and describe the lifecycle and edge cases in `notifications.md`.

## Commit

Schedule kickoff reminders

A slip now wakes once, 20 minutes before its first kickoff, so people who
still owe a pick or a lock-in hear about it without polling. The fire
rechecks the slip and start from SQLite and reserves a push_log row per
person, so a moved kickoff or a retried task cannot double a nudge.
