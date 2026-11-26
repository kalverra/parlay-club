# Task 9: Cloud Tasks scheduler and prod wiring

## Done

- `internal/reminders.CloudTasks` creates one task per slip start through the Cloud Tasks REST `tasks.create` call. It
  caches the runtime service account's metadata-server token until it nears expiry, names tasks
  `{queue}/tasks/{hash4}-slip-{id}-{startUnix}` so a repeat returns 409, and maps that 409 to `reminders.ErrAlreadyExists`.
  No new dependency.
- `REMINDER_QUEUE` is config. `cmd/server` wires `CloudTasks` in prod when the queue and token are both set, keeps
  `LocalTimers` in dev, and wires no scheduler in sandbox or when either value is missing.
- `deploy/setup.sh` enables `cloudtasks.googleapis.com`, creates `{service}-reminders` with 3 attempts and a 30 second
  minimum backoff, grants the runtime account `roles/cloudtasks.enqueuer` on that queue, and prints the full queue path.
- `ARCHITECTURE.md` "Cross-cutting concerns" gains **Outbound push and timed reminders**, with a diagram of `Sync`, the
  queue, the reminder endpoint, the after-commit `Notifier`, and the Apple and Google push services. The free-tier
  guardrail in `AGENTS.md` and the scheduled-jobs guardrail in `docs/concepts/data-refresh.md` now allow per-slip
  reminder tasks beside the optional prefill job.
- `httptest` tests stand in for the metadata server and the Cloud Tasks API: they check the task name shape,
  `scheduleTime`, the target URL, both bearer headers, token caching and refresh, 409, 503, and a metadata failure.
  Config tests cover `REMINDER_QUEUE`, and `NewCloudTasks` rejects a bad queue path, `BASE_URL`, or token.

## Deviations

- `NewCloudTasks` validates its config and fails at boot instead of enqueueing and logging later, and the prod wiring
  logs one error and leaves reminders off when that happens. The design assumed valid config; failing loudly at startup
  is the cheaper diagnosis.
- The 30-day limit stays where the design put it, in `Sync`'s `MaxHorizon`, because the scheduler has no clock. The
  `CloudTasks` doc comment says so.

## Open issues

- `deploy.yml` neither mounts a `reminder-token` secret nor sets `REMINDER_QUEUE`, so prod reminders stay off until
  someone adds both to the service. The deploy sets secrets explicitly, so a hand-set token is dropped on the next
  deploy. The docs/runbook task should add the pre-flight check and mount, plus the `reference/config.md` entries for
  `REMINDER_TOKEN` and `REMINDER_QUEUE`.
- `docs/reference/schema.md` version 20 and the missing `reminder_tasks` DDL are still the Phase 4 docs task's work, as
  task 8 noted.

## Next-phase notes

- Task names are deterministic, so a moved kickoff enqueues a second task with a new name. The old one fires, sees a
  start mismatch, and returns `{skipped: "stale"}`. There is no delete path and none is needed.
- The task body leaves `httpRequest.body` empty. Cloud Tasks sends an empty POST, which the handler reads only for its
  query parameters.
- `CloudTasks` uses `WithTasksEndpoint`, `WithMetadataURL`, and `WithHTTPClient` in tests. A future change to the target
  shape only needs the payload test updated.

## Commit

Schedule kickoff reminders with Cloud Tasks

Production needs a scheduler that survives scale-to-zero, and the REST API with a cached metadata token adds no client
library. One named task per slip start keeps the queue small, and a stale fire skips harmlessly.
