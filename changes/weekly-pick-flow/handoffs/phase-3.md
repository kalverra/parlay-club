# Phase 3 handoff: Add votes

## Done

Members can now up- or down-vote each other's picks on the hub, and votes behave like a tally rather than a second feed.

- **Migration (`internal/db/migrations/0008_leg_votes.sql`).** `leg_votes` has one row per member per leg, a `(-1, 1)`
  check, a primary key on `(leg_id, user_id)`, and a cascade from `legs`. `migrate_test.go` expects version 8 and the
  table, and `migrate_internal_test.go` expects version 8 after the 0002 replay.
- **Store (`internal/db/votes.go`).** `LegVoteValue` returns 0 when nobody voted; `SetLegVote` upserts one row per person;
  `ClearLegVote` deletes one row; `DeleteLegVotes` clears a leg in the caller's transaction; `VotesForSlip` joins the
  slip's legs and orders by leg and voter. `internal/db/votes_test.go` covers the upsert, the missing-vote zero, clearing
  only that person's vote, per-slip scoping, the delete cascade, and the table's refusal of a zero value.
- **Rules (`internal/slips/rules.go`).** `Service.Vote` validates the value, loads the leg and slip in one transaction,
  refuses a non-member (`ErrNotMember`), the owner (`ErrOwnVote`), a slip that is not `OPEN` (`ErrVotesClosed`), and
  values outside -1/0/1 (`ErrVoteInvalid`). The same value again returns without writing or publishing; a clear deletes
  the row; a change publishes `slip:{id}` and writes no activity entry. Kickoff does not close a vote, and unlock reopens
  it because the slip returns to `OPEN`.
- **Vote clearing (`UpdateLeg`).** `pickChanged` compares game, market, selection, team, player, side, and line. A real
  pick change deletes the leg's votes inside the edit transaction, before the activity entry; an odds-only edit keeps
  them. `DeleteLeg` still clears by cascade.
- **Service tests (`internal/slips/votes_test.go`).** Up, down, replace, clear, repeated vote with no publish, clear with
  nothing stored, own leg, non-member, bad value, closed after every member locks and reopened by unlock, still open with
  one member locked, no close at kickoff, line change clears, odds-only keeps, delete cascades, and no activity for any
  vote path.
- **Hub and panel (`internal/web/slips.go`, `web/templates/partials.html`).** `slipPageData` reads `VotesForSlip`;
  `voteViews` folds the rows into `Up`, `Down`, and the viewer's own value; `legVoteView` sets `CanVote`, `Has`, and the
  value each tap sends, so tapping the highlighted side clears it. The leg row renders the tally as `▲ n` and `▼ n`,
  as POST forms to `/legs/{id}/vote` while the slip is open, and as plain chips once it locks. The owner never gets a
  form. The panel and the full hub share the partial, so both show the same controls and tallies.
- **Route (`POST /legs/{id}/vote`).** The handler parses `value`, answers 422 for anything outside -1/0/1, maps the
  service's typed errors through the existing 422 path, and answers with the refreshed slip panel so the voter's tally
  updates in place. Other browsers refresh from `slip-updated`.
- **Web tests (`internal/web/votes_test.go`).** Someone else's leg gets controls and a zero tally on the panel and the
  hub; a vote updates the response and a fresh panel read; the same tap clears; the owner's leg has no form and a direct
  post is 422; a vote with one member locked still works; a locked slip strips the forms, keeps the tally, and refuses a
  post; an unknown value is 422; and a second client reading `/slips/{id}/events` receives `slip-updated` and its next
  panel fetch carries the new tally.
- **Docs.** `slips.md` gained its Votes section, editing rule 9 (the no-activity exception), the three new errors in the
  table, and the hub's read-only behavior. `schema.md` lists migration 0008, the table, its columns, and the storage
  notes. `live-updates.md` says `slip-updated` includes votes. `routes.md` has the vote route and its rules. `docs/llms.txt`
  was rebuilt.

## Deviations

- **Selection counts as part of the pick.** The design listed game, market, team, player, side, and line. `pickChanged`
  also compares `selection`, because a manual leg can change its label without changing any of those fields, and keeping
  votes across a visibly different pick would be wrong. The design's Votes approach now says so.
- **The tally is drawn only when it says something.** The design said "▲ 2 ▼ 1 on every leg". The template shows the
  tally when the viewer can vote or when the leg already has votes; an owner's unvoted leg shows nothing instead of
  "▲ 0 ▼ 0". Anyone who can vote always sees both buttons, zero counts included. The design's Votes bullet now matches.
- **`ErrVoteInvalid` is a new typed error.** The design named the three values but no error for anything else. The
  service returns `ErrVoteInvalid`, the handler maps it to 422 with "vote must be up, down, or clear", and the error table
  documents it.
- **The vote route answers with the panel.** The design only named the publish. Like add, remove, edit, and lock, the
  handler returns the refreshed panel so the actor does not wait for their own stream event. The design's Votes bullet
  records that, and `slip-updated` still refreshes everyone else.

## Open issues

- **A removed member's votes stay behind.** `leg_votes.user_id` references the user, not the membership, so removing a
  member from an open slip leaves their votes on the other members' legs (their own legs must already be deleted).
  Adding them back would also revive the old votes. The design does not cover removal, and the phase's tasks did not
  include it; a small follow-up can delete that person's votes for the slip inside `RemoveMember`.
- **Every vote re-renders the whole panel.** The response rebuilds members, legs, activity, and the analyzer. That is the
  existing pattern for leg routes and is cheap on one SQLite instance, but if a busy slip shows the cost, answer with
  just the leg row and swap that.
- **Owners see no zero tally.** An owner's leg with no votes has no row at all, so there is no hint that anyone could
  vote on it. This is intentional in the current template; phase 4's polish should keep it unless someone asks for it.
- **The pick board does not show votes.** Design placed votes on the hub, so nothing is missing, but someone deciding
  what to pick cannot see which picks the group liked. Adding tallies to `board.Row` would be a new design decision.
- **`leg_votes.updated_at` is written but not read.** The tally only needs the value today. It is there if the Record Book
  ever wants "recent votes" ordering; until then it is unused state.
- **Non-member behavior is only tested at the service.** The web layer 404s a non-member's leg before the vote handler
  runs, matching every other leg route, so the 422 path in `voteLeg` for `ErrNotMember` is not reachable through HTTP.
  That is fine, but do not rely on the web test to catch a regression in that guard.

## Next-phase notes

- Phase 4 owns the hand-off and first-run states; votes do not affect `NextStep` or any banner copy. Keep it that way:
  the `waiting` and `export` steps must not imply voting is still open, because the last lock in closes it.
- Any new view that shows legs and wants tallies should go through `slipPageData` (which already reads `VotesForSlip`) or
  reuse the pure `voteViews` and `legVoteView` helpers. A second tally implementation would drift from the hub.
- Keep `pickChanged` as the one definition of "a different pick". The board's replace path posts `PUT /legs/{old}` and
  relies on `UpdateLeg` dropping the old votes; changing the board's fields without touching `pickChanged` would let votes
  follow a swapped pick.
- The sandbox seeds an editable slip with named legs for both personas, so the phase 4 walk can vote as the second
  persona on the first persona's leg and watch the hub update in place, then open a second tab and see the
  `slip-updated` refetch. `cmd/server/sandbox_test.go` does not exercise votes yet; phase 4's walk is manual.
- There is no vote-specific CSS. The controls reuse the existing chip borders and colors, so they were checked at 375 px
  without new layout rules.
- `internal/web/votes_test.go` builds on the phase 2 `slipFixture` and `addVotableLeg`; reuse those helpers for any
  phase 4 UI assertions instead of writing another fixture.

## Files touched

- `docs/concepts/live-updates.md`
- `docs/concepts/slips.md`
- `docs/llms.txt`
- `docs/reference/routes.md`
- `docs/reference/schema.md`
- `internal/db/migrate_internal_test.go`
- `internal/db/migrate_test.go`
- `internal/db/migrations/0008_leg_votes.sql`
- `internal/db/votes.go`
- `internal/db/votes_test.go`
- `internal/slips/rules.go`
- `internal/slips/votes_test.go`
- `internal/web/slips.go`
- `internal/web/votes_test.go`
- `web/static/app.css`
- `web/templates/partials.html`

## Run log

- Mode: 
- Checkpoints: task-5, task-6
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
