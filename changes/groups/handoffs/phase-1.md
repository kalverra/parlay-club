# Phase 1 handoff: Groups exist

## Done

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

## Deviations

- `internal/groups.Store` has three of the design's six methods. `JoinByToken`, `ResetInvite`, and `Leave` belong to
  Phase 3, so the interface grows then; adding them earlier would leave uncalled stubs.
- The interface lives in `internal/groups` but the implementation stays on `*db.Store`, as the rest of the store layer
  does. `groups.Group` is an alias, so callers never import `db` for the type.
- Removing the gate also removed the name-based history link (`gate.LinkName` → `db.UnlinkedUserByName`) from `SignIn`.
  Resolution is now subject → email → create, with no way to claim an unlinked user by name. That was the gate's only
  caller of `UnlinkedUserByName`.
- The design's context assumed `slips-live`, `slate-and-lines`, and `history-and-deploy` had shipped. They have not:
  this branch has no `internal/slips`, `internal/live`, or `internal/seed`, and no slip routes. Phase 1 needed none of
  them. The design's context and last risk bullet now say so.
- Docs were left for Phase 3 as the design says, so `docs/concepts/access.md`, `docs/reference/config.md`,
  `docs/reference/schema.md`, `docs/guides/local-dev.md`, and `plan/architecture.md` still describe the allow list.

## Open issues

- Phase 2's first task cannot be done as written here: there is no `internal/slips` to thread `Actor` through and no
  slip-by-id route to guard or include in the isolation route table. Either land the three waiting changes first, or
  land only `groups.Scope` and the `pc_group` cookie now and move the slip work to the changes that build slips.
- `slips.group_id` is NOT NULL with no backfill, so an existing `data/parlay.db` must be deleted. Say this in the
  local-dev docs when Phase 3 touches them.
- `db.UnlinkedUserByName` has no caller after this phase. Delete it, or let the history seed use it deliberately.
- The history seed must set an email (or create users before their first sign-in). Name alone no longer links a Google
  account to a seeded user.
- `soloGroupName` is private to `internal/auth`. If Phase 2 or 3 needs the name elsewhere, move the constant into
  `internal/groups`.
- The `good-vibes check` warning disappears once `docs/concepts/access.md` drops `internal/auth/gate.go` from its
  sources; `good-vibes docs stale` and `docs index` should run with the Phase 3 docs work.

## Next-phase notes

- Put `groups.Scope` after `RequireUser`. Read the signed `pc_group` cookie, call `IsMember`, fall back to the first
  result from `ForUser`, and put the group on the request context. A forged or stale cookie is only a fallback.
- `ForUser` orders by `joined_at, id`, so the first row is a stable default group.
- Extend both `internal/groups.Store` and `*db.Store` when adding `JoinByToken`, `ResetInvite`, and `Leave`; keep the
  `var _ Store = (*db.Store)(nil)` assertion.
- Store tests use the package's `openTestStore` and `createUser` helpers and `t.Context()`. Run Go through mise, for
  example `mise x -- go test ./...`.
- The slip uniqueness change is already live in the schema, so the same slip name and week in two groups works as soon
  as slips are group-aware.

## Files touched

- `cmd/server/main.go`
- `cmd/server/main_test.go`
- `internal/auth/gate.go`
- `internal/auth/handlers.go`
- `internal/auth/handlers_test.go`
- `internal/auth/middleware.go`
- `internal/auth/middleware_test.go`
- `internal/auth/signin.go`
- `internal/auth/signin_test.go`
- `internal/config/config.go`
- `internal/config/config_test.go`
- `internal/db/groups.go`
- `internal/db/groups_test.go`
- `internal/db/migrate_internal_test.go`
- `internal/db/migrate_test.go`
- `internal/db/migrations/0001_init.sql`
- `internal/groups/groups.go`

## Run log

- Mode: 
- Checkpoints: task-1, task-2
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
