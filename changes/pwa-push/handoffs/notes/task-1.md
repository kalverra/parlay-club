# Task 1 handoff: installable app

## Done

- Added public `GET /manifest.webmanifest` and `GET /sw.js` routes ahead of auth in `newMux`. Both send `no-cache`; the
  manifest is `application/manifest+json` with hashed icon URLs, and the worker is `text/javascript`.
- Added `web/sw.js` with only `install`, `push`, and `notificationclick` listeners and no `fetch` handler or cache.
- Added `cmd/genicons`, invoked by `go generate`, which writes `icon-180.png`, `icon-192.png`, `icon-512.png`, and
  `icon-maskable-512.png` from the default theme's colors into `web/static`.
- Added the manifest link, 180 `apple-touch-icon`, Apple standalone meta tags, and top/bottom safe-area padding to
  `layout.html`, and registered `/sw.js` from `app.js` on load.
- Covered the new routes, worker listeners, icon dimensions, and layout tags with tests, and updated the public route
  table in `docs/reference/routes.md`.

## Deviations

- The manifest is marshaled in Go instead of served as a checked-in JSON file, so its icon URLs can carry the existing
  `/static` content hash. Without that, changing the icon art would leave installed clients on a year-cached old image.
- The 180 icon and the maskable 512 are full-bleed squares; iOS and Android apply their own masks. The 192 and 512 `any`
  icons keep rounded corners.

## Open issues

- `push` expects the JSON shape `{title, body, url, tag}`; nothing sends push payloads yet. That belongs to the next
  phase.
- The worker uses `/static/icon-192.png` for notification art without a content-hash query because the path is static
  JavaScript. The plain path still resolves; a future icon swap can update the constant or move the URL into the payload.

## Next-phase notes

- Keep the worker free of a `fetch` handler and any cache. The route test fails if either returns.
- `app.js` registers the worker on `window.load`; push subscription flow should reuse that registration rather than
  register again.
- Regenerate icons with `go generate ./...`; do not hand-edit the PNGs. Add a new size to `cmd/genicons` when needed.

## Commit

Serve the web manifest and service worker

The app needs a stable public shell before push can work: browsers fetch the manifest and worker without a session.
The worker intentionally has no fetch handler, so install and notifications arrive without putting a cache between the
app and the server.
