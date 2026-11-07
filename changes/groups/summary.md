---
slug: groups
run: 20261107-182432-groups
outcome: done
generated: 2026-11-07T18:52:05Z
---

# Run summary: groups

## Outcome

- Status: done
- Branch: groups
- Final verdict: agy@gemini-4-argon-medium approved (issues: 0)

## Totals

- Tasks: 7 of 7
- Phases: 3
- Checkpoints: 8
- Duration: 24m23s
- Cost: $1.07
- Input tokens: 26382989
- Output tokens: 221920
- Cache read tokens: 24187628

## Phases

### Phase 1: Groups exist

Tasks:

- [x] Add the groups tables and `slips.group_id` to `0001_init.sql`, and `internal/groups` `Create`, `ForUser`, and `IsMember`, with store tests
- [x] Open sign-up: delete `auth.Gate` and `ALLOWED_EMAILS`, and create a "Just me" group when `SignIn` creates a user, with tests for strangers and the solo group

Phase totals: 2 checkpoints, 5m38s, $0.26, 3874168 input / 48650 output / 3280216 cache tokens

Checkpoints: task-1, task-2 Review fixes: 0 Integration checks: 0

#### Done

- `internal/db/migrations/0001_init.sql` now has `groups` and `group_members`, `slips.group_id` (NOT NULL, references
  `groups`), and `UNIQUE (group_id, week_id, name)` in place of the week-only unique. Migration tests cover the fresh
  table list and the 0002 preservation fixture, which now carries a group and two members.
- `internal/db/groups.go` adds `Group{ID, Name, InviteToken}`, `ErrGroupNameRequired`, and `*db.Store.Create`,
  `ForUser`, and `IsMember`, plus `newInviteToken` (16 random bytes, 32 lowercase hex chars). `Create` trims the name,
  refuses a blank one, joins the creator as the first member, and reuses the caller's transaction through `Store.Tx`.
  `ForUser` returns the caller's groups oldest membership first and an empty slice, never nil.
- `internal/groups` is the seam: it aliases `db.Group`, declares the `Store` interface, and asserts `*db.Store`
  implements it. Only `Create`, `ForUser`, and `IsMember` are in the interface so far.
- Open sign-up: deleted `internal/auth/gate.go`, `auth.Gate`, `ErrNotInvited`, `ALLOWED_EMAILS`, its config parser and
  prod requirement, and the `RequireUser` gate check. `SignIn` now resolves subject, then email, then creates a user,
  and creates their "Just me" group in the same transaction. Tests cover a stranger signing in, the solo group, and
  signing in twice (still one group).
- `go test ./...` passes and `golangci-lint run ./...` reports 0 issues. `good-vibes check` reports 0 errors and one
  warning: `docs/concepts/access.md` still lists the deleted `internal/auth/gate.go` as a source.

### Phase 2: Everything stays inside a group

Tasks:

- [x] Add `groups.Scope` and the `pc_group` cookie, thread `Actor{UserID, GroupID}` through `internal/slips`, and guard slip-by-id routes by the slip's group, with the isolation test over every slip route and the same-name-two-groups case
- [x] Scope the SSE stream by membership, add the `group:{id}` topic, filter the Record Book and home list by group, and point `internal/seed` at the club group, with the live and credit tests

Phase totals: 2 checkpoints, 4m47s, $0.15, 2833309 input / 39665 output / 2557040 cache tokens

Checkpoints: task-3, task-4 Review fixes: 0 Integration checks: 0

#### Done

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

### Phase 3: Invites and screens

Tasks:

- [x] Add `JoinByToken`, `ResetInvite`, and `Leave`, the join routes, and the login return path through `pc_oauth`, with the join-flow tests
- [x] Build the `/groups` page, create form, invite copy and reset, leave, and the top-bar switcher at phone width
- [x] Docs: add `docs/concepts/groups.md`; update `docs/concepts/access.md`, `docs/concepts/slips.md`, `docs/concepts/live-updates.md`, `docs/reference/schema.md`, and `docs/reference/config.md`; fix Routes in `plan/architecture.md` and delete the Record Book group notes that this change moves into the docs

Phase totals: 4 checkpoints, 13m58s, $0.67, 19675512 input / 133605 output / 18350372 cache tokens

Checkpoints: task-5, task-6, task-7, final Review fixes: 0 Integration checks: 0

#### Done

- `internal/db/groups.go` completes the group store. `ByToken` resolves an invite to its group, `JoinByToken` inserts a
  membership with `ON CONFLICT (group_id, user_id) DO NOTHING` inside a transaction, `ResetInvite` mints a fresh 128-bit
  token in place, `Leave` refuses the last group and deletes only the membership row, and `Members` lists the active
  people in a group. The errors callers branch on are `ErrInviteNotFound`, `ErrGroupNotFound`, `ErrLastGroup`, and
  `ErrNotMember`.
- `internal/groups.Store` gained `JoinByToken`, `ResetInvite`, and `Leave`. A second `InviteStore` embeds `Store` and
  adds `ByToken` and `Members`, so the screens use one interface and `*db.Store` still satisfies both.
- `internal/groups/handlers.go` mounts the group screens inside `RequireUser` and `Scope`:
  - `GET /groups` lists the caller's groups with the active one marked, each invite link, copy and reset, leave when
    they have another group, and the create form.
  - `POST /groups` creates a group, makes the caller its first member, makes it active, and returns to `/groups`.
  - `POST /groups/switch` checks membership and sets `pc_group`; `POST /groups/{id}/reset-invite` and
    `POST /groups/{id}/leave` reject non-members with 404 so a stranger cannot learn which groups exist.
  - `GET /join/{token}` shows "Join {group}?" with member names and an already-a-member state. `POST /join/{token}` adds
    the membership idempotently, makes the group active, and lands on `/`.
  - `Leave` moves the active cookie to the caller's remaining first group. Reset, switch, and leave answer HTMX with
    `HX-Refresh`, because the group list and the top-bar switcher both change.
- The login return path now runs through `pc_oauth`. `RequireUser` records a `/join/` path when it sends a signed-out
  visitor to `/login`, `googleStart` carries that path through the OAuth round trip, and both the Google callback and
  the dev sign-in land on `destination(saved.ReturnTo)`. `safeReturn` accepts only a `/join/` path; everything else
  falls back to `/`. The OAuth cookie's path moved from `/auth` to `/` so a join link can set it before sign-in, and
  `SetReturnTo` writes a return-only cookie that `OAuthState` deliberately rejects until a real round trip starts.
- The top bar has the group switcher. With one group it shows the group name as a link to `/groups`; with more than one
  it shows the active name and opens a bottom sheet of groups, each a `POST /groups/switch` form. `groups.Options` reads
  the memberships `Scope` already loaded, so the switcher costs no extra query. `app.js` copies the invite link and
  closes the sheet on an outside tap; `app.css` was regenerated for the new classes.
- `web/templates/groups.html` and `web/templates/join.html` render the two screens, and both are phone-first: truncating
  names, `min-w-0` inputs, wrapping action rows, and a sheet with safe-area padding.
- Tests:
  - `internal/db/groups_test.go` covers invite lookup, join idempotence, reset invalidation, last-group refusal,
    non-member leave, and member ordering.
  - `internal/groups/handlers_test.go` wires the real auth, scope, store, cookies, and templates. It covers the join
    page, unknown and reset links, joining twice, leave, switch, create, blank names, non-member 404s, and the full
    signed-out invite flow through `/login` and back.
  - `internal/auth/handlers_test.go` and `middleware_test.go` cover the `pc_oauth` return path on both the Google and
    dev flows, including rejecting a non-join target.
  - `internal/web/layout_test.go` covers the switcher with one group and with several.
  - `cmd/server/main_test.go` keeps the scope wired around the shell and checks the fallback group sticks.
- Docs: added `docs/concepts/groups.md`; updated `docs/concepts/access.md`, `docs/reference/schema.md`,
  `docs/reference/config.md`, `docs/guides/local-dev.md`, `docs/runbooks/google-oauth-setup.md`, and `docs/llms.txt`;
  fixed the Routes and middleware rows in `plan/architecture.md`; and removed the Record Book group note from
  `changes/grading-record-book/proposal.md`, since `docs/concepts/groups.md` now owns it.
- `mise x -- go test ./...` passes, `mise x -- golangci-lint run ./...` reports 0 issues, and `good-vibes check`,
  `good-vibes docs lint`, and `good-vibes docs stale` are clean.

## Final gate

- Rounds used: 1; verify failures: 0
- Final verify: no commands configured
- Open issues: none
