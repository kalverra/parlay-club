# Task 6

## Done

- Updated `docs/concepts/slips.md` to document the `weekend` (`Weekend Slate`, Saturday/Sunday/Monday ET) slate scope across the overview, hub scope badges, create-slip form options, slate scope table, and multi-slip week hub, plus schedule-aware `slips.EnsureWeekSlips` skipping.
- Updated `docs/concepts/groups.md` to document `weekend` (`Weekend Slate`) in recurring slip schedules and how `EnsureWeekSlips` loads `GamesForWeek` and skips missing schedules without upcoming non-final matching games when stored games exist.
- Updated `docs/concepts/groups.md` front matter (removing legacy `sources` and `last_verified` fields) and added `## Overview` with a Mermaid diagram so `good-vibes docs lint` passes with zero errors.
- Regenerated `docs/llms.txt` via `good-vibes docs index` to include `Groups`.

## Deviations

- Removed unsupported `sources` and `last_verified` front-matter fields and converted `## Summary` to `## Overview` with a Mermaid diagram in `docs/concepts/groups.md` (and regenerated `docs/llms.txt`) because `good-vibes docs lint` treats legacy front-matter schema errors as hard failures on modified docs.

## Open issues

- None.

## Next-phase notes

- Phase 2 is complete. Any remaining references to legacy slate scopes in `docs/reference/schema.md` or `docs/reference/routes.md` were outside this task's scope.

## Commit

Document weekend scope and schedule skipping

The concept docs for slips and groups still listed only three slate scopes and described unconditional recurring slip creation after Phase 2 added `weekend` and schedule-aware `EnsureWeekSlips` skipping. Updating them keeps the living documentation accurate on playoff weekend slates and why inactive schedules no longer create dead slips.
