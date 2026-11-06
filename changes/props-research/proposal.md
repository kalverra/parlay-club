---
slug: props-research
status: proposed
created: "2026-11-01"
depends_on: [history-and-deploy]
---

# Props and research

## Context

Player props are the group's most common legs, but they must be typed by hand, and nobody checks whether a line is
realistic.

The app has no prop lines, no link between a sportsbook's player names and ESPN's players, and no recent game history to
judge a line.

## Goals and non-goals

**Goals:**

- Load player props for one game when someone opens it, within the Odds API budget.
- Match sportsbook player names to ESPN players and keep manual fixes.
- Fetch ESPN rosters and game logs for the matched players.
- Add `median_trap` to the analyzer.
- Show a research view with last-5 hit bars, median and mean marked, and key injuries.
- Add prop quick-add pills and player-name autocomplete to the leg form.

**Non-goals:**

- Loading props for games nobody opened or bypassing the cache window.
- Raising the Odds API budget or adding paid data sources.
- Auto-grading props. `grading-record-book` adds grading.
- Building a general-purpose stats browser.

## Decision

Load player props for a game when someone opens it, within the Odds API budget. Match players to ESPN, and add a
research view with last-5 hit bars, median against average, and key injuries. Add the median-versus-mean prop warning to
the analyzer.
