---
slug: dedicated-live-view
status: designed
created: "2026-11-27"
accepted_by:
    - Adam Hamrick
accepted_at: "2026-11-28T01:58:58Z"
area: slips
complexity: medium
risk: medium
---

# Dedicated Live view for locked parlays

## Context

Once a parlay locks in and the first game kicks off, the slip hub still treats the group as if they have not placed
their bets yet. The page stays in the `export` step ("Ready to place"), leads with the sportsbook export card, and
stacks the payout calculator, wager form, and member-grouped leg lists ahead of the live action. On a phone, friends
have to scroll past pre-game chrome and jump across different players' sections just to follow the games on TV right
now.

Live data updates are also sluggish during a game. Today, live ESPN scores and box scores only refresh when someone
triggers a full-page reload, sit behind a 2-minute staleness window, and run alongside slower pre-game sources like
weather, rosters, and projections. A friend watching an open slip page waits for someone else to reload before the SSE
stream nudges the screen.

Finally, live probability and game state are uneven across the slip. The parlay card shows a live chance beside a muted
pre-game chance, while leg rows only show a slim live strip once a game starts—omitting game-progress boxes and not
consistently pairing before (pre-game) and current (live) hit chances on every leg and on the full parlay.

```mermaid
flowchart LR
  Reload["Full-page reload only"] --> Refresh["General Refresh (2m live cadence + pre-game sources)"]
  Refresh --> Cache["SQLite Cache"]
  Cache --> Hub["Slip Hub (Export-first, grouped by player)"]
  Hub -. "Export card, wager, & member groups lead" .-> Phone["Phone screen"]
```

## Goals and non-goals

**Goals:**

- Automatically switch a `LOCKED` slip into a dedicated **Live** view as soon as its first game starts, assuming every
  player has placed their bet and putting live progress first on phone screens.
- Speed up live game and bet status updates using only free ESPN endpoints: add a fast, viewer-driven live refresh path
  that fetches only live scoreboard and active-game box scores while a viewer has the Live view open, and stops as soon
  as viewers leave or go idle so Cloud Run still scales to zero.
- Display clear progress bars and boxes for the current state of each game, each leg's progress, and the whole parlay,
  showing both **before (pre-game) %** and **current (live) %** hit chances on every leg and on the full parlay.
- Reorder legs in the Live view by **game time** (active live games and earliest kickoffs first, then upcoming games,
  with settled legs clearly badged) instead of grouping by the player who picked them, while keeping player attribution
  visible on each leg.
- Keep secondary controls (sportsbook export, wager logging, payout calculator, manual grade overrides, and activity)
  reachable in a collapsed or secondary section without crowding the live screen.

**Non-goals:**

- Buying or polling live in-game sportsbook odds from paid providers (Parlay API or The Odds API); live tracking stays
  100% free and deterministic.
- Adding a background cron job or always-on poller that keeps the server warm when nobody is watching.
- Building a play-by-play feed, drive chart, or field-position tracker.
- Changing how `OPEN` slips build parlays, how `FINAL` auto-grading settles legs, or how the Record Book records
  results.
- Blocking manual grade overrides or removing access to export links if someone still wants to check them late.

## Decision

Add a dedicated `live` slip view that activates automatically when a `LOCKED` slip's first game starts, backed by a
fast, viewer-scoped free ESPN refresh loop and a game-time-ordered progress UI that pairs before and current hit chances
on every leg and the parlay.

### Before

```mermaid
flowchart LR
  Reload["Full-page reload"] --> FullSync["Full Refresh Pass (2m live + pre-game sources)"]
  FullSync --> Cache["SQLite Cache"]
  Cache --> SSE["SSE Hub (idle while nobody reloads)"]
  Cache --> Progress["Live Progress (live % on active legs only)"]
  Progress --> Hub["Slip Hub (Export leads; legs grouped by player)"]
  SSE --> Hub
```

### After

```mermaid
flowchart LR
  Viewer["Active Live Viewer"] --> LiveLoop["Fast Live Refresh [NEW] (free ESPN scoreboard + active box scores, ~20–30s)"]
  LiveLoop -->|"stops when no viewers"| Cache["SQLite Cache"]
  Cache --> AutoGrade["Auto-Grader (immediate on FINAL)"]
  Cache --> SSE["SSE Hub (pushes on score/stat change)"]
  Cache --> Progress["Live Progress [CHANGED] (game state + before & current % on every leg & parlay)"]
  Progress --> LiveView["Dedicated Live View [NEW] (parlay hero first; legs ordered by game time; export/wager tucked away)"]
  SSE --> LiveView
```

Transitioning `LOCKED` slips into a dedicated Live view once the first game starts beats keeping live progress inside
the export-first hub because the group's job flips from *placing the bet* to *sweating the games*; collapsing export and
wager tools keeps them available without burying the live games below the fold on a phone.

Running a viewer-driven live-only refresh path over free ESPN endpoints beats both always-on background polling (which
breaks Cloud Run scale-to-zero and increases cost) and waiting for manual full-page reloads (which leaves the screen
stale for minutes). Deduplicating fetches per game across concurrent viewers keeps ESPN traffic light even when the
whole group watches together, while dropping pre-game sources cuts response latency down to just the live scoreboard and
active game summaries.

Ordering legs by game time instead of by player beats member-grouped lists during live play because friends watch games
chronologically—early window, late window, night game—and want every leg in the game currently on TV grouped together at
the top.

This change has `medium` complexity because it coordinates across multiple seams—slip lifecycle state, a fast
viewer-scoped live refresh path tied to SSE lifecycle, expanded game-state and before/current probability presentation,
and a phone-first Live view—requiring consistent contracts across modules without altering core storage or auth.

Its `risk` is `medium` because a live refresh loop tied to open streams carries a moderate blast radius around ESPN rate
limits and Cloud Run instance idle behavior if cleanup timers drift, and the new phone-first Live layout and HTMX
partial updates are only partially exercised by unit tests.
