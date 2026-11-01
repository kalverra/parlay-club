---
id: local-dev
title: Local development
type: guide
summary: Install the tools, build the CSS, and run the server on your machine with the dev defaults.
tags: [development, mise, tailwind, server]
sources: [mise.toml, web/*.go, web/tailwind/*.css, web/templates/*.html, cmd/server/main.go, internal/config/config.go]
last_verified: 2026-11-01
---

# Local development

## Before you start

Install [mise](https://mise.jdx.dev) and run every command from the repo root. mise reads `mise.toml` and pins the Go
toolchain, `golangci-lint`, and the Tailwind CLI, so there is nothing to install by hand.

## Install the tools

```sh
mise install
lefthook install
```

`mise install` installs every tool in `mise.toml`. `lefthook install` wires the git hooks, so lint and generation run
before a commit and the tests run before a push.

## Build the CSS

Templates use Tailwind classes. After you add or change a class in `web/templates/`, run:

```sh
go generate ./...
```

That runs the pinned Tailwind CLI and rewrites `web/static/app.css`. The file is committed, and the repo's checks expect
`go generate` to leave no diff, so commit it with your template change.

HTMX and `app.js` in `web/static/` are not built. They ship as written.

## Run the server

```sh
go run ./cmd/server
```

Then open <http://localhost:8080>. Dev defaults set `APP_ENV=dev`, port 8080, and the database at `./data/parlay.db`.
Override any of them with an environment variable, for example `PORT=9000 go run ./cmd/server`. The full list is in
[Configuration](../reference/config.md).

Dev logs print as readable console lines; prod logs are JSON.

Template edits show up on the next request, because in dev the server reads `web/templates/` from disk every time. Go
changes need a restart. So do static files such as `web/static/app.js`, because they are embedded in the binary.

## Passcode

In dev, `PASSCODE` defaults to `dev` and `SESSION_KEY` has a fixed dev value, so a fresh checkout starts without any
secrets. Prod rejects an empty passcode and a session key under 32 bytes; see [Configuration](../reference/config.md).

## Database

The first start creates `./data/parlay.db` and applies the migrations. `data/` is gitignored. To start over, stop the
server and delete it:

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

Every tab loads: Slip, Games, Research, and Record Book each show a placeholder for now.
