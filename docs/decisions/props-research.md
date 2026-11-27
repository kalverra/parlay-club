---
id: adr-props-research
title: Props and research
type: decision
summary: Load player props for a game when someone opens it, within the Odds API budget.
area: board-and-research
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

## Consequences

Adding player props to slips no longer requires manual typing. Users can click quick-add pills on expanded game cards or
select players via autocomplete on the leg form. Research is immediate: opening a game card or clicking research on a
slip leg surfaces last-5 hit bars, median and mean indicators, key offensive and defensive injuries, and prop lines. The
analyzer catches high-variance traps via `median_trap` when an OVER line exceeds the last-5 median yards.

Odds API and Parlay API credit usage is bounded: lazy on-demand fetch with 2-hour caching and singleflight prevents
repeated credit drain on UI card toggles. Sportsbook-to-ESPN player name mismatches can be linked in the UI via manual
match (`manual = 1`), persisting across syncs and automatically backfilling legs and odds.

Maintenance gets slightly harder because the app depends on unofficial ESPN endpoints (`/roster`, `/summary`, and
`site.web.api.espn.com` gamelogs) and two prop providers with credit floors and fallbacks.

We would revisit automated nickname aliasing (such as "Gabe" for "Gabriel") to reduce the need for manual player match
overrides.

## Alternatives considered

- **Eagerly syncing props for all weekly games:** Rejected due to tight API credit budgets (150-credit Parlay API floor
  and 25/100-credit The Odds API floors). On-demand fetch per opened game keeps credit consumption strictly tied to
  active user interest.
- **Fuzzy string matching for sportsbook player names:** Rejected to prevent false-positive links between different
  players. Strict normalization (stripping punctuation, case folding, dropping generational suffixes) combined with
  team-scoped matching and manual UI overrides guarantees correctness.
- **Client-side charting libraries (e.g., Chart.js):** Rejected to keep the frontend lightweight, dependency-free, and
  fast on mobile. Pure HTML and Tailwind CSS flex bars render instantly without JavaScript dependencies.
- **Blocking or rejecting legs for unmatched players:** Rejected to adhere to the product rule of being friendly and
  never blocking. Unmatched props still save with sportsbook names, allowing manual match resolution at any time.
- **Complex statistical or ML models for prop projections:** Rejected in favor of deterministic last-5 median vs season
  mean rules (`median_trap`), preserving transparent, explainable guidance without LLM calls.

## Implementation notes

- ESPN's unofficial athlete gamelog endpoint structures data by category tables and variable column headers; parsing
  dynamically resolves column positions from header names rather than static indices.
- ESPN summary injuries label offensive tackles as `OT` rather than `LT`/`RT`, requiring `OT` in the key-position injury
  filter.
- Games that have kicked off skip prop syncing, but game research explicitly refreshes stale team rosters to ensure
  roster context remains available post-kickoff.
- HTMX game card expansion uses `hx-trigger="toggle once"` to avoid repeated network calls on accordion toggles.

## Follow-ups

- Auto-grade player props against official box scores in `grading-record-book`.
- Nickname aliasing table for common athlete name variations.
