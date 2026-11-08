# Architecture

## Stack

| Concern  | Choice                                                                                  |
| -------- | --------------------------------------------------------------------------------------- |
| Server   | Go (version in `go.mod`), `net/http` method routing, `html/template`                    |
| Frontend | HTMX 2.x vendored into `web/static/`, plus a small hand-written `app.js`                |
| CSS      | Tailwind v4 standalone CLI run by `go generate`; output committed                       |
| Assets   | Templates and static files embedded with `go:embed`                                     |
| Database | SQLite via `modernc.org/sqlite` (pure Go)                                               |
| Backup   | Litestream replicating to GCS                                                           |
| Hosting  | Cloud Run, `us-central1`, min 0 / max 1 instance, `*.run.app` URL                       |
| Secrets  | Secret Manager                                                                          |
| CI/CD    | GitHub Actions; Workload Identity Federation to GCP                                     |
| Logging  | zerolog, JSON to stdout in prod, console writer in dev                                  |

The live client is a plain `EventSource` in `app.js` instead of the HTMX SSE extension, because the connection policy
needs to close and reopen on tab visibility. See [live updates](../docs/concepts/live-updates.md).

## Repo layout

```text
cmd/server/              main: config, wiring, HTTP server, graceful shutdown
internal/config/         env parsing and validation
internal/db/             Open (pragmas), migrations/*.sql (embedded), Store with all queries
internal/auth/           session and OAuth cookies, sign-in handlers, middleware
internal/groups/         membership, the active-group scope, and the invite screens
internal/slips/          slip, member, leg rules (docs/concepts/slips.md "Editing and locking"); publishes to live
internal/parlay/         odds math, payout, units
internal/live/           hub and SSE handler
internal/refresh/        staleness rules, single-flight background syncs
internal/ingest/espn/    ESPN client and parsers
internal/ingest/weather/ venue coordinates and Open-Meteo client
internal/ingest/odds/    The Odds API client, event matching
internal/analyzer/       pure rules engine
internal/grader/         auto-grading (`grading-record-book`)
internal/seed/           history.yaml loader
internal/web/            routes, handlers, template rendering
web/                     embed.go (go:generate for Tailwind), templates/, static/, tailwind/input.css
seed/history.yaml
deploy/                  litestream.yml, entrypoint.sh, setup.sh, blackout.sh
Dockerfile
.github/workflows/
```

Handlers stay thin: parse the request, call `slips`/`db`, render. Rules live in `internal/slips`, not in handlers or
SQL.

## Routes

Built routes are documented in [docs/reference/routes.md](../docs/reference/routes.md). These are the routes still to
come. They use Go 1.22+ method patterns and inherit the same sign-in and group scope as the built ones.

| Method + path                              | Purpose                                                  | Change |
| ------------------------------------------ | -------------------------------------------------------- | ------ |
| `PUT /legs/{id}/result`                    | Manual grade                                             | 5     |
| `GET /research/players/{espnID}?leg_id=`   | Player research partial                                  | 6     |
| `GET /research/games/{id}`                 | Game research partial                                    | 6     |
| `GET /record?season=`                      | Record Book for the active group                          | 7     |
| `PUT /slips/{id}/wagers/me`                | Log own wager                                            | 7     |

## Database access

- `db.Open(path)` uses DSN pragmas: `journal_mode(WAL)`, `busy_timeout(5000)`, `foreign_keys(1)`, `synchronous(NORMAL)`,
  and `_txlock=immediate`. `SetMaxOpenConns(1)`: traffic is tiny and this rules out `SQLITE_BUSY`.
- Hand-written SQL in `internal/db` methods on a `Store`. Multi-statement changes run in one transaction.
- Tests open a fresh DB file under `t.TempDir()` with migrations applied.
