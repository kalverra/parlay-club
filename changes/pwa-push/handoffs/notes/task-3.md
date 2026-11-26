# Task 3 handoff: push config, schema, and sender

## Done

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

## Deviations

- SQL lives in `internal/db` like every other table; `internal/push` owns the policy and calls the store. The design's
  sketch pointed push at the database directly, which in this repo means through the store.
- The push-host allowlist and the 10-device cap sit in `push.Subscribe`, not the HTTP route, so every caller gets them.
  The routes task only has to map `ErrInvalidSubscription` to 400 and `ErrDisabled` to 404.
- The subscribe upsert refreshes `created_at`, so re-subscribing a device counts as the newest one for the cap instead
  of being pruned first.

## Open issues

- `internal/push` is not wired into `cmd/server` or `web.Dependencies` yet; `PushEnabled()` has no production caller
  until the subscription routes land.
- `push_log` exists but has no helper yet. Graded and reminder dedupe belong to phases 3 and 4.
- `docs/reference/config.md` and `docs/reference/schema.md` are stale for the new variables and tables until this
  phase's docs task.

## Next-phase notes

- Build `POST`/`DELETE /push/subscription` on `Service.Subscribe` and `Service.Unsubscribe`; both return
  `ErrDisabled`, which should be a 404, and `Subscribe` wraps `ErrInvalidSubscription` for a 400.
- The browser's `PushSubscription.toJSON()` shape (`{endpoint, keys: {p256dh, auth}}`) unmarshals straight into
  `push.Subscription`.
- Read the VAPID public key from `Service.PublicKey()` for `pushManager.subscribe`; do not read the config twice.
- `/push/test` can send with `SendToUsers(ctx, []int64{callerID}, message)`.
- Give a send test's stand-in server a release channel in cleanup: a handler that only waits on `r.Context().Done()`
  keeps `httptest.Server.Close` blocked because the server does not notice the client hang-up.

## Commit

Add push config, key tool, and Web Push sender

Phase 2 needs a place to store device subscriptions and a working send path before the opt-in UI can call it. Push
stays inert unless both VAPID keys are set, and the httptest coverage pins the headers, deadline, and endpoint pruning
so the routes layer can build on a settled seam.
