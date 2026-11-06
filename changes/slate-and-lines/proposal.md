---
slug: slate-and-lines
status: designed
created: "2026-11-01"
depends_on: [slips-live]
---

# Slate, weather, and lines

## Context

Typing every leg by hand is slow and error-prone, and the group can't see the slate, lines, or weather in one place.

The app has no NFL data: no games, no lines, no weather.

## Goals and non-goals

**Goals:**

- Pull the Sunday and Monday NFL slate from ESPN.
- Pull stadium weather from Open-Meteo for the kickoff window.
- Pull game lines from The Odds API only when someone opens the app and the cached data is stale.
- Show a game board with lines and weather badges, with one-tap quick-add for spreads, moneylines, and totals.
- Use each member's home-book price for reference odds, falling back to the median.
- Stay well inside the Odds API's 500 free credits a month.

**Non-goals:**

- Player props, rosters, game logs, or player matching. `props-research` adds those.
- Scheduled refresh jobs. Sync stays lazy and page-load driven.
- Odds from outside the configured bookmaker list.
- Grading, history, or deploy work.

## Decision

Pull the Sunday and Monday slate from ESPN, stadium weather from Open-Meteo, and game lines from The Odds API, fetched
only when someone opens the app and the data is stale. Show a game board with lines and weather badges, and let friends
add a spread, moneyline, or total in one tap. Stay well inside the Odds API's 500 free credits a month.

Heads up on `groups`: slate, weather, and lines are the same for everyone, so they stay global and shared by every
group. That also keeps Odds API credit use flat when more groups arrive. Only the live-update topics and each member's
home-book prices touch a group.
