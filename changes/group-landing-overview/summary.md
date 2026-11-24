---
slug: group-landing-overview
run: 20261124-215808-group-landing-overview
outcome: done
generated: 2026-11-24T22:16:52Z
---

# Run summary: group-landing-overview

## Outcome

- Status: done
- Branch: group-landing-overview
- Close-out: written
- Close: pending
- Final verdict: agy@gemini-4-neon-medium approved (issues: 0)

## Totals

- Tasks: 4 of 4
- Phases: 2
- Checkpoints: 5
- Duration: 16m56s
- Cost: $—
- Fresh input tokens: 1024724
- Cache read tokens: 11806090
- Cache write tokens: 0
- Output tokens: 165598

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-medium | 9381151 | $— |
| gemini-4-neon-medium | 3615261 | $— |

## Phases

### Phase 1: Group overview data queries and spotlight classification

Tasks:

- [x] Add `Store.RecentSlipsForGroup(ctx, groupID, limit)` in `internal/db/slips.go` (returning recent slips with their week metadata) with store tests.
- [x] Add `GroupOverviewPage` view model and pure `isSpotlightSlip` classifier in `internal/web/home.go` that separates active/upcoming spotlight slips from recent/completed slips and assembles the group record snapshot, recurring schedules, and members, with unit tests covering open, locked-live, closed, graded, and empty states.

Phase totals: 2 checkpoints, 6m3s, $—, 1303271 input / 22648 output / 1198915 cache read / 0 cache write tokens

Checkpoints: task-1, task-2 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Add `Store.RecentSlipsForGroup(ctx, groupID, limit)` in `internal/db/slips.go` (returning recent slips with their week metadata) with store tests.

- Added `RecentSlipRow` (pairing `Slip` and `Week`) and `Store.RecentSlipsForGroup(ctx, groupID, limit)` in `internal/db/slips.go`, ordering results by `week_id DESC, id DESC`, guarding `limit <= 0`, and hydrating custom-slip `GameIDs`.
- Added `TestRecentSlipsForGroupOrdersByWeekAndIDAndHonorsLimit` in `internal/db/slips_test.go` covering empty groups, non-positive limits, cross-group isolation, week/ID ordering, limit capping, week metadata, and custom-slip `GameIDs` hydration.
- Verified with `go test -short ./internal/db/...` and `golangci-lint run ./internal/db/...` (both passing with 0 issues).

### Task 2: Add `GroupOverviewPage` view model and pure `isSpotlightSlip` classifier in `internal/web/home.go` that separates active/upcoming spotlight slips from recent/completed slips and assembles the group record snapshot, recurring schedules, and members, with unit tests covering open, locked-live, closed, graded, and empty states.

- Added `GroupOverviewPage`, `SpotlightSlipCard`, `GroupRecordSnapshot`, and `RecentSlipCard` view models in `internal/web/home.go`.
- Added pure `isSpotlightSlip` classifier in `internal/web/home.go` separating active/upcoming spotlight slips (open, lock-in, join, waiting, export, or locked with live/upcoming eligible games) from closed and graded slips.
- Added `buildGroupOverviewPage`, `buildGroupRecordSnapshot`, `activeStreakLabel`, `recentSlipResult`, and shared `slipProgressSummary`/`buildScheduleViews` helpers to assemble sorted spotlight cards, deduplicated recent slips (capped at 5), record W-L/streak snapshot, recurring schedules, and group members.
- Added `internal/web/home_overview_test.go` covering `isSpotlightSlip` and `buildGroupOverviewPage` across open, multi-slip, locked-live, closed, graded, and empty states.
- Verified with `go test -short ./internal/web/...` and `golangci-lint run ./internal/web/...` (both passing with 0 issues).

### Phase 2: Group landing page template and route migration

Tasks:

- [x] Update `groupWeek` in `internal/web/home.go` and redesign `web/templates/week.html` to always render the group overview with the conditional active/upcoming slip spotlight, quiet-state CTA, record snapshot, recent slips, recurring schedules (linking to `/slips/new?mode=recurring`), and members strip; update `internal/web/home_test.go` for active, multi-slip, quiet, and non-member cases.
- [x] Docs: update `docs/concepts/groups.md` and `docs/concepts/slips.md` to describe the per-group overview landing page and spotlight behavior.

Phase totals: 3 checkpoints, 10m12s, $—, 9478969 input / 133289 output / 8738687 cache read / 0 cache write tokens

Checkpoints: task-3, task-4, final Review fixes: 0 Integration checks: 0

#### Done

### Task 3: Update `groupWeek` in `internal/web/home.go` and redesign `web/templates/week.html` to always render the group overview with the conditional active/upcoming slip spotlight, quiet-state CTA, record snapshot, recent slips, recurring schedules (linking to `/slips/new?mode=recurring`), and members strip; update `internal/web/home_test.go` for active, multi-slip, quiet, and non-member cases.

- Updated `groupWeek` in `internal/web/home.go` (with `overviewRecordSeason`) to always assemble and render `GroupOverviewPage` via `buildGroupOverviewPage`, removing the single-slip redirect, inline create-slip branch, and obsolete `WeekPage`/`WeekSlipCard` types.
- Redesigned `web/templates/week.html` as a phone-first group overview with the group header (`+ New slip`), conditional active/upcoming spotlight cards or quiet-state CTA (`+ Start a slip`), 3-metric group record snapshot linking to `/record`, recent slips list, recurring schedules with HTMX controls and `/slips/new?mode=recurring` link, and members strip linking to `/groups`.
- Updated `internal/web/home_test.go` to cover single-slip spotlight, multi-slip spotlight deadline ordering, auto-created scheduled slips, quiet-state with and without an upcoming week, recent slips/record snapshot rendering, and non-member 404 enforcement.
- Verified with `go test ./internal/web/...` (passed) and `golangci-lint run ./internal/web/...` (0 issues).

### Task 4: Docs: update `docs/concepts/groups.md` and `docs/concepts/slips.md` to describe the per-group overview landing page and spotlight behavior.

- Updated `docs/concepts/groups.md` (`## The active group` and `## Screens`) to document that `/groups/{id}/week` always renders the per-group overview landing page with spotlight slips or the quiet state, record snapshot, recent slips, recurring schedules, and members strip.
- Updated `docs/concepts/slips.md` (`## Creating a slip` and `## Several slips`) to document `UpcomingWeeks`/`LatestWeek` resolution, `isSpotlightSlip` eligibility and urgency/deadline sorting, the quiet-state hero, and the overview sections below it.
- Verified with `good-vibes docs lint` (0 errors).

## Final gate

- Rounds used: 1; verify failures: 0
- Final verify: passed
- Open issues: none
