---
id: adr-hot-reload-dev
title: Hot reload local dev
type: decision
summary: Add [air](https://github.com/air-verse/air) as the dev runner, pinned in `mise.toml` from the mise registry.
area: infra-and-dev
---

# Hot reload local dev

## Context

Local dev is a manual loop. A fresh checkout needs `mise install`, `lefthook install`, and `go generate ./...` before
anything runs. Then `go run ./cmd/server` starts the app, and every Go edit needs Ctrl-C and a fresh start. Static files
such as `web/static/app.js` are embedded in the binary, so those edits need the same restart. Template edits reload from
disk, but new Tailwind classes still need a manual `go generate ./...` to reach `app.css`.

All of that turns a one-line UI tweak into several commands and a restart. The friction adds up while building the tabs
and the slip screen in the next few changes.

## Goals and non-goals

**Goals:**

- One `mise run dev` command goes from a clean checkout to a running app on port 8080.
- Go edits rebuild and restart the server without a manual command.
- Template edits rebuild the CSS and show up on reload.
- Static asset edits restart the server, so the embedded copy is fresh.
- One Ctrl-C stops everything.
- Keep prod and the plain `go run ./cmd/server` path unchanged.

**Non-goals:**

- Browser auto-refresh. The page still reloads by hand; this change only rebuilds and restarts the server.
- Running tests, lint, or `go generate` on every change outside the dev loop.
- Changing the Docker build, the embed setup, or deploy.
- Devcontainers, editor configs, or a task runner other than mise.

## Decision

Add [air](https://github.com/air-verse/air) as the dev runner, pinned in `mise.toml` from the mise registry. A
repo-level air config watches Go, template, JS, and CSS files, excludes the generated `app.css` and the build directory,
and runs the project's own build and `go generate` before each restart. That way the Tailwind output and the embedded
assets are always in step with the source, without a second long-running watcher to manage.

A `mise run dev` task does the one-time setup and then hands off to air: it installs the pinned tools and seeds the CSS,
so a clean checkout works with one command. A separate `mise run setup` task installs the git hooks, since hooks are a
per-clone choice rather than part of every dev run. Air beats replacing it with a hand-rolled watcher or a second
process for Tailwind because it already rebuilds, restarts, and shows build errors, and it exits cleanly on Ctrl-C.

The effort is easy: a config file, a task, and a docs update. No app code changes.

## Consequences

Local dev is one command. `mise run dev` installs tools, regenerates the CSS, builds, and serves on port 8080; a save
lands within about two seconds. Go, template, and static edits all restart the server, so the embedded assets and the
Tailwind output never drift from the source. Ctrl-C stops air and the server together and removes `tmp/`. The plain
`go run ./cmd/server` path is untouched for anyone who does not want a watcher.

The cost is a dotfile and a pinned tool to maintain. Air only watches the extensions in `.air.toml`, so a new kind of
source file needs a config change. A generated file that is not excluded makes the loop rebuild forever. Long bursts of
agent writes still restart the server about once every two seconds, which is visible but bounded; `delay` is the knob if
that becomes noisy.

Revisit `delay` once the tabs and slip screens are in daily use. Two seconds is a compromise between fast human feedback
and low agent churn.

## Alternatives considered

- **A second Tailwind watcher.** It adds a process to start and stop, and the CSS output still must be re-embedded, so
  the server would restart anyway. Running generation before each build keeps one process and one lifecycle.
- **Air rules for templates.** Rules run a command without rebuilding, but the regenerated CSS still needs a rebuild to
  reach the embedded copy. The rule would only add a step in front of the same restart.
- **Serving static files from disk in dev.** This would avoid restarts for JS and CSS, but it adds a dev-only branch to
  the static handler for a convenience the pre-build already gives.
- **A hand-rolled watcher.** Air already rebuilds, restarts, prints build errors, forwards signals, and exits cleanly.
  Replacing it would mean owning that behavior for no product gain.

## Implementation notes

Air 1.67 names the run target `entrypoint`; the older `bin` field is deprecated. The config runs `go generate ./...` as
`pre_cmd`, builds `./tmp/server`, and excludes both `web/static/app.css` and `tmp/` so its own output cannot feed the
watcher. `delay = 2000` is a rate cap on bursts, not a quiet window. `stop_on_error = false` keeps the last good binary
serving while a partial edit fails to compile. `send_interrupt` plus `kill_delay = "2s"` lets the server run its
existing shutdown path before air cleans up.

`mise run setup` stays separate from `dev` because git hooks are a per-clone choice. The local-dev guide documents the
watched set, the exclusions, and the fallback path.

Two limits are worth knowing. Air watches only the extensions listed in `.air.toml`, so watch a new extension there
before expecting reloads. And any new generated output needs an `exclude_file` or `exclude_dir` entry; the watcher log
names the file that starts the loop.

## Follow-ups

- Fix the repo-wide `rumdl check .` failures outside this change: three MD013 line-length issues in
  `changes/google-oauth/proposal.md` and MD041 in `CLAUDE.md`.
- Consider a separate change for browser auto-refresh over the existing SSE hub. It was a non-goal here, and the page
  still reloads by hand.
