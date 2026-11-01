# Design: Foundation

## Context

The repo holds only tooling: `go.mod`, lint config, lefthook, mise. The target design lives in `plan/`. This change
builds the parts every later change stands on. Read before starting:

- `plan/architecture.md`: Stack, Repo layout, Config, Routes (change 1 rows), Auth, Database access.
- `plan/schema.md`: all of it.
- `plan/product.md`: Users and access; Screens (top bar and tab bar only).

## Approach

- `cmd/server` loads config, opens the DB, runs migrations, builds the mux, and serves. SIGTERM triggers
  `http.Server.Shutdown` with an 8 s deadline.
- `internal/config` parses env per the Config table. Prod rejects a missing `PASSCODE` or a `SESSION_KEY` under 32
  bytes.
- `internal/db` owns `Open`, the embedded migration runner (`PRAGMA user_version`), and a `Store`. `0001_init.sql` holds
  the full schema from `plan/schema.md`, so later changes add no migrations. This change only adds `Store` methods for
  users.
- `web/` embeds templates and static files. `go generate` runs the Tailwind v4 standalone CLI (pinned in `mise.toml`)
  and writes `web/static/app.css`, which is committed. HTMX 2.x is vendored with its version in the filename. In dev,
  templates reload from disk on each request.
- `internal/auth` holds cookie signing, the login, who, and add-user handlers, and two middlewares: `RequirePasscode`
  and `RequireUser`. The CSRF check (non-GET needs `HX-Request: true`, three exempt forms) is a third middleware.
- `internal/web` renders the layout: top bar with the current user's avatar, and a bottom tab bar whose four tabs show
  placeholders.

## Risks

- Tailwind v4 standalone CLI availability through mise: if no mise backend works, download a pinned release in the
  `go:generate` script and record the checksum.
- `modernc.org/sqlite` DSN pragma syntax differs from `mattn/go-sqlite3`; a test asserts `journal_mode` is `wal` and
  `foreign_keys` is on.

## TODO

### Phase 1: Server and database

- [x] Add internal/config with the env table from plan/architecture.md and tests for dev defaults and prod errors
- [x] Add internal/db Open with pragmas and MaxOpenConns(1); test that WAL and foreign keys are on
- [x] Add the embedded migration runner and 0001_init.sql; test fresh DB version and idempotent rerun
- [x] Add cmd/server with zerolog, /healthz, and graceful shutdown on SIGTERM
- [x] Docs: add docs/reference/config.md and docs/reference/schema.md; delete plan/schema.md and plan Config section

### Phase 2: Web shell

- [x] Add the Tailwind v4 CLI to mise.toml and a go:generate directive in web/embed.go that writes web/static/app.css
- [x] Vendor HTMX 2.x into web/static and add app.js with the local-time formatter for time elements
- [x] Add the base layout with top bar and bottom tab bar (Slip, Games, Research, Record Book placeholders)
- [x] Serve /static with cache headers and render templates from embed, reloading from disk when APP_ENV=dev
- [x] Docs: add docs/guides/local-dev.md (mise install, go generate, run the server, passcode dev)

### Phase 3: Passcode and user picker

- [x] Add users Store methods: list active, create with next avatar color, get by ID
- [x] Add internal/auth cookie signing and constant-time verification for pc_auth and pc_user
- [ ] Add /login, /who, and POST /users handlers with templates; wrong passcode waits 1 s
- [ ] Add RequirePasscode, RequireUser, and CSRF middleware with HX-Redirect handling
- [ ] Test redirects for missing, tampered, and inactive-user cookies, HX-Redirect, and CSRF rejection
- [ ] Docs: add docs/concepts/access.md; delete Users and access and Auth sections from plan/
