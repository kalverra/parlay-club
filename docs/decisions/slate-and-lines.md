---
id: adr-slate-and-lines
title: Slate, weather, and lines
type: decision
summary: Pull the Sunday and Monday slate from ESPN, stadium weather from Open-Meteo, and game lines from The Odds API, fetched only when someone opens the app and the data is stale.
area: board-and-research
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
- Use the slip members' home books as the reference price, falling back to the leg's own price.
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

## Consequences

- Adding a leg no longer means typing teams and prices. The Games board carries the week's lines and weather, and one
  tap prefills the normal add-leg path, so membership, the per-member limit, the kickoff lock, and the slip lock still
  apply unchanged.
- The app now has a real slate. The leg form picks from games, kickoff-locked picks fail, and slips show reference odds
  beside the combined price.
- The three sources stay behind plain clients that never touch the database, and one refresh service owns staleness,
  single-flight, backoff, and credit floors. Props can reuse the same clients, the odds table, and event matching.
- Credit spend is flat in group count: games, lines, weather, and sync state are global, and only slip topics and each
  member's home book are per group. Repeated page loads inside a staleness window spend nothing.
- Page loads can now start background work, so a slow board can mean a sync in flight. `sync_state` is the place to look
  when data is old.
- The `odds` table only grows. The web layer hides rows from an older sync, but nothing deletes them.
- Venue coordinates are a hand-kept 2026 snapshot. A new or renamed stadium means a map edit, and an unknown venue fails
  quietly to no forecast.
- Revisit if the board ever needs more than one week, if a second odds source appears, or if the Odds API changes its
  credit headers.

## Alternatives considered

- **Scheduled refresh jobs.** Rejected: the free-tier guardrail and AGENTS.md both say sync is lazy. Scheduled runs
  would bill the instance and spend credits without a reader.
- **Fetch on every board load with no cache.** Rejected: one `/odds` call costs about 3 credits, so a few curious
  reloads would burn the month. Caching in `sync_state` with per-source windows keeps the app useful and the budget
  safe.
- **One odds call per game.** Rejected in favor of one `/odds` response for the whole week, with only matched games
  written. Cheaper and simpler, at the cost of dropping lines for events not on the local slate.
- **An alias map for odds team names up front.** Deferred: exact `displayName` equality matched all 14 games on a full
  saved slate. An alias map is added only when a real week misses.
- **Deriving venue coordinates from ESPN.** Not possible: the scoreboard has no latitude or longitude and most venues
  have no usable address. The map is seeded by hand keyed to ESPN venue IDs, with a fixture test that keeps it in step
  with the season.
- **A prune of old odds rows.** Passed over for a web-layer filter on the newest fetch time. The filter was enough to
  keep a card from mixing prices and could ship in this change; a prune is still the cleaner fix.
- **Per-group slate, lines, and weather.** Rejected: the data is the same for everyone. Sharing it keeps credits flat as
  groups grow, and only live topics and home-book prices vary by group.
- **The slip's reference odds from the viewer's home book.** Changed: a slip belongs to the group, so the reference is
  the median across every member's home book instead of the viewer's book with a median fallback.
- **A dedicated quick-add route.** Rejected: the pills post the existing add-leg route with prefilled fields, so every
  slip rule stays in one place and the confirmation reuses the normal HTMX swap.

## Implementation notes

- The slate filter is Sunday or Monday in Eastern time, which is why a 9:30 a.m. ET London kickoff and a Monday night
  game stored in UTC on Tuesday both survive. Only the regular season has a saved fixture; the postseason shares the
  filter but is unproven.
- Every parser is strict: an unknown status, a missing team, a missing hour, a null, or a total that is not over or
  under is an error, not a zero. Provider changes surface as failed syncs with old rows kept, rather than silent bad
  data.
- No sync deletes. A game ESPN drops or moves stays under its old week, and `games.odds_event_id` is unique and never
  cleared, so a rematch would collide until the old value is cleared.
- Every odds row carries the fetch time, and a new fetch time counts as a change. A real sync therefore publishes even
  when no price moved, which is what keeps the "lines as of" note moving. Tests use a fixed clock to exercise the true
  no-op path.
- A pass below the credit floor still records the balance and the free matches, so the app waits for the next staleness
  window instead of rechecking on every load. The stored balance never wedges the source.
- The live window ends at kickoff for a scheduled game. If ESPN is late flipping a game to live, a stream opened at
  kickoff waits out the 10 minute idle deadline until the next scoreboard sync.
- The current-week game requirement lives only in the form's `required` attribute. `slips.Service.AddLeg` still accepts
  a nil game, so a crafted POST can create a leg the docs say is invalid.
- Server-rendered kickoff text is a UTC fallback; the `<time>` element makes it local. The game picker is a `<select>`,
  whose options cannot carry a `<time>`, so its labels stay in UTC.
- The board runs one odds query per game. Fourteen queries is fine for one SQLite instance, but it is the first thing to
  batch if the board widens.

## Follow-ups

- **Prune odds rows.** Delete a game's older odds rows when a sync writes a new set, or key rows by source and sync, so
  the table stops growing. The web-layer filter can then go.
- **Enforce the game requirement in the service.** Move the current-week "a leg needs a game" rule from the form into
  `slips.Service.AddLeg`, with a test for a crafted request.
- **Batch the board's odds reads.** Replace the per-game `OddsForGame` loop with one query for the week's games.
- **Add an odds alias map when a week misses.** Keep exact name matching until a real slate fails, then map the missed
  pair rather than loosening the match for everything.
- **Save a postseason scoreboard fixture.** Prove the week ID and Sunday/Monday filter under `seasontype=3`.
- **Log skipped weather.** A venue ID missing from the map currently yields no forecast and no log entry; warn once so a
  new stadium is noticed.
- **Clear or prune dropped games.** Give the scoreboard sync a way to move or remove a game ESPN reassigns, and clear
  `odds_event_id` before rematching.
- **Inject the clock in the web layer.** Card and picker tests use real time with wide margins; a clock would make the
  kickoff edges testable.
