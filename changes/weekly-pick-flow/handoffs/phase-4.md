# Phase 4 handoff: Finish the hand-off and first-run states

## Done

The last week of the flow now has a clear hand-off and every dead end has a screen.

- **Export leads the export step.** `web/templates/slip.html` renders the `slip-export` wrapper first at every step. The
  wrapper is empty while the slip is open, so the card appears at the top the moment the slip locks, whether that comes
  from a page load or a `slip-updated` swap. The card's note reads "Prices may have moved since `<time>`" instead of
  "Links as of"; `linksNote` keeps the newest `odds.fetched_at` among the slip's games.
  `TestHubLeadsWithTheExportCardOnceEveryoneLocks` asserts the card precedes `#slip-panel`, carries the prices note, and
  no longer says "Links as of"; `export_test.go` covers the card and the hidden-until-locked partial.
- **Waiting offers a nudge.** `nextView` emits one `NextNudge` per member still picking, so the banner shows "Nudge Ben"
  and "Nudge Cleo" for each name. The button carries `data-nudge-slip`, `data-nudge-path`, and, when the week has a
  kickoff ahead, `data-nudge-close`. `app.js` builds "Waiting on you for {slip}. Picks close {time}. {url}", opens
  `navigator.share`, and falls back to the clipboard through the shared `copyText` helper. A past week keeps the nudge
  with no picks-close sentence. `TestHubNudgesThePeopleStillPicking`,
  `TestHubNudgeHasNoCloseTimeWithoutADeadline`, and `TestHubSaysClosedWhenAMemberNeverFinished` cover the three cases.
- **First-run and empty states.**
  - A solo group's create form says "It's just you for now. Invite friends from Groups whenever you like." and the group
    card reads "No slip yet" (`TestGroupWeekCreateFormWelcomesTheOnlyMember`, `TestLandingListsEveryGroup`).
  - A group with no slip names the week ("No slip for Week 9 yet") and offers the form; a group with no stored week
    points at the typed past-week fields and hides the empty week picker
    (`TestGroupWeekShowsTheCreateFormWithNoSlip`, `TestGroupWeekCreateFormWithoutAWeekSaysSo`).
  - The pick board has distinct screens for no stored games, games with no lines yet, every game kicked off, and a
    search that matches nothing (`TestPickBoardSaysWhenTheWeekHasNoGames`,
    `TestPickBoardSaysWhenTheWeekHasNoLines`, `TestPickBoardSaysWhenEveryGameStarted`). A week with no odds rows shows
    one "No lines for this week yet" note instead of a "Load them" prompt per game.
  - At the deadline the hub's `closed` step names the member who never finished and drops the nudge, so nobody is asked
    to chase a pick that cannot arrive.
- **Route cleanup.** `/games` is a 303 to the active group's week, and the sandbox tests assert it in place of the old
  board request. `sandbox-replay.md` and `local-dev.md` now name the pick board and the group list instead of `/games`
  and `/`.
- **Docs.** `export.md` documents the card leading the hub, the prices note, and the wrapper-first swap. `slips.md`
  already carried the hub steps, nudges, closed state, and board empty states. `plan/product.md#Screens` was rewritten
  around the group list, this week, hub, and pick board, and the tab-by-tab detail now lives only in `docs/`.
- **Sandbox walk.** Walked the sandbox's week 9 flow over HTTP as Ada, Ben, and Cleo: the list showed "Just me / No slip
  yet" and "Sunday Sandbox / Ready to lock in"; the group week 303'd to the hub; the board showed "2 of 2" with
  "Review and lock in"; Ada's lock moved the banner to "Waiting on Ben and Cleo" with two nudges and a close time; Ben
  voted on Ada's leg; Ben's and Cleo's locks moved Ada's hub to "Ready to place" with the export card first, the prices
  note, and the banner after it; `/games` 303'd to `/groups/2/week`. The templates are phone-first, the sandbox test
  asserts the phone viewport, and no new width-specific rule landed outside the board's `lg` tray column.

Verification in this worktree: `go test ./...` (every package passes), `golangci-lint run ./...` (0 issues),
`good-vibes docs lint` (ok), `good-vibes check weekly-pick-flow` (passes; 10 pre-existing stale-source warnings in docs
this change does not touch).

## Deviations

- **The export wrapper always sits first and is empty until the slip locks.** The design described the card moving above
  the banner in the `export` step. Moving a node on `slip-updated` would need out-of-band swapping; an always-present
  wrapper gives the same visible order and keeps the card in place when a lock arrives without a reload. The design's
  Screens bullet now says so.
- **The board's empty states are five, not four.** Besides the four named first-run states, the board separates "no
  games stored" from "games but no lines yet" and from a search that matches nothing. A whole week with no odds rows
  suppresses the per-game "Load them" prompts, because one week-level sentence is clearer than one prompt per game.
  The design's Screens section now lists the empty-state set.
- **The solo state lives on the create form.** A person with one group never sees the group list; `/` sends them to the
  week. "Only Just me" is therefore the create form's welcome line, while "No slip yet" stays the group card's summary
  for a group whose newest slip is in an older week. That closes the phase 1 open issue without adding a list the
  redirect skips.
- **`closed` outranks `waiting` when someone cannot finish.** A locked viewer whose week is over and whose slip still
  has a member short of legs sees `closed` with that member named, not `waiting` with a nudge. The design's `closed`
  bullet already promised "picks are over for you this week"; this is the case that makes the promise useful.

## Open issues

- **Chip counts still follow the whole week, not the search.** `categoryChips` and `windowChips` in `internal/web/pick.go`
  count every built row, so a filtered board can show a count that includes hidden rows; an active chip stays visible at
  zero. This was a phase 2 open issue and phase 4 did not change it. If it matters, count the rows `board.Search`
  returned and keep the active chip.
- **A past-week tray still draws remove buttons.** `trayFrom` skips the kickoff lock when `WeekPast` is true, so the tray
  offers remove on a week whose games all kicked off; the service then refuses with "that game already kicked off". The
  board is empty for that week, so this is a small inconsistency. The phase 2 note asked the empty-state pass to decide;
  it did not, and this remains open.
- **The old Games page is dead code.** `web/templates/games.html`, the `games` entry in `render.go`, and the `games`
  name in `render_test.go` survive though nothing routes to them. They are harmless and can go in a cleanup.
- **The board rebuilds the week's odds on every request.** One `OddsForWeek` read plus in-memory grouping is still the
  cost per keystroke, filter, and sort. The design's cache-per-(week, fetch time) idea remains open.
- **The nudge's browser half has no automated test.** The server side (attributes, deadline, path) is tested, and the
  message builder is small, but the share-sheet call, the clipboard fallback, and the "Nudge copied" label are only
  checked by hand. The same is true of other `app.js` behavior.
- **Layout is checked against phone-first markup, not screenshots.** The sandbox walk exercised the flow and the suite
  asserts the viewport, but there is no automated check of the 375 px and `lg` renderings. The board's two-column tray
  is the only new width-specific rule.
- **A removed member's votes still survive.** This is the phase 3 open issue; phase 4 did not touch `RemoveMember`.
- **The group list still reads four queries per card.** The risk that a person in many groups pays the cost on every
  landing and refetch-on-visible is unchanged and unmeasured.

## Next-phase notes

- The change is ready for close-out; all four phases' tasks are complete. Write `changes/weekly-pick-flow/closeout.md`
  and let a person accept it before `good-vibes done weekly-pick-flow`.
- Keep the export wrapper first in `slip.html`. The order is what makes the card lead the `export` step after a live
  lock, and moving it inside a conditional would reintroduce the reorder the design avoided.
- Empty-state copy lives in four places: `nextView` in `internal/web/slips.go` (`closed`, `waiting`), `slip.html` (the
  create form), `pickPageData` in `internal/web/pick.go` (`NoGames`, `NoLines`, `AllStarted`, `Empty`), and `home.go`
  ("No slip yet"). Change the copy and the matching tests together; the docs describe all four.
- The nudge stays a client-only message. Its inputs are `data-nudge-slip`, `data-nudge-path`, and the optional
  `data-nudge-close`; a new deadline source must keep `CloseISO` in RFC 3339 because `app.js` parses it with `Date`.
- Votes close when the last member locks. Do not add vote prompts or vote counts to the `waiting`, `export`, or `done`
  copy; voting is already over by then.
- To redo the sandbox walk: run `SUNDAY_SANDBOX=1 mise run sandbox`, sign in through `/dev/login` as
  `ada@sandbox.test`, then use the persona switch at `/dev/sandbox`. The shared group is "Sunday Sandbox" and the solo
  group is "Just me".
- If the chip-count or past-week tray issues get fixed, they are independent of the close-out and can be separate small
  changes. The phase 2 and 3 open issues are otherwise still accurate.

## Files touched

- `docs/concepts/export.md`
- `docs/concepts/slips.md`
- `docs/guides/local-dev.md`
- `docs/reference/routes.md`
- `docs/reference/sandbox-replay.md`
- `internal/slips/next.go`
- `internal/slips/next_test.go`
- `internal/web/export.go`
- `internal/web/export_test.go`
- `internal/web/home_test.go`
- `internal/web/next_test.go`
- `internal/web/pick.go`
- `internal/web/pick_test.go`
- `internal/web/slips.go`
- `plan/product.md`
- `web/static/app.js`
- `web/templates/partials.html`
- `web/templates/slip.html`

## Run log

- Mode: 
- Checkpoints: task-7, task-8, task-9
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
