# Task 7: Docs: update `docs/reference/routes.md` and `docs/concepts/slips.md` for wager endpoints and behavior

## Done

- Updated `docs/reference/routes.md` (`last_verified: 2026-11-14`, `sources`, and `tags`) to document `PUT /slips/{id}/wagers/me` and `DELETE /slips/{id}/wagers/me`, including request fields, group-membership access, 422 validation rules, and the `slip-wager` partial response.
- Updated `docs/concepts/slips.md` (`sources`, `tags`, `## The hub`, typed errors table, and a new `## Wagers` section) to describe the **My wager** form (`slip-wager`), its placement outside `slip-panel`, validation/default rules, live `slip:{id}` publishing, and `ErrWager*` errors.
- Verified documentation front matter and links with `good-vibes docs lint` (`ok`).

## Deviations

None.

## Open issues

None.

## Next-phase notes

- No code changes were made in this task; all docs match the Phase 3 wager implementation in `internal/db/wagers.go`, `internal/slips/wagers.go`, `internal/web/slips.go`, `web/templates/slip.html`, and `web/templates/partials.html`.
