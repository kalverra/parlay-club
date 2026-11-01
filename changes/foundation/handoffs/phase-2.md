# Phase 2 handoff: Web shell

## Done

- `mise.toml` pins the Tailwind v4 standalone CLI (`aqua:tailwindlabs/tailwindcss` 4.3.3). `web/embed.go` carries the
  `go:generate` line that compiles `web/tailwind/input.css` into `web/static/app.css`, which is committed.
- `web/tailwind/input.css` imports Tailwind v4 with automatic source detection off and scans only `web/templates` and
  `internal`. Docs, plan files, and the generated CSS cannot leak classes in, and color classes written in Go are
  compiled.
- `web/static/htmx-2.0.11.min.js` vendors HTMX 2.0.11. Its header records the upstream URL, the 0BSD license, and the
  downloaded file's SHA-256. `web/static/app.js` formats every `<time datetime>` in the browser's zone with
  `Intl.DateTimeFormat`, on first load and after `htmx:afterSwap` and `htmx:oobAfterSwap`.
- `web/templates/layout.html` is the base layout: skip link, top bar (week label, slip switcher when the week has more
  than one slip, avatar linking to `/me`), the page content slot, and a fixed bottom tab bar with four tabs (Slip,
  Games, Research, Record Book) that marks the current tab with `aria-current="page"`. Dark, phone first, safe-area
  padding on the tab bar.
- Four placeholder pages, `web/templates/{slip,games,research,record}.html`, each defining the `content` block the
  layout wraps. The placeholders name what will replace them.
- `internal/web` (new): the layout data (`Page`, `Tab`, `NavTab`, `SlipOption`, `User`), `Page.Tabs()`, `Tab.Label()`,
  and `Initials()`. `Renderer` parses layout plus page once in prod and re-reads from disk per render when reload is on.
  `AssetURL` adds a 10-hex content hash. `StaticHandler` serves `web/static` with
  `Cache-Control: public, max-age=31536000, immutable`. `New(logger, renderer)` mounts `/` (Slip), `/games`,
  `/research`, `/record`, and `GET /static/{path...}`.
- `cmd/server`: `newRenderer` uses the embedded templates in prod and `os.DirFS("web")` with reload in dev. `newMux`
  mounts the shell at `/` and keeps `/healthz` outside it.
- Tests: embedded assets, the htmx filename/version convention, the tab bar and current tab, top bar with and without
  data, switcher at one versus many slips, avatar, each placeholder page, every template asset URL resolving to a
  vendored file, renderer reload versus cache, broken and unknown templates, asset hashing, static cache/type/404,
  path-traversal rejection, all four pages through the mux, and `/static` through the shell.
- Docs: added `docs/guides/local-dev.md` (mise install, `go generate`, run the server, dev passcode, database reset, the
  check commands) and recorded that `data/` is gitignored in `docs/reference/config.md`. Regenerated `docs/llms.txt`.

## Deviations

- Tailwind installed cleanly through mise's aqua backend, so the design's fallback (download a pinned release in
  `go:generate` and record its checksum) was not used.
- The design names four placeholder tabs but no URLs. Added full placeholder pages at `/`, `/games`, `/research`, and
  `/record`. The plan's Routes table has no `/games` or `/research` pages: games and research arrive as partials in
  later changes, so those pages will be replaced or folded into panels. `/record` matches change 7.
- Static URLs carry `?v=<10 hex chars of SHA-256>` and are served as immutable for a year, per the plan's "hashed via
  query". HTMX gets the query too, even though its filename already carries the version.
- The avatar renders only when `Page.User` is set (name, initials, avatar class). That lets Phase 3 pass a user without
  touching the layout, and no avatar shows today.
- Avatar colors are full Tailwind class strings supplied from Go (`User.AvatarClass`), so `input.css` also scans
  `internal/`. A class has to appear literally in Go source; building it by concatenation would not compile. No palette
  exists yet.
- `Initials` returns up to two uppercase letters, word by word. The design did not say where that lived.
- Dev template loading uses `os.DirFS("web")`, so the server must run from the repo root; the startup error says so.
  Templates reload, but static files stay embedded in dev: editing `app.js` needs a restart, and a new Tailwind class
  needs `go generate`.
- `Renderer` validates every page at startup even in reload mode, so a broken template fails startup rather than the
  first request. Renders are buffered, so a failure writes no partial page.
- The shell's root route is `GET /{$}` so it matches only `/`. The shell is mounted at `/` on the outer mux, and
  `/healthz` stays outside it.

## Open issues

- `/games`, `/research`, and `/record` are full placeholder pages with no data. Changes 2, 3, 6, and 7 have to replace
  them. The plan's games and research routes are partials, so those two may become panels at `lg` rather than pages.
- The shell is unauthenticated. Phase 3 middleware must wrap the shell or its routes while leaving `/healthz`, `/login`,
  `/who`, and `/static` reachable. `newMux(shell)` takes one handler, which keeps that wrapper simple.
- HTML responses set no `Cache-Control`. Phase 3 should decide `no-store` for pages that carry a user's data.
- The avatar expects `User.Name`, `User.Initials`, and `User.AvatarClass`, and links to `/me`, which change 2 adds.
  Until then the link 404s once Phase 3 starts setting users.
- `AssetURL` returns an unversioned URL for a name it does not know. The layout test fails if a template links a file
  that is not embedded, so a typo is caught, but the runtime fallback itself is silent.
- A bare `/static/` lists the embedded files. They are public assets, so this is harmless; tighten it if a later change
  cares.
- Dev console logs keep ANSI colors when redirected, because zerolog's `ConsoleWriter` defaults to color. Carried over
  from Phase 1 and still cosmetic.
- `docs/guides/local-dev.md` says the tabs show placeholders and does not mention signing in. The Phase 3 docs task
  should add the login step with the dev passcode.
- `plan/architecture.md`'s Repo layout now matches the tree. Its Routes table lists `/static` under change 1 (now built)
  and has no rows for the shell's `/games` and `/research` pages; Phase 3 can keep the target table as is or note the
  shell pages.

## Files touched

- `mise.toml`
- `web/embed.go`, `web/embed_test.go`
- `web/tailwind/input.css`, `web/static/app.css`, `web/static/app.js`, `web/static/htmx-2.0.11.min.js`
- `web/templates/layout.html`, `web/templates/slip.html`, `web/templates/games.html`, `web/templates/research.html`,
  `web/templates/record.html`
- `internal/web/page.go`, `internal/web/page_test.go`
- `internal/web/render.go`, `internal/web/render_test.go`
- `internal/web/static.go`, `internal/web/static_test.go`
- `internal/web/routes.go`, `internal/web/routes_test.go`
- `internal/web/layout_test.go`
- `cmd/server/main.go`, `cmd/server/main_test.go`
- `docs/guides/local-dev.md`, `docs/reference/config.md`, `docs/llms.txt`

## Next-phase notes

- Phase 3 adds `internal/auth` and users. Handlers should pass `Page.User{Name, Initials, AvatarClass}`; the layout
  already renders it and links to `/me`.
- `internal/web.New(logger, renderer)` is the shell mux. Register auth routes on the outer mux before the `/` mount, or
  wrap the shell, so `/login`, `/who`, `/users`, and `/static` bypass the passcode and user checks as the design says.
- `web/templates/layout.html` assumes a page tab. With `Page.Tab == ""`, `Page.Tabs()` marks nothing current, so auth
  pages can reuse the layout, or add a minimal layout if they should not show the tab bar.
- Add the avatar palette classes as literal Go strings in `internal/db` (the design puts the palette there) and pass the
  chosen class in `User.AvatarClass`; `input.css` scans `internal/` and compiles them.
- Login, who, and add-user templates should follow the same `{{define "content"}}` convention and join `pageFiles`, or
  use their own template set if they need a different layout.
- Update `docs/guides/local-dev.md` in the Phase 3 docs task: the dev passcode is `dev`, and the guide should say where
  to type it.
- Docs staleness: no doc lists `internal/web/*` in `sources`, so changes there will not warn. `docs/guides/local-dev.md`
  sources `web/*`, `web/tailwind/*`, `web/templates/*`, `cmd/server/main.go`, and `internal/config/config.go`. Changing
  those will mark it stale, which is correct.

## Run log

- Mode: balanced
- Checkpoints: task-6, task-7, task-8, task-9, task-10
- Verify: passed
- Rounds used: 1 of 3
- Nits:
  - phase 2, task 6: `web/tailwind/input.css` has `@source "../templates"`, but `web/templates/` does not exist yet. Tailwind runs fine without it, and later phases add templates, so no action needed now.
  - phase 2, task 9: in dev, `StaticHandler` serves from the embedded FS and sets the one-year immutable cache on every response. After `go generate` rewrites `app.css`, a dev browser keeps the stale file until the server is rebuilt (which changes the `?v=` hash) or the cache is cleared. Consider a shorter/no-cache header when `APP_ENV=dev`, or serving static from disk in dev like templates.

## Reviewer check

- Reviewer 1 (opencode:top): requested changes
  - blocker: Open issues claims "plan/architecture.md's Repo layout now matches the tree." It does not: the plan lists `internal/auth`, `internal/slips`, `internal/parlay`, `internal/live`, `internal/refresh`, `internal/ingest/*`, `internal/analyzer`, `internal/grader`, `internal/seed`, `deploy/`, `Dockerfile`, `seed/history.yaml`, and `.github/workflows/`, none of which exist yet. Rephrase as "matches the parts built so far" or drop the claim.
  - nit: The bare-`/static/` directory-listing open issue is asserted but not covered by a test; a one-line test would pin the behavior either way.
