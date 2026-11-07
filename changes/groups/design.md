# Design: Groups and solo play

## Context

`google-oauth` gives each person a stable user and a `SignIn` function. Nothing is deployed and there is no data, so
this is a rebuild: the schema is edited in place and nothing is migrated. This change is landing before `slips-live`,
`slate-and-lines`, and `history-and-deploy`: this branch has no `internal/slips`, `internal/live`, or `internal/seed`,
and no slip routes. Their designs leave seams for it: `internal/live` topics are plain strings, `internal/slips` methods
take the acting user, `internal/seed` resolves people in one function, and games, odds, weather, and `sync_state` are
global. Phase 2 lands the scope check on the routes that exist; the slip, live, and seed work joins the changes that
build those packages. Anything designed after this one (`grading-record-book`) must take the group from the actor.

SQLite runs one connection and one Cloud Run instance. The hub is in memory. Read before starting:

- `docs/concepts/access.md`, `docs/concepts/slips.md`, `docs/concepts/live-updates.md`, `docs/reference/schema.md`.
- `plan/architecture.md`: Routes and Live updates. `plan/product.md`: Record Book.
- `changes/slips-live/design.md`, `changes/slate-and-lines/design.md`, `changes/history-and-deploy/design.md`: the
  "Groups later" notes.

## Approach

**Schema.** Edit `0001_init.sql` in place. A group is a boundary, nothing more.

```sql
CREATE TABLE groups (
  id           INTEGER PRIMARY KEY,
  name         TEXT NOT NULL,                    -- not unique
  invite_token TEXT NOT NULL UNIQUE,             -- 128-bit random, hex
  created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);
CREATE TABLE group_members (
  group_id  INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
  user_id   INTEGER NOT NULL REFERENCES users(id),
  joined_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
  PRIMARY KEY (group_id, user_id)
);
```

`slips` gains `group_id INTEGER NOT NULL REFERENCES groups(id)` and `UNIQUE (group_id, week_id, name)` in place of
`UNIQUE (week_id, name)`. Weeks, games, odds, player matches, and sync state are untouched. Anyone with a local database
deletes `data/parlay.db`.

**Solo is a group of one.** `SignIn` creates a group named "Just me" in the same transaction as a new user. Any group
can mint an invite, so a solo group can become shared with no mode switch. The UI hides the group switcher while a
person has one group. You cannot leave your last group.

**Open sign-up.** Delete `auth.Gate`, `ALLOWED_EMAILS`, and the gate check in `RequireUser`. The cost guard is the
shared cache and credit floors, which this change does not touch.

**One scope check, one place.** `internal/groups` owns membership.

```go
type Store interface {
    Create(ctx context.Context, userID int64, name string) (Group, error)
    ForUser(ctx context.Context, userID int64) ([]Group, error)
    IsMember(ctx context.Context, groupID, userID int64) (bool, error)
    JoinByToken(ctx context.Context, userID int64, token string) (Group, error)
    ResetInvite(ctx context.Context, groupID int64) (string, error)
    Leave(ctx context.Context, groupID, userID int64) error
}
```

- The active group is a signed `pc_group` cookie holding a group ID. `groups.Scope` middleware (after `RequireUser`)
  reads it, confirms membership, falls back to the person's first group, and puts the group on the context. A forged or
  stale cookie is just a fallback.
- Routes that name a slip (`/slips/{id}/...`, its SSE stream) resolve the slip's group and require the caller to be a
  member of it, whatever `pc_group` says. A member opening a link to another of their groups switches the active group.
  A non-member gets 404, the same as a missing slip.
- `internal/slips` takes `Actor{UserID, GroupID}` and every query filters on `slips.group_id`. The home list and the
  Record Book read from the active group. Handlers never build a group filter themselves.

**Live updates.** The hub is unchanged. `slip:{id}` is private to a group because the SSE handler checks membership
before subscribing. Add a `group:{id}` topic, published when a slip is created or renamed, so the home list refreshes.
`week:{weekID}` topics stay global; they are signals with no group data. Deadline and idle-close rules do not change.

**Invites.** `GET /join/{token}` shows "Join {group}?" with the member names. A signed-out visitor is sent to `/login`
with the path kept in the signed `pc_oauth` cookie; only a `/join/` path is accepted as a return target.
`POST /join/{token}` (HTMX) adds the member and is idempotent. A member can reset the link, which kills the old one.
Leaving removes the membership only; the person's legs stay on the group's slips.

**History seed.** `internal/seed` loads into a group named "Sunday Parlay Club", creating it and its members when
missing. `seed/history.yaml` has no group field.

**Screens.** A `/groups` page lists groups with a create form, each group's invite link with copy and reset, and leave.
A bottom sheet switcher in the top bar picks the active group. Phone first.

Rejected: per-group database files (split the shared cache); filters in handlers (leak by omission); a group in every
URL (rewrites every route in the other designs); roles (not wanted).

## Verification

- `go test ./...` and `golangci-lint run ./...` pass.
- Isolation test: two groups with slips. For every slip route, the SSE stream, the Record Book, and the home list, a
  non-member gets 404 or sees none of the other group's rows. A forged `pc_group` cookie changes nothing. A route table
  drives the test, so a new route that skips the check fails it.
- The same slip name and week in two groups both succeed.
- Live test: an update in group A reaches A's subscribers and no one in B; a new slip fires `group:{id}`.
- Credit test: two groups open the same game's props; the Odds API stub is called once and `sync_state` is shared.
- Join flow: a signed-out visitor follows an invite, signs in, lands on the join page, joins, and sees the slips. A
  reset link kills the old one. Joining twice changes nothing. Leaving your last group is refused. A solo player signs
  in with no invite and can create a slip. An unlisted stranger can sign in and gets a solo group.
- At phone width the `/groups` page, the join page, and the switcher fit with no horizontal scroll.

## Risks

- A route that forgets `Scope` leaks a group's data. The route table test makes forgetting loud.
- Open sign-up plus lazy sync: strangers can trigger page loads. The cache and credit floors bound the cost; watch
  `sync_state` credits after launch.
- Invite links are bearer secrets. 128-bit tokens, one-click reset, and a join page that shows only names limit the
  harm.
- `groups` is landing before the three waiting changes, so Phase 2 lands only `groups.Scope` and the `pc_group` cookie.
  The slip, live, and seed work waits for the code those changes build, and their slip code is written group-aware
  instead of adapted. The route-table isolation, same-name, live, and credit tests move with that code; they cannot
  exist until there are slip routes, a hub, and a seed.

## TODO

Effort: hard (a new scope check on every route, plus open sign-up, across slips, live updates, and the Record Book)

### Phase 1: Groups exist

- [x] Add the groups tables and `slips.group_id` to `0001_init.sql`, and `internal/groups` `Create`, `ForUser`, and
  `IsMember`, with store tests
- [x] Open sign-up: delete `auth.Gate` and `ALLOWED_EMAILS`, and create a "Just me" group when `SignIn` creates a user,
  with tests for strangers and the solo group

### Phase 2: Everything stays inside a group

- [x] Add `groups.Scope` and the `pc_group` cookie, thread `Actor{UserID, GroupID}` through `internal/slips`, and guard
      slip-by-id routes by the slip's group, with the isolation test over every slip route and the same-name-two-groups
      case
- [x] Scope the SSE stream by membership, add the `group:{id}` topic, filter the Record Book and home list by group, and
  point `internal/seed` at the club group, with the live and credit tests

### Phase 3: Invites and screens

- [ ] Add `JoinByToken`, `ResetInvite`, and `Leave`, the join routes, and the login return path through `pc_oauth`, with
  the join-flow tests
- [ ] Build the `/groups` page, create form, invite copy and reset, leave, and the top-bar switcher at phone width
- [ ] Docs: add `docs/concepts/groups.md`; update `docs/concepts/access.md`, `docs/concepts/slips.md`,
  `docs/concepts/live-updates.md`, `docs/reference/schema.md`, and `docs/reference/config.md`; fix Routes in
  `plan/architecture.md` and delete the Record Book group notes that this change moves into the docs
