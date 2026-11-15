# Task 4

## Done

- Updated `AGENTS.md` free-tier guardrails to reflect first-authenticated-Sunday-visit pre-fill (`prefill:sunday` in `sync_state`) and `POST /internal/prefill` / Cloud Scheduler as an optional fallback.
- Updated `docs/concepts/data-refresh.md` (summary, auth/route boundary diagram, `Kick()` / `RunPrefill()` Sunday orchestration, one-pass flow, `prefill:sunday` staleness row, and guardrails).
- Updated `docs/reference/data-sources.md` budget and bulk-props sections for first-Sunday-visit `RunPrefill` (`prefill:sunday`) and optional `POST /internal/prefill` fallback.
- Updated `docs/guides/gcp-setup.md` and `docs/runbooks/deploy.md` to document `prefill-token` and the Cloud Scheduler job as an optional fallback alongside automatic first-Sunday-visit pre-fill.
- Ran `good-vibes docs lint`: `0 error(s), 38 warning(s)` (all warnings are pre-existing retired front-matter notices in untouched files; zero warnings on edited files).

## Deviations

None.

## Open issues

None.

## Next-phase notes

- All Phase 2 tasks are complete.
