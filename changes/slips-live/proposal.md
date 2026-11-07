---
slug: slips-live
status: implementing
created: "2026-11-01"
depends_on:
    - foundation
---

# Live shared slips

## Context

Picks live in a group chat today. On Sunday morning nobody knows whose legs are in, what the parlay pays, or whether
someone changed their mind.

The group needs one shared slip that everyone edits at the same time and that updates on every phone without a refresh.

## Goals and non-goals

**Goals:**

- Give each week one or more shared slips with a chosen set of members and the same number of legs per member.
- Enforce membership, leg limits, kickoff locks, lock-in, and unlock rules on the server.
- Show combined odds, reference odds, and a payout calculator with stake and profit boost.
- Push changes to every open phone within a second over SSE.
- Let friends type legs in by hand in this change.

**Non-goals:**

- NFL data, game lines, weather, or player props. `slate-and-lines` and `props-research` add those.
- Grading, the Record Book, or loading past slips.
- Sportsbook integrations or handling money.

## Decision

Add shared slips: each week has one or more slips, each slip has members, and every member adds the same number of legs.
Members lock in when done; legs lock at kickoff; anyone can unlock with a visible note. Changes appear on every open
phone within a second. The slip shows combined odds and a payout calculator with stake and profit boost. Friends type
legs in by hand in this change.

Heads up on `groups`: this change builds for one club, which is all the group needs on Nov 8. A later `groups` change
will put each slip inside a group and scope live updates per group. Nothing here should hard-code "the one club", so
that change stays small.
