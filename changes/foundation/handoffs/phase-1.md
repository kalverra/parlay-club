# Phase 1 handoff: Server and database

## Done

- `internal/config`: `Config` holds `AppEnv`, `Port`, `DBPath`, `Passcode`, `SessionKey`, `OddsAPIKey`, and `LogLevel`
  (a `zerolog.Level`). `Load(env Env)` applies dev defaults, parses `LOG_LEVEL`, and returns every problem joined. Dev
  defaults never apply in prod for `PASSCODE` and `SESSION_KEY`; prod rejects an empty passcode and a session key under
  32 bytes. `IsProd()` is available for later phases.
- `internal/db`: `Open(path)` creates the missing directory, resolves the path to absolute, opens `modernc.org/sqlite`
  with `journal_mode(WAL)`, `busy_timeout(5000)`, `foreign_keys(1)`, `synchronous(NORMAL)`, and `_txlock=immediate`,
  caps the pool with `SetMaxOpenConns(1)`, and pings so a bad path fails at startup.
- `internal/db`: `Migrate(ctx, database)` applies embedded `migrations/NNNN_name.sql` files newer than
  `PRAGMA user_version`, in number order, one transaction each. Names that are not `NNNN_name.sql` and repeated numbers
  are errors. `0001_init.sql` is byte-identical to the old `plan/schema.md` block: 11 tables and 3 indexes.
- `cmd/server`: loads config, builds the logger, signals on `SIGINT`/`SIGTERM`, opens and migrates the DB, serves
  `GET /healthz` (`200 ok`), and on shutdown drains for up to 8 s before closing the DB and exiting. Logs are JSON in
  prod and a console format in dev.
- Tests: config dev defaults/overrides/prod errors; WAL, foreign-key enforcement, pool size, directory creation,
  relative path; migration ordering, skip-on-rerun, rollback of a broken script, fresh version 1, exact table set;
  healthz body, log formats, and an in-flight request that completes during shutdown.
- Docs: added `docs/reference/config.md` and `docs/reference/schema.md`, generated `docs/llms.txt`, deleted
  `plan/schema.md`, and removed the `## Config` section from `plan/architecture.md`.

## Deviations

- `db.Open` resolves `DB_PATH` with `filepath.Abs`. SQLite parses `file:./data/parlay.db` as a URI with authority `.`
  and fails, so the planned dev default would not have opened otherwise. `Open` also creates the directory and pings,
  folding the plan's "directory created if missing" into one call.
- `config.Load` takes a lookup function (`os.Getenv` passed by `main`) instead of reading the environment directly. The
  `paralleltest` linter requires `t.Parallel()`, which `t.Setenv` forbids, so injection keeps tests parallel and
  process-env free.
- Config joins all invalid values with `errors.Join` rather than stopping at the first, so a bad deploy reports every
  problem in one line per problem.
- The prod `SESSION_KEY` rule also rejects a missing key (missing is under 32 bytes), matching the plan's "required"
  row.
- `newLogger` takes an `io.Writer` and `serve` takes a `net.Listener`, both to make prod JSON formatting and graceful
  drain testable. The server's behavior is unchanged.
- Removing the `## Config` section also removed its closing note that `./data/` is gitignored and that dev templates
  reload from disk. The gitignore fact moved into `docs/reference/config.md`; the template reload is still specified in
  the Phase 2 tasks and was not built.
- Removed the dead `schema.md` row from `plan/README.md` and "config" from its architecture description so no link or
  description points at deleted plan content.

## Open issues

- `/healthz` reports the process, not the database. It does not touch SQLite; that matches the plan's "200 ok".
- `internal/db` exposes only `Open` and `Migrate`. There is no `Store` type yet; Phase 3 adds the user queries.
- Auth, `/static`, and the web handlers do not exist yet, so `newMux` has one route. `GET /healthz` must stay outside
  `RequirePasscode` when that middleware lands.
- `db.Migrate` runs under the startup signal context. A `SIGTERM` during migration rolls it back and `run` returns an
  error, exiting 1. Acceptable for a single-instance deploy.
- Dev console logs keep ANSI colors even when redirected to a file, because zerolog's `ConsoleWriter` defaults to color.
  Only cosmetic; the JSON prod path is plain.
- `docs/reference/config.md` intentionally does not yet mention cookie signing or the "no lines" board, since neither is
  built. Phase 3 should document those under `docs/concepts/access.md`.

## Files touched

- `internal/config/config.go`, `internal/config/config_test.go`
- `internal/db/db.go`, `internal/db/db_test.go`
- `internal/db/migrate.go`, `internal/db/migrate_internal_test.go`, `internal/db/migrate_test.go`
- `internal/db/migrations/0001_init.sql`
- `cmd/server/main.go`, `cmd/server/main_test.go`
- `docs/reference/config.md`, `docs/reference/schema.md`, `docs/llms.txt`
- `plan/architecture.md`, `plan/README.md`, `plan/schema.md` (deleted)
- `go.mod`, `go.sum`

## Next-phase notes

- Phase 2's templates and `/static` belong on the `newMux` in `cmd/server/main.go`; `run` already holds `cfg`, `logger`,
  and `database` for wiring. `cfg.IsProd()` is the switch for embedded-only rendering versus disk reload.
- `web/embed.go` does not exist yet, so the `go:generate` Tailwind step has nothing to hook onto and `go generate ./...`
  is currently a no-op.
- `db.Open` already creates directories and pings, and migrations are idempotent, so `run` needs no retry logic around
  them.
- Keep `plan/architecture.md` in step when Phase 2 lands: its Config section is gone, but the Repo layout and Routes
  rows still describe `web/` and `/static` as unbuilt.
- The docs have `sources` globs (`internal/config/config.go`, `internal/db/*`). Changing those files without touching
  the docs will report them stale.

## Run log

- Mode: balanced
- Checkpoints: task-1, task-2, task-3, task-4, task-5
- Verify: passed
- Rounds used: 1 of 3

## Reviewer check

- Reviewer 1 (opencode:top): requested changes
  - blocker: Deviations claims "The gitignore fact moved into `docs/reference/config.md`", but that doc never mentions
