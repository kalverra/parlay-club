# Design: Google sign-in

## Context

Today `internal/auth` has two cookies (`pc_auth`, `pc_user`), a passcode form, and a name picker. `RequireUser`
re-reads the user row on every request and puts a `db.User` on the context; handlers read it with `auth.UserFrom`. That
seam stays. The users table has `name UNIQUE`, no email, and no Google ID. Migrations apply in one transaction each, so
`PRAGMA foreign_keys` cannot be toggled inside one. Read before starting:

- `docs/concepts/access.md`, `docs/reference/config.md`, `docs/reference/schema.md`, `docs/guides/local-dev.md`.
- `plan/architecture.md`: Routes (the `/login` row) and Deploy (secrets).
- `changes/history-and-deploy/design.md`: `internal/seed` resolves each person through one function.

## Approach

**Sign-in is one function.** `auth.SignIn(ctx, store, gate, Identity) (db.User, error)` takes a verified identity
(`Sub, Email, Name, Picture`) and returns a user or `ErrNotInvited`. Google's callback and the dev login both call it,
so linking and the gate have one code path and one set of tests.

Resolution order inside one transaction:

1. A user with this `google_sub`: update the picture, done.
2. Else the email must pass the gate, or return `ErrNotInvited`. Nothing is written for a stranger.
3. Else a user with this email (case-insensitive): set `google_sub`, set the picture.
4. Else the gate entry for this email carries a name (`bob@x.com=Bob`) and an unlinked user (no `google_sub`) has that
   name: link it. This is the one-time hand match for users from the history import. It needs no tool and no boot
   step: the person signs in, and their old slips are theirs.
5. Else create a user from Google's name and picture.

A linked user keeps the name they already have. Google's name is used only when creating, so history never gets renamed
under people. The picture refreshes on every sign-in.

**Gate.** `ALLOWED_EMAILS` is a comma list of `email` or `email=Name`, parsed in `internal/config` into
`[]AllowedEmail`. `auth.Gate` is a small type with `Allows(email) bool` and `LinkName(email) string`. It is the only
place that knows the list, so `groups` can delete it. `RequireUser` also asks the gate on every request, so removing an
email stops that person's next page load, not only their next sign-in. In dev an empty list allows everyone.

**Google flow.** Use `golang.org/x/oauth2` and `github.com/coreos/go-oidc/v3`: the authorization code flow with PKCE,
`state`, and `nonce`, scopes `openid email profile`. Reject an ID token with `email_verified` false.

```go
// Provider is Google in prod and a fake in tests.
type Provider interface {
    AuthURL(state, nonce, challenge string) string
    Exchange(ctx context.Context, code, verifier, nonce string) (Identity, error)
}
```

- `GET /auth/google/start` makes `state`, `nonce`, and a PKCE verifier, stores them in a signed `pc_oauth` cookie (10
  minutes, `Path=/auth`), and redirects to Google.
- `GET /auth/google/callback` checks `state` against the cookie, exchanges the code, calls `SignIn`, sets the session,
  clears `pc_oauth`, and redirects to `/`. `ErrNotInvited` renders the login page with a friendly 403 ("That Google
  account isn't on the list yet. Ask whoever runs the club to add you."). It sets no cookie.

**Session cookie.** One `pc_session` cookie replaces `pc_auth` and `pc_user`: `{userID}.{expiryUnix}.{mac}` with the MAC
over `"session:" + userID + "." + expiryUnix`, 30 days, `HttpOnly`, `SameSite=Lax`, `Secure` in prod. A new name means
every old cookie is simply invalid. `SESSION_KEY` still signs it. `RequirePasscode`, `PASSCODE`, `/who`, `POST /users`,
and `who.html` are deleted. `POST /logout` clears the cookie (plain form, exempt from CSRF like the old forms).

**Dev login.** When `APP_ENV=dev` and `GOOGLE_CLIENT_ID` is empty, `GET /dev/login` shows a form (email, name) and
`POST /dev/login` calls `SignIn` with `Sub: "dev:" + email`. The routes are not registered in prod, and prod config
requires the Google values, so a misdeploy cannot expose them. A dev with real client values uses the real flow.

**Config.** Add `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `BASE_URL` (dev default `http://localhost:{PORT}`; the
redirect URI is `BASE_URL + /auth/google/callback`), and `ALLOWED_EMAILS`. Prod requires all four. Remove `PASSCODE`.

**Schema (next migration, `0002`).** Rebuild `users` with the SQLite 12-step method, inside the migration's
transaction, using `PRAGMA defer_foreign_keys = ON`: stage the old rows in a temp table, create `users_new`, drop
`users`, rename, then copy the rows back. The rebuild exists to drop `name UNIQUE`, since Google names collide. New
columns: `google_sub TEXT UNIQUE`,
`email TEXT UNIQUE COLLATE NOCASE`, `picture_url TEXT`, all nullable. Keep a plain index on `name COLLATE NOCASE`.
`users.active` stays and still turns a person off without deleting history. `db.User` gains `Email` and `PictureURL`;
`CreateUser(name)` goes away and `SignIn` owns creation. The next palette color still goes to a new user as the avatar
fallback.

**Avatar.** One `avatar` template partial draws the Google picture with `referrerpolicy="no-referrer"` and falls back
to colored initials. `web.Initials` stays.

Rejected: a hosted auth service (one provider, one instance, no gain); a server-side session table (a signed cookie
already works and the user row is re-read each request); a boot-time link step or CLI for history users (the `=Name`
entry does it with no tool).

## Verification

- `go test ./...` and `golangci-lint run ./...` pass.
- `SignIn` table tests, one per resolution step: known sub; allowed new email creates; email match links; `=Name` links an
  unlinked user and keeps its name; the same name already linked is not stolen; a stranger returns `ErrNotInvited` and
  leaves the users table unchanged; email case differs; two Google users with the same display name both exist.
- Migration test: seed a 0001 database with users, slips, legs, and wagers, migrate to 0002, and assert every row and
  foreign key survived and `PRAGMA foreign_key_check` is empty.
- Flow tests with a fake `Provider`: a wrong `state`, an expired or missing `pc_oauth`, a replayed callback, and an
  unverified email are all refused. One test runs the real `Provider` against an `httptest` server that serves OIDC
  discovery, JWKS, and a token endpoint, and covers a bad signature, wrong audience, and wrong nonce.
- Session tests: a tampered MAC, an expired cookie, an old `pc_user` cookie, an inactive user, and an email removed from
  `ALLOWED_EMAILS` all redirect to `/login`. An HTMX request gets `HX-Redirect`.
- Config tests: prod without a Google value or `ALLOWED_EMAILS` fails with all problems listed; dev with none starts.
- `/dev/login` returns 404 in prod and works in dev; a dev sign-in lands on `/`.
- Manual, once: sign in with a real Google account locally and in prod, and confirm a second account off the list sees
  the friendly 403 page. At phone width the login page and the 403 page fit with no horizontal scroll.

## Risks

- The users rebuild runs on real data. Dropping `users` with foreign keys on trips deferred checks unless the rename
  lands in the same transaction. The migration test above covers it; take a Litestream snapshot before the first prod
  deploy.
- A wrong redirect URI at Google gives `redirect_uri_mismatch`. The runbook lists the exact local and prod values.
- The consent screen in "Testing" mode expires tokens and caps test users. We ask only for `openid email profile`, which
  needs no Google verification, so the runbook says to publish it to production.
- A typo in `ALLOWED_EMAILS` locks someone out. The 403 page says who to ask, and startup logs how many entries parsed.
- `history-and-deploy` still lists `PASSCODE` in its deploy secrets. Its Cloud Run flags change to the Google secrets and
  `ALLOWED_EMAILS` when it is implemented; this change fixes `plan/architecture.md` to match.
- Cloud Run cold starts add a second to the callback. Acceptable at this traffic.

## TODO

Effort: medium (small flow, but a table rebuild, account linking, and two Google environments each need care)

### Phase 1: Identities and the gate

- [x] Add the 0002 users migration and `auth.SignIn` with the gate, covering all five resolution steps, plus the
  `ALLOWED_EMAILS` parser in `internal/config`, with the migration and resolution tests above. The old passcode flow
  keeps working.

### Phase 2: Replace the passcode

- [x] Add the Google `Provider`, the start and callback handlers, the `pc_session` cookie, `/login`, the friendly 403,
  `POST /logout`, and the dev login. Delete the passcode, picker, and `pc_auth`/`pc_user`. Prod config requires the Google
  values and drops `PASSCODE`. Tests cover the flow, session, and config cases above.
- [x] Add the `avatar` partial with picture and initials fallback, and show it in the top bar, with a render test.
- [x] Docs: rewrite `docs/concepts/access.md`; update `docs/reference/config.md`, `docs/reference/schema.md`, and
  `docs/guides/local-dev.md`; add `docs/runbooks/google-oauth-setup.md` (create the Web client, the two redirect URIs,
  publish the consent screen, set the secrets); fix the Routes and Deploy secrets rows in `plan/architecture.md`.
