# Task 4

## Done

- Updated `docs/concepts/groups.md` (`## The active group` and `## Screens`) to document that `/groups/{id}/week` always renders the per-group overview landing page with spotlight slips or the quiet state, record snapshot, recent slips, recurring schedules, and members strip.
- Updated `docs/concepts/slips.md` (`## Creating a slip` and `## Several slips`) to document `UpcomingWeeks`/`LatestWeek` resolution, `isSpotlightSlip` eligibility and urgency/deadline sorting, the quiet-state hero, and the overview sections below it.
- Verified with `good-vibes docs lint` (0 errors).

## Deviations

None.

## Open issues

None.

## Next-phase notes

Documentation in `docs/concepts/groups.md` and `docs/concepts/slips.md` now matches the per-group overview landing page implemented on `GET /groups/{id}/week`.

## Commit

Document group overview landing page and spotlight

Update the groups and slips concept docs so they reflect the always-rendered per-group overview page, spotlight eligibility rules, and quiet-state behavior on `/groups/{id}/week` instead of the removed single-slip redirect and inline create form.
