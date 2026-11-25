---
id: local-dev
title: Local development
type: guide
summary: Start the app with hot reload, run the offline Sunday sandbox, set up the git hooks, run offline evals, and retrain the v2 grading model.
tags: [development, mise, air, tailwind, server, sandbox, offline, ml, train-v2]
budget:
  max_lines: 380
  reason: Covers the full local developer workflow in one guide, including dev server, Sunday sandbox, profiling, offline evals, and v2 model training.
---

# Local development

## Before you start

Install [mise](https://mise.jdx.dev) and run every command from the repo root. mise reads `mise.toml` and pins the Go
toolchain, `golangci-lint`, the Tailwind CLI, and the air dev runner, so there is nothing to install by hand.

## Start the app

```sh
mise run dev
```

That command installs the missing tools on the first run, regenerates `web/static/app.css`, builds the server, and
starts it. Once `/healthz` answers, it opens the app in Chrome. If Chrome is not installed it uses your default browser;
with no desktop opener it prints the URL instead. The browser opens once per run, so rebuilds do not open more tabs.
Leave it running while you edit.

Dev defaults set `APP_ENV=dev`, port 8080, and the database at ./data/parlay.db. Override any of them with an
environment variable, for example `PORT=9000 mise run dev`, which opens port 9000. The full list is in
[Configuration](../reference/config.md).

With no Google values set, the app opens on `/login` with a "Sign in without Google (dev)" link. Click it, enter any
email, and optionally a name, and the group list loads. The dev form calls the same sign-in code as Google: a new person
gets a "Just me" group in the same transaction. See [Access](../concepts/access.md) for the session and the cookies.

Dev logs print as readable console lines; prod logs are JSON.

## Run the Sunday sandbox

The sandbox is the same app running against a fixed week 9 Sunday. It replays saved provider responses instead of calling
out, so it needs no API keys and spends no Odds API credits.

```sh
mise run sandbox
# or, equivalently:
SUNDAY_SANDBOX=1 mise run dev
```

The hot-reload loop is unchanged. The differences are all in startup:

- The database is always ./data/sunday-sandbox.db. `DB_PATH` is ignored, so a sandbox run cannot touch the normal
  ./data/parlay.db.
- Startup skips history import and Google provider construction. The dev sign-in form is always on, even when `.env`
  holds Google values.
- Every ingest client points at `internal/sandbox`'s fail-closed replay transport. No request can reach a provider, and
  replayed calls never enter the response archive.
- A fresh sandbox runs the morning sync synchronously before it serves, so the slate, lines, projections, and forecasts
  are there on the first page load. Later restarts restore the saved stage and scenario clock instead of rebuilding.

The scenario clock starts on the week 9 Sunday morning. Game pickers, kickoff locks, the export card, and nfelo
freshness all read it, so the app agrees with the data it shows. Auth cookies and SSE timers keep real time so sessions
and open streams behave normally.

### Sandbox controls

Sign in, then open [http://localhost:8080/dev/sandbox](http://localhost:8080/dev/sandbox). The page shows the active
stage, the scenario time, who the browser is signed in as, and the active group. It also lists the four stages and the
four seeded people.

The stage buttons post to `/dev/sandbox/stage`. Each one rebuilds the Sunday morning baseline first, so every button
works from any previous state, then runs the real refresh at that stage's clock:

- **Sunday morning** is the baseline. The full week is scheduled, the shared slip is open, and every pick is pending.
- **Early loss** puts the Madrid game in the third quarter with Atlanta leading 17-10. It locks the shared slip and
  records a scripted `LOST` grade on Ada's Bengals pick. The other five picks stay pending, and the normal slip rules
  settle the slip `LOST` at once.
- **All clear** makes every Sunday game final with a scripted score and leaves the Monday night game upcoming. Every
  shared member locks in, and all six picks are scripted `WON`, so the slip settles `WON`.
- **Sunday complete** makes every Sunday and Monday game final with scores. The slip keeps its scripted wins, and no
  upcoming-game flow treats the week as Sunday morning.

The stage grades are manual scripted outcomes, not analyzer or score-derived results. A stage change removes and
recreates the seeded people, so the browser's real session and group cookies are reissued for the same persona. The
reset button is the same operation as the **Sunday morning** stage.

- Ada, Ben, and Cleo are in the shared **Sunday Sandbox** group. Each has two picks on the **Week 9 Sandbox** slip.
- Dex is the outsider. Dex has their own group, cannot see the shared slip, and gets the normal 404 on its URL.
- **Sign in as** posts to `/dev/sandbox/persona`. It calls the same sign-in code as `/dev/login`, then sets the real
  session and group cookies. Activity and grading name the person you chose, and the group switcher follows the same
  membership rules as any other browser.
- **Open shared invite** is the shared group's normal join link. Switching to Dex and opening it exercises the real
  invite flow; reset restores the outsider view.
- **Reset to Sunday morning** posts to `/dev/sandbox/reset`. It deletes every sandbox person, group, slip, pick,
  activity line, cache row, and manual edit, rebuilds the morning through the real services, and signs the browser back
  in as the same persona. It never touches ./data/parlay.db.

The controls page and its forms exist only while `SUNDAY_SANDBOX=1`. They sit inside the ordinary sign-in, group scope,
and HTMX CSRF checks, so a signed-out browser lands on `/login` and a plain form post is refused. In normal dev the paths
are not registered.

To start from a clean sandbox, stop the server and delete its file:

```sh
rm -f data/sunday-sandbox.db*
```

The normal dev database is separate, so that command never touches your own slips. Production rejects
`SUNDAY_SANDBOX=1`; see [Configuration](../reference/config.md).

### What reloads

Air reads `.air.toml` and watches Go files, templates, and static JS and CSS. Before every build it runs
`go generate ./...`, then builds ./tmp/server and restarts it.

- **Go edit:** rebuild and restart, and the new behavior serves.
- **Template edit:** the CSS regenerates, the server restarts, and the page shows the change on reload. Templates also
  come from disk in dev, so the next request always sees the current file.
- **Static edit:** a change to `web/static/app.js`, HTMX, or the CSS input restarts the server so the embedded copy is
  fresh. Asset URLs carry a content hash, so the new URL busts the browser cache.
- **Generated output:** `web/static/app.css` is rewritten by the pre-build, so it stays out of the watcher. So do
  `data/`, `tmp/`, `docs/`, `changes/`, `plan/`, Markdown files, and test files.

Air waits two seconds after a burst of writes begins, drains the events that queued up, then rebuilds once. A save lands
in about two seconds. A long agent run that keeps writing rebuilds about once every two seconds, not once per write.

Any new generated file must be excluded from the watcher in `.air.toml`. If a generated output is watched, each build
triggers the next one forever. Add the file to `exclude_file`, or its directory to `exclude_dir`, and the watcher log
names the file that caused the loop.

If a build fails, air keeps the last good server running and prints the error. The next edit retries.

## Stop the app

Ctrl-C stops air and the server together, and removes `tmp/`. The server's normal shutdown path still runs.

## Set up the git hooks

```sh
mise run setup
```

That runs `lefthook install`. The hooks lint and regenerate on commit and run the short tests on push. Hooks are a
per-clone choice, so `setup` is separate from `dev`.

## Run without hot reload

```sh
go run ./cmd/server
```

This starts the same server with no watcher. It is the right path when you only need the app running, or when air is in
the way. Template edits still show up on the next request, because dev reads `web/templates/` from disk. Go and static
edits need a restart. After a template change, run `go generate ./...` by hand to refresh `web/static/app.css`.

## Build the CSS

Templates use Tailwind classes. `mise run dev` runs `go generate ./...` before every build, so the dev loop handles it.
Outside the loop, after you add or change a class in `web/templates/`, run:

```sh
go generate ./...
```

That runs the pinned Tailwind CLI and rewrites `web/static/app.css`. The file is committed, and the repo's checks expect
`go generate` to leave no diff, so commit it with your template change.

HTMX and `app.js` in `web/static/` are not built. They ship as written.

## Sign-in

Sign-up is open. In dev, `SESSION_KEY` has a fixed value, so a fresh checkout starts without secrets and the dev form
lets any email in. Prod rejects a short session key and missing Google values; see
[Configuration](../reference/config.md).

Each sign-in starts a session that lasts 30 days. To start fresh, clear the site's cookies, use a private window, or post
to `/logout` (the shell has no sign-out link yet). Changing `SESSION_KEY` signs every browser out.

To test invites, sign in as one person, open `/groups`, copy the invite link, then sign in as someone else and open it.
See [Groups](../concepts/groups.md) for the join, reset, and leave rules.

### With real Google

To use the real Google flow locally, create a Web client and register the local redirect URI in the
[Google OAuth setup runbook](../runbooks/google-oauth-setup.md). Then start the server with the client values:

```sh
GOOGLE_CLIENT_ID="..." \
GOOGLE_CLIENT_SECRET="..." \
mise run dev
```

`BASE_URL` defaults to `http://localhost:8080`, which must match the registered redirect URI
`http://localhost:8080/auth/google/callback`. If you change `PORT`, change `BASE_URL` to match. With a client ID set,
`/dev/login` is not registered, so use the Google button.

## Database

The first start creates ./data/parlay.db and applies the migrations. `data/` is gitignored and excluded from the
watcher. Groups added `slips.group_id` by editing `0001_init.sql` in place, so a database created before groups existed
has the wrong shape. Delete it and let the migrations rebuild. To start over for any reason, stop the server and delete
it:

```sh
rm -rf data
```

## Benchmarks and profiling

Run the prod-scale benchmarks (`14` games, `15,500` odds rows, `2,100` roster rows) across `internal/board`, `internal/db`, and `internal/web`:

```sh
mise run bench
```

That runs `go test -run='^$' -bench=. -benchmem ./internal/board ./internal/db ./internal/web`. To focus on a single benchmark, pass `-bench` to `go test` directly:

```sh
go test -run='^$' -bench=BenchmarkBuild -benchmem ./internal/board
```

### Request phase timings (`Server-Timing`)

In non-prod (`APP_ENV` other than `prod`), every HTTP response except `/events` SSE streams includes a `Server-Timing` header with `db`, `board`, `render`, and `total` durations in milliseconds. Check it in browser DevTools under **Network > Timing**, or from the terminal:

```sh
curl -sI http://localhost:8080/healthz | grep -i server-timing
```

The server also logs every request with `duration_ms`, `db_ms`, `board_ms`, and `render_ms` (`WARN` at `>= 250ms`, `DEBUG` otherwise), and logs SQLite operations in `Store.Tx`, `UpsertOdds`, and `OddsForWeek` at `WARN` when they take `>= 100ms`.

### CPU and heap profiling (`pprof`)

`/debug/pprof/` is mounted in non-prod and returns `404` when `APP_ENV=prod`. Capture a 15-second CPU profile or inspect the live heap while reproducing slow requests locally (for example, against `mise run sandbox` or a restored database):

```sh
go tool pprof -http=:6060 "http://localhost:8080/debug/pprof/profile?seconds=15"
go tool pprof -http=:6060 http://localhost:8080/debug/pprof/heap
```

## Offline grading evals

Run offline grading evals to check how well A–F grades calibrate against settled slips or market-wide pre-kickoff lines,
or to compare a candidate `Rubric` JSON file against the baseline `v1` rubric. The eval CLI opens the database
read-only, makes no external API calls, and reconstructs historical inputs strictly from pre-kickoff `api_archive` rows
(including sharp benchmarks from archived odds) and prior-week `nflverse` data (including L5 game-log fallbacks when
archived ESPN game logs are missing) so live overwritten tables never leak future outcomes.

### Against a local database

```sh
go run ./cmd/eval --db ./data/parlay.db --rubric v1
```

Pass `--mode market` to evaluate deduplicated pre-kickoff consensus lines on `FINAL` games instead of settled user
slips (`--mode slips`, the default). Market mode grades game lines and team totals from final scores, grades player props
from completed archived ESPN box scores or same-week `nflverse` rows, and omits empty slip calibration and slip factor
sections from text, Markdown, and JSON outputs (`--mode` cannot be combined with `--dataset`):

```sh
go run ./cmd/eval --db ./data/parlay.db --mode market --all
```

Filter by season or week range, or emit JSON for scripting:

```sh
go run ./cmd/eval --db ./data/parlay.db --season 2025 --from-week 1 --to-week 12 --format json
```

To compare the built-in `v1` rubric against a candidate rubric file (for example, `rubric-v2.json` matching `analyzer.Rubric` JSON tags):

```sh
go run ./cmd/eval --db ./data/parlay.db --baseline v1 --candidate-file ./rubric-v2.json
```

Use `--candidate <name>` instead of `--candidate-file` when comparing two built-in rubrics, or `--all` to evaluate every built-in rubric and emit side-by-side diffs. Coverage output includes `sharp=<N>` and `l5_fallback=<N>` counts alongside game logs, injuries, nflverse, nfelo, and odds. The diff output prints data coverage counts, per-band leg and slip calibration deltas (win rate, lift over implied odds, flat ROI), per-factor attribution, and the leg band migration matrix.

To replay the synthetic benchmark fixture (the same check CI runs with `--format markdown` to populate `$GITHUB_STEP_SUMMARY`):

```sh
go run ./cmd/eval --dataset internal/eval/testdata/benchmark_dataset.json --all --format markdown
```

### Against a Litestream-restored database

Run `mise run eval-prod` (backed by `scripts/eval-prod.sh`) to restore a read-only production snapshot into a temporary directory under ./data/, run `cmd/eval`, and clean up the temporary files automatically on exit:

```sh
LITESTREAM_BUCKET=<bucket> mise run eval-prod
```

By default, `eval-prod` passes `--all` to `cmd/eval`. Pass custom `cmd/eval` flags after `--` (set `LITESTREAM_REPLICA_URL` instead of `LITESTREAM_BUCKET` if using a custom replica URL):

```sh
LITESTREAM_BUCKET=<bucket> mise run eval-prod -- --mode market --all
```

## Train the v2 grading model

The v2 grading model trains offline in Python under `ml/` and exports flat decision trees, calibrators, and copula
matrices as JSON embedded into the Go binary. The `ml/` workspace is excluded via `.dockerignore`, so Cloud Run stays a
single static Go binary.

### Prerequisites

Install [uv](https://docs.astral.sh/uv/). Python 3.12 is pinned in `ml/.python-version` and dependencies are locked in
`ml/uv.lock`.

### Run `mise run train-v2`

```sh
mise run train-v2
```

That runs `uv run python export.py` in `ml/`. The first run downloads historical nflverse data (seasons 2019–2025 by
default) and caches Parquet files under ml/cache (gitignored). Pass script flags after `--` to refresh cached data or
change the season range and HPO budget:

```sh
mise run train-v2 -- --refresh --seasons 2019-2025 --trials 8
```

Before writing any artifact, `ml/train.py` runs expanding-window walk-forward validation across at least three held-out
seasons, logs KS covariate-shift checks, drops collinear features with pairwise Spearman `> 0.7`, and enforces these
gates:

- At least 80% of calibration probability bins populated.
- Walk-forward Log Loss and Brier Score at or below the de-vigged market baseline.
- Copula SGP Log Loss below naive leg independence.
- EWMA half-life `±1` week perturbation MAE `<= 0.05`.

When all gates pass, `ml/export.py` writes three files:

- `internal/modelv2/artifacts/v2.json` — model bundle embedded into Go via `internal/modelv2`.
- `internal/modelv2/testdata/parity_vectors.json` — leg and copula test vectors for Go/Python parity checks.
- `ml/reports/latest.md` — walk-forward gate metrics, pruned features, and top KS shifts.

### Retraining cadence

- **Pre-season (August):** run a full retrain across completed seasons before Week 1.
- **Mid-season and pre-playoffs:** evaluate archived production slates with `mise run eval-prod`; retrain only if
  covariate drift or Log Loss degrades enough to justify updating artifacts.
- **Feature or schema changes:** retrain whenever features in `ml/features.py` or `internal/modelv2` change.

### PR verification steps

Before opening a PR that updates `ml/` or model artifacts, run the Python tests, Go parity tests, and standard repo
checks:

```sh
(cd ml && uv run pytest -v)
go test ./internal/modelv2
golangci-lint run ./...
go test -short -race ./...
```

When you have access to the production replica, compare v2 against archived slates:

```sh
LITESTREAM_BUCKET=<bucket> mise run eval-prod
```

Commit `internal/modelv2/artifacts/v2.json`, `internal/modelv2/testdata/parity_vectors.json`, and `ml/reports/latest.md`
together, and include the gate summary table from `ml/reports/latest.md` in the PR description.

## Check your work

```sh
curl -s localhost:8080/healthz   # prints ok
go generate ./...                # leaves no diff
golangci-lint run ./...
go test -short -race ./...
```

Every screen loads: after signing in, a person in one group lands on that group's week, which shows the create-slip form
until a slip exists. The hub's pick action opens the pick board, and `/games` redirects to the active group's week.
