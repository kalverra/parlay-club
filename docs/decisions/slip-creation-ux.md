---
id: adr-slip-creation-ux
title: Redesign slip creation UX
type: decision
summary: 'Unify slip creation around upcoming weeks and split the scope controls by slip type: recurring slips pick a repeating slate rule, while one-off slips pick exact games with slate shortcuts.'
touched:
    - internal/db
    - internal/db/migrations
    - internal/slips
    - internal/web
    - web/static
    - web/templates
related:
    - routes
    - schema
    - slips
area: slips
---

# Redesign slip creation UX

## Context

Creating a new slip feels broken and rigid for how groups actually play:

1. **Past weeks are offered.** The week picker lists every stored week and even includes a manual "add a past week"
   form, letting members create slips for weeks that are long over.
2. **Finished weeks are offered.** A week whose games have already kicked off or finished (such as a completed Week 11)
   still appears as a valid target for a new slip.
3. **One-off vs. recurring is split apart.** The new-slip screen only makes one-off slips. Setting up a recurring weekly
   slip lives on a separate schedules screen, so members have to know where to go before they start.
4. **One-off game selection is too coarse.** One-off slips force a choice between three fixed slate buckets (Thursday,
   Sunday Main, Full Week). A group cannot build a slip for a specific subset of games—like a Sunday afternoon
   doubleheader, a Thanksgiving trio, or two primetime matchups—without opening the whole slate.

```mermaid
flowchart TD
  Form["New Slip Form"] --> PastWeeks["All stored weeks + manual past-week entry"]
  Form --> OneOffOnly["One-off slip only (recurring lives elsewhere)"]
  Form --> CoarseScope["Fixed slate scope: Thursday / Sunday Main / Full Week"]
  CoarseScope --> PickBoard["Pick Board & Deadlines"]
```

## Goals and non-goals

### Goals

- Restrict slip creation to weeks that still have upcoming games; hide older and already-completed weeks and remove
  manual past-week entry from the form.
- Let a member choose **One-off** or **Recurring** directly in the new-slip flow.
- When creating a **One-off** slip, let the member pick the exact eligible games from the chosen week, with one-tap
  shortcuts for common slates (such as "All Sunday", "All Thursday", "All Monday", and "All games").
- Keep deadlines, pick-board filtering, and leg validation aligned with the slip's selected games.

### Non-goals

- Changing how existing historical or graded slips are viewed, archived, or recorded in the Record Book.
- Supporting per-game selection on recurring schedules; recurring slips continue to use repeating slate rules (e.g.,
  Thursday, Sunday Main, Full Week) because future weeks' games are not fixed in advance.
- Changing sportsbook export, analyzer rules, or free-tier API fetch behavior.

## Decision

Unify slip creation around upcoming weeks and split the scope controls by slip type: recurring slips pick a repeating
slate rule, while one-off slips pick exact games with slate shortcuts.

### Before

```mermaid
flowchart LR
  NewSlip["New Slip Form"] --> AnyWeek["Any Stored or Typed Past Week"]
  NewSlip --> FixedScope["Fixed Slate Scope"]
  Schedules["Separate Schedules Screen"] --> Recurring["Recurring Schedule"]
```

### After

```mermaid
flowchart LR
  NewSlip["New Slip Form"] --> OpenWeeks["Upcoming Weeks Only (games still ahead)"]
  NewSlip --> Mode{"One-off or Recurring?"}
  Mode -->|"One-off"| GamePicker["Game Checklist + Slate Shortcuts (All Sunday, All Thursday, All Monday, All)"]
  Mode -->|"Recurring"| RecurringRule["Repeating Slate Rule -> Group Schedule + Current Week Slip"]
  GamePicker --> SlipEligibility["Slip Eligible Games (Pick Board, Deadlines, Leg Validation)"]
  RecurringRule --> SlipEligibility
```

Restricting the week list to weeks with upcoming games prevents nonsensical slips for past or finished weeks at the
source, rather than letting users create dead slips that immediately land in a closed state. Combining one-off and
recurring creation into one form beats keeping two separate entry points because members think in terms of "start a
slip" first and decide whether it repeats second. Giving one-off slips an explicit game list with quick-select shortcuts
beats adding more hardcoded slate enums because it handles holiday slates, primetime pairs, and custom friend-group
slates without needing new scope types later.

`complexity: medium` fits because the change coordinates week eligibility, recurring schedule creation, and per-game
slip scoping across the creation UI, slip domain rules, pick board, and storage. `risk: medium` fits because changing
how a slip defines its eligible games touches deadlines, pick-board filtering, and leg validation, though existing
slate-scoped slips remain supported and automated tests cover those boundaries.

## Overview

```mermaid
flowchart LR
  NewSlip["GET /slips/new"] --> Upcoming["Upcoming Weeks Only (kickoff > now)"]
  Upcoming --> Mode{"One-off or Recurring?"}
  Mode -->|"One-off"| Custom["Game Checklist + Dynamic Weekday Shortcuts"]
  Mode -->|"Recurring"| Rule["Repeating Slate Rule (Thu / Sun / Weekend / Full Week)"]
  Custom -->|"POST /slips"| CustomSlip["Custom Slip (slate_scope = custom + slip_games)"]
  Rule -->|"POST /slips"| Schedule["Group Schedule + EnsureWeekSlips"]
  CustomSlip --> Filter["FilterGamesForSlip -> Pick Board, Deadlines, Leg Validation"]
  Schedule --> Filter
```

## Consequences

- **Easier:** Starting either a one-off slip or a recurring schedule from `/slips/new` without navigating to a separate
  schedules screen.
- **Easier:** Building custom friend-group slates (primetime pairs, holiday games, or specific Sunday matchups) via
  explicit per-game selection and dynamic weekday shortcuts.
- **Easier:** Avoiding dead slips for past or completed weeks by restricting week selection to weeks with upcoming games
  (`kickoff > now`) and removing manual past-week entry.
- **Harder:** Maintaining two eligibility models (`slip_games` rows for `custom` slips and kickoff-day rules for
  repeating scopes); every pick, deadline, and validation path must go through `FilterGamesForSlip` rather than
  `FilterGamesByScope`.
- **Harder:** Listing slips for a week or group requires batch-hydrating `GameIDs` for `custom` slips to avoid N+1
  queries.
- **Revisit:** Aligning `mode=recurring` on `/slips/new` so that any selected week or member defaults behave
  consistently with one-off creation when materializing the first slip.

## Alternatives considered

- **Adding more hardcoded `SlateScope` enums (e.g., primetime or holiday slates):** Rejected because fixed enums cannot
  cover arbitrary friend-group game subsets and require ongoing schema and UI changes.
- **Storing a JSON array of game IDs on `slips`:** Rejected in favor of a normalized `slip_games` join table so SQLite
  enforces foreign-key integrity against `games(id)`.
- **Keeping recurring schedule creation on a separate screen:** Rejected because members start with the intent to create
  a slip and decide whether it repeats second.
- **Supporting per-game selection on recurring schedules:** Rejected because future weeks' games are not fixed in
  advance; recurring schedules stay on repeating slate rules (`ValidForSchedule`).

## Implementation notes

- Implementation matched the design with no deviations.
- **Empty-schedule limit:** When no stored games have `kickoff_utc > now` (such as before a sync or during the
  offseason), `UpcomingWeeks` returns empty and `/slips/new` hides the creation form.
- **Fixed custom scope after creation:** Once a `custom` slip is created, its `slip_games` rows are fixed; there is no
  UI to add or remove eligible games on an existing custom slip.
- **Schedule boundary:** `SlateScope.Valid()` permits `custom` for slips, while `SlateScope.ValidForSchedule()` rejects
  `custom` for recurring schedules at both the store and handler boundaries.

## Follow-ups

- Add an edit-games action for open `custom` slips so creators can adjust eligible games before kickoff without
  recreating the slip.
- Evaluate whether `mode=recurring` on `/slips/new` should apply the form's selected members when materializing the
  first upcoming week's slip.
