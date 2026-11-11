---
slug: weekly-pick-flow
run: 20261111-192956-weekly-pick-flow
outcome: done
generated: 2026-11-11T20:34:57Z
---

# Run summary: weekly-pick-flow

## Outcome

- Status: done
- Branch: weekly-pick-flow
- Final verdict: claude@claude-opus-5-5 approved (issues: 0)

## Totals

- Tasks: 9 of 9
- Phases: 4
- Checkpoints: 10
- Duration: 1h2m
- Cost: $1.91
- Input tokens: 92822608
- Output tokens: 440458
- Cache read tokens: 90007594

## Phases

### Phase 1: Land on groups and the hub

Tasks:

- [x] Serve the group list at `/` (one-group redirect, refetch on visible), add `GET /groups/{id}/week` with the `SetGroup` seam, switch the active group when a member opens another group's slip, and change the bottom bar to Groups, This week, Record Book with Games still reachable from the hub; test landing, switching, and non-member 404s, and update `groups.md`, `routes.md`, and `access.md` where they describe the landing page
- [x] Add `slips.NextStep` and reshape the hub around its banner, deadline, and one primary action (pick links to `/games` until the board lands); test every step with an injected clock and update `slips.md`

Phase totals: 2 checkpoints, 14m2s, $0.47, 24792746 input / 117177 output / 24177228 cache tokens

Checkpoints: task-1, task-2 Review fixes: 0 Integration checks: 0

#### Done

`GET /` is now the group list and every slip screen says what to do next.

- **Landing (`internal/web/home.go`, `web/templates/home.html`).** One card per membership, oldest first, showing the
  group name and the viewer's `NextStep` label on that group's newest slip of the latest week ("Pick 1 more", "Waiting
  on Ben", "Ready to place", "No slip yet"). A person in one group gets a 303 to `/groups/{id}/week` from a full page
  load; an HTMX request still gets the list. The `#group-list` element carries `data-refresh-on-visible` and
  `hx-get="/"`, and `app.js` triggers the refetch on `visibilitychange` and `focus`, so the page holds no SSE stream.
- **This week (`GET /groups/{id}/week`).** Checks membership (404 for a non-member, before any cookie is written), then
  makes the group active through a new `Dependencies.SetGroup` seam that `cmd/server/main.go` wires to
  `cookies.SetGroup`. It 303s to the group's newest slip of the latest week, or renders the create form. The shell
  marks the group the page acts in, not the one the cookie named a moment ago.
- **Group switching on slip links.** `loadSlip` no longer 404s a slip in another of the viewer's groups. It checks
  membership, reads the group, writes `pc_group` through the same seam, and renders the hub under that group. A slip
  outside every group the viewer belongs to still 404s, unchanged.
- **Joining a slip.** `slips.AddMember` gained `loadJoin`, so a group member who is not on the slip can add themselves
  while it is `OPEN`, with an "Ada joined" activity entry. Adding someone else still requires the caller to be on the
  slip, and everyone else still gets `ErrNotMember`.
- **Bottom bar (`internal/web/page.go`).** Groups (`/`), This week (`/groups/{active}/week`, falling back to `/` when
  the page has no active group), Record Book. `TabSlip`, `TabGames`, and `TabResearch` are gone. Games and Research
  keep full pages and leave the bar; the hub's pick action links to `/games`, and leg and game rows keep their
  Research links.
- **Hub (`internal/slips/next.go`, `internal/web/slips.go`, `web/templates/partials.html`).** The step banner sits at
  the top of `#slip-panel`: title, one line of detail, the deadline as
  `Picks close for MIA @ BUF at <time datetime="…">`, and one action. Pick links to `/games`; lock in posts
  `/slips/{id}/lock`; join posts `/slips/{id}/members` with the viewer's id. The old inline Lock in button is gone, so
  the banner is the only primary action. Members, legs, export, payout, and activity keep their order and partials, and
  the hub still opens one stream.
- **`slips.NextStep`.** A pure function over the slip, members, legs, the week's games, the viewer, and `now`. It
  returns the step (`join`, `pick`, `closed`, `lock_in`, `waiting`, `export`, `done`), legs still needed, names not
  locked in (member order), the first kickoff still ahead, and the result. `Label()` holds the shared copy. An empty
  week reads as `pick`, not `closed`; a graded slip wins over every other state; the last lock reads as `export` even
  before the status row catches up; unlocking drops back to `pick` or `lock_in`.
- **Store.** `Store.GroupByID` reads one group and wraps `ErrGroupNotFound`.
- **Tests.** `internal/slips/next_test.go` tables every step, the unlock path, the three-name list, the London 9:30
  a.m. ET first kickoff as the deadline, a past week with no deadline, an empty week, and each grade label.
  `internal/web/home_test.go` covers the list, the one-group redirect, the HTMX partial, the group switch from a card,
  the create form, the non-member 404, the slip-link switch, the outsider 404, and the three-tab bar.
  `internal/web/next_test.go` drives each banner through the injected clock, including the join round trip and the
  deadline disappearing once everyone locks. Layout, page, routes, and render tests moved to three tabs and the home
  page.
- **Docs.** `groups.md`, `access.md`, `routes.md`, and `slips.md` describe the group list, the week route, the cookie
  switch, the three-tab shell, the hub table, joining, and Research leaving the bar. Front matter sources and
  `last_verified` are updated, and `docs/llms.txt` carries the new `slips.md` summary.
- **CSS.** `web/static/app.css` was regenerated with the pinned Tailwind CLI; the checked-in build predated the
  banner's `bg-sky-950/50`, which would otherwise have rendered with no background.

Verification in this worktree: `go test -count=1 ./...` (every package passes), `golangci-lint run ./...` (0 issues),
`good-vibes docs lint` (0 errors; 15 pre-existing stale-source warnings in docs this change does not touch), and
`good-vibes check weekly-pick-flow` (passes).

### Phase 2: Build the pick board

Tasks:

- [x] Add `internal/board` (`Build`, `Search`) and `Store.OddsForWeek`, serve `GET /slips/{id}/pick` and its results partial with search, category and window chips, sorts, the one-game view, toggle add and remove, and the "Load them" prompt; point the hub's pick action at it, make `/games` a 303, take `?slip=` on `/games/{id}/props`, and test search, sorts, windows, and zero prop syncs; update `slips.md`, `routes.md`, and `data-refresh.md`
- [x] Add the tray (progress, remove, heads-ups on your legs, Done or Review and lock in) and replace at the limit through `PUT /legs/{id}`; test add, remove, replace, the kickoff and lock-in refusals, and the tray warning; update `slips.md` and `analyzer.md`

Phase totals: 2 checkpoints, 16m41s, $0.55, 27806172 input / 136485 output / 27054538 cache tokens

Checkpoints: task-3, task-4 Review fixes: 0 Integration checks: 0

#### Done

The pick board is now the place picks happen, and the tray finishes the round trip from the hub.

- **`internal/board` (`board.go`).** `Build` turns one week's stored odds into one row per selection. It uses the
  viewer's home-book line and price when that book carries the pick, the median across books otherwise, fills in the
  player ID from the roster, labels the row ("Josh Allen Over 245.5 Pass Yds"), maps it to a category (lines, team
  totals, passing, rushing, receiving, TDs, kicking) and an Eastern-time window (Sunday morning before noon, early
  before 4, late before 7, night, Monday), and leaves out games that kicked off. `Search` folds accents, lowercases,
  and strips punctuation, matches row text, player, team, and matchup, filters by category, window, and game, applies
  one of four sorts (kickoff, player, category, odds shortest first), counts started games, and names games whose props
  were never fetched. A picked row carries the viewer's leg ID. `IsPropRow` moved here so the board and the props
  partial share one definition.
- **`Store.OddsForWeek` (`internal/db/odds.go`).** One ordered read of every price for a week's games, so a board
  request does not query per game.
- **Pick routes (`internal/web/pick.go`, `internal/web/slips.go`).**
  - `GET /slips/{id}/pick` renders the full page with a back link, no bottom bar, the slip's `data-live-url`, and the
    tray in a right column at `lg`.
  - `GET /slips/{id}/pick/results` answers the search, chip, sort, and one-game requests. The search input debounces at
    250 ms; chips and sorts are links that swap the same partial and push the full page URL. Chips only appear for
    categories and windows with rows.
  - `GET /slips/{id}/pick/swap` renders the "Swap which pick?" sheet from the viewer's unlocked legs.
  - `GET /slips/{id}/tray` renders the tray partial.
  - `?game=` shows one game and pairs both sides of each line in one row, with a link back to the whole week.
- **Add and remove.** An unpicked row posts `POST /slips/{id}/legs` with `quick=1&from=board`; a picked row sends
  `DELETE /legs/{id}?from=board`. Both answer with the row and fire `Hx-Trigger: tray-updated`, so the tray refreshes
  without a reload. A row at the limit offers "swap in" instead of add.
- **"Load them".** A game whose props were never fetched appears as a prompt when the search names its team, matchup,
  or a roster player on it. The button calls `GET /games/{id}/props?slip=`, which runs `SyncGameProps` for that one game
  and answers an empty 200 with `Hx-Trigger: board-updated`; the results partial then re-runs the current search.
  Opening, typing, filtering, sorting, and the tray call `SyncGameProps` zero times.
- **Hub and old Games route.** The hub's pick action points at `/slips/{id}/pick`. `GET /games` is a 303 to the active
  group's week (or `/` when no group is active). `GET /games/{id}/props` still renders the standalone prop partial when
  no `?slip=` is given, so old game and Research links keep working.
- **Tray.** The viewer's legs with remove buttons (or "Locked" once the game kicks off), progress ("1 of 2"), the
  analyzer warnings that name one of the viewer's legs, and one action: "Done" back to the hub, or "Review and lock in"
  when `NextStep` says `lock_in`. It refreshes on `tray-updated` and `slip-updated`.
- **Replace at the limit.** The swap sheet lists the viewer's legs that are not kickoff-locked; a choice sends
  `PUT /legs/{old}` with `from=board` and the new pick's fields. `UpdateLeg` enforces ownership, lock in, and kickoff on
  both the old and the new game, writes one "changed" activity entry, and the handler answers with the refreshed
  results partial and `tray-updated`. A kickoff-locked old leg and a locked-in member are still refused. When every leg
  has kicked off, the sheet says "Every one of your picks has kicked off."
- **One props builder.** `games.go` now keeps only the standalone props rendering; it groups `board.Build` rows into
  team totals and per-player pills, so the board and the props partial cannot drift.
- **`currentOdds` fix.** A week read exposed that the old helper kept only the newest prop timestamp across every game,
  so fetching one game's props later hid another game's props (and made the board offer "Load them" for a game whose
  props were stored). It now keeps the newest fetch per game and sync group. The old single-game callers behave the
  same.
- **Tests.** `internal/board/board_test.go` covers every category and window, the Monday-night UTC-Tuesday kickoff,
  each sort, accent-insensitive search, started games hidden and counted, the home-book price over the median, an
  unloaded game named from its roster, and picked rows carrying the leg ID. `internal/web/pick_test.go` covers the
  page, search/filter/sort, two games keeping their own props, the unloaded prompt and one `SyncGameProps` tap, zero
  syncs on open/search/filter/sort/tray, add and remove round trips, swap at the limit, kickoff and lock-in refusals,
  the tray warning filter, the one-game pairing, the tray's `Review and lock in`, and the `/games` redirect.
  `internal/web/current_odds_test.go` pins the per-game behavior. The sandbox test now asserts the redirect and that the
  board hides kicked-off games at the scenario clock.
- **Docs and client.** `slips.md` rewrote the Games tab as the pick board, the tray, and replace; `routes.md` added the
  board, results, swap, tray, redirect, and `?slip=` rows; `data-refresh.md` explains "Load them" and that the board
  spends no credits; `analyzer.md` describes the tray's filtered heads-ups. `app.js` opens and closes the swap sheet and
  refreshes the board on `board-updated`; the CSS was rebuilt for the board layout.

### Phase 3: Add votes

Tasks:

- [x] Add the `leg_votes` migration, store methods, `Service.Vote`, and vote clearing in `UpdateLeg`; test every rule in the Votes verification list; update `schema.md` and the editing rules in `slips.md`
- [x] Draw tallies and vote buttons on the hub's leg rows, read-only once the slip locks; test the views and a second client's refresh from `slip-updated`; update `slips.md` and `live-updates.md`

Phase totals: 2 checkpoints, 11m54s, $0.31, 13530786 input / 70554 output / 13048534 cache tokens

Checkpoints: task-5, task-6 Review fixes: 0 Integration checks: 0

#### Done

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

### Phase 4: Finish the hand-off and first-run states

Tasks:

- [x] Put export first in the `export` step with the prices note, and add the share-sheet nudge in the `waiting` step; test both and update `export.md`
- [x] Give each empty state a real screen (only "Just me", a group with no slip, a week with no lines, a member still missing at the deadline) and walk the full flow in the sandbox as two personas at phone width; update `local-dev.md` and `sandbox-replay.md` where they name `/` or `/games`
- [x] Docs: rewrite `plan/product.md#Screens` for the new flow, delete what `docs/` now covers, and run `good-vibes docs lint` and `good-vibes docs index`

Phase totals: 4 checkpoints, 19m38s, $0.58, 26692904 input / 116242 output / 25727294 cache tokens

Checkpoints: task-7, task-8, task-9, final Review fixes: 0 Integration checks: 0

#### Done

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

## Final gate

- Rounds used: 1; verify failures: 0
- Final verify: no commands configured
- Open issues: none
