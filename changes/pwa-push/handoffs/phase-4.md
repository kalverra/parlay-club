# Phase 4: Phase 4: Kickoff reminders

## Done

### Task 8: Add `slips.FirstKickoff(slip, games)`, the fixed slip start (not the rolling `NextStep.Deadline`). Add migration `0021_reminder_tasks.sql`, `internal/reminders` (`Due`, `Sync`, the `Scheduler` seam, and `LocalTimers`), `REMINDER_TOKEN` config, and `POST /internal/reminders?slip=&start=` beside prefill in `newMux`, with the fire-window and stale checks and once-per-person-per-slip `push_log` dedupe. Call `Sync` from a new `refresh.Service.SetAfterPass` hook and after `POST /slips`. Add the sandbox **Send reminders now** button. Cover it with `Due`, `Sync`, and handler tests.

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

### Task 9: Add the `CloudTasks` scheduler (REST plus a cached metadata-server token, hash-prefixed task names, the 30-day horizon, and no new client library) and `REMINDER_QUEUE` config. In `deploy/setup.sh`, enable the API, create the queue, and grant `roles/cloudtasks.enqueuer`. Amend `ARCHITECTURE.md` "Cross-cutting concerns" with an "Outbound push and timed reminders" subsection, including a diagram of `Sync`, the Cloud Tasks queue, the reminder endpoint, the after-commit Notifier, and the Apple and Google push services. Update the free-tier guardrail in `AGENTS.md` and the scheduled-jobs guardrail in `docs/concepts/data-refresh.md` so both allow per-slip reminder tasks on Cloud Tasks next to the optional prefill job. Cover it with `httptest` tests.

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

### Task 10: Docs: add reminders, including the lifecycle and edge cases, to `docs/concepts/notifications.md`. Add `REMINDER_TOKEN` and `REMINDER_QUEUE` to `docs/reference/config.md` and `reminder_tasks` to `docs/reference/schema.md`. Add the queue setup and the first-week check to `docs/runbooks/push-setup.md`. Add the glossary term "Slip start" to `GLOSSARY.md`.

- `docs/concepts/notifications.md` gains a Reminders section: the slip start, `Sync` scheduling, the fire window and
  `push_log` dedupe, message text, edge cases, the sandbox button, configuration, and the reminder tests.
- `docs/reference/config.md` documents `REMINDER_TOKEN` and `REMINDER_QUEUE`; `docs/reference/schema.md` moves to version
  21, adds `0021_reminder_tasks.sql`, the `reminder_tasks` DDL and table row, and the `push_log` writer conventions.
- `docs/runbooks/push-setup.md` adds the queue and token setup, the first-week task and delivery check, rollback for the
  reminder variables, and four troubleshooting rows.
- `GLOSSARY.md` defines **Slip start** beside the push terms.
- `docs/reference/routes.md` now lists `POST /internal/reminders`, its bearer auth, and its status codes, and
  `docs/llms.txt` was regenerated from the new summaries.

## Deviations

### Task 8

- `reminder_tasks` also stores `scheduled_at` and `updated_at`, not just the start. The extra columns make the row useful
  when reading the queue later and cost nothing.
- `Sync` reads open slips, the task map, and each week's games rather than literally one query. That is still a constant
  number of reads with no per-slip work; the design sentence now says that instead of "one SQLite query".
- `SetAfterPass` has no clock, so the wiring calls a small `SyncNow` wrapper. `Sync(ctx, now)` remains the tested seam.
- Local timers are wired in dev only when `REMINDER_TOKEN` is set, matching the design's "off when the token is unset".
  Sandbox never gets a scheduler; its button calls `SendNow`, which keeps the stale checks and skips the window.
- The sandbox button acts on the active group's newest open slip instead of a slip carried in the form, because a
  restored sandbox has no in-memory seed.

### Task 9

- `NewCloudTasks` validates its config and fails at boot instead of enqueueing and logging later, and the prod wiring
  logs one error and leaves reminders off when that happens. The design assumed valid config; failing loudly at startup
  is the cheaper diagnosis.
- The 30-day limit stays where the design put it, in `Sync`'s `MaxHorizon`, because the scheduler has no clock. The
  `CloudTasks` doc comment says so.

### Task 10

- The task named five docs; `routes.md` was also stale because it listed only the prefill internal post. The route row
  and status-code notes were added there, and `good-vibes docs index` refreshed `docs/llms.txt`.
- `notifications.md` and `push-setup.md` crossed the docs line budgets, so each gained a `budget` override with a reason
  instead of a split. Both belong together: install and push share a worker, and reminders share the same secrets and
  first-week check.

## Open issues

### Task 8

- Production has no scheduler yet: with a VAPID pair and token, the endpoint is mounted but `Sync` is a no-op. The next
  task owns `CloudTasks`, `REMINDER_QUEUE`, and the deploy setup.
- `docs/reference/schema.md` still says version 20 and that nothing writes `push_log`; `docs/reference/config.md`,
  `docs/concepts/notifications.md`, `docs/runbooks/push-setup.md`, and the glossary are also waiting on the Phase 4 docs
  task.
- A fire reserves `push_log` before the send, so a push-service failure is not retried. That is the intended trade:
  a missed nudge beats a doubled one.

### Task 9

- `deploy.yml` neither mounts a `reminder-token` secret nor sets `REMINDER_QUEUE`, so prod reminders stay off until
  someone adds both to the service. The deploy sets secrets explicitly, so a hand-set token is dropped on the next
  deploy. The docs/runbook task should add the pre-flight check and mount, plus the `reference/config.md` entries for
  `REMINDER_TOKEN` and `REMINDER_QUEUE`.
- `docs/reference/schema.md` version 20 and the missing `reminder_tasks` DDL are still the Phase 4 docs task's work, as
  task 8 noted.

### Task 10

- `deploy.yml` still neither mounts a `reminder-token` secret nor sets `REMINDER_QUEUE`. The runbook sets both with
  `gcloud run services update --update-env-vars`, which later deploys keep, but a deploy that replaced the environment
  would turn reminders off until the runbook ran again. Mounting the secret is the durable fix.

## Next-phase notes

### Task 8

- Implement `Scheduler` with the Cloud Tasks REST API, return `reminders.ErrAlreadyExists` on a 409, and wire it in
  `cmd/server` when `REMINDER_QUEUE` and `REMINDER_TOKEN` are set. `Sync` and `Fire` need no changes.
- Keep `reminder_tasks` updated only after a successful enqueue or an `ErrAlreadyExists`, so a failed call retries.
- The docs task should add `reminder_tasks` to the schema DDL, name `reminder` in the `push_log` conventions, document
  `REMINDER_TOKEN` and `REMINDER_QUEUE`, and describe the lifecycle and edge cases in `notifications.md`.

### Task 9

- Task names are deterministic, so a moved kickoff enqueues a second task with a new name. The old one fires, sees a
  start mismatch, and returns `{skipped: "stale"}`. There is no delete path and none is needed.
- The task body leaves `httpRequest.body` empty. Cloud Tasks sends an empty POST, which the handler reads only for its
  query parameters.
- `CloudTasks` uses `WithTasksEndpoint`, `WithMetadataURL`, and `WithHTTPClient` in tests. A future change to the target
  shape only needs the payload test updated.

### Task 10

- Do the manual phone checks before close: Android and iPhone install and Send test, then the sandbox **Send reminders
  now** button with one persona short a leg.
- In the first production game week, run the runbook's first-week check: one queue task per open slip at start minus 20
  minutes, delivery inside 15–30 minutes, and no second reminder before later kickoffs.
- `docs/reference/routes.md` now states the reminder endpoint's 400 and 500 behavior, so a handler change must update
  that row and the `notifications.md` firing section together.

## Files touched

- `AGENTS.md`
- `ARCHITECTURE.md`
- `GLOSSARY.md`
- `cmd/server/main.go`
- `cmd/server/main_test.go`
- `cmd/server/push_test.go`
- `cmd/server/reminders_test.go`
- `cmd/server/sandbox.go`
- `cmd/server/sandbox_test.go`
- `deploy/setup.sh`
- `docs/concepts/data-refresh.md`
- `docs/concepts/notifications.md`
- `docs/llms.txt`
- `docs/reference/config.md`
- `docs/reference/routes.md`
- `docs/reference/schema.md`
- `docs/runbooks/push-setup.md`
- `internal/config/config.go`
- `internal/config/config_test.go`
- `internal/db/migrate_internal_test.go`
- `internal/db/migrate_test.go`
- `internal/db/migrations/0021_reminder_tasks.sql`
- `internal/db/reminders.go`
- `internal/refresh/refresh.go`
- `internal/refresh/run_test.go`
- `internal/reminders/cloud_tasks.go`
- `internal/reminders/cloud_tasks_test.go`
- `internal/reminders/local_timers_test.go`
- `internal/reminders/reminders.go`
- `internal/reminders/reminders_test.go`
- `internal/reminders/scheduler.go`
- `internal/reminders/sync_test.go`
- `internal/slips/next.go`
- `internal/slips/next_test.go`
- `internal/web/routes.go`
- `internal/web/slips.go`
- `internal/web/slips_test.go`
- `web/templates/sandbox.html`

## Run log

- Mode: 
- Checkpoints: task-8, task-9, task-10
- Verify: verify passed: test 400ms; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
