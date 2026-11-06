# Phase 1 handoff: Identities and the gate

## Done

- Added migration `0002_google_identity.sql`. It rebuilds `users` inside the migration transaction with
  `PRAGMA defer_foreign_keys = ON`, adds `google_sub TEXT UNIQUE`, `email TEXT UNIQUE COLLATE NOCASE`, and
  `picture_url TEXT`, and drops the unique rule on `name`. A plain `users_name` index keeps lookups by name fast.
- Extended `db.User` with `Email` and `PictureURL`, and updated every user query and `scanUser` to read the new columns.
- Added the store helpers sign-in needs: `Tx`, `UserByGoogleSub`, `UserByEmail`, `UnlinkedUserByName`, `LinkGoogle`,
  `SetUserPicture`, and `CreateUserWithIdentity`. Existing `CreateUser` and the passcode flow still work unchanged.
- Added `auth.SignIn` with all five resolution steps: known subject refreshes the picture; the gate refuses strangers
  before any write; an existing email links; a gate `=Name` entry links one unlinked history user and keeps its name; and
  otherwise a new user is created from Google's name and picture. A linked user keeps their existing name.
- Added `auth.Gate` with `Allows` and `LinkName`. Emails compare without case, and an empty list allows everyone.
- Added the `ALLOWED_EMAILS` parser in `internal/config`, accepting comma-separated `email` or `email=Name`, trimming
  entries, lowercasing emails, ignoring empty segments, and reporting every malformed entry at once.
- Updated `docs/reference/config.md` for `ALLOWED_EMAILS` and `docs/reference/schema.md` for migration 0002 and the new
  user columns.
- Tests: one `SignIn` case per resolution step plus case-insensitive email and duplicate display names; gate unit tests;
  a migration test that seeds 0001 rows across every table pointing at `users`, migrates, and asserts row counts,
  preserved values, empty `PRAGMA foreign_key_check`, and the new uniqueness rules; config parser tests for valid,
  trimmed, empty, and malformed entries.
- `go test ./...`, `go vet ./...`, `good-vibes docs lint`, and `good-vibes check` pass.

## Deviations

- `SignIn` takes the store and gate explicitly (`SignIn(ctx, store, gate, identity)`) instead of the design's
  `SignIn(ctx, Identity)` shape. That keeps the function testable without package-level state.
- The migration stages the old rows in a temp table and copies them back after the rename, rather than copying into
  `users_new` before the drop. This keeps the old rows out of the new table until it carries the `users` name, so the
  deferred foreign-key checks resolve at commit. The design's approach paragraph now says this.
- `SignIn` adds `ErrIncompleteIdentity` for an identity with no subject or email. The design did not name that error, but
  a user cannot be owned without both values.
- `UnlinkedUserByName` reads the name through the column's `COLLATE NOCASE` and picks the lowest `id` when several
  unlinked rows share a name. The design did not specify a tie-break.
- `CreateUser(name)` and `PASSCODE` remain in place because this phase keeps the old flow working. The design's
  statements that creation moves to `SignIn` and passcode goes away land in phase 2.

## Open issues

- `ALLOWED_EMAILS` is parsed but not yet required in prod, and no Google settings exist in `Config` yet. Phase 2 adds
  `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, and `BASE_URL`, requires all four values in prod, and drops `PASSCODE`.
  `internal/config/config_test.go` still treats `PASSCODE` as the prod secret, so those expectations change in phase 2.
- A known `google_sub` sign-in still requires an email; if Google ever returns none, `SignIn` returns
  `ErrIncompleteIdentity`. The dev login form must require an email for the same reason.
- If two unlinked history users share a name, only the lowest-id row is ever linked through the gate's `=Name` entry.
  The other stays unlinked, so any import duplicates need a manual merge or a second email entry.
- Linking by email can collide with the unique `email` column when an account already linked to another subject holds
  that address. Normal Google sign-in cannot produce that, but hand-edited rows can; the error surfaces as a failed
  sign-in rather than a friendly page.
- The migration test exercises the rebuild on seeded data, but the first prod deploy still needs a Litestream snapshot
  per the risks section.

## Next-phase notes

- Call `auth.NewGate(cfg.AllowedEmails)` once in `cmd/server` and pass it to both `SignIn` and the session middleware.
  `Gate` is a small value type, so copying it is safe.
- `RequireUser` should ask `gate.Allows(user.Email)` on every request, not only at sign-in, so removing an address stops
  the next page load.
- Migration version is now 2. Phase 2 needs no further schema change.
- Delete `CreateUser`, `ErrNameTaken`, `nameTaken`, the passcode cookie helpers, and `/who`, `/users`, and `who.html`
  once `SignIn` is wired into the handlers.
- The friendly 403 for `ErrNotInvited` should set no cookie and render the login page; `SignIn` has already written
  nothing for that identity.
- `docs/reference/config.md` and `docs/reference/schema.md` are current for phase 1. Phase 2 still needs the access
  concept, local-dev guide, Google setup runbook, and `plan/architecture.md` updates.

## Files touched

- `docs/reference/config.md`
- `docs/reference/schema.md`
- `internal/auth/gate.go`
- `internal/auth/signin.go`
- `internal/auth/signin_test.go`
- `internal/config/config.go`
- `internal/config/config_test.go`
- `internal/db/migrate_internal_test.go`
- `internal/db/migrate_test.go`
- `internal/db/migrations/0002_google_identity.sql`
- `internal/db/store.go`
- `internal/db/users.go`

## Run log

- Mode: 
- Checkpoints: task-1
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
