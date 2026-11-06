---
id: adr-foundation
title: Foundation
type: decision
summary: Build a bootable web server with a local database, a shared group passcode, and a "who are you?" picker that remembers each friend on their device.
last_verified: "2026-11-05"
---

# Foundation

## Context

Three friends build a shared NFL parlay every Sunday morning over group chat. Picks get lost in the thread, nobody sees
the combined odds, and there is no record of who picked what. There is no app yet: the repo holds only tooling config.

Nothing can be built until there is a running server that stores data, knows who each friend is, and keeps strangers
out.

## Goals and non-goals

**Goals:**

- Run a bootable Go web server with a local SQLite database, config validation, migrations, a health check, and graceful
  shutdown.
- Keep strangers out with one shared group passcode, then let each friend pick their name so their device remembers
  them.
- Ship a phone-first page shell with the Slip, Games, Research, and Record Book tabs ready for later changes.
- Enforce session cookies and CSRF on every request that changes data.

**Non-goals:**

- Slips, games, lines, weather, the analyzer, grading, or history. Later changes build those.
- Accounts, passwords, OAuth, or per-person logins.
- Scheduled jobs or outside data fetches.

## Decision

Build a bootable web server with a local database, a shared group passcode, and a "who are you?" picker that remembers
each friend on their device. It ships with an empty phone-first page shell that later changes fill in. No accounts or
passwords.

## Consequences

The repo now boots one Go binary with no external services: env config, an embedded SQLite database migrated at startup,
structured logs, a health check, and an eight-second graceful drain. Every later change starts from that instead of from
an empty repo.

What got easier:

- The full schema ships in `0001_init.sql`: 11 tables and 3 indexes. That was meant to spare later changes from writing
  migrations, so they can add query methods and build features without touching the database shape.
- Access control is three wrappers applied once. A route mounted inside the shell is protected by default; only the auth
  screens and static assets sit outside it.
- Templates and static files come from one embed. Content-hashed asset URLs make a one-year immutable cache safe.
- The layout already handles phone width, the four tabs, and chrome-less auth pages, so new screens reuse it.
- A fresh checkout runs with `go run ./cmd/server` and no secrets.

What got harder:

- The schema is frozen at version 1. Anything the schema lacks needs `0002`, and no real migration has exercised a
  second step; only tests with synthetic scripts have.
- `SetMaxOpenConns(1)` serializes every query. That is fine for a group, but lazy refresh and SSE work in later changes
  will queue behind page loads, and one slow query stalls the whole server.
- The DSN pragmas are specific to `modernc.org/sqlite`. Moving to another driver means rewriting `dsn` and the WAL and
  foreign-key tests.
- CSRF requires `HX-Request: true` on every non-GET unless the method-and-path is one of three hardcoded exemptions. A
  new plain form gets a 403 until someone remembers to add it.
- Every page template must be registered in `internal/web.pageFiles`; a missing entry fails startup.
- In dev, static files still come from the embedded FS with the production cache header. Editing `app.css` or `app.js`
  needs a rebuild and a cache clear. Templates do reload.
- Dev templates load through `os.DirFS("web")`, so the server must start from the repo root.

What we would revisit:

- `AGENTS.md` now says Google OAuth is the only auth model. Foundation deliberately ships a shared passcode and a name
  picker. One of the two has to move before change 2 adds a profile.
- CSRF treats only GET as safe, so HEAD and OPTIONS get a 403.
- The one-second pause is the only brake on passcode guessing.

## Alternatives considered

- **Accounts, passwords, or OAuth.** A non-goal in the proposal. A shared passcode plus a name picker fits three
  friends, needs no signup, reset, or account flow, and keeps identity in two cookies. The cost is that anyone with the
  passcode can pick any name, and revocation means rotating a secret.
- **Server-side sessions instead of signed cookies.** Signed HMAC cookies need no session table, no cleanup job, and
  survive restarts. They cannot be revoked per device; changing `PASSCODE` or `SESSION_KEY` signs everyone out.
- **`mattn/go-sqlite3` instead of `modernc.org/sqlite`.** The CGO driver would tie the build to a C toolchain and break
  the `CGO_ENABLED=0` static binary. The pure-Go driver keeps the Alpine image simple; the cost is its own DSN pragma
  syntax.
- **Tailwind through npm instead of the standalone CLI.** The standalone binary avoids Node and a lockfile, and mise
  already pins tools. The design's fallback, downloading a pinned release and recording a checksum in `go:generate`, was
  not needed because mise's aqua backend installed the CLI.
- **HTMX from a CDN or npm instead of vendoring it.** A versioned file with a recorded SHA-256 keeps the binary
  self-contained and gives the asset URL a stable hash.
- **Adding migrations as each change lands instead of shipping the full schema up front.** One `0001` means later
  changes never touch migrations. The cost is a large first script and no record of how the schema evolved; a later
  column still needs `0002`.
- **A 302 redirect for HTMX.** HTMX swaps a 302's empty body into the page instead of navigating. The middleware returns
  200 with `HX-Redirect` when `HX-Request: true`, and a normal 302 otherwise.

## Implementation notes

- The plan's dev DSN, `file:./data/parlay.db`, does not open with modernc: it reads `.` as the URI authority. `db.Open`
  resolves the path to absolute, creates the directory, and pings, so a bad `DB_PATH` fails at startup.
- `config.Load` takes an env lookup function. The `paralleltest` linter requires `t.Parallel()`, and `t.Setenv` cannot
  be used in a parallel test, so injection keeps the tests process-env free.
- `/who` and `POST /users` require the passcode but not a user cookie. A literal reading of the routes table would
  require both, which would trap a first-time visitor: adding a name is how they get a user.
- Auth page data embeds `web.Page`, so `Renderer.Render` accepts `any`. The layout still reads the shared fields.
- `/static` is mounted on the outer mux and again inside `web.New`. The outer route wins and serves assets before auth;
  the inner route is redundant but keeps `web.New` self-contained for its tests.
- Avatar colors are stored as palette names (`sky`, not `bg-sky-500`), and `AvatarColor.Class` maps them to literal
  Tailwind classes in Go. The store rotates over every user, active or not, so reactivating someone keeps their color.
- Name length (1–24 characters) is enforced in the auth handler, not in `db.CreateUser`. Another caller of the store can
  write a longer name.
- `RequireUser` re-reads the user row on every request, so deactivating someone takes effect on their next page load.
- `/healthz` reports the process, not SQLite. It never queries the database.
- Shell pages send no `Cache-Control`; auth pages send `no-store`. Change 2 adds user data to the shell and should
  decide the shell policy then.
- The avatar links to `/me`, which belongs to change 2 and 404s until then.
- `AssetURL` returns an unversioned URL for a file it does not know. A template typo is caught by the layout test, but
  the runtime fallback is silent.
- A bare `/static/` lists the embedded file names. The files are public, so this is harmless.
- `Migrate` runs under the startup signal context. A SIGTERM during migration rolls it back and exits 1.
- `go test -short -race ./...`, `go generate ./...`, and `golangci-lint run ./...` are clean; `good-vibes docs lint`
  reports the docs are fresh.

## Follow-ups

- Reconcile the auth rule: either update `AGENTS.md` to allow the shared passcode or propose an OAuth change. Do this
  before change 2 builds the profile.
- Add a lockout or rate limit to `POST /login`, or document that the one-second pause is the accepted risk. A private
  URL with a long passcode is the current defense.
- Treat HEAD as safe in `CSRF` (and OPTIONS if needed), or document why a HEAD request should get a 403.
- Move the name rule into `db.CreateUser` so every caller enforces 1–24 characters, not only the web handler.
- Serve static files from disk in dev, or shorten the dev cache header, so `go generate` and `app.js` edits show up
  without a rebuild.
- Make `AssetURL` fail loudly for an unknown file instead of silently returning an unversioned URL.
