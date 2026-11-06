---
slug: google-oauth
run: 20261106-015000-google-oauth
outcome: done
generated: 2026-11-06T02:20:31Z
---

# Run summary: google-oauth

## Outcome

- Status: done
- Branch: google-oauth
- Final verdict: opencode:top approved (blockers: 0, nits: 5)

## Totals

- Tasks: 4 of 4
- Phases: 2
- Checkpoints: 5
- Duration: 20m4s
- Cost: $—
- Input tokens: 0
- Output tokens: 0
- Cache read tokens: 0

## Phases

### Phase 1: Identities and the gate

Tasks:

- [x] Add the 0002 users migration and `auth.SignIn` with the gate, covering all five resolution steps, plus the
      `ALLOWED_EMAILS` parser in `internal/config`, with the migration and resolution tests above. The old passcode flow
      keeps working.

Phase totals: 1 checkpoints, 6m40s, $—, 0 input / 0 output / 0 cache tokens

Checkpoints: task-1 Review fixes: 0 Integration checks: 0

#### Done

- Added migration `0002_google_identity.sql`. It rebuilds `users` inside the migration transaction with
  `PRAGMA defer_foreign_keys = ON`, adds `google_sub TEXT UNIQUE`, `email TEXT UNIQUE COLLATE NOCASE`, and
  `picture_url TEXT`, and drops the unique rule on `name`. A plain `users_name` index keeps lookups by name fast.
- Extended `db.User` with `Email` and `PictureURL`, and updated every user query and `scanUser` to read the new columns.
- Added the store helpers sign-in needs: `Tx`, `UserByGoogleSub`, `UserByEmail`, `UnlinkedUserByName`, `LinkGoogle`,
  `SetUserPicture`, and `CreateUserWithIdentity`. Existing `CreateUser` and the passcode flow still work unchanged.
- Added `auth.SignIn` with all five resolution steps: known subject refreshes the picture; the gate refuses strangers
  before any write; an existing email links; a gate `=Name` entry links one unlinked history user and keeps its name;
  and otherwise a new user is created from Google's name and picture. A linked user keeps their existing name.
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

### Phase 2: Replace the passcode

Tasks:

- [x] Add the Google `Provider`, the start and callback handlers, the `pc_session` cookie, `/login`, the friendly 403,
      `POST /logout`, and the dev login. Delete the passcode, picker, and `pc_auth`/`pc_user`. Prod config requires the
      Google values and drops `PASSCODE`. Tests cover the flow, session, and config cases above.
- [x] Add the `avatar` partial with picture and initials fallback, and show it in the top bar, with a render test.
- [x] Docs: rewrite `docs/concepts/access.md`; update `docs/reference/config.md`, `docs/reference/schema.md`, and
      `docs/guides/local-dev.md`; add `docs/runbooks/google-oauth-setup.md` (create the Web client, the two redirect
      URIs, publish the consent screen, set the secrets); fix the Routes and Deploy secrets rows in
      `plan/architecture.md`.

Phase totals: 4 checkpoints, 13m25s, $—, 0 input / 0 output / 0 cache tokens

Checkpoints: task-2, task-3, task-4, final Review fixes: 0 Integration checks: 0

#### Done

- Added `auth.GoogleProvider` over `golang.org/x/oauth2` and `github.com/coreos/go-oidc/v3`. It discovers Google's OIDC
  document, builds the authorization URL with `state`, `nonce`, and PKCE `S256`, and verifies the ID token's signature,
  issuer, audience, expiry, and nonce. An unverified email returns `ErrEmailNotVerified`.
- Added the Google round trip. `GET /auth/google/start` creates `state`, `nonce`, and a verifier, writes them to a
  signed `pc_oauth` cookie, and redirects to Google. `GET /auth/google/callback` checks the state and code, exchanges
  the code, calls `auth.SignIn`, sets `pc_session`, clears `pc_oauth`, and redirects to `/`. A bad round trip renders
  the login page with a 400. `ErrNotInvited` renders the login page with a friendly 403 and sets no session cookie.
- Replaced `pc_auth` and `pc_user` with one `pc_session` cookie: `{userID}.{expiryUnix}.{mac}`, signed over
  `session:{userID}.{expiry}`, 30 days, `HttpOnly`, `SameSite=Lax`, `Secure` in prod. Old cookies are ignored.
- `RequireUser` now loads the session user, refuses an unknown or inactive user, asks the gate on every request, and
  sends HTMX requests `HX-Redirect: /login`.
- Removed the passcode flow: `RequirePasscode`, `PASSCODE`, the passcode form, `/who`, `POST /users`, `who.html`,
  `CreateUser`, `ErrNameTaken`, and `nameTaken` are gone. `/login` is the only sign-in screen.
- Added `GET /login` with a Google button and, in dev without Google values, a link to the local form. Added
  `POST /logout`, which clears the session and returns to `/login`.
- Added the dev sign-in: `GET /dev/login` shows an email and optional name form; `POST /dev/login` calls `SignIn` with
  `Sub: "dev:" + email`. Both routes exist only when `APP_ENV` is not prod and `GOOGLE_CLIENT_ID` is empty. The form
  rejects a blank or malformed email with a 422.
- Config adds `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `BASE_URL`, and `ALLOWED_EMAILS`. `BASE_URL` defaults to
  `http://localhost:{PORT}` in dev, and the redirect URI is `BASE_URL + /auth/google/callback`. Prod requires all four
  plus a 32-byte `SESSION_KEY`, and reports every missing value at once. `PASSCODE` is gone.
- Added the `avatar` partial. It shows `picture_url` with `referrerpolicy="no-referrer"`, or initials on the user's
  palette color when there is no picture. The top bar uses it.
- Rewrote `docs/concepts/access.md`; updated `docs/reference/config.md`, `docs/reference/schema.md`, and
  `docs/guides/local-dev.md`; added `docs/runbooks/google-oauth-setup.md`; updated the Routes and Deploy secrets rows in
  `plan/architecture.md`; regenerated `docs/llms.txt`.
- Tests cover the flow (wrong state, missing, expired, and replayed `pc_oauth`, unverified email, friendly 403, logout,
  dev sign-in, prod 404), the real provider against an `httptest` OIDC server (bad signature, wrong audience, wrong
  nonce, expired token), the session middleware (tampered, expired, old `pc_user`, unknown, inactive, unlisted, and HTMX
  redirects), prod and dev config cases, and the avatar render.
- `go test ./...`, `go vet ./...`, `golangci-lint run ./...`, `go generate ./...`, `good-vibes docs lint`, and
  `good-vibes check` pass.

## Final gate

- Rounds used: 1; verify failures: 0
- Final verify: no commands configured
- Open blockers: none

## Open nits

### final review

- `internal/auth/handlers.go:305 validEmail` and `internal/config/config.go:150 looksLikeEmail` duplicate the same
  email-shape check. Export one helper and reuse it so the dev form and the `ALLOWED_EMAILS` parser cannot drift.
- `internal/db/users.go:62` still says `ListActiveUsers` feeds "the picker", which this change deletes, and the function
  now has no production caller. Update the comment and either keep the helper explicitly for the next screen or delete
  it.
- `docs/guides/local-dev.md:125-126` tells readers to change `BASE_URL` when changing `PORT`, but
  `internal/config/config.go:118` already derives the dev default from the parsed port, contradicting
  `docs/reference/config.md`'s `http://localhost:{PORT}` default. Drop the instruction or explain only the non-default
  case.
- `internal/auth/signin.go:56-61` refreshes only the picture on the known-`google_sub` path, never `email`. If a Google
  account changes its address, the gate stays keyed to the stale email, so updating `ALLOWED_EMAILS` to the new address
  still locks the person out. Refresh the email on that path or document the limitation.
- `internal/auth/handlers.go:305 validEmail` and `internal/config/config.go:150 looksLikeEmail` are the same email-shape
  check in two packages. They are only a second copy, but exporting one helper would keep the dev form and the
  `ALLOWED_EMAILS` parser from drifting.
- `internal/db/users.go:62` still describes `ListActiveUsers` as feeding "the picker", which this change deletes, and
  the function now has no production caller. Update the comment and either keep the helper explicitly for the next
  member screen or delete it.
- `docs/guides/local-dev.md:125-126` tells readers to change `BASE_URL` when changing `PORT`, but
  `internal/config/config.go:118` already derives the dev default from the parsed port. Drop the instruction or explain
  only the non-default case.
- `internal/auth/signin.go:56-61` refreshes only the picture on the known-`google_sub` path, never `email`. If a Google
  account changes its address, the gate stays keyed to the stale email, so updating `ALLOWED_EMAILS` to the new address
  still locks the person out. Refresh the email there or document the limitation.
- `internal/web/routes.go:9-21` makes the view package import `internal/db` and take a
  `func(context.Context) (db.User, bool)`. A `UserFrom` returning the `web.User` view model, built in `cmd/server` or
  `auth`, would keep `internal/web` independent of the storage layer.
