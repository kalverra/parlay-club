---
id: adr-slips-live
title: Live shared slips
type: decision
summary: 'Add shared slips: each week has one or more slips, each slip has members, and every member adds the same number of legs.'
last_verified: "2026-11-07"
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

## Consequences

The group has one shared slip per week, end to end. Friends type a leg on their phone, and every other open phone sees
it within a second. Members lock in, legs lock at kickoff, and anyone can unlock with a note in the activity feed. The
combined odds and payout calculator update as the stake and boost change.

What got easier:

- Every slip rule lives in `internal/slips`, so handlers only parse, call one service method, and map typed errors to
  422. A new route cannot invent its own locking behavior.
- Odds math is a pure package with no database or clock. Grading and the Record Book can reuse `Result`, `Payout`, and
  `Units` without taking a dependency on slip rules.
- Live updates send signals, not HTML. A missed event is harmless because the page refetches its own view, and no
  person's data crosses the hub.
- The activity feed records one entry per real change, and no-ops write nothing, so the feed is a usable history rather
  than noise.

What got harder:

- The hub is in memory, so the app is pinned to one Cloud Run instance. A second instance would silently miss publishes.
- Every open stream keeps the instance billed. The 10-minute idle deadline, the 55-minute live window, and the
  visibility close are load-bearing, not nice-to-haves.
- There is still no current-week concept. Home and the create form use `LatestWeek`, so a week with no games can be
  opened and used.
- The browser client has no automated tests. Reconnect on visibility, the bottom sheet, and the 422 toast are manual
  checks.
- `web/static/app.css` is a checked-in minified build, so template changes need `go generate ./...` before the new
  utilities appear.

We would revisit the definition of a past week. Today a week becomes history when its last game kicks off, which makes a
live Sunday editable in some places and not others. `slate-and-lines` should replace that implicit clock with an
explicit current week.

## Alternatives considered

- **Send HTML fragments over SSE.** Rejected. It would put one person's view on the wire, leak data between members, and
  make a dropped event a visible gap. Signals plus HTMX refetch keep the hub small and make a missed event self-healing.
- **Store decimal odds.** Rejected. `legs.odds` and `wagers.odds` stay American integers at rest, and only the math and
  handler edges convert. Storing the converted value would lose the book's quoted price and invite drift.
- **Enforce rules in handlers.** Rejected. Two routes could disagree about the leg limit or kickoff lock. One service
  with typed errors gives the same answer to every caller and makes the rules table-testable with an injected clock.
- **Add groups to the schema now.** Rejected for this change. The app builds for one club, but the hub already keys on
  opaque string topics and the service takes the acting user, so a later `groups` change can prefix a group id without
  reshaping either.
- **Put `Result` and `Counts()` in `internal/slips`.** Rejected. Combined odds and units need to know which results
  count. Keeping that on `parlay` lets grading use it without importing slip rules, and avoids a second result type.
- **Add a `ReferenceOdds` helper.** Rejected. `ReferencePrice` takes the candidate prices as input, so the caller
  composes reference odds from the legs it already has. The package stays ignorant of books and members.
- **Define a past week by comparing week ids.** Rejected. Before the slate lands there is no current-week signal, and
  kickoff times are the only clock available. `WeekPast` uses the last kickoff, which also makes a current week editable
  once its last game starts.

## Implementation notes

- Reference odds are not shown yet. `parlay.ReferencePrice` exists and is tested, but nothing calls it because the
  `odds` rows it needs arrive with `slate-and-lines`. The slip shows combined odds and payout only.
- Payout combines every leg on the slip, including legs still `PENDING`. Pushes and voids are dropped only once results
  exist. Grading owns the final answer.
- `CombinedOdds` returns even money (`1`) when every leg is pushed or voided. That is not a win condition; the slip
  result still comes from grading.
- A `GRADED` slip is not frozen. `LockIn`, `Unlock`, and `AddLeg` can still change it. `grading-record-book` must freeze
  graded slips, or a status check belongs in the service before then.
- `AddMember` checks that the user exists, not that they belong to the slip's group. The handlers limit the picker and
  the request to active group members, so the active-group guard is the only boundary today.
- `IsLive` is `nil` in production, so every stream uses the 10-minute idle deadline. No `week:{weekID}` publisher
  exists; `internal/refresh` will add it.
- `board-updated` fires with no listener because there is no Games board yet.
- Kickoff locks are enforced but have no UI beyond the 422 message. The leg form carries an optional hidden `game_id`
  and accepts `?game_id=`, but there is no game picker.
- Editing a leg checks kickoff on both its old and its new game, so a leg cannot be moved into a game that already
  started.
- The service publishes `slip:{id}` once, after the transaction commits. Publishing earlier would tell clients about a
  write that could roll back.
- The slip page puts `data-live-url` on the slip section, not the body. Pages without a slip do not open a stream.
- `GET /{$}` and `GET /slips/new` use `LatestWeek` as a stand-in for the current week. A slip can therefore be created
  for a week with no games.
- The browser client is covered only by handler-side stream tests. The sheet, toast, and visibility paths are manual
  checks; the repo has no JS test runner.

## Follow-ups

- **`slate-and-lines`: define the current week and use it.** Switch `GET /{$}`, `GET /slips/new`, and the week default
  from `LatestWeek` to an explicit current-week signal, and stop treating a week as past the moment its last game kicks
  off.
- **`slate-and-lines`: wire the game-aware live window.** Pass a real `IsLive(weekID)` to `live.NewStream` so games in
  progress get the 55-minute window, and add the `board-updated` listener to the Games board.
- **`internal/refresh`: publish week topics.** When a sync changes games or odds, publish `week:{weekID}` and the
  affected `slip:{id}` topics. Without this, live streams only hear about slip mutations.
- **Freeze `GRADED` slips before grading ships.** Add a status check to `internal/slips` so lock in, unlock, and leg
  writes refuse a graded slip, or make `grading-record-book` own the freeze.
- **Show reference odds.** Query each member's home book `odds` row filtered to the same game, market, player, side, and
  exact line, feed the candidates to `parlay.ReferencePrice`, and render the result next to combined odds.
- **Enforce group membership in the service.** Add the group check to `slips.Service.AddMember` and create defaults, or
  document in `docs/concepts/slips.md` that the active-group handler guard is the only boundary.
- **`groups`: scope live topics.** Prefix `slip:` and `week:` topics with the group id and scope `ServeSlip` to the
  active group, so one club's stream cannot see another club's signal.
- **Add a browser test harness.** Cover the `EventSource` client's visibility close and reopen, the `bye` handling, the
  bottom sheet, and the 422 toast, so those paths stop being manual checks.
- **Make payout result-aware.** Once grading exists, exclude pushed and voided legs and label a slip with pending legs,
  rather than quoting a payout as if every leg were still live.
