# Task 10: Reminder and push documentation

## Done

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

- The task named five docs; `routes.md` was also stale because it listed only the prefill internal post. The route row
  and status-code notes were added there, and `good-vibes docs index` refreshed `docs/llms.txt`.
- `notifications.md` and `push-setup.md` crossed the docs line budgets, so each gained a `budget` override with a reason
  instead of a split. Both belong together: install and push share a worker, and reminders share the same secrets and
  first-week check.

## Open issues

- `deploy.yml` still neither mounts a `reminder-token` secret nor sets `REMINDER_QUEUE`. The runbook sets both with
  `gcloud run services update --update-env-vars`, which later deploys keep, but a deploy that replaced the environment
  would turn reminders off until the runbook ran again. Mounting the secret is the durable fix.

## Next-phase notes

- Do the manual phone checks before close: Android and iPhone install and Send test, then the sandbox **Send reminders
  now** button with one persona short a leg.
- In the first production game week, run the runbook's first-week check: one queue task per open slip at start minus 20
  minutes, delivery inside 15–30 minutes, and no second reminder before later kickoffs.
- `docs/reference/routes.md` now states the reminder endpoint's 400 and 500 behavior, so a handler change must update
  that row and the `notifications.md` firing section together.

## Commit

Document kickoff reminders and their setup

The reminder code shipped in the previous tasks without living docs, so the schema, configuration, lifecycle, and queue
setup were invisible to anyone operating the app. This adds the operational path and the first-week check that catches a
misconfigured queue before a silent game day.
