# Task 4 handoff: push subscription routes and the notifications card

## Done

- Added `POST /push/subscription`, `DELETE /push/subscription`, and `POST /push/test` in `internal/web/push.go`, mapped
  through `push.Service` and returning 404 when VAPID is unset.
- Added the `/me` notifications card: browser-driven on/off state, the iPhone install hint, the denied hint, turn
  on/off, and Send test, all wired from `app.js`.
- Added a sign-out form to `/me`; its submit handler drops the browser subscription and the stored endpoint before the
  normal logout post.
- Wired `push.New` into `cmd/server` and passed it through `web.Dependencies.Push`.
- Added route tests for CSRF, auth, the expired-session redirect, reassign, the push-host allowlist, the 10-device cap,
  unsubscribe ownership, the caller-only Send test, the VAPID-off 404, and the card's presence on `/me`.

## Deviations

- `web/sw.js` already had the `push` and `notificationclick` handlers from Task 1. This task only kept the existing
  route test that proves they are present, so the file did not change.
- The app had `POST /logout` but no logout form anywhere in the UI, so the `/me` screen gained a Sign out form. The
  design now says the `/me` sign-out form runs the unsubscribe.

## Open issues

- `/push/test` returns 204 even when the caller has no stored devices; `SendToUsers` logs and drops. The card only shows
  Send test while a local subscription exists, so this is not reachable by tapping, but a stale browser with a pruned
  endpoint sees a success toast and gets nothing.
- `docs/reference/routes.md` does not list `POST /push/subscription`, `DELETE /push/subscription`, or `POST /push/test`.
  The phase's docs task should add them even though the task list names only the notifications, config, and schema
  pages.
- Real-phone behavior (permission prompt inside the tap, the iOS install flow, and a dropped subscription showing Off)
  remains manual-only.

## Next-phase notes

- `push.Service` is now built in `cmd/server` and exposed as `web.Dependencies.Push`. Phase 3 can reuse that instance
  for the `slips.Notifier` seam.
- Pass an untyped nil `Notifier` when push is off, never a nil `*push.Service`, because a typed nil in an interface is
  not `== nil`.
- `push_log` still has no helper. Graded dedupe (phase 3) and reminder reservation (phase 4) own that.

## Commit

Add push subscription routes and profile card

People can now turn notifications on or off for one device, send themselves a test, and know a shared phone drops the
subscription at sign-out. The routes keep the existing CSRF and session rules, and push stays fully off when VAPID keys
are unset.
