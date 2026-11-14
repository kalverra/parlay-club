---
slug: grading-record-book
status: implementing
created: "2026-11-01"
accepted_by:
    - Adam Hamrick
accepted_at: "2026-11-14T14:35:57Z"
depends_on:
    - props-research
---

# Auto-grading and Record Book

## Context

Grading by hand every week is tedious, and the group wants season bragging rights.

Results aren't graded automatically, and there is no season record or leaderboard.

## Goals and non-goals

**Goals:**

- Grade legs from ESPN box scores once games go final, lazily through refresh.
- Let any member override a grade, keeping `graded_by` accurate.
- Show a Record Book per season: group record and units, streaks, a leaderboard, and the Parlay Terrorist badge.
- Let each member log an optional wager per slip and show dollar P/L.

**Non-goals:**

- Live in-game grading before a game is final.
- Scheduled grading jobs or a second process.
- Sportsbook integrations or handling money.
- Changing the grading rules in `plan/product.md`.

## Decision

Grade legs automatically from ESPN box scores once games end, with hand overrides. Add the Record Book: group record and
units, streaks, a leaderboard, the Parlay Terrorist badge, and optional per-person dollar tracking.
