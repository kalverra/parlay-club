# Task 5: Push docs

## Done

- Extended `docs/concepts/notifications.md` with the per-device opt-in flow, the card states and hints, the push-host
  and 10-device rules, and the send path: VAPID, the 3 second cap, TTL, Urgency, Topic, and 404/410 pruning.
- Added `VAPID_PUBLIC_KEY` and `VAPID_PRIVATE_KEY` to `docs/reference/config.md`, including the push-off behavior with
  either key missing and the `BASE_URL` subject.
- Added `0020_push.sql`, `push_subscriptions`, and `push_log` to `docs/reference/schema.md`, with DDL and conventions.
- Added `docs/runbooks/push-setup.md` for local and production key setup, Secret Manager storage, Send test
  verification, rotation, rollback, and troubleshooting.
- Added the "Push subscription" glossary term and the three push routes to `docs/reference/routes.md`.

## Deviations

- The deploy workflow does not pass or mount the VAPID pair, and nothing else in the change wires it. The runbook
  stores both halves in Secret Manager and sets them on the Cloud Run service with `gcloud run services update
  --update-env-vars`; later deploys keep them because the workflow updates env vars. The design's approach and risks
  sections now record this.
- `docs/reference/routes.md` was updated even though the task names only the notifications, config, and schema pages.
  Task 4's handoff explicitly assigned the missing route rows to this task.
- To keep changed docs lint-clean, the `DB_PATH` and `SUNDAY_SANDBOX` path references in `config.md` became
  "`parlay.db` in `data`" instead of nonexistent `./data/...` paths.

## Open issues

- Production push depends on the manual `gcloud run services update` in the runbook. A deploy that replaced env vars
  instead of updating them would drop the pair. Mounting `vapid-private-key` in `deploy.yml` and passing the public key
  is the durable fix.
- Key rotation is manual. A rotated pair makes sends fail with `403`, and those rows stay because only `404`/`410`
  prune. Each device must turn notifications off and on.
- `deploy/setup.sh` does not create `vapid-public-key` or `vapid-private-key`, so a fresh project needs the runbook's
  `gcloud secrets create` steps.
- `push_log` is documented as a dedupe table, but nothing writes it yet. Phases 3 and 4 own that.
- The three push routes are authenticated, so `docs/concepts/access.md` did not need a public-route note.

## Next-phase notes

- Phase 3 extends the same notifications page with the event table and adds the `Notifier` seam to
  `docs/concepts/slips.md`. Keep the opt-in and sending sections as they are.
- Phase 4 extends `docs/runbooks/push-setup.md` with Cloud Tasks queue setup and the first-week check, adds
  `REMINDER_TOKEN` and `REMINDER_QUEUE` to config, and adds `reminder_tasks` to schema.
- If the deploy workflow gains the VAPID secret mount, replace the runbook's manual env step and delete the design risk
  bullet about the missing mount.
- Phase 4 also needs `POST /internal/reminders` in `docs/reference/routes.md`.

## Commit

Document Web Push setup and delivery

The phase adds per-device opt-in and sending, so the concept, config, schema, and runbook pages now explain the key
pair, subscription policy, and send path. The runbook keeps production setup usable until the deploy workflow mounts the
VAPID secret.
