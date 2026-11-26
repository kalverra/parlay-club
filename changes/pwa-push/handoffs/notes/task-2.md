# Task 2 handoff: install docs and public routes

## Done

- Added `docs/concepts/notifications.md`: installing on Android, iOS, and desktop; the manifest; icon generation; the
  service worker's listeners and missing `fetch` handler; the Apple tags and safe-area padding; tests and trade-offs.
- Noted the public routes in `docs/concepts/access.md`: health probes, static files, manifest, service worker, screenshot
  capability URLs, sign-in routes, and the prefill endpoint.
- Brought the access front matter up to the current schema and added an Overview diagram so the edited doc passes lint.
- Ran `good-vibes docs lint` (0 errors) and `good-vibes docs index`; `docs/llms.txt` now lists Access and Notifications.

## Deviations

- The design asked only for the public route note in Access. Lint treats issues in changed docs as errors, so the Access
  doc's unsupported `sources` and `last_verified` fields and its missing Overview diagram had to be fixed as well.
- The doc states plainly that push is not wired yet. The later phases extend this page rather than rewriting the install
  material.

## Open issues

- `docs/concepts/notifications.md` covers install only. Opt-in, sending, events, and reminders land in later tasks.
- The repo still has 47 pre-existing lint warnings in other docs; none are in the two docs this task changed.

## Next-phase notes

- Extend the same page with the opt-in flow, subscription routes, and send behavior instead of adding a second concept doc.
- The worker payload shape `{title, body, url, tag}` is already documented; keep the server side in step with it.
- Phase 2 also adds config and schema reference updates, `docs/runbooks/push-setup.md`, and the "Push subscription"
  glossary term.

## Commit

Document app install and public routes

Phase 1 makes the app installable, so people need one place that explains the home-screen flow and the routes that must
stay public before sign-in. Push is called out as not wired yet so the next phase can extend the same page instead of
replacing it.
