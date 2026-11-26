# Phase 2: Phase 2: Push opt-in per device

## Done

### Task 3: Add `VAPID_PUBLIC_KEY` and `VAPID_PRIVATE_KEY` to config, with push off unless both are set. Add `cmd/vapidkeys`, migration `0020_push.sql` (`push_subscriptions`, `push_log`), and `internal/push` with subscription storage and `SendToUsers` over `webpush-go`, using the 3 second cap, TTL, Urgency, Topic, and 404/410 pruning. Cover it with an `httptest` push endpoint.

- Added `VAPID_PUBLIC_KEY` and `VAPID_PRIVATE_KEY` to config, trimmed like the other secrets, with `PushEnabled()`
  reporting true only when both halves are set.
- Added `cmd/vapidkeys`, which prints a fresh pair as shell-sourceable `VAPID_PUBLIC_KEY` and `VAPID_PRIVATE_KEY`
  assignments.
- Added migration `0020_push.sql` with `push_subscriptions` (endpoint unique, keys, created_at) and `push_log`
  (kind, slip, user, key). Store methods save/reassign by endpoint, drop the oldest past a cap, delete only the caller's
  endpoint or one by ID, and list endpoints for active users.
- Added `internal/push`: the VAPID key pair, browser-shaped subscription JSON, the https push-host allowlist,
  subscribe/unsubscribe, and `SendToUsers`. Sends run concurrently under the 3 second cap with TTL 6 hours,
  `Urgency: normal`, a `Topic` from the tag, and 404/410 pruning.
- Covered it with `httptest` push-service tests (one request per device, headers, the cap, 404/410 versus 500, inactive
  users) and advanced the migration tests to schema version 20.

### Task 4: Add `POST /push/subscription`, `DELETE /push/subscription`, and `POST /push/test`. Add the notifications card on `/me`: browser-driven state, the iPhone install hint, the denied hint, on/off/test, and unsubscribe on logout. Add the `push` and `notificationclick` handlers to `sw.js`. Add route tests for auth, CSRF, reassign, the push-host allowlist, the 10-subscription cap, the expired-session redirect, and the VAPID-off 404.

- Added `POST /push/subscription`, `DELETE /push/subscription`, and `POST /push/test` in `internal/web/push.go`, mapped
  through `push.Service` and returning 404 when VAPID is unset.
- Added the `/me` notifications card: browser-driven on/off state, the iPhone install hint, the denied hint, turn
  on/off, and Send test, all wired from `app.js`.
- Added a sign-out form to `/me`; its submit handler drops the browser subscription and the stored endpoint before the
  normal logout post.
- Wired `push.New` into `cmd/server` and passed it through `web.Dependencies.Push`.
- Added route tests for CSRF, auth, the expired-session redirect, reassign, the push-host allowlist, the 10-device cap,
  unsubscribe ownership, the caller-only Send test, the VAPID-off 404, and the card's presence on `/me`.

### Task 5: Docs: extend `docs/concepts/notifications.md` with opt-in and sending. Update `docs/reference/config.md` and `docs/reference/schema.md`. Add `docs/runbooks/push-setup.md` (generate keys, store the private key in Secret Manager, set env, and verify with Send test). Add the glossary term "Push subscription" to `GLOSSARY.md`.

- Extended `docs/concepts/notifications.md` with the per-device opt-in flow, the card states and hints, the push-host
  and 10-device rules, and the send path: VAPID, the 3 second cap, TTL, Urgency, Topic, and 404/410 pruning.
- Added `VAPID_PUBLIC_KEY` and `VAPID_PRIVATE_KEY` to `docs/reference/config.md`, including the push-off behavior with
  either key missing and the `BASE_URL` subject.
- Added `0020_push.sql`, `push_subscriptions`, and `push_log` to `docs/reference/schema.md`, with DDL and conventions.
- Added `docs/runbooks/push-setup.md` for local and production key setup, Secret Manager storage, Send test
  verification, rotation, rollback, and troubleshooting.
- Added the "Push subscription" glossary term and the three push routes to `docs/reference/routes.md`.

## Deviations

### Task 3

- SQL lives in `internal/db` like every other table; `internal/push` owns the policy and calls the store. The design's
  sketch pointed push at the database directly, which in this repo means through the store.
- The push-host allowlist and the 10-device cap sit in `push.Subscribe`, not the HTTP route, so every caller gets them.
  The routes task only has to map `ErrInvalidSubscription` to 400 and `ErrDisabled` to 404.
- The subscribe upsert refreshes `created_at`, so re-subscribing a device counts as the newest one for the cap instead
  of being pruned first.

### Task 4

- `web/sw.js` already had the `push` and `notificationclick` handlers from Task 1. This task only kept the existing
  route test that proves they are present, so the file did not change.
- The app had `POST /logout` but no logout form anywhere in the UI, so the `/me` screen gained a Sign out form. The
  design now says the `/me` sign-out form runs the unsubscribe.

### Task 5

- The deploy workflow does not pass or mount the VAPID pair, and nothing else in the change wires it. The runbook
  stores both halves in Secret Manager and sets them on the Cloud Run service with `gcloud run services update
  --update-env-vars`; later deploys keep them because the workflow updates env vars. The design's approach and risks
  sections now record this.
- `docs/reference/routes.md` was updated even though the task names only the notifications, config, and schema pages.
  Task 4's handoff explicitly assigned the missing route rows to this task.
- To keep changed docs lint-clean, the `DB_PATH` and `SUNDAY_SANDBOX` path references in `config.md` became
  "`parlay.db` in `data`" instead of nonexistent `./data/...` paths.

## Open issues

### Task 3

- `internal/push` is not wired into `cmd/server` or `web.Dependencies` yet; `PushEnabled()` has no production caller
  until the subscription routes land.
- `push_log` exists but has no helper yet. Graded and reminder dedupe belong to phases 3 and 4.
- `docs/reference/config.md` and `docs/reference/schema.md` are stale for the new variables and tables until this
  phase's docs task.

### Task 4

- `/push/test` returns 204 even when the caller has no stored devices; `SendToUsers` logs and drops. The card only shows
  Send test while a local subscription exists, so this is not reachable by tapping, but a stale browser with a pruned
  endpoint sees a success toast and gets nothing.
- `docs/reference/routes.md` does not list `POST /push/subscription`, `DELETE /push/subscription`, or `POST /push/test`.
  The phase's docs task should add them even though the task list names only the notifications, config, and schema
  pages.
- Real-phone behavior (permission prompt inside the tap, the iOS install flow, and a dropped subscription showing Off)
  remains manual-only.

### Task 5

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

### Task 3

- Build `POST`/`DELETE /push/subscription` on `Service.Subscribe` and `Service.Unsubscribe`; both return
  `ErrDisabled`, which should be a 404, and `Subscribe` wraps `ErrInvalidSubscription` for a 400.
- The browser's `PushSubscription.toJSON()` shape (`{endpoint, keys: {p256dh, auth}}`) unmarshals straight into
  `push.Subscription`.
- Read the VAPID public key from `Service.PublicKey()` for `pushManager.subscribe`; do not read the config twice.
- `/push/test` can send with `SendToUsers(ctx, []int64{callerID}, message)`.
- Give a send test's stand-in server a release channel in cleanup: a handler that only waits on `r.Context().Done()`
  keeps `httptest.Server.Close` blocked because the server does not notice the client hang-up.

### Task 4

- `push.Service` is now built in `cmd/server` and exposed as `web.Dependencies.Push`. Phase 3 can reuse that instance
  for the `slips.Notifier` seam.
- Pass an untyped nil `Notifier` when push is off, never a nil `*push.Service`, because a typed nil in an interface is
  not `== nil`.
- `push_log` still has no helper. Graded dedupe (phase 3) and reminder reservation (phase 4) own that.

### Task 5

- Phase 3 extends the same notifications page with the event table and adds the `Notifier` seam to
  `docs/concepts/slips.md`. Keep the opt-in and sending sections as they are.
- Phase 4 extends `docs/runbooks/push-setup.md` with Cloud Tasks queue setup and the first-week check, adds
  `REMINDER_TOKEN` and `REMINDER_QUEUE` to config, and adds `reminder_tasks` to schema.
- If the deploy workflow gains the VAPID secret mount, replace the runbook's manual env step and delete the design risk
  bullet about the missing mount.
- Phase 4 also needs `POST /internal/reminders` in `docs/reference/routes.md`.

## Files touched

- `GLOSSARY.md`
- `cmd/server/main.go`
- `cmd/server/push_test.go`
- `cmd/vapidkeys/main.go`
- `cmd/vapidkeys/main_test.go`
- `docs/concepts/notifications.md`
- `docs/llms.txt`
- `docs/reference/config.md`
- `docs/reference/routes.md`
- `docs/reference/schema.md`
- `docs/runbooks/push-setup.md`
- `go.mod`
- `go.sum`
- `internal/config/config.go`
- `internal/config/config_test.go`
- `internal/db/migrate_internal_test.go`
- `internal/db/migrate_test.go`
- `internal/db/migrations/0020_push.sql`
- `internal/db/push.go`
- `internal/db/push_test.go`
- `internal/push/push.go`
- `internal/push/push_test.go`
- `internal/push/send.go`
- `internal/push/send_internal_test.go`
- `internal/push/send_test.go`
- `internal/web/push.go`
- `internal/web/push_test.go`
- `internal/web/routes.go`
- `internal/web/slips.go`
- `web/static/app.css`
- `web/static/app.js`
- `web/templates/profile.html`

## Run log

- Mode: 
- Checkpoints: task-3, task-4, task-5
- Verify: verify passed: test 1.2s; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
