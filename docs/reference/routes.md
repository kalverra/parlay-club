---
id: routes
title: Routes
type: reference
summary: Every HTTP route the server serves, the middleware in front of them, and the status codes, HTMX headers, and SSE frames handlers return.
tags: [routes, http, handlers, middleware, htmx, sse, props, prefill, research, schedules, wagers, record, analyzer, guide]
---

# Routes

## Summary

One `http.ServeMux` holds every route, using Go 1.22 method patterns. Handlers stay thin: they parse the request, call
`internal/slips` or the database store, and render a page or partial. Routes still to be built are listed in the
[architecture plan](../../plan/architecture.md).

## Middleware

All routes except `/healthz`, `/readyz`, `/static/`, `/screenshots/{token}.jpg`, `/login`, `POST /logout`, the Google
sign-in routes, the dev sign-in routes, and `POST /internal/prefill` need a signed-in user. Those public/internal routes
are registered outside session checks (`POST /internal/prefill` also sits on an outer mux outside `CSRF` and uses bearer
auth instead). Everything else runs through three wrappers, outermost first. See [Access](../concepts/access.md) for
cookies and sign-in.

| Middleware    | Rejects                                                        | Response                          |
| ------------- | -------------------------------------------------------------- | --------------------------------- |
| `CSRF`        | A non-GET request without `HX-Request: true`                   | 403                               |
| `RequireUser` | A missing, tampered, expired, inactive, or unknown session      | `/login`, or `HX-Redirect` for HTMX |
| `Scope`       | Nothing for a valid session; a user with no group gets a 500    | —                                 |

`CSRF` exempts two plain forms, `POST /logout` and `POST /dev/login`. Every other write must come from HTMX.

`Scope` picks the active group and puts it on the request context. Handlers read it instead of taking a group id in the
path. A group or slip outside the person's membership answers 404, the same as a missing one. A slip or week in another
of their groups makes that group active instead.

## Public routes

| Method + path                       | Purpose                                                                 | Notes |
| ----------------------------------- | ----------------------------------------------------------------------- | ----- |
| `GET /healthz`                      | Startup and liveness probe                                              | Pings SQLite with a two-second timeout. 200 `ok` when it answers, 503 `unavailable` when it does not; the SQLite error goes to the log. |
| `GET /static/{path...}`             | Embedded CSS, JS, and other assets                                      | One-year cache. A missing or escaping path is 404. |
| `GET /screenshots/{token}.jpg`      | Capability URL for a bug report screenshot                              | Registered as `GET /screenshots/{file}` and checks the `.jpg` suffix. One-year immutable cache with `nosniff`; unknown token, missing image, or non-`.jpg` suffix is 404. |
| `GET /login`                        | Sign-in page                                                            | Shows the Google button, plus a dev link when Google is off. |
| `POST /logout`                      | Clear the session and active-group cookies                              | 303 to `/login`. |
| `GET /auth/google/start`            | Start the Google round trip                                             | Registered only when a Google client is configured. Redirects to Google. |
| `GET /auth/google/callback`         | Finish Google sign-in                                                   | Bad `state`, code exchange, or identity answers 400 with the sign-in page. |
| `GET /dev/login`, `POST /dev/login` | Local sign-in form and submit                                           | Registered only when `APP_ENV=dev` and `GOOGLE_CLIENT_ID` is empty. |
| `POST /internal/prefill`            | Run the Sunday morning pre-fill sweep synchronously                     | Outside cookie/CSRF middleware; requires `Authorization: Bearer <PREFILL_TOKEN>` (constant-time compare). Returns 404 when `PREFILL_TOKEN` is empty, 401 on missing/invalid token, 503 in sandbox mode, 200 JSON `PrefillReport` on success, or 500 JSON on failure. |

`GET /healthz` is what Cloud Run's startup and liveness probes call. It pings SQLite with a two-second timeout derived
from the request, so a probe never waits on a busy database. A healthy handle answers 200 with the body `ok`; a closed
or unreachable one answers 503 with `unavailable`, and the SQLite error goes to the log instead of the response. The
route stays outside auth, so a revision whose database failed can still be replaced.

### Sandbox-only routes

These routes are mounted only when `SUNDAY_SANDBOX=1`. They sit inside the normal `RequireUser`, group scope, and HTMX
CSRF checks, so they are not public.

| Method + path          | Purpose                                                    | Response |
| ---------------------- | ---------------------------------------------------------- | -------- |
| `GET /dev/sandbox`     | Controls page: stage, scenario time, persona, group, invite | Full page. Shows stage, persona, and reset controls. |
| `POST /dev/sandbox/stage` | Rebuild morning and apply one scripted stage            | `stage` is `morning`, `early-loss`, `all-clear`, or `sunday-complete`. 200 with `HX-Refresh` and new `pc_session`/`pc_group` cookies. Unknown stage is 422 and changes nothing. |
| `POST /dev/sandbox/persona` | Sign the browser in as a seeded persona               | `persona` names one of `ada`, `ben`, `cleo`, or `dex`. 200 with `HX-Refresh` and new `pc_session`/`pc_group` cookies. Unknown persona is 422 and changes nothing. |
| `POST /dev/sandbox/reset` | Rebuild the fixed week 9 morning, people, and slip      | 200 with `HX-Refresh`, and reissued session and group cookies for the same persona |

Every stage rebuilds the Sunday morning baseline, then sets its clock and replay snapshot, runs the real refresh, locks
the shared slip, and calls the normal slip service for that stage's scripted manual grades. It never infers a result
from a score. The stage publishes the normal week and slip signals after its writes commit; the acting browser reloads
from `HX-Refresh`, and other connected clients refetch after the SSE signals. `POST /dev/sandbox/reset` is the same
operation as the `morning` stage.

The persona switch calls the normal sign-in code and then sets the real cookies; the request context is never given a
user the middleware did not load. Ada, Ben, and Cleo land in the shared group; Dex lands in their own group until they
accept the shared invite. A forged group cookie still falls back to a real membership, so the outsider gets the usual
404 for shared slips.

## Authenticated routes

### Shell and tabs

| Method + path             | Purpose                                                                                          |
| ------------------------- | ------------------------------------------------------------------------------------------------ |
| `GET /{$}`                | Landing page: one card per group; a person with one group is sent to that group's week            |
| `GET /groups/{id}/week`   | Make the group active, ensure scheduled slips exist, then redirect to the slip (1 slip), render the week hub (>1 slips), or show create form (0 slips) |
| `GET /games`              | 303 to the active group's week; the pick board owns the week                                     |
| `GET /research`           | Research page: week games, player search, injury reports, and last-5 hit bars                     |
| `GET /record`             | Season Record Book for the active group (optional `?season=`)                                    |
| `GET /analyzer/guide`     | Grading & Analyzer Guide: live `A`–`F` rubric, pick-board badges, and every warning rule         |

The bottom bar has three destinations: Groups (`/`), This week (`/groups/{active}/week`), and Record Book. Research
keeps a full page but leaves the bar; the hub links to the pick board, and pick rows link to Research.

`GET /{$}` lists every group oldest first and carries `data-refresh-on-visible`, so the browser refetches the list when
the tab becomes visible instead of holding a stream. A person in one group gets a 303 to `/groups/{id}/week`.

`GET /groups/{id}/week` answers 404 for a group the person does not belong to, sets `pc_group`, and calls
`slips.EnsureWeekSlips` to auto-generate slips for active recurring schedules. With zero slips for the week, it renders
the create form. With exactly one slip, it 303s directly to `/slips/{id}`. With multiple slips, it renders the week hub
(`week.html`) with cards for each slip (name, scope badge, next step, deadline, progress) and a button to add a one-off
slip. It uses the same group-switch seam as a slip link from another group.

The slip handler file also serves `/me`, `/record`, and the slip and leg routes below. The pick board, Research, and
Record Book are described below.

### Groups

| Method + path                                     | Purpose                                                                        | Response |
| ------------------------------------------------- | ------------------------------------------------------------------------------ | -------- |
| `GET /groups`                                     | Manage groups: create form, invite links, and leave buttons                     | Full page |
| `POST /groups`                                    | Create a group from `name` and make it active                                   | 303 to `/groups` |
| `POST /groups/switch`                             | Make the group in `group_id` active                                             | `HX-Refresh` |
| `POST /groups/{id}/reset-invite`                  | Mint a new invite token and kill the old one                                    | `HX-Refresh` |
| `POST /groups/{id}/leave`                         | Leave the group, keeping the person's legs                                      | 303 to `/groups` |
| `GET /join/{token}`                               | Join page naming the group and its active members                               | Full page; unknown token 404 |
| `POST /join/{token}`                              | Join the group idempotently and make it active                                  | 303 to `/` |
| `GET /groups/{id}/schedules`                      | List recurring slip schedules for the group                                    | JSON array of schedules |
| `POST /groups/{id}/schedules`                     | Add a recurring slip schedule from `name`, `slate_scope`, and `legs_per_member` | `HX-Refresh`, 201 JSON, or 303 to `/groups/{id}/week` |
| `POST /groups/{id}/schedules/{scheduleID}/active` | Set or toggle schedule active status (`active` on/off)                          | `HX-Refresh`, 200 JSON, or 303 to `/groups/{id}/week` |
| `PATCH /groups/{id}/schedules/{scheduleID}`       | Update `name`, `slate_scope`, or `legs_per_member`                              | `HX-Refresh`, 200 JSON, or 303 to `/groups/{id}/week` |
| `DELETE /groups/{id}/schedules/{scheduleID}`      | Delete a recurring slip schedule                                               | `HX-Refresh`, 204 No Content, or 303 to `/groups/{id}/week` |

A signed-out visitor is sent to `/login` with the invite path kept in `pc_oauth`, so sign-in returns them to the join
page. Only a `/join/` path is accepted as a return target.

`POST /groups/{id}/reset-invite` and `POST /groups/{id}/leave` answer 404 when the caller is not in that group, so a
stranger cannot learn which groups exist.

Slip schedule routes manage recurring parlays for a group. They accept both HTML form submissions (returning
`HX-Refresh: true` for HTMX or 303 redirects) and JSON payloads (returning JSON entities). Handlers verify caller
membership in the target schedule's group; an unknown schedule or a request from a non-member answers 404 to avoid
leaking group existence. Input validation errors (blank name, duplicate name within the group, invalid slate scope, or
legs outside 1–5) answer 422.

### Slips

Slip routes require membership in the slip's group. A slip in another of the person's groups makes that group active; a
slip outside every group is 404.

| Method + path                              | Purpose                                                         | Request and response |
| ------------------------------------------ | --------------------------------------------------------------- | -------------------- |
| `GET /slips/new`                           | Create-slip page                                                | Optional `?week=` and `?slate_scope=` (or `?scope=`); unknown week 404. Full page. |
| `POST /slips`                              | Create the slip from `week_id` (or a typed `season`, `season_type`, and `week`), `name`, `slate_scope`, `legs_per_member`, and `member_id` | 303 to `/slips/{id}`; a fixable error re-renders the form. |
| `GET /slips/{id}`                          | Full slip page                                                  | Full page, with the payout form at the default stake. |
| `GET /slips/{id}/pick`                     | Focused pick board: the game slate, the open game's picks, and the dock | Full page; no bottom bar. |
| `GET /slips/{id}/pick/results`             | `#pick-body` for a search, window, or game change                | Optional `?q=`, `?cat=`, `?win=`, `?sort=`, and `?game=`; `#pick-body` partial. |
| `GET /slips/{id}/pick/detail`              | `#pick-detail` for a tab or sort change                          | Optional `?q=`, `?cat=`, `?sort=`, `?win=`, and `?game=`; `#pick-detail` partial, or the "Pick a game" placeholder without an open game. |
| `POST /slips/{id}/pick/refresh`            | Force a pick-time odds refresh on an `OPEN` slip                 | Optional `game`, `q`, `cat`, `win`, `sort`; `#pick-body` partial (2m cooldown). Returns 422 when the slip is not `OPEN`. |
| `GET /slips/{id}/pick/swap`                | Swap sheet for one unpicked row                                 | Row fields in the query; swap partial. |
| `GET /slips/{id}/pick/dock`                | The fixed dock's count and combined price                       | Dock partial. Refreshes on `tray-updated` and `slip-updated`. |
| `GET /slips/{id}/tray`                     | The viewer's picks, warnings, and one action, loaded into the dock's sheet | Tray partial. Refreshes on `tray-updated` and `slip-updated`. |
| `GET /slips/{id}/panel`                    | Members, legs, odds, lock state, and banner                     | Panel partial. Refreshes on `slip-updated`. |
| `GET /slips/{id}/export`                   | Export card for a `LOCKED` slip                                 | Export partial. Refreshes on `slip-updated`. |
| `GET /slips/{id}/payout`                   | Payout calculator                                               | Optional `?stake=` and `?boost=`; defaults 10 and 0. Payout partial. |
| `GET /slips/{id}/activity`                 | Activity feed                                                   | Activity partial. Refreshes on `slip-updated`. |
| `POST /slips/{id}/members`                 | Add a group member from `user_id`                               | Panel partial. |
| `DELETE /slips/{id}/members/{userID}`      | Remove a member                                                 | Panel partial. |
| `PATCH /slips/{id}`                        | Change `name` or `legs_per_member`                              | Panel partial. |
| `POST /slips/{id}/legs`                    | Add the signed-in member's leg                                  | Panel partial, the `leg-added` partial for a quick add, or the refreshed pair row for `from=board`; `Hx-Trigger: leg-saved` or `tray-updated`. |
| `GET /legs/{id}/edit`                      | Edit form filled from the leg with player autocomplete          | Leg form partial. |
| `PUT /legs/{id}`                           | Save the leg's pick fields                                      | Panel partial with `Hx-Trigger: leg-saved`, or the refreshed pair row with `Hx-Trigger: tray-updated` for `from=board` (plus `board-updated` when the leg moves to another line). |
| `PUT /legs/{id}/result`                    | Grade the leg from an upper-case `result`                       | Panel partial. Any member may grade any leg; an unknown result is 422. |
| `POST /legs/{id}/vote`                     | Vote on someone else's leg with `value` 1, -1, or 0 to clear   | Panel partial. Only slip members vote, not on their own pick; a closed slip is 422. |
| `DELETE /legs/{id}`                        | Delete the leg                                                  | Panel partial; `?from=board` answers the refreshed pair row, `?from=tray` the tray. |
| `POST /slips/{id}/lock`                    | Record the signed-in member's lock in                           | Panel partial with `Hx-Trigger: slip-updated`. |
| `POST /slips/{id}/unlock`                  | Clear every member's lock                                       | Panel partial with `Hx-Trigger: slip-updated`. |
| `PUT /slips/{id}/wagers/me`                | Save or update the signed-in group member's personal wager      | `slip-wager` partial; invalid inputs answer 422. |
| `DELETE /slips/{id}/wagers/me`             | Clear the signed-in group member's personal wager               | Reset `slip-wager` partial. |
| `GET /slips/{id}/events`                   | Live updates for the slip and its week                          | `text/event-stream`; see below. |

The leg form takes `market_type`, `selection`, `team`, `player_name`, `player_id`, `side`, `line`, `odds`, and
`game_id`. The form provides a `<datalist>` of player names (`WeekPlayerNames`) populated from the week's rosters and
props boards. When a player market leg is saved, the handler automatically resolves `player_id` (ESPN athlete ID) and
`team` via `ResolvePlayerMatch`. The game picker lists the week's games that have not kicked off; a past week lists them
all and allows no game. When `selection` is blank, the handler builds the display text from the market fields. Each leg
row on the slip panel includes a "Research &rarr;" link to `/research` when it has a resolved player or game. An
unreadable number is 422 with "Odds need to be a number.", "Line needs to be a number.", or a similar short message.

`?quick=1` turns the form into a quick add: the target becomes the game card's `#quick-add-{gameID}` element, and a
successful `POST` answers with the `leg-added` partial instead of the slip panel.

`POST /legs/{id}/vote` records an up vote (`1`), a down vote (`-1`), or a clear (`0`) on another member's leg. Voting
again with the same value changes nothing, and a change publishes `slip:{id}` without writing an activity entry, so the
actor's panel response and the other browsers' refresh both show the new tally. See [Slips](../concepts/slips.md#votes).

`PUT /slips/{id}/wagers/me` saves or updates the signed-in group member's personal wager on the slip (`stake`, `odds`,
optional `boost`, and optional `book`) and renders the `slip-wager` partial. `DELETE /slips/{id}/wagers/me` removes the
saved wager idempotently and renders the reset form. Any member of the slip's group can log or clear their own wager
regardless of slip status. Unreadable numbers, non-positive stakes, American odds between `-99` and `99` (including
`0`), negative boosts, or book names over 40 characters answer 422. See [Slips](../concepts/slips.md#wagers).

`GET /slips/{id}/events` serves one connection for one slip. It subscribes to `slip:{id}` and `week:{weekID}`, sends
`slip-updated` and `board-updated` signals, pings every 25 seconds, and ends with `bye` at its deadline. The slip page
sets `data-live-url` so the client opens the stream. See [Live updates](../concepts/live-updates.md).

`GET /slips/{id}/export` renders the export card on a `LOCKED` slip: a one-tap combined DraftKings or FanDuel link when
the book can take several picks, per-leg taps otherwise, the manual list, and the copy text. It reads stored odds only
and never fetches or writes. The slip page includes it, and the wrapper refreshes on `slip-updated`, so an unlock hides
the card. The lock and unlock responses carry `Hx-Trigger: slip-updated`, so the person who acts sees the card change
without waiting for the SSE stream. See [Export](../concepts/export.md).

When a week has multiple slips, `GET /slips/{id}` and `GET /slips/{id}/panel` render a horizontal sibling slip switcher
bar in the header with links to each sibling slip and their status summaries (`(Locked)`, `(1/2 Picks)`, `(Won)`).

### Pick board

| Method + path                   | Purpose                                                                  | Response |
| ------------------------------- | ------------------------------------------------------------------------ | -------- |
| `GET /slips/{id}/pick`          | Focused pick board: the game slate, the open game's picks, and the dock   | Full page, no bottom bar |
| `GET /slips/{id}/pick/results`  | `#pick-body` for a search, window, or game change                         | `#pick-body` partial |
| `GET /slips/{id}/pick/slate`    | `#pick-slate` for a live board update                                     | Slate partial |
| `GET /slips/{id}/pick/detail`   | `#pick-detail` for a tab or sort change                                   | `#pick-detail` partial, or the "Pick a game" placeholder |
| `POST /slips/{id}/pick/refresh` | Manual pick-time odds refresh for an `OPEN` slip with upcoming games      | `#pick-body` partial (2m cooldown); 422 when not `OPEN`  |
| `GET /slips/{id}/pick/swap`     | Swap sheet for one unpicked row                                          | Swap partial |
| `GET /slips/{id}/pick/dock`     | The fixed dock's count and combined price                                | Dock partial |
| `GET /slips/{id}/tray`          | The viewer's picks and warnings, loaded into the dock's sheet             | Tray partial |
| `GET /games`                    | 303 to the active group's week                                           | Redirect |
| `GET /games/{id}/props`         | Load one game's props on demand, or render its prop partial               | 200 with `Hx-Trigger: board-updated` when `?slip=`, otherwise a partial |

`GET /slips/{id}/pick` is the hub's pick action. It filters the week's games by the slip's `slate_scope` via
`slips.FilterGamesByScope`, so games outside the scope are omitted from the slate, window chips, and detail drilldowns.
It reads the filtered games, one `OddsForWeek` read, the week's rosters, and the viewer's legs, then hands them to
`internal/board`. `board.Build` folds one row per selection, using the viewer's home-book price when it carries the pick
and the median across books otherwise, and drops games that have kicked off. `board.Slates` turns the same rows into one
card per upcoming game for the slate, `board.StartedCount` counts the games that already kicked off for the empty state,
and `board.Detail` shapes the open game's header, tabs, and the active tab's lines or player cards. A card is marked
unloaded when its game has lines but no prop rows. The handler sets `Page.Wide`, so the header and main use `max-w-6xl`,
and sets the slip's `data-live-url`, so the page opens one stream.

The board's query contract:

| Parameter | Screen        | Meaning |
| --------- | ------------- | ------- |
| `game`    | slate, detail | The open game. Absent shows the slate. An unknown game, a kicked-off game, or a game the window hides is dropped. |
| `cat`     | detail        | The open tab: `lines`, `passing`, `rushing`, `receiving`, `tds`, or `kicking`. `team_totals` folds into `lines`. An unknown value opens `lines` when it has rows, otherwise the first tab that does. |
| `q`       | slate, detail | Search text. Matches a matchup, team, venue, or roster player on the slate, and a row in the open game. |
| `win`     | slate         | Kickoff window: `thursday`, `sun_morning`, `sun_early`, `sun_late`, `sun_night`, or `monday`, in Eastern time. Unknown values fall back to any time. |
| `sort`    | detail        | `player` (default), `odds` (shortest price first), `line` (highest line first), or `matchup` (strongest suggestion signal and softest opposing defense first). Kickoff, category, and unknown values resolve to `player`. |

The slate shows one card per upcoming game, ordered by kickoff, with the matchup, venue, kickoff, spread, total,
moneyline, and either "Explore N props" or "No props yet". A game with no stored odds still gets a card, with no lines
and no props. The open game is highlighted.

The search input debounces at 250 ms and swaps `GET /slips/{id}/pick/results`. Window chips are Thursday, Sun morning
(before noon Eastern), Sun early (before 4), Sun late (before 7), Sun night, and Monday. A chip shows how many rows it
covers and appears only when it has rows. A card is a link with a plain `href` to `/slips/{id}/pick?game=` and an
`hx-get` to the results URL; both swap `#pick-body` and push the full page URL. The results response also carries
`HX-Push-Url` with the full page URL built from the resolved query, so a search writes `q` into the address bar.
`#pick-slate` re-fetches itself from `GET /slips/{id}/pick/slate` on `board-updated`, so the cards follow a prop sync
without touching the open detail.

Inside the open game, the detail has a pinned header with the back link, matchup, kickoff, venue, and a Research link,
then a horizontal tab bar and the sort chips. The tabs are Lines, Passing, Rushing, Receiving, TDs, and Kicking. Team
totals fold into Lines, a tab appears only when it has rows or is open, and each badge counts its rows, so the Lines
badge includes both sides of each line. Tab and sort chips swap `#pick-detail` and push the full page URL, leaving the
slate and its scroll alone. `#pick-detail` re-fetches itself from its resolved tab and sort on `board-updated`, and a
tab or sort response out-of-band syncs the search form's hidden `cat` and `sort`, so a later search or live refresh
keeps the open tab. An unknown category opens Lines when it has rows, otherwise the first tab that does; `sort` resolves
to `player`, `odds`, `line`, or `matchup`, and anything else reads as `player`. The "All games" back link drops `game` and `cat`,
keeps `q` and `win`, and swaps `#pick-body`. A search filters the open game's rows. An unknown game, a kicked-off game,
or a game the window hides falls back to the slate instead of a 404.

`GET /slips/{id}/pick/detail` answers with `#pick-detail`: the open game's header, tabs, sort chips, and the active
tab's rows. The Lines tab shows the game lines first, then team totals grouped by team; a prop tab shows one card per
player with position and team and every market that player has in the tab. Without an open game the route renders the
"Pick a game" placeholder, not the slate.

A side of an unpicked line posts `POST /slips/{id}/legs` with `quick=1&from=board`; the picked side sends
`DELETE /legs/{id}?from=board`. The other side of a line that already holds a pick sends `PUT /legs/{id}` with the new
side's fields, so the pair keeps one leg. All three answer with the same partial: the refreshed `pick-line-group` for
that line, both sides carrying their current picks, plus `Hx-Trigger: tray-updated`. The row swaps with
`hx-target="closest .pick-line-group"` and `hx-swap="outerHTML"`, so only that line changes. A line the board can no
longer rebuild, such as a hand-typed leg with no stored odds, answers one option for that leg so the row still adds,
removes, or switches.

At the limit, a line with neither side picked opens `GET /slips/{id}/pick/swap`, which lists the viewer's unlocked legs;
a choice sends `PUT /legs/{old}` with `from=board` and the new pick's fields. The sheet sits outside the row, so its
form targets the row's `pick-line-...` id instead of `closest`, and the handler answers with the same refreshed pair row
and `Hx-Trigger: tray-updated`. A switch that moves the leg to another line adds `board-updated` to the trigger, so the
line left behind loses its stale delete button.

`GET /slips/{id}/tray` renders the viewer's legs with remove buttons (`DELETE /legs/{id}?from=tray`), progress, the
analyzer warnings that name one of their legs, and one action back to the hub. It loads into the dock's sheet and
refreshes on `tray-updated` and `slip-updated`. `GET /slips/{id}/pick/dock` renders the fixed dock's "N of M" and the
combined price of the viewer's legs, and refreshes on the same two signals. Removing a leg from the tray fires both
`tray-updated` and `board-updated`, so the dock and the board follow.

A game with stored lines but no prop rows offers "Load them" on its slate card and in its open detail. The button calls
`GET /games/{id}/props?slip=`, which runs `SyncGameProps` for that one game and answers 200 with
`Hx-Trigger: board-updated`; `#pick-slate` refetches from `/pick/slate` and `#pick-detail` refetches from its resolved
tab and sort. When the slip is `OPEN` and upcoming games remain in its slate scope, `GET /slips/{id}/pick` runs
`SyncPickLines` (or `SyncPickGame` when an upcoming game is open), and `GET /slips/{id}/pick/results` and
`GET /slips/{id}/pick/detail` run `SyncPickGame` when an upcoming game is selected (`game != ""`). Because `sync_state`
stays fresh for 15 minutes (lines) and 30 minutes (props) on game days (2 hours otherwise), subsequent typing,
filtering, sorting, and `board-updated` reloads read SQLite only. `GET /slips/{id}/pick/slate` and locked or graded
slips never trigger odds syncs. On `OPEN` slips with upcoming games, `pick-slate` and `pick-detail` also show a "Refresh
odds" button that posts `POST /slips/{id}/pick/refresh`, enforces group scope and `OPEN` status (422 if locked), runs
`ForcePickRefresh` with a 2-minute cooldown, and returns `#pick-body`. A week with no stored games says the slate has
not synced. A week with games but no odds says the lines have not synced once, above cards that carry no lines. Once
every game has kicked off, the slate says picks are closed.

`GET /games` uses the old Games link as a 303 to `/groups/{active}/week`, where the week route opens the group's newest
slip. Without an active group it redirects to `/`.

### Game props

`GET /games/{id}/props` without `?slip=` renders the standalone prop partial for one game. It syncs props on demand
(`odds:props:{game}`) and renders team totals and player prop quick-add pills grouped by player for passing, rushing,
receiving, anytime TD, and kicker points, alongside links to game research and each player's research view. Each pill
posts to `POST /slips/{id}/legs` with `quick=1` and the game, market, team, player, side, line, and price prefilled,
swapping the "Added {selection}" partial into that card. The pills appear only when the newest slip in the week is
`OPEN`. With no slip, the card links to `/slips/new`; with a locked slip, it says picks cannot be added. "Custom leg"
opens the same leg form with `game_id` set and `quick=1`.

### Research

| Method + path                          | Purpose                                                              | Response |
| -------------------------------------- | -------------------------------------------------------------------- | -------- |
| `GET /research`                        | Full research page or target partial                                 | Full page or partial |
| `GET /research/games/{id}`             | Game research: filtered injuries and prop player match list          | Partial |
| `GET /research/players/{espnID}`       | Player research: last-5 hit bars, median, mean, and market stats      | Partial |
| `POST /research/matches`               | Save manual player match from `odds_name`, `team`, and `athlete_id`  | Partial (refreshed game research) |

`GET /research` renders the week's game selector and player autocomplete picker. It accepts optional `?game_id=`,
`?player_id=`, `?leg_id=`, `?market=`, and `?line=`. When requested with an `Hx-Request` header and `player_id` or
`game_id`, it delegates directly to the matching partial for `#research-panel`.

`GET /research/games/{id}` syncs injuries on demand (`espn:summary:{game}`) and renders injuries filtered to key
positions (`QB`, `RB`, `WR`, `TE`, `LT`, `OT`, `C`, `CB`, `S`). It also lists sportsbook prop names with their matched
ESPN athlete, and provides a form to manually link unmatched players.

`GET /research/players/{espnID}` syncs game logs on demand (`espn:gamelog:{id}`). It renders last-5 hit bars (green when
over the line, red when under), the last-5 median, the season mean, and total games played. When fresh nflverse data is
cached, it also renders `Snap % (L3)`, `Target share`, `WOPR`, and `Opp Pass Def` / `Opp Rush Def` rank for the active
market. If `market` or `line` is omitted, it resolves them from `?leg_id=` or the player's stored odds.

`POST /research/matches` accepts `odds_name`, `team`, and `athlete_id`. It saves a manual player match (`manual=1`) in
`player_matches`, removes conflicting non-manual matches for that game, backfills `player_id` and `team` on matching
odds and legs, and returns the refreshed game research partial.

### Record Book

| Method + path            | Purpose                                           | Response |
| ------------------------ | ------------------------------------------------- | -------- |
| `GET /record?season=`    | Season Record Book for the active group           | Full page; invalid or non-positive `season` is 400. |

`GET /record` reads the active group from `Scope` and resolves the season in order: explicit `?season=` query parameter,
latest season with `GRADED` slips, latest season with any slip, latest loaded week's season, or the current calendar
year. An unreadable or non-positive `?season=` answers `400 Bad Request`.

The handler loads the group's `GRADED` slips, their legs, participating users (including former members who authored a
leg on a graded slip), and logged wagers for that season, computes `record.Build`, and renders
`web/templates/record.html`. When no group is active or the selected season has no graded slips, it renders an empty
state. Otherwise it shows the season selector, four summary cards (Group record, Units P/L, Win streak, Loss streak),
and the member leaderboard (stacked cards below `sm:` and a table at `sm:+`). See
[Record Book](../concepts/record-book.md).

### Analyzer guide

| Method + path            | Purpose                                                         | Response |
| ------------------------ | --------------------------------------------------------------- | -------- |
| `GET /analyzer/guide`    | Grading & Analyzer Guide backed by `RuleDocs()` and `GradeRubric()` | Full page; optional `?from=` back-link path. |

`GET /analyzer/guide` renders `web/templates/analyzer_guide.html` behind `RequireUser` and `Scope`. It displays the
live `A`–`F` leg and slip grading rubric (`analyzer.GradeRubric()`), pick-board badge descriptions, and every active
warning rule (`analyzer.RuleDocs()`) grouped by category with stable anchors (`#leg-grades`, `#slip-grade`,
`#suggestions`, `#rules`, and `#rule-<id>`). An optional `?from=` query parameter renders a `"← Back"` link when it
passes `safeReturnPath` (starts with `/` and not `//`); external or protocol-relative targets are ignored. See
[Analyzer](../concepts/analyzer.md) and [Analyzer rules](analyzer-rules.md).

### Profile

| Method + path                  | Purpose                                                   | Response |
| ------------------------------ | --------------------------------------------------------- | -------- |
| `GET /me`                      | Profile: home book, plus the group member list             | Full page |
| `POST /me`                     | Save `home_book`; blank means the median price             | 303 to `/me` |
| `POST /users/{id}/active`      | Turn a group member on or off from `active`                | 303 to `/me` |

`POST /users/{id}/active` answers 422 for "You can't turn yourself off." and for a value that is not on or off. A user
outside the active group is 404.

### Bug reports

| Method + path      | Purpose                                                                      | Response |
| ------------------ | ---------------------------------------------------------------------------- | -------- |
| `POST /bug-reports` | Save a bug report (`description`, optional `page_url`, `screen_size`, `user_agent`, and base64 `screenshot` data URL) and optionally file a GitHub issue | 200 `bug-report-success` partial, or 400 `bug-report-error` partial on validation failure. |

`POST /bug-reports` caps request bodies at 2 MiB and accepts `image/jpeg`, `image/png`, or `image/webp` `data:` URLs for
`screenshot`. It persists the report in SQLite before attempting GitHub issue creation, so the user gets a 200
confirmation even when GitHub is unconfigured or unreachable.

## Responses

### Status codes

| Code | When |
| ---- | ---- |
| 200  | Pages, partials, and successful HTMX writes. Redirects and refreshes send 200 with an `HX-*` header. |
| 201  | A created resource for JSON requests, such as adding a recurring schedule. |
| 204  | A successful write with no content returned, such as deleting a schedule. |
| 303  | A plain form redirect or landing redirect, such as create slip, save profile, logout, joining a group, or the one-group landing. |
| 400  | A malformed form body, a bug report validation error, or a failed Google round trip. |
| 401  | Missing or invalid bearer token on `POST /internal/prefill`. |
| 403  | A non-GET request without `HX-Request: true`. |
| 404  | A missing slip, leg, member, or group; a resource in another group; an unknown invite token; or `POST /internal/prefill` when `PREFILL_TOKEN` is empty. |
| 422  | A fixable rule refusal or form error. The body is the short message shown in the form. |
| 500  | An unexpected store or render failure, a signed-in user with no group, or a failed `RunPrefill` pass. |
| 503  | `POST /internal/prefill` in sandbox mode, or `/healthz` when SQLite is unreachable. |

Rule violations from `internal/slips` (`ErrLegLimit`, `ErrKickoffLocked`, `ErrGameOutsideScope`, `ErrLockedIn`,
`ErrLegsNotReady`, and the rest) and schedule rules (`ErrScheduleNameRequired`, `ErrScheduleNameTaken`, invalid scope,
or invalid leg count) answer 422 with a short descriptive message. The form shows it and keeps every pick allowed.

### HTMX headers

| Header        | Meaning |
| ------------- | ------- |
| `HX-Redirect` | Navigate the browser to a new page without swapping an empty body. |
| `HX-Refresh`  | Reload the current page, used after a group switch, invite reset, or leave. |
| `HX-Trigger`  | Fire a client event. `leg-saved` closes the leg form's bottom sheet. `tray-updated` refreshes the pick dock and tray, and `board-updated` refreshes `#pick-slate` and the open game's `#pick-detail`. |

Writes that change the slip return the refreshed panel, so the page reflects the new state without a reload. Partials
listen with `hx-trigger="slip-updated from:body"` for live updates.

### Server-sent events

The stream answers with `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`, and
`X-Accel-Buffering: no`, and flushes every frame.

| Frame                    | Meaning |
| ------------------------ | ------- |
| `event: slip-updated`    | The slip changed; refetch the panel and activity. |
| `event: board-updated`   | The week's games or odds changed; refetch the board. |
| `event: bye`             | The stream hit its deadline; close without retrying, and reopen when the tab becomes visible. |
| `: ping`                 | A comment every 25 seconds to keep proxies from closing the connection. |

Every event carries `data: {}`. Events are signals only, never slip data, so the page fetches its own view.

## Related

- [Slips](../concepts/slips.md): members, legs, and the editing and locking rules behind the 422s.
- [Live updates](../concepts/live-updates.md): the hub, stream deadlines, and the `EventSource` client.
- [Access](../concepts/access.md): sign-in, cookies, and the middleware.
- [Groups](../concepts/groups.md): invites, the active group, and what it scopes.
