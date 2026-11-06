---
id: local-dev
title: Local development
type: guide
summary: Start the app with hot reload, set up the git hooks, and fall back to a plain server run when you need it.
tags: [development, mise, air, tailwind, server]
sources: [mise.toml, .air.toml, scripts/dev.sh, lefthook.yml, web/*.go, web/tailwind/*.css, web/templates/*.html, cmd/server/main.go, internal/config/config.go, internal/auth/*.go]
last_verified: 2026-11-05
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

The app asks for the passcode first. In dev it is `dev`. After that, pick a name or add one, and the tabs load. See
[Access](../concepts/access.md) for how the cookies and the picker work.

Dev logs print as readable console lines; prod logs are JSON.

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

## Passcode

In dev, `PASSCODE` defaults to `dev` and `SESSION_KEY` has a fixed dev value, so a fresh checkout starts without any
secrets. Prod rejects an empty passcode and a session key under 32 bytes; see [Configuration](../reference/config.md).

The browser stores two signed cookies for a year, so the passcode step is a one-time thing per browser. To start over,
clear the site's cookies or use a private window. Changing `PASSCODE` or `SESSION_KEY` signs every browser out.

## Database

The first start creates `./data/parlay.db` and applies the migrations. `data/` is gitignored and excluded from the
watcher. To start over, stop the server and delete it:

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

Every tab loads: after signing in, Slip, Games, Research, and Record Book each show a placeholder for now.
