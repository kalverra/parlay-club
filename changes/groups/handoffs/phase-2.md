# Phase 2 handoff: Everything stays inside a group

## Done

- `internal/groups/groups.go` now has `Scope`, the middleware that resolves a request's active group. It reads the
  signed `pc_group` cookie, checks the ID against the person's memberships from `ForUser`, and falls back to their first
  group when the cookie is missing, forged, stale, or names someone else's group. The chosen group goes on the request
  context through `groups.From`; the fallback is written back to the cookie. A store failure or a signed-in person with
  no group answers 500 rather than serving an empty scope.
- `internal/auth/cookies.go` signs the `pc_group` cookie with `SetGroup`, reads it with `GroupID`, and expires it with
  `ClearGroup`. It carries a group ID and a 30-day expiry, uses a separate `group:` MAC scope so a session value cannot
  pass as a group, and keeps the same HttpOnly, SameSite=Lax, and prod-Secure attributes as the other cookies.
- `cmd/server/main.go` mounts the scope on the authenticated mux only: `handlers.RequireUser(scope(app))`. Health,
  static files, and the sign-in routes stay outside it, so they never ask for a group. `auth.Handlers.logout` now clears
  `pc_group` as well as the session, so signing out forgets the active-group choice.
- Tests:
  - `internal/groups/scope_test.go` runs the middleware against a real SQLite database and real signed cookies. It
    covers fallback to the first group, an explicit member choice, a forged signature, another person's group, a user
    with no group, and a failing store.
  - `internal/auth/cookies_test.go` covers setting, reading, expiring, tampering, and cross-cookie MAC confusion for
    `pc_group`.
  - `cmd/server/main_test.go` asserts the shell runs after `Scope` and that the fallback group sticks in the cookie.
  - `internal/auth/handlers_test.go` asserts logout clears the group cookie.
- `mise x -- go test ./...` passes and `mise x -- golangci-lint run ./...` reports 0 issues.

## Deviations

- The slip, SSE, Record Book, home-list, and seed halves of both approved tasks did not land. This branch has no
  `internal/slips`, `internal/live`, or `internal/seed`, no slip routes, and no home list or Record Book to filter.
  There was nothing to thread `Actor` through, nothing to guard by slip group, no stream to scope, no `group:{id}`
  publisher, and no seed to point at a club group. Those pieces move to `slips-live`, `slate-and-lines`, and
  `history-and-deploy`, where the code they touch is built. The isolation route table, same-name-two-groups, live, and
  credit tests move with them.
- `Scope` resolves membership with one `ForUser` call and checks the cookie against that list, instead of calling
  `IsMember` and then `ForUser` for the fallback. The result is the same with one query fewer. `IsMember` stays on the
  interface for the slip-by-id guard that Phase 3 and the waiting changes will use.
- `Scope` writes the fallback group back to the cookie rather than only using it for the request. The design calls the
  cookie a preference; writing it makes the fallback stick and stops the lookup from repeating on every request.
- A signed-in person with no group gets 500. `SignIn` always creates a "Just me" group, so this only happens if a
  membership row disappears, which is a real inconsistency worth failing loudly on.
- Logout clearing `pc_group` is not in the design. Without it, a shared browser keeps the previous person's group
  preference after sign-out.
- `groups.Scope` takes a `UserFrom` function and a small `GroupCookie` interface (`GroupID`, `SetGroup`) instead of
  importing `internal/auth`. That keeps `groups` as the membership seam and lets the scope tests pass stubs and real
  cookies. `*auth.Cookies` implements the interface and `auth.UserFrom` satisfies `UserFrom`.

## Open issues

- Every group-scoped route added later needs the work this phase could not do: slip-by-id guards that resolve the slip's
  group, the `group:{id}` topic, membership-checked SSE subscriptions, group filters on the home list and Record Book,
  and the club group in `internal/seed`. Do not assume any of it exists.
- The `pc_group` cookie is cleared on logout but not on leaving a group. Phase 3's `Leave` should clear it or let
  `Scope` fall back; the stale-cookie path already falls back safely.
- A member opening a slip from another of their groups should switch the active group. No slip routes exist yet, so that
  handler still has to call `SetGroup` after confirming membership.
- There is no group switcher yet, so `pc_group` only changes through the fallback. `Scope` already honours a valid
  choice, so the Phase 3 switcher only needs to set the cookie and reload.
- `slips.group_id` is NOT NULL with no backfill. Delete an existing `data/parlay.db` before running the app.
- `db.UnlinkedUserByName` still has no caller. The history seed must set an email or create users before their first
  sign-in.
- The `good-vibes check` warning about `docs/concepts/access.md` listing the deleted `internal/auth/gate.go` remains
  until the Phase 3 docs work.

## Next-phase notes

- Add `JoinByToken`, `ResetInvite`, and `Leave` to both `internal/groups.Store` and `*db.Store`, and keep the
  `var _ Store = (*db.Store)(nil)` assertion.
- `Leave` must refuse to remove a person's last group, and it must only remove the membership row so their legs stay on
  the group's slips.
- The join route needs the `pc_oauth` return path: keep only a `/join/` path as an accepted return target.
- Handlers read the active group with `groups.From(ctx)` and pass its ID into queries. Do not rebuild group filters in
  handlers.
- `Scope` is mounted inside `RequireUser` on the app mux, so new authenticated routes inherit it. A route mounted
  outside that call must not read `groups.From`.
- Follow `internal/groups/scope_test.go` for tests: `db.Open` in `t.TempDir()`, `db.Migrate`, `db.NewStore`, and
  `auth.NewCookies`. Run Go through mise, for example `mise x -- go test ./...`.
- If the Phase 3 screens need the solo group name, move `soloGroupName` out of `internal/auth` into `internal/groups`.

## Files touched

- `cmd/server/main.go`
- `cmd/server/main_test.go`
- `internal/auth/cookies.go`
- `internal/auth/cookies_test.go`
- `internal/auth/handlers.go`
- `internal/auth/handlers_test.go`
- `internal/groups/groups.go`
- `internal/groups/scope_test.go`

## Run log

- Mode: 
- Checkpoints: task-3, task-4
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
