# Phase 2 handoff: Replace the passcode

## Done

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

## Deviations

- `pc_oauth` carries an issued Unix timestamp as a fourth payload field, so the 10-minute limit is checked server-side
  and not only by the browser's `Max-Age`. This also means the cookie has five dot-separated parts.
- `POST /dev/login` joined `POST /logout` in the CSRF exempt list. Both are plain HTML forms, and the design named only
  logout.
- The Google provider discovers the OIDC document at startup in `cmd/server`, not on the first sign-in. That keeps the
  first callback fast, but it makes startup depend on Google's discovery endpoint. `NewGoogleProvider` takes an issuer
  argument so tests can point it at a local server.
- Provider failures and `ErrIncompleteIdentity` share the generic 400 page. `ErrNotInvited` is the only sign-in error
  with its own friendly 403 message.
- The dev login is enabled by `!cfg.IsProd() && cfg.GoogleClientID == ""` rather than checking for the literal `dev`
  value. Config rejects every `APP_ENV` other than `dev` and `prod`, so the result is the same.
- The avatar links to `/me`, a route that lands in a later change. The partial is in place and the link is unchanged
  from the foundation layout.

## Open issues

- A real Google sign-in has not been run manually. It needs the Web client, both redirect URIs, and the consent screen
  from the runbook, first locally and then in prod. The fake and `httptest` providers cover the code paths but not the
  real Google endpoints.
- Startup now calls Google's OIDC discovery endpoint. If discovery is unreachable or blocked, the server exits instead
  of starting. Cloud Run retries the instance, and a cold start pays that request once.
- `ErrNotInvited` leaves `pc_oauth` in the browser until it expires. It holds no session and the next start overwrites
  it, so this is harmless, but clearing it would be tidier.
- The phase 1 caveats still apply: two unlinked history users with the same name link only the lowest-id row, and a
  hand-edited email collision can fail a sign-in instead of showing a friendly page.
- The shell still has no sign-out control. `POST /logout` works, and the local-dev guide says to post to it by hand.
- The first prod deploy still needs a Litestream snapshot because migration 0002 rebuilds `users` on real data.

## Next-phase notes

- This change has no further phase after this one. Next is the close-out: review the handoff, write `closeout.md`, then
  run `good-vibes done google-oauth` after a person accepts it.
- Before closing, do the manual Google round trip from the runbook in dev and prod, including the 403 page for an
  account off the list and a phone-width check of both screens.
- Keep the gate check in `RequireUser`. It is what makes removing an email effective on the next page load.
- Wire the top bar avatar to the real profile page when `/me` lands. Feed `web.User.PictureURL` from
  `db.User.PictureURL` wherever a page builds its `web.Page`.
- When `history-and-deploy` is implemented, its Cloud Run flags must carry `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`,
  `BASE_URL`, and `ALLOWED_EMAILS`, and must not reintroduce `PASSCODE`.

## Files touched

- `cmd/server/main.go`
- `cmd/server/main_test.go`
- `docs/concepts/access.md`
- `docs/guides/local-dev.md`
- `docs/llms.txt`
- `docs/reference/config.md`
- `docs/reference/schema.md`
- `docs/runbooks/google-oauth-setup.md`
- `go.mod`
- `go.sum`
- `internal/auth/cookies.go`
- `internal/auth/cookies_test.go`
- `internal/auth/handlers.go`
- `internal/auth/handlers_test.go`
- `internal/auth/middleware.go`
- `internal/auth/middleware_test.go`
- `internal/auth/provider.go`
- `internal/auth/provider_test.go`
- `internal/auth/signin_test.go`
- `internal/config/config.go`
- `internal/config/config_test.go`
- `internal/db/users.go`
- `internal/db/users_test.go`
- `internal/web/layout_test.go`
- `internal/web/page.go`
- `internal/web/render.go`
- `internal/web/render_test.go`
- `plan/architecture.md`
- `web/static/app.css`
- `web/templates/avatar.html`
- `web/templates/dev-login.html`
- `web/templates/layout.html`
- `web/templates/login.html`
- `web/templates/who.html`

## Run log

- Mode:
- Checkpoints: task-2, task-3, task-4
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
