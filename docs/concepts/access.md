---
id: access
title: Access
type: concept
summary: How Google sign-in, the email allow list, and one signed session cookie decide who is using the app.
tags: [auth, google, oauth, cookies, middleware, csrf]
sources: [internal/auth/cookies.go, internal/auth/handlers.go, internal/auth/middleware.go, internal/auth/signin.go, internal/auth/gate.go, internal/auth/provider.go, internal/db/users.go, cmd/server/main.go, web/templates/login.html, web/templates/dev-login.html, web/templates/avatar.html]
last_verified: 2026-11-05
---

# Access

## Summary

People sign in with Google. Google proves the email, the email allow list decides who gets in, and one signed session
cookie keeps a browser signed in. There are no passwords, no account forms, and no name picker.

## Sign-in flow

1. `GET /login` is public. It shows a "Sign in with Google" button, plus a dev-only link when Google is not configured.
2. `GET /auth/google/start` makes a random `state`, `nonce`, and PKCE verifier, writes them to a signed `pc_oauth`
   cookie, and redirects to Google. The request asks for `openid email profile` with online access and PKCE `S256`.
3. Google returns the browser to `GET /auth/google/callback`. The handler checks `state` against the cookie, exchanges
   the code with the PKCE verifier, and verifies the ID token's signature, issuer, audience, expiry, and nonce.
4. An ID token with `email_verified` false is refused. Google must prove the address before it can name a user.
5. The verified identity goes through `auth.SignIn`. A known or allowed person gets a `pc_session` cookie, the
   round-trip cookie is cleared, and the browser lands on `/`.

A missing or expired `pc_oauth`, a wrong `state`, or a failed exchange returns the sign-in page with a 400 and "Sign-in
didn't work. Try again." An email off the allow list returns a 403 with "That Google account isn't on the list yet. Ask
whoever runs the club to add you." Neither case sets a session cookie.

## Who gets in

`ALLOWED_EMAILS` is a comma list of `email` or `email=Name` entries. `internal/config` parses it, and `auth.Gate` is the
only code that reads the list. An empty list allows everyone in dev; prod refuses to start with an empty list.

`auth.SignIn` resolves a verified identity inside one transaction, in this order:

1. A user with this Google subject (`google_sub`): refresh the picture and finish.
2. The email must pass the gate. A stranger stops here, and nothing is written.
3. A user with this email, ignoring case: attach the subject and picture to that user.
4. The gate entry carries a `=Name` and an unlinked user (no `google_sub`) has that name: attach the subject to that
   user. This is the one-time hand match for people from the history import. Their old slips become theirs.
5. Otherwise create a user from Google's name and picture.

A linked user keeps the name they already have. Google's name is used only when creating a user, so a changed Google
display name never renames someone's history. The picture refreshes on every sign-in.

`RequireUser` also asks the gate on every request. Removing an email stops that person's next page load, not only their
next sign-in. An inactive user (`users.active = 0`) is refused the same way.

## Cookies

| Cookie       | Value                                                 | Proves                                |
| ------------ | ----------------------------------------------------- | ------------------------------------- |
| `pc_session` | `{userID}.{expiryUnix}.{mac}`                          | The browser is signed in as that user. |
| `pc_oauth`   | `{state}.{nonce}.{verifier}.{issuedUnix}.{mac}`        | This browser started a Google trip.   |

Both cookies are `HttpOnly` and `SameSite=Lax`, and carry `Secure` in prod. The server compares each MAC in constant
time. `SESSION_KEY` signs both.

`pc_session` lives for 30 days at `Path=/`. Its MAC covers `"session:" + userID + "." + expiryUnix`, so a changed ID or
expiry breaks the cookie. `pc_oauth` lives for 10 minutes at `Path=/auth` and its MAC covers `"oauth:" +` the rest of
the value. The short life bounds a round trip that is abandoned or replayed.

The session cookie's name changed from the foundation's `pc_auth` and `pc_user`. Old cookies are simply ignored, so the
first request after this change sends every browser back to `/login`. Rotating `SESSION_KEY` likewise invalidates every
session.

## Middleware

The app shell passes through two wrappers:

| Middleware    | Rejects                                                               | Sends them to |
| ------------- | --------------------------------------------------------------------- | ------------- |
| `CSRF`        | A non-GET request without `HX-Request: true`                           | 403           |
| `RequireUser` | A missing, tampered, expired, inactive, unknown, or unlisted session  | `/login`      |

`CSRF` exempts two plain forms: `POST /logout` and `POST /dev/login`. Every other non-GET request must come from HTMX. A
custom header cannot be sent cross-site without CORS, and `SameSite=Lax` covers the exempt forms.

`RequireUser` re-reads the user row on every request, puts it on the request context, and handlers read it with
`auth.UserFrom`. It sends an HTMX request a 200 with `HX-Redirect: /login` instead of a 302, because HTMX would swap a
redirect's empty body into the page. Auth pages send `Cache-Control: no-store` because they show sign-in state.

## Avatars

One `avatar` partial draws a user. When `picture_url` is set it shows the Google picture with
`referrerpolicy="no-referrer"`. Otherwise it shows initials on the user's palette color. A new user gets the next color
from the fixed eight-color rotation. A linked history user keeps the name and color already stored, and Google only
refreshes the picture.

## Dev sign-in

`GET /dev/login` and `POST /dev/login` exist only when `APP_ENV=dev` and `GOOGLE_CLIENT_ID` is empty. The form takes an
email (required) and a name (optional). It calls `SignIn` with `Sub: "dev:" + email`, so the gate, email linking, and
history linking all run the same code as Google.

A dev with real client values uses the real Google flow, and the dev routes are not registered. Prod requires the Google
values and never registers them, so a misdeploy cannot expose a local sign-in form.

## Trade-offs

- Access is an email allow list, managed by hand. Adding or removing someone means editing `ALLOWED_EMAILS` and
  restarting or redeploying.
- A signed cookie cannot be revoked per device. Removing the email or deactivating the user stops their next request;
  rotating `SESSION_KEY` signs everyone out at once.
- A stolen session cookie works until it expires. There is no server-side session table to delete.
- Google is the only identity provider. A Google outage blocks new sign-ins, while already signed-in browsers keep
  working because each request reads SQLite and the gate.
- The app stores only what sign-in returns: subject, email, display name, and picture URL. It requests no Google API
  scopes beyond `openid email profile`, keeps no refresh token, and calls no Google API after sign-in.
- A linked user keeps their history name. Their Google display name can differ, which is intentional.

## Related

- [Local development](../guides/local-dev.md): the dev sign-in and how to test the real Google flow.
- [Configuration](../reference/config.md): `ALLOWED_EMAILS`, the Google client values, and `SESSION_KEY`.
- [Database schema](../reference/schema.md): the `users` identity columns.
- [Google OAuth setup](../runbooks/google-oauth-setup.md): the Web client, redirect URIs, consent screen, and secrets.
