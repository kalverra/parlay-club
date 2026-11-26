---
slug: pwa-push
run: 20261126-055006-pwa-push
outcome: done
generated: 2026-11-26T06:58:58Z
---

# Run summary: pwa-push

## Outcome

- Status: done
- Branch: pwa-push
- Close-out: written
- Close: pending
- Final verdict: claude@claude-opus-5-5 approved (issues: 0)

## Totals

- Tasks: 10 of 10
- Phases: 4
- Checkpoints: 11
- Duration: 1h7m
- Cost: $2.89
- Fresh input tokens: 1785087
- Cache read tokens: 87061612
- Cache write tokens: 151908
- Output tokens: 418828

| Model | Tokens | Cost |
| --- | ---: | ---: |
| fireworks-ai/accounts/fireworks/models/deepseek-v4p1-flash-1017 | 87389031 | $1.54 |
| claude-opus-5-5 | 2028404 | $1.35 |

## Phases

### Phase 1: Installable app

Tasks:

- [x] Serve `GET /manifest.webmanifest` and `GET /sw.js` (no-cache, with only `install`, `push`, and `notificationclick` listeners and no `fetch` handler) outside auth in `newMux`. Add an icon generator that writes the 180, 192, 512, and maskable 512 PNGs into `web/static`. Add the manifest link, `apple-touch-icon`, Apple standalone meta tags, and safe-area padding to `layout.html`. Register the worker from `app.js`. Add route and layout tests.
- [x] Docs: add `docs/concepts/notifications.md` with an "Installing the app" section. Note the public routes in `docs/concepts/access.md`. Run `good-vibes docs lint` and `good-vibes docs index`.

Phase totals: 2 checkpoints, 6m24s, $0.17, 8486129 input / 41655 output / 8251376 cache read / 0 cache write tokens

Checkpoints: task-1, task-2 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Serve `GET /manifest.webmanifest` and `GET /sw.js` (no-cache, with only `install`, `push`, and `notificationclick` listeners and no `fetch` handler) outside auth in `newMux`. Add an icon generator that writes the 180, 192, 512, and maskable 512 PNGs into `web/static`. Add the manifest link, `apple-touch-icon`, Apple standalone meta tags, and safe-area padding to `layout.html`. Register the worker from `app.js`. Add route and layout tests.

- Added public `GET /manifest.webmanifest` and `GET /sw.js` routes ahead of auth in `newMux`. Both send `no-cache`; the
  manifest is `application/manifest+json` with hashed icon URLs, and the worker is `text/javascript`.
- Added `web/sw.js` with only `install`, `push`, and `notificationclick` listeners and no `fetch` handler or cache.
- Added `cmd/genicons`, invoked by `go generate`, which writes `icon-180.png`, `icon-192.png`, `icon-512.png`, and
  `icon-maskable-512.png` from the default theme's colors into `web/static`.
- Added the manifest link, 180 `apple-touch-icon`, Apple standalone meta tags, and top/bottom safe-area padding to
  `layout.html`, and registered `/sw.js` from `app.js` on load.
- Covered the new routes, worker listeners, icon dimensions, and layout tags with tests, and updated the public route
  table in `docs/reference/routes.md`.

### Task 2: Docs: add `docs/concepts/notifications.md` with an "Installing the app" section. Note the public routes in `docs/concepts/access.md`. Run `good-vibes docs lint` and `good-vibes docs index`.

- Added `docs/concepts/notifications.md`: installing on Android, iOS, and desktop; the manifest; icon generation; the
  service worker's listeners and missing `fetch` handler; the Apple tags and safe-area padding; tests and trade-offs.
- Noted the public routes in `docs/concepts/access.md`: health probes, static files, manifest, service worker, screenshot
  capability URLs, sign-in routes, and the prefill endpoint.
- Brought the access front matter up to the current schema and added an Overview diagram so the edited doc passes lint.
- Ran `good-vibes docs lint` (0 errors) and `good-vibes docs index`; `docs/llms.txt` now lists Access and Notifications.

### Phase 2: Push opt-in per device

Tasks:

- [x] Add `VAPID_PUBLIC_KEY` and `VAPID_PRIVATE_KEY` to config, with push off unless both are set. Add `cmd/vapidkeys`, migration `0020_push.sql` (`push_subscriptions`, `push_log`), and `internal/push` with subscription storage and `SendToUsers` over `webpush-go`, using the 3 second cap, TTL, Urgency, Topic, and 404/410 pruning. Cover it with an `httptest` push endpoint.
- [x] Add `POST /push/subscription`, `DELETE /push/subscription`, and `POST /push/test`. Add the notifications card on `/me`: browser-driven state, the iPhone install hint, the denied hint, on/off/test, and unsubscribe on logout. Add the `push` and `notificationclick` handlers to `sw.js`. Add route tests for auth, CSRF, reassign, the push-host allowlist, the 10-subscription cap, the expired-session redirect, and the VAPID-off 404.
- [x] Docs: extend `docs/concepts/notifications.md` with opt-in and sending. Update `docs/reference/config.md` and `docs/reference/schema.md`. Add `docs/runbooks/push-setup.md` (generate keys, store the private key in Secret Manager, set env, and verify with Send test). Add the glossary term "Push subscription" to `GLOSSARY.md`.

Phase totals: 3 checkpoints, 23m35s, $0.50, 27332284 input / 146917 output / 26785300 cache read / 0 cache write tokens

Checkpoints: task-3, task-4, task-5 Review fixes: 0 Integration checks: 0

#### Done

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

### Phase 3: Slip event notifications

Tasks:

- [x] Add the `slips.Notifier` seam and `Event`. Raise `legs_added` from `AddLeg` and `AddLegs`, `slip_locked` from the final `LockIn`, and `slip_graded` from `SetLegResult` and `AutoGradeLegs`, each only after commit. Implement `Notify` in `internal/push`: recipients, message text in the app's voice, tag and URL, and graded dedupe through `push_log`. Wire it in `cmd/server`, as an untyped nil when push is off. Filter recipients to active users. Test with a recording notifier and push service tests.
- [x] Docs: add the events table to `docs/concepts/notifications.md` and the Notifier seam beside `Publisher` in `docs/concepts/slips.md`.

Phase totals: 2 checkpoints, 10m0s, $0.21, 11358765 input / 47905 output / 11086144 cache read / 0 cache write tokens

Checkpoints: task-6, task-7 Review fixes: 0 Integration checks: 0

#### Done

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

### Phase 4: Kickoff reminders

Tasks:

- [x] Add `slips.FirstKickoff(slip, games)`, the fixed slip start (not the rolling `NextStep.Deadline`). Add migration `0021_reminder_tasks.sql`, `internal/reminders` (`Due`, `Sync`, the `Scheduler` seam, and `LocalTimers`), `REMINDER_TOKEN` config, and `POST /internal/reminders?slip=&start=` beside prefill in `newMux`, with the fire-window and stale checks and once-per-person-per-slip `push_log` dedupe. Call `Sync` from a new `refresh.Service.SetAfterPass` hook and after `POST /slips`. Add the sandbox **Send reminders now** button. Cover it with `Due`, `Sync`, and handler tests.
- [x] Add the `CloudTasks` scheduler (REST plus a cached metadata-server token, hash-prefixed task names, the 30-day horizon, and no new client library) and `REMINDER_QUEUE` config. In `deploy/setup.sh`, enable the API, create the queue, and grant `roles/cloudtasks.enqueuer`. Amend `ARCHITECTURE.md` "Cross-cutting concerns" with an "Outbound push and timed reminders" subsection, including a diagram of `Sync`, the Cloud Tasks queue, the reminder endpoint, the after-commit Notifier, and the Apple and Google push services. Update the free-tier guardrail in `AGENTS.md` and the scheduled-jobs guardrail in `docs/concepts/data-refresh.md` so both allow per-slip reminder tasks on Cloud Tasks next to the optional prefill job. Cover it with `httptest` tests.
- [x] Docs: add reminders, including the lifecycle and edge cases, to `docs/concepts/notifications.md`. Add `REMINDER_TOKEN` and `REMINDER_QUEUE` to `docs/reference/config.md` and `reminder_tasks` to `docs/reference/schema.md`. Add the queue setup and the first-week check to `docs/runbooks/push-setup.md`. Add the glossary term "Slip start" to `GLOSSARY.md`.

Phase totals: 4 checkpoints, 25m58s, $1.98, 41173947 input / 174527 output / 40341706 cache read / 151908 cache write tokens

Checkpoints: task-8, task-9, task-10, final Review fixes: 0 Integration checks: 0

#### Done

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

## Final gate

- Rounds used: 1; verify failures: 0
- Final verify: passed
- Open issues: none

## Ship

- Sync: none
- Recipe: ask
