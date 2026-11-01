---
id: access
title: Access
type: concept
summary: How one shared passcode, two signed cookies, and the name picker decide who is using the app.
tags: [auth, cookies, middleware, csrf]
sources: [internal/auth/cookies.go, internal/auth/handlers.go, internal/auth/middleware.go, internal/db/users.go, cmd/server/main.go, web/templates/login.html, web/templates/who.html]
last_verified: 2026-11-01
---

# Access

## Summary

There are no accounts. One shared passcode gets a browser into the app, and a picker names which person is using it. Two
signed cookies carry that state, and middleware checks them on every request.

## Cookies

| Cookie    | Value                                                    | Proves                          |
| --------- | -------------------------------------------------------- | ------------------------------- |
| `pc_auth` | `HMAC-SHA256(SESSION_KEY, PASSCODE)` as lowercase hex     | The browser knows the passcode. |
| `pc_user` | `{userID}.{HMAC-SHA256(SESSION_KEY, "user:" + userID)}`   | The browser picked that user.   |

Both cookies are `HttpOnly`, `Path=/`, `SameSite=Lax`, live for one year, and carry `Secure` in prod. The server
compares each MAC in constant time. The passcode never reaches the browser; only its HMAC does.

`SESSION_KEY` signs both cookies, and `pc_auth` is derived from `PASSCODE`. Changing either value therefore logs every
browser out, which is the intended way to revoke access.

## Sign-in flow

1. Every route except `/healthz`, `/static`, `/login`, `/who`, and `POST /users` needs both cookies.
2. `GET /login` shows the passcode form. `POST /login` checks the submitted passcode in constant time. A wrong value
   waits one second, then returns the form with a 401 and a short message. A right value sets `pc_auth` and redirects to
   `/who`.
3. `GET /who` lists the active users with their initials and avatar color, plus an "Add a name" field.
4. `POST /who` signs in the chosen user and redirects to `/`. `POST /users` creates a name, signs in the new user, and
   redirects to `/`. Names are trimmed, 1–24 characters, and unique ignoring case. A new name gets the next color from a
   fixed eight-color palette, round robin over every stored user.

`/who` and `POST /users` need only the passcode cookie; everything past the picker needs the user cookie too.

Inactive users keep their history. They never appear in the picker, and a signed cookie for one is sent back to `/who`.
`RequireUser` re-reads the user row on each request, so deactivating someone stops their next page load.

## Middleware

Requests pass through three wrappers, in this order:

| Middleware        | Rejects                                                  | Sends them to |
| ----------------- | -------------------------------------------------------- | ------------- |
| `CSRF`            | A non-GET request without `HX-Request: true`             | 403           |
| `RequirePasscode` | A missing, tampered, or stale `pc_auth` cookie            | `/login`      |
| `RequireUser`     | A missing, tampered, unknown, or inactive `pc_user` cookie | `/who`        |

`RequireUser` puts the loaded `db.User` on the request context, and handlers read it with `auth.UserFrom`.

`CSRF` exempts the three plain forms: `POST /login`, `POST /who`, and `POST /users`. Every other non-GET request must
come from HTMX. A custom header cannot be sent cross-site without CORS, and `SameSite=Lax` covers the exempt forms.

Auth pages send `Cache-Control: no-store` because they show names and sign-in state.

## Redirects

A normal request gets a 302 with `Location`. An HTMX request (`HX-Request: true`) gets a 200 with an `HX-Redirect`
header and an empty body, because HTMX would swap a 302's empty body into the page instead of navigating.

## Trade-offs

- One passcode means anyone who knows it can pick any active name. That is fine for a group of friends, and it keeps the
  app free of accounts, passwords, and resets.
- Access is all-or-nothing per browser. There is no per-device revocation; rotate `PASSCODE` or `SESSION_KEY` to sign
  everyone out.
- A stolen cookie works until it expires or the group rotates the passcode or session key.
- `SameSite=Lax` keeps normal links working. Writes rely on the `HX-Request` rule rather than `SameSite=Strict`.

## Related

- [Local development](../guides/local-dev.md): the dev passcode and how to sign in.
- [Configuration](../reference/config.md): `PASSCODE` and `SESSION_KEY`.
- [Database schema](../reference/schema.md): the `users` table.
