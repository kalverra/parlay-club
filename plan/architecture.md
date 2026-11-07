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
below needs to close and reopen on tab visibility.

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

Go 1.22+ method patterns. All routes except `/healthz`, `/static/`, `/login`, `POST /logout`, the Google sign-in routes,
and the dev sign-in routes need a signed-in user. Every authenticated route also runs the group scope, which picks the
active group or falls back to one the person belongs to.

| Method + path                              | Purpose                                                  | Change |
| ------------------------------------------ | -------------------------------------------------------- | ------ |
| `GET /healthz`                             | 200 `ok`                                                 | 1     |
| `GET /static/{path...}`                    | Embedded assets, long cache headers, hashed via query    | 1     |
| `GET /login`                               | Sign-in page: Google button, plus a dev link when Google is off | 1     |
| `POST /logout`                             | Clear the session and active-group cookies and return to `/login` | 1     |
| `GET /auth/google/start`                   | Start the Google round trip                              | 1     |
| `GET /auth/google/callback`                | Finish Google sign-in                                    | 1     |
| `GET /dev/login`, `POST /dev/login`        | Local sign-in, dev only and only when Google is not configured | 1     |
| `GET /groups`                              | Group list, create form, invite links, leave             | groups |
| `POST /groups`                             | Create a group and make it active                        | groups |
| `POST /groups/switch`                      | Make one of the caller's groups active                   | groups |
| `POST /groups/{id}/reset-invite`           | Mint a new invite link and kill the old one              | groups |
| `POST /groups/{id}/leave`                  | Leave a group, keeping the person's legs                 | groups |
| `GET /join/{token}`                        | Join page naming the group and its members               | groups |
| `POST /join/{token}`                       | Join the group, idempotently                             | groups |
| `GET /me`, `POST /me`                      | Profile: home book; `POST /users/{id}/active` toggles    | 2     |
| `GET /`                                    | Newest slip of the current week in the active group, else create form | 2     |
| `GET /slips/new`, `POST /slips`            | Create slip                                              | 2     |
| `GET /slips/{id}`                          | Full page                                                | 2     |
| `GET /slips/{id}/panel`                    | Slip panel partial (members, legs, odds, banner)         | 2     |
| `GET /slips/{id}/payout?stake=&boost=`     | Payout partial                                           | 2     |
| `GET /slips/{id}/activity`                 | Activity partial                                         | 2     |
| `POST /slips/{id}/members`                 | Add member                                               | 2     |
| `DELETE /slips/{id}/members/{userID}`      | Remove member                                            | 2     |
| `PATCH /slips/{id}`                        | Change `legs_per_member` or name                         | 2     |
| `GET /slips/{id}/legs/new?game_id=`        | Leg form partial                                         | 2     |
| `POST /slips/{id}/legs`                    | Add leg                                                  | 2     |
| `GET /legs/{id}/edit`, `PUT /legs/{id}`    | Edit leg                                                 | 2     |
| `DELETE /legs/{id}`                        | Delete leg                                               | 2     |
| `POST /slips/{id}/lock`                    | Current user locks in                                    | 2     |
| `POST /slips/{id}/unlock`                  | Unlock slip                                              | 2     |
| `GET /slips/{id}/events`                   | SSE stream                                               | 2     |
| `GET /weeks/{weekID}/games`                | Game board partial                                       | 3     |
| `GET /games/{id}/quickadd?slip_id=`        | Quick-add pills (`props-research`: triggers prop fetch)           | 3     |
| `PUT /legs/{id}/result`                    | Manual grade                                             | 5     |
| `GET /research/players/{espnID}?leg_id=`   | Player research partial                                  | 6     |
| `GET /research/games/{id}`                 | Game research partial                                    | 6     |
| `GET /record?season=`                      | Record Book for the active group                          | 7     |
| `PUT /slips/{id}/wagers/me`                | Log own wager                                            | 7     |

Rule violations from `internal/slips` return 422 with a short message rendered in the form. HTMX responses that change
the slip return the refreshed panel.

## Live updates

### Hub (`internal/live`)

- Topics are strings: `slip:{id}` and `week:{weekID}`.
- `Subscribe(topic) (<-chan struct{}, func())` returns a channel with buffer 1 and an unsubscribe func.
- `Publish(topic)` does a non-blocking send to each subscriber; a full buffer means an update is already queued, so
  bursts coalesce.
- Events are signals. Clients refetch the partials they show, so no per-user HTML goes through the hub.
- `internal/slips` publishes `slip:{id}` after every mutation commits. `internal/refresh` publishes `week:{id}` when a
  sync changes games or odds, and `slip:{id}` for that week's slips (odds and analyzer output depend on games).

### SSE handler (`GET /slips/{id}/events`)

- Subscribes to `slip:{id}` and `week:{weekID}`. Sends `event: slip-updated` or `event: board-updated` with `data: {}`.
- Sends a `: ping` comment every 25 s.
- **Deadline:** if any game in the week is live or kicks off within 2 hours, the stream lives up to 55 minutes;
  otherwise it closes after 10 minutes without an event (each event resets the timer). At the deadline it sends
  `event: bye` and returns. The clock is injected for tests.

### Client (`web/static/app.js`)

- One `EventSource` per page, opened from the slip page's `data-live-url`.
- On `slip-updated`: `htmx.trigger(document.body, "slip-updated")`. Same for `board-updated`. Panels declare
  `hx-get="..." hx-trigger="slip-updated from:body"`.
- On `bye`: close the stream; reopen on the next `visibilitychange` to visible or `focus`.
- When the tab becomes hidden: close the stream. When visible again: reopen and trigger both refresh events.
- After every `htmx:afterSwap`: format `<time>` elements in local time with `Intl.DateTimeFormat`.

## Refresh

`internal/refresh` owns when outside data is fetched. No cron, no Cloud Scheduler.

- `Kick()` runs from middleware on full-page `GET` loads (not partials, not SSE). It returns at once.
- For each source it reads `sync_state.fetched_at` and runs stale ones in a background goroutine with
  `context.WithoutCancel`, a 30 s timeout, and `singleflight` keyed by source so two page loads never double-fetch.
- Staleness rules are in data-sources.md, next to each source.
- On failure: log, keep old data, leave `fetched_at` unchanged, and back off that source for 5 minutes (in memory).
- After a successful sync that changed rows, publish to the hub (see Live updates).
- Grading (`grading-record-book`) is another source: it runs when any game is `FINAL` and has legs still `PENDING`.

## Database access

- `db.Open(path)` uses DSN pragmas: `journal_mode(WAL)`, `busy_timeout(5000)`, `foreign_keys(1)`, `synchronous(NORMAL)`,
  and `_txlock=immediate`. `SetMaxOpenConns(1)`: traffic is tiny and this rules out `SQLITE_BUSY`.
- Hand-written SQL in `internal/db` methods on a `Store`. Multi-statement changes run in one transaction.
- Tests open a fresh DB file under `t.TempDir()` with migrations applied.

## Persistence

- The container's entrypoint (`deploy/entrypoint.sh`):

  ```sh
  litestream restore -if-db-not-exists -if-replica-exists -config /etc/litestream.yml "$DB_PATH"
  exec litestream replicate -config /etc/litestream.yml -exec /app/server
  ```

- `deploy/litestream.yml` replicates `$DB_PATH` to `gcs://$LITESTREAM_BUCKET/parlay.db`. Pin one Litestream release in
  the Dockerfile and check the config syntax against that release's docs; it changed between 0.3 and 0.5.
- On SIGTERM, Litestream forwards the signal, the server shuts down within 8 s (`http.Server.Shutdown`), and Litestream
  does a final sync. Cloud Run allows 10 s.
- Cloud Run throttles CPU between requests, so replication can lag until the next request or shutdown. That is
  acceptable at this traffic.
- **Two-writer risk:** during a deploy, the new instance restores before the old one stops, so writes made in those
  seconds can be lost. The startup lease from the planning session is dropped: Cloud Run waits for the new instance to
  become ready before stopping the old one, so a lease would block every deploy. The deploy blackout below covers the
  risk instead.

## Deploy

### Image

Multi-stage `Dockerfile`: build stage `golang` (version from `go.mod`) runs `go build -trimpath -ldflags="-s -w"` with
`CGO_ENABLED=0`. The final stage is `alpine` (the entrypoint needs a shell) with the pinned Litestream binary, the
server at `/app/server`, `deploy/litestream.yml` at `/etc/litestream.yml`, and `deploy/entrypoint.sh`. Run as a non-root
user. `DB_PATH=/data/parlay.db`.

### Cloud Run service

```text
gcloud run deploy parlay-club --image ... --region us-central1 \
  --min-instances 0 --max-instances 1 --concurrency 80 --cpu 1 --memory 512Mi \
  --timeout 3600 --cpu-boost --allow-unauthenticated \
  --service-account parlay-runtime@PROJECT.iam.gserviceaccount.com \
  --set-env-vars APP_ENV=prod,DB_PATH=/data/parlay.db,LITESTREAM_BUCKET=BUCKET \
  --set-env-vars BASE_URL=https://SERVICE_URL \
  --set-secrets SESSION_KEY=session-key:latest,GOOGLE_CLIENT_ID=google-client-id:latest \
  --set-secrets GOOGLE_CLIENT_SECRET=google-client-secret:latest,ODDS_API_KEY=odds-api-key:latest
```

### One-time setup (`deploy/setup.sh`, run by the user)

Idempotent `gcloud` script taking `PROJECT_ID` and `BILLING_ACCOUNT`:

- enable `run`, `artifactregistry`, `secretmanager`, `iamcredentials`, `sts`, `billingbudgets` APIs;
- GCS bucket in `us-central1` for Litestream;
- Artifact Registry Docker repo with a cleanup policy keeping the last 3 images (free tier is 0.5 GB);
- service account `parlay-runtime`: `storage.objectAdmin` on the bucket, `secretmanager.secretAccessor` on the 4
  secrets;
- service account `parlay-deployer`: `run.admin`, `artifactregistry.writer`, `iam.serviceAccountUser` on
  `parlay-runtime`;
- Workload Identity pool and GitHub OIDC provider limited to repo `kalverra/parlay-club`, bound to `parlay-deployer`;
- secrets `session-key`, `google-client-id`, `google-client-secret`, `odds-api-key` created empty (the user adds
  versions);
- a $5 billing budget with email alerts at 50%, 90%, 100%;
- print the values to store as GitHub repo variables: `GCP_PROJECT`, `GCP_REGION`, `WIF_PROVIDER`, `DEPLOY_SA`,
  `RUNTIME_SA`, `LITESTREAM_BUCKET`, `AR_REPO`.

The user sets `BASE_URL` as a Cloud Run environment variable at deploy time. The Google client values come from the Web
client in the [Google OAuth setup runbook](../docs/runbooks/google-oauth-setup.md).

`docs/deploy.md` walks the user through running it, adding secret values, and setting the GitHub variables.

### Blackout

`deploy/blackout.sh` exits 1 when the current time in `America/New_York` is Sunday 09:00–23:59 or Monday 20:00–23:59,
else 0. It reads `NOW` (ISO time) when set, for tests.

## CI

- `.github/workflows/ci.yml` on pull requests and pushes to `main`: `jdx/mise-action`, `go generate ./...` then
  `git diff --exit-code`, `golangci-lint run ./...`, `go test -short -race ./...`.
- `.github/workflows/deploy.yml` on pushes to `main` after CI passes, and on `workflow_dispatch` with a `force` input:
  run `deploy/blackout.sh` (skip when `force`), authenticate with `google-github-actions/auth` via WIF, build and push
  the image tagged with the commit SHA, then `gcloud run deploy` as above. `concurrency: deploy` so deploys queue.
- `.github/workflows/live-apis.yml` (`live-api-watch`): weekly Wednesday cron and manual dispatch, runs
  `go test -run Live ./internal/ingest/...` without `-short`.
