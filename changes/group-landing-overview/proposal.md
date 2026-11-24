---
slug: group-landing-overview
status: implementing
created: "2026-11-24"
accepted_by:
    - Adam Hamrick
accepted_at: "2026-11-24T17:08:06Z"
depends_on:
    - slip-creation-ux
complexity: medium
risk: low
---

# Redesign per-group landing page

## Context

Opening a group today jumps straight into a single week's slip mechanics: if the latest week has one slip, the group
immediately redirects to that slip; if it has multiple slips, it shows a minimal week list; if it has none, it drops the
member onto the create-slip form.

That shortcut works while a slip is actively open or about to kick off, when the member's only goal is to pick, lock in,
or check legs. Outside that window—after a week finishes, between weeks, or when a group has no live slip—landing on a
closed slip or an empty creation form feels abrupt and hides the group's bigger picture (recent results, recurring
schedules, members, and what is coming up next).

```mermaid
flowchart LR
  OpenGroup["Open Group"] --> CheckWeek{"Latest week has slips?"}
  CheckWeek -->|"1 slip"| RedirectSlip["Redirect straight to Slip Hub"]
  CheckWeek -->|"Multiple slips"| WeekList["Bare Multi-Slip Week List"]
  CheckWeek -->|"0 slips"| CreateForm["Create-Slip Form"]
```

## Goals and non-goals

### Goals

- Make the per-group landing page a dedicated **group overview** rather than an unconditional redirect to a slip or
  create form.
- Prominently highlight any **active or soon-upcoming slip** at the top of the page with its current step, deadline, and
  primary action so live game-week play stays one tap away.
- When no slip is active or soon upcoming, lead with the group overview: recent slip results, group record snapshot,
  active recurring schedules, members, and a clear action to start a new slip.
- Keep the layout phone-first and scannable at every width.

### Non-goals

- Replacing the individual Slip Hub (members still open a slip to pick, vote, lock in, export, or grade).
- Replacing the full Record Book or group invite/management screens; the overview summarizes and links to them.
- Changing how recurring schedules auto-create slips or how external odds are fetched.

## Decision

Turn the per-group entry point into a state-aware group landing page that always renders a coherent overview, elevating
active or soon-upcoming slips into a hero spotlight when present.

### Before

```mermaid
flowchart TD
  GroupEntry["Group Entry"] --> AutoRedirect["Unconditional Redirect / Branch"]
  AutoRedirect --> SlipHub["Single Slip (even if finished)"]
  AutoRedirect --> BareList["Multi-Slip Week List"]
  AutoRedirect --> EmptyCreate["Create-Slip Form"]
```

### After

```mermaid
flowchart TD
  GroupEntry["Group Landing Page"] --> HeroCheck{"Active or soon-upcoming slip?"}
  HeroCheck -->|"Yes"| Spotlight["Active / Upcoming Slip Spotlight (status, deadline, primary action)"]
  HeroCheck -->|"No"| QuietState["No Active Slip + Start New Slip Action"]
  Spotlight --> Overview["Group Overview (recent slips, record snapshot, recurring schedules, members)"]
  QuietState --> Overview
  Spotlight -->|"Open slip"| SlipHub["Slip Hub"]
```

Keeping one consistent landing page with a conditional spotlight beats redirecting on active weeks and showing an
overview on quiet weeks because members always know where they land when they tap a group, and they can still reach
group context (schedules, recent slips, record) without hunting through menus during a live week. Summarizing key group
data inline beats linking out to separate tabs for everything because a club's status—who is in, how the group is doing,
and what is next—fits naturally on one phone-first screen.

`complexity: medium` fits because the landing page aggregates slip state, upcoming-slip detection, recent results,
recurring schedules, and group record summaries into one view. `risk: low` fits because it is a read-heavy presentation
change on the group entry route that leaves slip mutations, grading, and background sync untouched, with straightforward
handler and template test coverage.
