# Task 3

## Done

- Updated `docs/concepts/data-refresh.md` to document `GameDayPrefillSource` (`"prefill:gameday"`), `maybeRunGameDayPrefill`, `gameDayPrefillWindow`, `gameDayPrefillStale`, scoreboard-derived `isGameDay` staleness rules, and `RunPrefill` prop warming scoped to `upcomingGamesOnDate` (with week-wide fallback on non-game-day manual invocations).
- Added `## Overview` (with the existing Mermaid diagram) and a `budget` front-matter override in `docs/concepts/data-refresh.md`, and trimmed `summary` to <= 240 characters so `good-vibes docs lint` passes with zero errors.
- Updated the `## Free-tier guardrails` section in `AGENTS.md` to replace Sunday-only `prefill:sunday` wording with per-game-day `prefill:gameday` pre-fill and date-scoped Parlay-only prop warming.
- Regenerated `docs/llms.txt` via `good-vibes docs index` to reflect the updated `data-refresh` summary.

## Deviations

- None.

## Open issues

- Reference, guide, and runbook docs outside this task's scope (`docs/reference/data-sources.md`, `docs/reference/routes.md`, `docs/guides/gcp-setup.md`, and `docs/runbooks/deploy.md`) still contain legacy `prefill:sunday` / Sunday-only pre-fill wording.

## Next-phase notes

- Phase 2 only touches slip schedules (`Weekend` slate scope and schedule-aware `EnsureWeekSlips` in `docs/concepts/slips.md` and `docs/concepts/groups.md`), which is independent of `internal/refresh` pre-fill docs.

## Commit

Document per-game-day prefill and date-scoped props

The refresh documentation and project guardrails still described Sunday-only pre-fill and week-wide prop warming after Phase 1 generalized pre-fill to any scheduled NFL game day. Updating them keeps contributors and agents aligned on the `prefill:gameday` idempotency key, credit conservation across multi-day weeks, and scoreboard-derived staleness windows.
