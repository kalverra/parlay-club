---
id: slips
title: Slips
type: concept
summary: How a group builds one shared parlay a week from the slate, with the hub's next-step banner and deadline, the searchable pick board and its tray, members, legs, quick-add, create-slip defaults, grading, the editing and locking rules, and research.
tags: [slips, weeks, legs, members, locking, grading, results, activity, groups, games, quick-add, nfelo, props, autocomplete, research, next-step, hub, pick-board, search, tray]
sources: [internal/slips/slips.go, internal/slips/rules.go, internal/slips/next.go, internal/board/board.go, internal/db/slips.go, internal/db/legs.go, internal/db/odds.go, internal/db/players.go, internal/db/weeks.go, internal/db/games.go, internal/web/games.go, internal/web/pick.go, internal/web/slips.go, internal/web/home.go, internal/web/research.go]
last_verified: 2026-11-11
---

# Slips

## Summary

A group builds one shared parlay at a time. A **week** is an ESPN NFL week: a season, a season type (regular or
postseason), and a week number. Its id is `season*1000 + season_type*100 + week_num`, so Week 9 of the 2026 regular
season is `2026209`.

A **slip** is one shared parlay inside a group and a week. A week usually has one slip, but a group can make more, and a
person can belong to several in the same week. The app tracks the picks. Each person places the bet at their own book
with their own stake, and no money passes through the app.

## The hub

`GET /slips/{id}` is the hub. A step banner leads, then the members and their legs, then the export card, the payout
calculator, and the activity feed. The bottom bar keeps Groups, This week, and Record Book; the hub's one primary action
is the way into picking.

`slips.NextStep` is one pure function over the slip, its members, its legs, the week's games, the viewer, and `now`. It
returns the step, the legs still needed, the names the slip waits on, the first kickoff still ahead, and the slip's
result. The hub banner and each group card read its label, so the two always agree.

| Step      | When                                                      | Banner             | Primary action                                      |
| --------- | --------------------------------------------------------- | ------------------ | --------------------------------------------------- |
| `join`    | The viewer is in the group but not on the slip.           | Join this slip     | Adds the viewer through `POST /slips/{id}/members`. |
| `pick`    | The viewer has fewer legs than `legs_per_member`.         | Pick N more        | Opens `/slips/{id}/pick`.                           |
| `closed`  | The viewer needs legs and every game has kicked off.      | Picks are closed   | None; the slip stays visible.                       |
| `lock_in` | The viewer has every leg and has not locked in.           | Ready to lock in   | Posts `/slips/{id}/lock`.                           |
| `waiting` | The viewer is locked in and someone else is not.          | Waiting on {names} | None.                                               |
| `export`  | Everyone is locked in.                                    | Ready to place     | The export card carries the picks.                  |
| `done`    | The slip is `GRADED`.                                     | Won, Lost, or Push | None.                                               |

The deadline is the earliest game in the slip's week whose kickoff is still ahead of `now`. It renders as
`Picks close for BUF @ MIA at <time>`, and `app.js` formats the time in the reader's zone. A past week has no kickoff
ahead, so it has no deadline. Unlocking clears every lock, so the banner reopens the pick step; a member who already has
every leg sees `lock_in` instead.

Opening a slip from another of the viewer's groups switches the active group, the same as opening that group's week. A
group member who is not on the slip sees `join`; a person outside the group gets a 404.

## Creating a slip

The landing list opens a group's week (`GET /groups/{id}/week`), which switches the active group and opens that group's
newest slip of the latest week. The slate sync keeps that week current, so it is normally the week being played. When the
group has none, it shows the create-slip form.

- Week defaults to the latest week. The picker lists every stored week with its season. A past week the slate never
  stored can be typed in by season, season type, and week number, and the form creates the week. A typed week later than
  the latest stored week is refused.
- Name defaults to the week label, `Week 9`, then `Week 9 #2`.
- `legs_per_member` defaults to 2 and allows 1–5.
- Members default to the previous slip's members.

`slips.CreateDefaults` copies from the group's previous slip: the latest slip in that week or any earlier week. A second
slip in the same week copies that week's first. With no previous slip it returns `DefaultLegsPerMember` (2) and no
members. `db.Store.CreateSlip` trims the name, refuses a blank name, enforces the 1–5 range, requires at least one
member, and ignores repeat member IDs.

## Members

A slip has **members**: a subset of the group's users, guests included. `slip_members` pairs a slip with a person and
holds `locked_at` for a lock in.

Members and `legs_per_member` can change only while the slip is `OPEN`. A member with legs cannot be removed until their
legs are deleted. Adding someone already on the slip changes nothing. A group member who is not on the slip can join it
themselves through the same add-member route while it is `OPEN`.

## Legs

A **leg** is one pick. It holds an owner, an optional game, a market, a team, an optional player, a side, an optional
line, American odds, and a display `selection` generated from those fields.

| Market       | Fields                         | Selection example                   |
| ------------ | ------------------------------ | ----------------------------------- |
| `SPREAD`     | team, line                     | `BUF -3.5`                          |
| `ML`         | team                           | `BUF ML`                            |
| `TOTAL`      | side (OVER/UNDER), line        | `BUF@MIA Over 47.5`                 |
| `TEAM_TOTAL` | team, side, line               | `BUF Team Total Over 27.5`          |
| `PASS_YDS`   | player, team, side, line       | `Josh Allen Over 245.5 Pass Yds`    |
| `PASS_TDS`   | player, team, side, line       | `Josh Allen Over 1.5 Pass TDs`      |
| `RUSH_YDS`   | player, team, side, line       | `James Cook Over 64.5 Rush Yds`     |
| `RUSH_ATT`   | player, team, side, line       | `James Cook Over 14.5 Rush Att`     |
| `REC_YDS`    | player, team, side, line       | `Khalil Shakir Over 52.5 Rec Yds`   |
| `RECEPTIONS` | player, team, side, line       | `Khalil Shakir Over 4.5 Receptions` |
| `ANYTIME_TD` | player, team (side `YES`)      | `James Cook Anytime TD`             |
| `KICKER_PTS` | player, team, side, line       | `Tyler Bass Over 7.5 Kicking Pts`   |
| `OTHER`      | free-text selection, team opt. | whatever the user typed             |

- Team is the ESPN abbreviation of the team the leg is about. For `TOTAL`, team is empty.
- Odds default to -110 in the form. Quick-add pills prefill the price; the formulas are in
  [Odds math](../reference/odds-math.md).
- The game picker lists the week's games that have not kicked off. A current week requires a game; a past week lists
  every game and allows no game. `?game_id=` preselects one when a card opens the form.
- The leg form provides a `<datalist>` of player names for autocomplete, drawn from the week's rosters and prop boards.
  Saving a player market leg resolves the player's ESPN athlete ID and team automatically through player matching.
- A leg needs a game, except on a slip for a past week, where the game is optional and `OTHER` is the usual market.
- `OTHER` legs skip the analyzer and grade by hand.

The slip panel draws each leg as a row: the selection, then the odds, a "Research &rarr;" link when the leg has a
resolved player or game, the model chip, and the result. A `SPREAD` or `ML` leg with a fresh nfelo model shows it beside
the odds, from that leg's team's side: `nfelo -5.0` for a spread or `nfelo 76%` for a moneyline. Totals and props show no
chip, and a missing row, a fetch older than 72 hours, or a live game leaves it off.

`db.Store.CreateLeg` starts every leg `PENDING` and treats an odds value of 0 as -110.

The slip's **reference odds** multiply each leg's reference price: the median of the prices the slip members' home books
carry for the same game, market, player, side, and line, one price per book. A leg whose members' books do not price it
keeps its own price. The formulas are in [Odds math](../reference/odds-math.md).

## The pick board

The hub's pick step opens `GET /slips/{id}/pick`, a focused screen with a back link to the hub and no bottom bar. A tray
holds the viewer's own picks beside the list on wide screens and below it on a phone. The page carries the slip's
`data-live-url`, so it still opens one stream per page.

`internal/board` turns the week's stored odds into rows. It reads nothing itself. `board.Build` folds one row per
selection, using the viewer's home book when it prices the selection and the median across every book otherwise. Rows
from an older odds sync are dropped first, the same as the old Games board. A game that has kicked off is left out and
counted. Each row carries its game, market, team, player, side, line, price, display text, a category, and a kickoff
window.

Search is case- and accent-insensitive over the row text, team abbreviation and name, player, and matchup. The input
debounces at 250 ms and swaps the results partial. Chips filter by category (lines, team totals, passing, rushing,
receiving, TDs, kicking) and by Eastern-time window (Sunday morning before noon, Sunday early before 4, Sunday late
before 7, Sunday night, Monday). A Monday-night game that lands on Tuesday in UTC still reads Monday. Chips only appear
for categories and windows with rows. Sorts are kickoff (default), player, category, and odds (shortest price first).
Chips and sorts are links that swap the same partial and push the full page URL.

Tapping an unpicked row adds it with the normal add-leg route (`quick=1&from=board`), and tapping a picked row removes
that leg. Both answer with the row and fire `tray-updated`, so the tray refreshes without a reload. At the limit, an
unpicked row opens "Swap which pick?" and lists the viewer's legs that are not kickoff-locked. Choosing one sends
`PUT /legs/{old}` with the new pick's fields. `UpdateLeg` already enforces the owner, the lock in, and kickoff on both
games, and its activity entry reads "Adam changed X to Y".

`?game=` shows one game with both sides of each line in one row, and a link returns to the whole week.

A game whose props were never fetched appears as a prompt when the search matches its team, its matchup, or a roster
player on it: "Props for BUF @ MIA aren't loaded yet." The "Load them" button calls `GET /games/{id}/props?slip=`, which
runs `SyncGameProps` for that one game (credit floors and all) and then fires `board-updated`. The board itself never
syncs props; opening, typing, filtering, and sorting spend no credits. See [Data refresh](data-refresh.md).

The tray shows the viewer's legs with remove buttons, progress ("1 of 2"), the analyzer warnings that name one of the
viewer's legs, and one action: "Done" back to the hub, or "Review and lock in" at the limit. It refreshes on
`tray-updated` and `slip-updated`. A leg whose game has kicked off shows "Locked" instead of a remove button.

`GET /games` is a 303 to the active group's week, so old links still land somewhere useful. `GET /games/{id}/props`
without `?slip=` still renders the standalone prop partial for one game, with team totals and player props grouped
under each athlete and links to game research and player research.

## Research

The Research page (`GET /research`) offers game injury reports and player hit bars. It opens from a player, prop, leg,
or game link. It is not a bottom-bar tab.

- **Game research (`GET /research/games/{id}`):** fetches ESPN summary injuries on demand and displays injuries
  filtered to key positions (`QB`, `RB`, `WR`, `TE`, `LT`, `OT`, `C`, `CB`, `S`). A player-match section lists
  sportsbook prop names against matched ESPN roster athletes, with a manual match form (`POST /research/matches`) to
  fix unmatched names or corrections.
- **Player research (`GET /research/players/{espnID}`):** fetches the player's ESPN game log on demand and displays
  last-5 hit bars (green over the line, red under), callouts for the last-5 median and season mean, and total games
  played this season. Opening from a slip leg or prop pill preselects that market and line; otherwise it picks the
  player's primary prop line or the first available market.
- **Responsive layout:** Phone and desktop screens keep Research in a single column, reached from the page or leg that
  names the player or game.

## Grading

Every leg carries a result: `PENDING`, `NEEDS_GRADING`, `WON`, `LOST`, `PUSH`, or `VOID`. New legs are `PENDING`.

Any slip member can grade any leg, not just their own, at any time; locking in or a kickoff does not stop a grade. The
result chip on each leg opens a menu of `PENDING`, `WON`, `LOST`, `PUSH`, and `VOID`. Each choice sends
`PUT /legs/{id}/result` with the upper-case result, and the refreshed panel shows the recomputed slip. Someone who is
not on the slip sees a plain chip and cannot grade.

`internal/slips.Service.SetLegResult` records the member's name in `graded_by`, writes "{name} set {selection} to
{result}" to the activity feed, and publishes `slip:{id}`. Grading a leg the same way again by the same member changes
nothing, so it writes no activity entry and publishes nothing. A later grade by any member overrides the earlier one.

The slip result recomputes from every leg after each change:

- any leg `LOST` → `LOST` (a slip can bust before all its games end);
- else, any leg `PENDING` or `NEEDS_GRADING` → `PENDING`;
- else, every leg `PUSH` or `VOID` → `PUSH`;
- else → `WON`.

`PUSH` and `VOID` legs drop out of the slip's odds (see [Odds math](../reference/odds-math.md)) and count as neither a
hit nor a miss.

`OTHER` legs and any leg on a past-week slip normally grade by hand. The auto-grader (`grading-record-book`) will fill
in the other markets from the ESPN final box score and leave `NEEDS_GRADING` where it cannot decide; any member can then
override its result the same way. The per-market rules are in [Product](../../plan/product.md).

## Status and result

A slip's status is `OPEN`, `LOCKED`, or `GRADED`, and its result is `PENDING`, `WON`, `LOST`, or `PUSH`.

- New slips are `OPEN`.
- The status becomes `LOCKED` when every member locks in.
- The status becomes `GRADED` once the result is no longer `PENDING`. Until then it follows the locks: `LOCKED` when
  every member has locked in, otherwise `OPEN`.

## Editing and locking

The rules live in `internal/slips.Service`. Each method checks them, writes the change and one activity entry in one
transaction, updates the slip's status, then publishes `slip:{id}`. The clock is injected through `now`, so kickoff and
locking stay testable.

1. Only slip members can add legs. A member can edit or delete only their own legs. A non-member gets `ErrNotMember`,
   and someone else's leg gets `ErrNotOwner`.
2. A member can have at most `legs_per_member` legs. Adding past the limit returns `ErrLegLimit`.
3. A leg is **kickoff-locked** once `now >= game.kickoff_utc`. A kickoff-locked leg cannot be edited or deleted, and no
   leg can be added for a game that has kicked off. A leg without a game skips this check.
4. **Lock in** sets the member's `locked_at`. It requires exactly `legs_per_member` legs. Too few or too many returns
   `ErrLegsNotReady`. Locking in twice changes nothing.
5. The slip's status becomes `LOCKED` when every member is locked in.
6. **Unlock** is available to any member while the slip is `LOCKED` or any member is locked in. It clears every member's
   `locked_at`, sets the status `OPEN`, and writes "unlocked by {name}" to the activity feed. With nobody locked in it
   returns `ErrNotLocked`. Kickoff locks still apply.
7. Members, name, and `legs_per_member` can change only while the slip is `OPEN`. A new limit cannot drop below a
   member's current leg count. A member with legs cannot be removed until their legs are deleted.
8. Slips for past weeks skip rule 3, so history can be entered. A week counts as past once every game in it has kicked
   off (`db.WeekPast`), so a current week becomes history when its last game starts.
9. Every mutation writes one activity entry and publishes `slip:{id}` after it commits. An add reads "Adam added BUF
   ML", a lock in reads "Sam locked in", and a no-op writes nothing.

The service returns typed errors so handlers can answer with a 422 and a short message.

| Error                    | Trigger                                         | Message                                  |
| ------------------------ | ----------------------------------------------- | ---------------------------------------- |
| `ErrNotMember`           | The actor is not on the slip.                   | you're not on this slip                  |
| `ErrNotOwner`            | The actor touches another member's leg.         | that leg belongs to someone else         |
| `ErrLegLimit`            | The member already has `legs_per_member` legs.  | you've already added all your legs       |
| `ErrLegsPerMemberTooLow` | The new limit is below someone's leg count.     | a member already has more legs than that |
| `ErrKickoffLocked`       | The game has kicked off.                        | that game already kicked off             |
| `ErrLockedIn`            | A locked-in member changes a leg.               | you've locked in                         |
| `ErrLegsNotReady`        | Lock in without exactly the limit.              | add all your legs before locking in      |
| `ErrSlipNotOpen`         | A member, name, or limit change after lock in.  | the slip is locked                       |
| `ErrNotLocked`           | Unlock with nobody locked in.                   | nobody is locked in                      |
| `ErrMemberHasLegs`       | Remove a member who still has legs.             | delete their legs first                  |

## Several slips

A person can belong to several slips in a week. The top bar switches between the week's slips.

## Related

- [Odds math](../reference/odds-math.md): conversions, combined odds, reference price, payout, and units.
- [Database schema](../reference/schema.md): the `weeks`, `slips`, `slip_members`, `legs`, and `activity` tables.
- [Routes](../reference/routes.md): endpoints for the pick board, slips, props, and research.
- [Data sources](../reference/data-sources.md): rosters, injuries, game logs, and player matching.
- [Groups](groups.md): how the active group scopes slips, and how membership works.
- [Product](../../plan/product.md): the auto-grading rules, the Record Book, and the screens.
