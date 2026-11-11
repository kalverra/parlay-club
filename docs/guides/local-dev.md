---
id: local-dev
title: Local development
type: guide
summary: Start the app with hot reload, run the offline Sunday sandbox, set up the git hooks, and fall back to a plain server run when you need it.
tags: [development, mise, air, tailwind, server, sandbox, offline]
sources: [mise.toml, .air.toml, scripts/dev.sh, lefthook.yml, web/*.go, web/tailwind/*.css, web/templates/*.html, cmd/server/main.go, cmd/server/sandbox.go, internal/config/config.go, internal/sandbox/*.go, internal/auth/*.go, internal/groups/*.go]
last_verified: 2026-11-10
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

Dev defaults set `APP_ENV=dev`, port 8080, and the database at `./data/parlay.db`. Override any of them with an
environment variable, for example `PORT=9000 mise run dev`, which opens port 9000. The full list is in
[Configuration](../reference/config.md).

With no Google values set, the app opens on `/login` with a "Sign in without Google (dev)" link. Click it, enter any
email, and optionally a name, and the tabs load. The dev form calls the same sign-in code as Google: a new person gets a
"Just me" group in the same transaction. See [Access](../concepts/access.md) for the session and the cookies.

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

- The database is always `./data/sunday-sandbox.db`. `DB_PATH` is ignored, so a sandbox run cannot touch the normal
  `./data/parlay.db`.
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
  in as the same persona. It never touches `./data/parlay.db`.

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
`go generate ./...`, then builds `./tmp/server` and restarts it.

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

The first start creates `./data/parlay.db` and applies the migrations. `data/` is gitignored and excluded from the
watcher. Groups added `slips.group_id` by editing `0001_init.sql` in place, so a database created before groups existed
has the wrong shape. Delete it and let the migrations rebuild. To start over for any reason, stop the server and delete
it:

```sh
rm -rf data
```

## Check your work

```sh
curl -s localhost:8080/healthz   # prints ok
go generate ./...                # leaves no diff
golangci-lint run ./...
go test -short -race ./...
```

Every tab loads: after signing in, the Games tab shows the latest week's board once the slate syncs, and Research and
Record Book show a placeholder for now.
