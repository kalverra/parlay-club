# Phase 1: Phase 1: Installable app

## Done

### Task 1: Serve `GET /manifest.webmanifest` and `GET /sw.js` (no-cache, with only `install`, `push`, and `notificationclick` listeners and no `fetch` handler) outside auth in `newMux`. Add an icon generator that writes the 180, 192, 512, and maskable 512 PNGs into `web/static`. Add the manifest link, `apple-touch-icon`, Apple standalone meta tags, and safe-area padding to `layout.html`. Register the worker from `app.js`. Add route and layout tests.

- Added public `GET /manifest.webmanifest` and `GET /sw.js` routes ahead of auth in `newMux`. Both send `no-cache`; the
  manifest is `application/manifest+json` with hashed icon URLs, and the worker is `text/javascript`.
- Added `web/sw.js` with only `install`, `push`, and `notificationclick` listeners and no `fetch` handler or cache.
- Added `cmd/genicons`, invoked by `go generate`, which writes `icon-180.png`, `icon-192.png`, `icon-512.png`, and
  `icon-maskable-512.png` from the default theme's colors into `web/static`.
- Added the manifest link, 180 `apple-touch-icon`, Apple standalone meta tags, and top/bottom safe-area padding to
  `layout.html`, and registered `/sw.js` from `app.js` on load.
- Covered the new routes, worker listeners, icon dimensions, and layout tags with tests, and updated the public route
  table in `docs/reference/routes.md`.

### Task 2: Docs: add `docs/concepts/notifications.md` with an "Installing the app" section. Note the public routes in `docs/concepts/access.md`. Run `good-vibes docs lint` and `good-vibes docs index`.

- Added `docs/concepts/notifications.md`: installing on Android, iOS, and desktop; the manifest; icon generation; the
  service worker's listeners and missing `fetch` handler; the Apple tags and safe-area padding; tests and trade-offs.
- Noted the public routes in `docs/concepts/access.md`: health probes, static files, manifest, service worker, screenshot
  capability URLs, sign-in routes, and the prefill endpoint.
- Brought the access front matter up to the current schema and added an Overview diagram so the edited doc passes lint.
- Ran `good-vibes docs lint` (0 errors) and `good-vibes docs index`; `docs/llms.txt` now lists Access and Notifications.

## Deviations

### Task 1

- The manifest is marshaled in Go instead of served as a checked-in JSON file, so its icon URLs can carry the existing
  `/static` content hash. Without that, changing the icon art would leave installed clients on a year-cached old image.
- The 180 icon and the maskable 512 are full-bleed squares; iOS and Android apply their own masks. The 192 and 512 `any`
  icons keep rounded corners.

### Task 2

- The design asked only for the public route note in Access. Lint treats issues in changed docs as errors, so the Access
  doc's unsupported `sources` and `last_verified` fields and its missing Overview diagram had to be fixed as well.
- The doc states plainly that push is not wired yet. The later phases extend this page rather than rewriting the install
  material.

## Open issues

### Task 1

- `push` expects the JSON shape `{title, body, url, tag}`; nothing sends push payloads yet. That belongs to the next
  phase.
- The worker uses `/static/icon-192.png` for notification art without a content-hash query because the path is static
  JavaScript. The plain path still resolves; a future icon swap can update the constant or move the URL into the payload.

### Task 2

- `docs/concepts/notifications.md` covers install only. Opt-in, sending, events, and reminders land in later tasks.
- The repo still has 47 pre-existing lint warnings in other docs; none are in the two docs this task changed.

## Next-phase notes

### Task 1

- Keep the worker free of a `fetch` handler and any cache. The route test fails if either returns.
- `app.js` registers the worker on `window.load`; push subscription flow should reuse that registration rather than
  register again.
- Regenerate icons with `go generate ./...`; do not hand-edit the PNGs. Add a new size to `cmd/genicons` when needed.

### Task 2

- Extend the same page with the opt-in flow, subscription routes, and send behavior instead of adding a second concept doc.
- The worker payload shape `{title, body, url, tag}` is already documented; keep the server side in step with it.
- Phase 2 also adds config and schema reference updates, `docs/runbooks/push-setup.md`, and the "Push subscription"
  glossary term.

## Files touched

- `cmd/genicons/main.go`
- `cmd/server/main.go`
- `cmd/server/main_test.go`
- `docs/concepts/access.md`
- `docs/concepts/notifications.md`
- `docs/llms.txt`
- `docs/reference/routes.md`
- `internal/web/layout_test.go`
- `internal/web/pwa.go`
- `internal/web/pwa_test.go`
- `internal/web/static_test.go`
- `web/embed.go`
- `web/static/app.css`
- `web/static/app.js`
- `web/static/icon-180.png`
- `web/static/icon-192.png`
- `web/static/icon-512.png`
- `web/static/icon-maskable-512.png`
- `web/sw.js`
- `web/templates/layout.html`

## Run log

- Mode: 
- Checkpoints: task-1, task-2
- Verify: verify passed: test 300ms; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
