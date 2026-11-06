# Design: Google sign-in

## Context

Today `internal/auth` has two cookies (`pc_auth`, `pc_user`), a passcode form, and a name picker. `RequireUser` re-reads
the user row on every request and puts a `db.User` on the context; handlers read it with `auth.UserFrom`. That seam
stays. Nothing is deployed and there is no data, so this is a rebuild: the old system is deleted, not migrated. Read
before starting:

- `docs/concepts/access.md`, `docs/reference/config.md`, `docs/reference/schema.md`, `docs/guides/local-dev.md`.
- `plan/architecture.md`: Routes (the `/login` row) and Deploy (secrets).

## Approach

**Schema.** Edit `0001_init.sql` in place. `users` loses `name UNIQUE` (Google names collide) and gains
`google_sub TEXT NOT NULL UNIQUE`, `email TEXT NOT NULL UNIQUE COLLATE NOCASE`, and `picture_url TEXT`. Keep an index on
`name`. `active` stays so a person can be turned off without losing history. A palette color is still assigned to each
new user as the avatar fallback. Anyone with a local database deletes `data/parlay.db`.

**Sign-in is one function.** `auth.SignIn(ctx, Identity) (db.User, error)` takes a verified identity
(`Sub, Email, Name, Picture`). Google's callback and the dev login both call it. In one transaction:

1. If the email fails the gate, return `ErrNotInvited` and write nothing.
2. Find the user by `google_sub`, else create one from Google's name and picture.
3. Refresh the picture. The name is set at creation only.

**Gate.** `ALLOWED_EMAILS` is a comma list of emails, parsed in `internal/config`. `auth.Gate.Allows(email)` is the only
place that knows the list, so `groups` can delete it. `RequireUser` also asks the gate on every request, so removing an
email stops that person's next page load. In dev an empty list allows everyone.

**Google flow.** Use `golang.org/x/oauth2` and `github.com/coreos/go-oidc/v3`: authorization code with PKCE, `state`,
and `nonce`, scopes `openid email profile`. Reject a token with `email_verified` false.

```go
// Provider is Google in prod and a fake in tests.
type Provider interface {
    AuthURL(state, nonce, challenge string) string
    Exchange(ctx context.Context, code, verifier, nonce string) (Identity, error)
}
```

- `GET /auth/google/start` makes `state`, `nonce`, and a PKCE verifier, stores them in a signed `pc_oauth` cookie (10
  minutes, `Path=/auth`), and redirects to Google.
- `GET /auth/google/callback` checks `state`, exchanges the code, calls `SignIn`, sets the session, clears `pc_oauth`,
  and redirects to `/`. `ErrNotInvited` renders the login page with a friendly 403 ("That Google account isn't on the
  list yet. Ask whoever runs the club to add you.") and sets no cookie.

**Session cookie.** One `pc_session` cookie: `{userID}.{expiryUnix}.{mac}`, the MAC over
`"session:" + userID + "." + expiryUnix` keyed by `SESSION_KEY`, 30 days, `HttpOnly`, `SameSite=Lax`, `Secure` in prod.
`RequirePasscode`, `PASSCODE`, `pc_auth`, `pc_user`, `/who`, `POST /users`, `POST /login`, and `who.html` are deleted.
`POST /logout` clears the cookie (plain form, exempt from CSRF).

**Dev login.** When `APP_ENV=dev` and `GOOGLE_CLIENT_ID` is empty, `GET /dev/login` shows an email and name form, and
`POST /dev/login` calls `SignIn` with `Sub: "dev:" + email`. The routes are not registered in prod, and prod config
requires the Google values.

**Config.** Add `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `BASE_URL` (dev default `http://localhost:{PORT}`; redirect
URI is `BASE_URL + /auth/google/callback`), and `ALLOWED_EMAILS`. Prod requires all four. Remove `PASSCODE`.

**Avatar.** One `avatar` template partial draws the Google picture (`referrerpolicy="no-referrer"`) and falls back to
colored initials.

Rejected: a hosted auth service (one provider, one instance); a server-side session table (a signed cookie plus the
per-request user read is enough).

## Verification

- `go test ./...` and `golangci-lint run ./...` pass.
- `SignIn` table tests: known sub; new allowed email creates; the same sub with a changed picture updates it and keeps
  the name; a stranger returns `ErrNotInvited` and leaves the table unchanged; email case differs; two users with the
  same display name both exist.
- Flow tests with a fake `Provider`: a wrong `state`, a missing or expired `pc_oauth`, a replayed callback, and an
  unverified email are refused. One test runs the real `Provider` against an `httptest` server serving OIDC discovery,
  JWKS, and a token endpoint, and covers a bad signature, wrong audience, and wrong nonce.
- Session tests: a tampered MAC, an expired cookie, an inactive user, and an email removed from `ALLOWED_EMAILS` all
  redirect to `/login`. An HTMX request gets `HX-Redirect`.
- Config tests: prod missing a Google value or `ALLOWED_EMAILS` lists every problem; dev with none starts.
- `/dev/login` is 404 in prod and works in dev.
- Manual, once: sign in with a real Google account locally and in prod, and confirm an unlisted account sees the 403
  page. At phone width the login and 403 pages fit with no horizontal scroll.

## Risks

- A wrong redirect URI gives `redirect_uri_mismatch`. The runbook lists the exact local and prod values.
- A consent screen left in "Testing" mode expires tokens and caps test users. We ask only for `openid email profile`,
  which needs no Google verification, so the runbook says to publish it.
- A typo in `ALLOWED_EMAILS` locks someone out. Startup logs how many entries parsed, and the 403 page says who to ask.
- `history-and-deploy` still lists `PASSCODE` in its deploy secrets, and its history file creates users by name. When it
  is implemented, its Cloud Run flags use the Google secrets and `ALLOWED_EMAILS`, and the seed must supply each
  person's email so their first sign-in lands on their history. This change fixes `plan/architecture.md` to match.

## TODO

Effort: medium (small flow, but a new schema, a session scheme, and two Google environments need care)

### Phase 1: Identities and the gate

- [ ] Edit the users table in `0001_init.sql`, add `auth.SignIn`, the `Gate`, and the `ALLOWED_EMAILS` parser, with the
  `SignIn` and config tests above

### Phase 2: Replace the passcode

- [ ] Add the Google `Provider`, start and callback handlers, the `pc_session` cookie, `/login`, the 403 page,
      `POST /logout`, and the dev login. Delete every passcode, picker, and old cookie path. Prod config requires the
      Google values. Tests cover the flow, session, and config cases above.
- [ ] Add the `avatar` partial and show it in the top bar, with a render test
- [ ] Docs: rewrite `docs/concepts/access.md`; update `docs/reference/config.md`, `docs/reference/schema.md`, and
      `docs/guides/local-dev.md`; add `docs/runbooks/google-oauth-setup.md` (create the Web client, the two redirect
      URIs, publish the consent screen, set the secrets); fix the Routes and Deploy secrets rows in
      `plan/architecture.md`
