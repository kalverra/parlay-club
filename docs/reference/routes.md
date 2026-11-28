---
id: routes
title: Routes
type: reference
summary: Every HTTP route the server serves, the middleware in front of them, and the status codes, HTMX headers, and SSE frames handlers return.
tags: [routes, http, handlers, middleware, htmx, sse, props, prefill, research, schedules, wagers, record, analyzer, guide, docs, glossary, themes, manifest, service-worker, pwa]
budget:
  max_lines: 620
  reason: Every HTTP route, the middleware around it, and its response behavior live in one reference table.
---

# Routes

## Summary

One `http.ServeMux` holds every route, using Go 1.22 method patterns. Handlers stay thin: they parse the request, call
`internal/slips` or the database store, and render a page or partial. Routes still to be built are listed in the
[architecture plan](../../plan/architecture.md).

## Middleware

All routes except `/healthz`, `/readyz`, `/static/`, `/manifest.webmanifest`, `/sw.js`, `/screenshots/{token}.jpg`,
`/login`, `POST /logout`, the Google sign-in routes, the dev sign-in routes, `POST /internal/prefill`, and
`POST /internal/reminders` need a signed-in user. Those public/internal routes are registered outside session checks
(the internal posts also sit on an outer mux outside `CSRF` and use bearer auth instead). Everything else runs through
three wrappers, outermost first. See [Access](../concepts/access.md) for cookies and sign-in.

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
| `GET /static/{path...}`             | Embedded CSS, JS, icons, and other assets                               | One-year cache. A missing or escaping path is 404. |
| `GET /manifest.webmanifest`         | Web app manifest for home-screen install                                | `application/manifest+json`, `no-cache`. Icon URLs carry the static content hash, and the list includes 192, 512, and a maskable 512. |
| `GET /sw.js`                        | Service worker for install and Web Push                                 | `text/javascript`, `no-cache`, `nosniff`. Has `install`, `push`, and `notificationclick` listeners and no `fetch` handler, so it never intercepts a page load. |
| `GET /screenshots/{token}.jpg`      | Capability URL for a bug report screenshot                              | Registered as `GET /screenshots/{file}` and checks the `.jpg` suffix. One-year immutable cache with `nosniff`; unknown token, missing image, or non-`.jpg` suffix is 404. |
| `GET /login`                        | Sign-in page                                                            | Shows the Google button, plus a dev link when Google is off. |
| `POST /logout`                      | Clear the session and active-group cookies                              | 303 to `/login`. |
| `GET /auth/google/start`            | Start the Google round trip                                             | Registered only when a Google client is configured. Redirects to Google. |
| `GET /auth/google/callback`         | Finish Google sign-in                                                   | Bad `state`, code exchange, or identity answers 400 with the sign-in page. |
| `GET /dev/login`, `POST /dev/login` | Local sign-in form and submit                                           | Registered only when `APP_ENV=dev` and `GOOGLE_CLIENT_ID` is empty. |
| `POST /internal/prefill`            | Run the per-game-day pre-fill sweep synchronously                       | Warms lines and Parlay-only props for today's upcoming non-`FINAL` games in ET (or all upcoming week games on a non-game day) and records `prefill:gameday`. Outside cookie/CSRF middleware; requires `Authorization: Bearer <PREFILL_TOKEN>` (constant-time compare). Returns 404 when `PREFILL_TOKEN` is empty, 401 on missing/invalid token, 503 in sandbox mode, 200 JSON `PrefillReport` on success, or 500 JSON on failure. |
| `POST /internal/reminders`          | Fire one slip's kickoff reminder                                        | Reads `?slip=` and `?start=` (unix), then rechecks the slip, its start, the 30-to-10-minute fire window, and each member's next step from SQLite. Outside cookie/CSRF middleware; requires `Authorization: Bearer <REMINDER_TOKEN>` (constant-time compare). Returns 404 when the token is empty, 401 on missing/invalid token, 400 on a bad slip or start, 200 JSON `reminders.Report` for a send or a skip, or 500 JSON on a store error so Cloud Tasks retries. |

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
| `GET /groups/{id}/week`   | Make the group active, ensure scheduled slips exist, and render the per-group overview landing page |
| `GET /games`              | 303 to the active group's week; the pick board owns the week                                     |
| `GET /research`           | Research tab: slate-wide bet explorer, targeted game and player search, and research sheets       |
| `GET /record`             | Season Record Book for the active group (optional `?season=`)                                    |
| `GET /docs`               | Docs hub: quick jumps, "How systems work" explainers, and common questions                       |
| `GET /docs/glossary`      | Searchable glossary of betting and app terms (optional `?q=`)                                    |
| `GET /docs/grading`       | Grading & Analyzer Guide inside Docs; same page as `/analyzer/guide`                             |
| `GET /docs/evals`         | Grading Results inside Docs; same page as `/analyzer/results`                                    |
| `GET /analyzer/guide`     | Grading & Analyzer Guide: live `A`–`F` rubric, pick-board badges, and every warning rule         |
| `GET /analyzer/results`   | Grading Results: global or market-wide `A`–`F` calibration, rubric diffs, weekly progression, and factors |

The bottom bar has five destinations: Groups (`/`), This week (`/groups/{active}/week`), Research (`/research`), Record
Book (`/record`), and Docs (`/docs`). Research is the bet explorer; the hub links to the pick board, and pick rows link
to Research.

`GET /{$}` lists every group oldest first and carries `data-refresh-on-visible`, so the browser refetches the list when
the tab becomes visible instead of holding a stream. A person in one group gets a 303 to `/groups/{id}/week`.

`GET /groups/{id}/week` answers 404 for a group the person does not belong to, sets `pc_group`, and calls
`slips.EnsureWeekSlips` when an upcoming week exists to auto-generate slips for active recurring schedules. It always
renders the per-group overview landing page (`week.html`): a hero with conditional spotlight cards for active or
upcoming slips (ordered by urgency and deadline) or a quiet-state CTA to `/slips/new`, followed by the group record
snapshot, recent slips, recurring schedules, and members strip. It uses the same group-switch seam as a slip link from
another group.

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
| `GET /slips/new`                           | Unified one-off / recurring create-slip page                    | Lists `UpcomingWeeks`; optional `?week=`, `?mode=recurring`, and `?slate_scope=` (or `?scope=`). Full page. |
| `GET /slips/new/games`                     | `#slip-create-games` partial for an upcoming week               | Query `?week_id=` (or `?week=`); 404 when not an upcoming week. |
| `POST /slips`                              | Create a one-off `custom` slip (`mode=one_off` with upcoming `week_id` and `game_id`s) or a recurring schedule (`mode=recurring` with repeating `slate_scope`) | 303 to `/slips/{id}` (or `/groups/{id}/week` for recurring with no upcoming match); 422 on validation errors. |
| `GET /slips/{id}`                          | Full slip page                                                  | Full page, with the payout form at the default stake. |
| `GET /slips/{id}/pick`                     | Focused pick board: the game slate, the open game's picks, and the dock | Full page; no bottom bar. |
| `GET /slips/{id}/pick/results`             | `#pick-body` for a search, window, or game change                | Optional `?q=`, `?group=`, `?cat=`, `?win=`, `?sort=`, and `?game=`; `#pick-body` partial. |
| `GET /slips/{id}/pick/detail`              | `#pick-detail` for a group, category, or sort change             | Optional `?q=`, `?group=`, `?cat=`, `?sort=`, `?win=`, and `?game=`; `#pick-detail` partial, or the "Pick a game" placeholder without an open game. |
| `POST /slips/{id}/pick/refresh`            | Force a pick-time odds refresh on an `OPEN` slip                 | Optional `game`, `q`, `group`, `cat`, `win`, `sort`; `#pick-body` partial (2m cooldown). Returns 422 when the slip is not `OPEN`. |
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
| `POST /slips/{id}/legs`                    | Add the signed-in member's leg                                  | Panel partial, `leg-added` for quick add, refreshed pair row for `from=board` (`Hx-Trigger: tray-updated`), OOB `#pick-detail` for `from=detail` (`Hx-Trigger: tray-updated, slip-updated`), `#pick-body` for `from=pick` (`Hx-Trigger: tray-updated, slip-updated`), or `HX-Redirect` to `return` (default `/research`) for `from=research` (`Hx-Trigger: slip-updated`). |
| `POST /slips/{id}/legs/batch`              | Atomically add a combo of legs for the signed-in member         | Panel partial (`Hx-Trigger: slip-updated`), `#pick-body` for `from=board` or `from=pick` (`Hx-Trigger: tray-updated, slip-updated`), or `pick-dock` for `from=dock` (`Hx-Trigger: tray-updated, board-updated, slip-updated`). |
| `GET /legs/{id}/edit`                      | Edit form filled from the leg with player autocomplete          | Leg form partial. |
| `PUT /legs/{id}`                           | Save the leg's pick fields                                      | Panel partial with `Hx-Trigger: leg-saved`, refreshed pair row with `Hx-Trigger: tray-updated` for `from=board` (plus `board-updated` when moving lines), or OOB `#pick-detail` with `Hx-Trigger: tray-updated, slip-updated` for `from=detail`. |
| `PUT /legs/{id}/result`                    | Grade the leg from an upper-case `result`                       | Panel partial. Any member may grade any leg; an unknown result is 422. |
| `POST /legs/{id}/vote`                     | Vote on someone else's leg with `value` 1, -1, or 0 to clear   | Panel partial. Only slip members vote, not on their own pick; a closed slip is 422. |
| `DELETE /legs/{id}`                        | Delete the leg                                                  | Panel partial; `?from=board` answers refreshed pair row, `?from=detail` answers OOB `#pick-detail` (`Hx-Trigger: tray-updated, slip-updated`), `?from=tray` answers the tray. |
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

`POST /slips/{id}/legs/batch` accepts indexed leg fields (`legs[0][game_id]`, `legs[0][market_type]`, `legs[0][team]`,
`legs[0][player_name]`, `legs[0][player_id]`, `legs[0][side]`, `legs[0][line]`, `legs[0][odds]`, and optional
`legs[0][selection]`). It validates that the slip is `OPEN` and the viewer is unlocked, skips any selection already on
the viewer's slip (`db.SameSelection`), checks remaining `legs_per_member` capacity, inserts all new legs in one SQLite
transaction (`slips.Service.AddLegs`), writes activity entries, and publishes `slip:{id}`. Unreadable fields, empty
batches, locked slips, or insufficient capacity answer 422.

`POST /legs/{id}/vote` records an up vote (`1`), a down vote (`-1`), or a clear (`0`) on another member's leg. Voting
again with the same value changes nothing, and a change publishes `slip:{id}` without writing an activity entry, so the
actor's panel response and the other browsers' refresh both show the new tally. See [Slips](../concepts/slips.md#votes).

`PUT /slips/{id}/wagers/me` saves or updates the signed-in group member's personal wager on the slip (`stake`, `odds`,
optional `boost`, and optional `book`) and renders the `slip-wager` partial. `DELETE /slips/{id}/wagers/me` removes the
saved wager idempotently and renders the reset form. Any member of the slip's group can log or clear their own wager
regardless of slip status. Unreadable numbers, non-positive stakes, American odds between `-99` and `99` (including
`0`), negative boosts, or book names over 40 characters answer 422. See [Slips](../concepts/slips.md#wagers).

`GET /slips/{id}/events` serves one connection for one slip. If the slip is already `GRADED`, it answers immediately
with `event: bye` and returns without subscribing. Otherwise it subscribes to `slip:{id}` and `week:{weekID}`, sends
`slip-updated` and `board-updated` signals, pings every 25 seconds, and ends with `bye` at its deadline. The slip page
sets `data-live-url` for `OPEN` and `LOCKED` slips so the client opens the stream, and omits it for `GRADED` slips. See
[Live updates](../concepts/live-updates.md).

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
| `GET /slips/{id}/pick/detail`   | `#pick-detail` for a group, category, or sort change                      | `#pick-detail` partial, or the "Pick a game" placeholder |
| `POST /slips/{id}/pick/refresh` | Manual pick-time odds refresh for an `OPEN` slip with upcoming games      | `#pick-body` partial (2m cooldown); 422 when not `OPEN`  |
| `GET /slips/{id}/pick/swap`     | Swap sheet for one unpicked row                                          | Swap partial |
| `GET /slips/{id}/pick/dock`     | The fixed dock's count and combined price                                | Dock partial |
| `GET /slips/{id}/tray`          | The viewer's picks and warnings, loaded into the dock's sheet             | Tray partial |
| `GET /games`                    | 303 to the active group's week                                           | Redirect |
| `GET /games/{id}/props`         | Load one game's props on demand, or render its prop partial               | 200 with `Hx-Trigger: board-updated` when `?slip=`, otherwise a partial |

`GET /slips/{id}/pick` is the hub's pick action. It filters the week's games for the slip via
`slips.FilterGamesForSlip`, so games outside the slip's scope or `slip_games` are omitted from the slate, window chips,
and detail drilldowns. It reads the filtered games, one `OddsForWeek` read, the week's rosters, and the viewer's legs,
then hands them to `internal/board`. `board.BuildDualBook` filters to selections quoted by both DraftKings and FanDuel,
folds one row per selection using the viewer's home book (`draftkings` or `fanduel`, or FanDuel when unset) with no
median line averaging, and drops games that have kicked off. `board.Slates` turns
the same rows into one card per upcoming game for the slate, `board.StartedCount` counts the games that already kicked
off for the empty state, and `board.Detail` shapes the open game's header, tabs, and the active tab's lines or player
cards. A card is marked unloaded when its game has lines but no prop rows. The handler sets `Page.Wide`, so the header
and main use `max-w-6xl`, and sets `data-live-url` for `OPEN` and `LOCKED` slips so the page opens one stream, omitting
it for `GRADED` slips.

The board's query contract:

| Parameter | Screen        | Meaning |
| --------- | ------------- | ------- |
| `game`    | slate, detail | The open game. Absent shows the slate. An unknown game, a kicked-off game, or a game the window hides is dropped. |
| `group`   | detail        | Grouping mode: `category` (default), `player`, or `grade`. Empty or unknown values resolve to `category`. |
| `cat`     | detail        | Optional category filter: empty/`All` (default, shows all non-empty sections), `lines`, `passing`, `rushing`, `receiving`, `tds`, or `kicking`. `team_totals` folds into `lines`. |
| `q`       | slate, detail | Search text. Matches a matchup, team, venue, or roster player on the slate, and a row in the open game. |
| `win`     | slate         | Kickoff window: `thursday`, `sun_morning`, `sun_early`, `sun_late`, `sun_night`, or `monday`, in Eastern time. Unknown values fall back to any time. |
| `sort`    | detail        | `player` (default), `grade` (highest score first), `odds` (shortest price first), `line` (highest line first), or `matchup` (strongest suggestion signal and softest opposing defense first). Unknown values resolve to `player`. |

The slate shows one card per upcoming game, ordered by kickoff, with the matchup, venue, kickoff, spread, total,
moneyline, an `"N A-grade"` pill when that game has unpicked A-grade legs, and either "Explore N props" or "No props
yet". A game with no stored odds still gets a card, with no lines and no props. The open game is highlighted.

The search input debounces at 250 ms and swaps `GET /slips/{id}/pick/results`. Window chips are Thursday, Sun morning
(before noon Eastern), Sun early (before 4), Sun late (before 7), Sun night, and Monday. A chip shows how many rows it
covers and appears only when it has rows. A card is a link with a plain `href` to `/slips/{id}/pick?game=` and an
`hx-get` to the results URL; both swap `#pick-body` and push the full page URL. The results response also carries
`HX-Push-Url` with the full page URL built from the resolved query, so a search writes `q` into the address bar.
`#pick-slate` re-fetches itself from `GET /slips/{id}/pick/slate` on `board-updated`, so the cards follow a prop sync
without touching the open detail.

Inside the open game, the detail has a pinned header with the back link, matchup, kickoff, venue, a Research link, and a
horizontal category chip bar (`All` plus non-empty categories). Below—when the slip is `OPEN` and the viewer has open
slots—sits the `"Top plays in this game"` strip (up to 3 A-grade legs and 1 same-game A-grade combo), followed by the
`Group by` switcher (`Category` default, `Player`, `Grade`) and sort chips (`Player`, `Grade`, `Odds`, `Line`,
`Matchup`). Category, `Group by`, and sort chips swap `#pick-detail` and push the full page URL, leaving the slate and
its scroll alone. `#pick-detail` re-fetches itself on `board-updated`, and a `#pick-detail` swap out-of-band syncs the
search form's hidden `group`, `cat`, and `sort` (`#pick-group`, `#pick-cat`, `#pick-sort`). The "All games" back link
drops `game` and `cat`, keeps `q` and `win`, and swaps `#pick-body`.

`GET /slips/{id}/pick/detail` answers with `#pick-detail`: the open game's header, category chips, `Group by` and sort
chips, and grouped `Sections` (`Lines` or `Players` in `Category`/`Player` modes, or single-side `Options` bucketed
`Grade A`–`F` in `Grade` mode). `board.ScoreRows` scores every gradable row from cached SQLite signals (even at
`remainingSlots == 0`) so options render sibling `Grade A–F` `<details>` popovers outside the pick `<form>`. Without
an open game the route renders the "Pick a game" placeholder, not the slate.

In `Category` and `Player` modes, options use `from=board` (`POST /slips/{id}/legs` with `quick=1&from=board`,
`DELETE /legs/{id}?from=board`, `PUT /legs/{id}` with `from=board`) and swap `closest .pick-line-group` with the
refreshed pair row (including pre-pick grade pills via `pickPairRow`) plus `Hx-Trigger: tray-updated`. In `Grade` mode,
options use `from=detail` (carrying `#pick-q, #pick-cat, #pick-win, #pick-sort, #pick-group, #pick-game`) and return an
out-of-band `#pick-detail` swap with `Hx-Trigger: tray-updated, slip-updated`.

At the limit, an unpicked option opens `GET /slips/{id}/pick/swap`, which lists unlocked legs; choosing one sends
`PUT /legs/{old}` with `from=board` (targeting `pick-line-...`) or `from=detail` in `Grade` mode (targeting
`#pick-detail`). A switch across lines in `from=board` adds `board-updated` to `Hx-Trigger`.

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

`GET /games` uses the old Games link as a 303 to `/groups/{active}/week`, where the week route renders the group's
overview landing page. Without an active group it redirects to `/`.

### Game props

`GET /games/{id}/props` without `?slip=` renders the standalone prop partial for one game. It syncs props on demand
(`odds:props:{game}`) and renders team totals and player prop quick-add pills grouped by player for passing, rushing,
receiving, anytime TD, and kicker points, alongside links to game research and each player's research view. Each pill
posts to `POST /slips/{id}/legs` with `quick=1` and the game, market, team, player, side, line, and price prefilled,
swapping the "Added {selection}" partial into that card. The pills appear only when the newest slip in the week is
`OPEN`. With no slip, the card links to `/slips/new`; with a locked slip, it says picks cannot be added. "Custom leg"
opens the same leg form with `game_id` set and `quick=1`.

### Research

| Method + path                     | Purpose                                                                    | Response |
| --------------------------------- | -------------------------------------------------------------------------- | -------- |
| `GET /research`                   | Research tab: the explorer plus a targeted game or player sheet            | Full page, or an HTMX sheet partial |
| `GET /research/explore`           | `#research-explore` for a search, chip, or sort change                     | Explorer partial with `HX-Push-Url` |
| `GET /research/bet`               | Per-bet research: cached prices, grade, and the research for the bet's market | Full page; 404 for a selection not on the cached slate |
| `GET /research/games/{id}`        | Game research: filtered injuries and prop player match list                | Partial |
| `GET /research/players/{espnID}`  | Player research: matchup strip, L5 and opp-defense charts, hit rates, usage | Partial |
| `POST /research/games/{id}/props` | Load one game's props from an explicit tap                                 | Explorer partial with `HX-Push-Url` |
| `POST /research/matches`          | Save manual player match from `odds_name`, `team`, and `athlete_id`        | Partial (refreshed game research) |

`GET /research` renders the week's game and player pickers, the explorer, and, when targeted, the deeper sheet. It
accepts `?q=`, `?game_id=`, `?player_id=`, `?cat=`, `?grade=`, `?signal=`, `?sort=`, and `?limit=`, plus the older
`?leg_id=`, `?market=`, `?line=`, and `?player=` deep links (`?player=` resolves a roster athlete by normalized name).
Requested with `Hx-Request` and a `player_id` but no `game_id`, it renders the player sheet partial; with a `game_id`,
it renders the game sheet partial.

The explorer reads one week of cached games, odds, and rosters (`GamesForWeek`, `OddsForWeek`, `RosterForTeams`), folds
them with `board.BuildDualBook` (requiring both DraftKings and FanDuel quotes, with no median line averaging), and scores each row with `analyzer.ScoreLeg` before `board.Explore` filters and sorts. It never
syncs odds, injuries, or game logs, so searching, filtering, and sorting spend no Odds API or Parlay API credits.

| Parameter | Meaning |
| --------- | ------- |
| `q`       | Case- and accent-insensitive search over player, team, matchup, and market. |
| `game_id`, `player_id` | Target one game or one ESPN athlete. Unknown ids match nothing. |
| `cat`     | Bet type: `lines`, `team_totals`, `passing`, `rushing`, `receiving`, `tds`, or `kicking`. An unknown value filters nothing. |
| `grade`   | Grade band: `a` (A only), `ab` (A and B), or `c` (C or better). An unknown value filters nothing. |
| `signal`  | Signal: `smash`, `volume`, `role_bump`, or `fits_slip`. An unknown value filters nothing. |
| `sort`    | `grade` (default, highest score then best price), `matchup` (strongest signal and softest opposing EPA rank), `odds` (shortest price), `line` (highest line), `kickoff` (earliest), or `player` (alphabetical). An unknown value reads as `grade`. |
| `limit`   | Page size, default 50. Show more adds 50. |

Each chip shows a count tallied as if only that chip's own filter were cleared. Each card shows the selection, market,
odds with implied chance, grade with its strongest factor, any signal pill, matchup, kickoff, and links to the per-bet
page and both sheets. Its `Research bet →` link carries `db.SameSelection` identity (`game_id`, `market`, `side`,
`team`, `player_id`, `player_name`) plus the explorer URL as `from`, never line or price, so shared links survive drift.

The search input debounces at 300 ms. A search, chip, or sort calls `GET /research/explore`, swaps `#research-explore`
with `outerHTML`, and sets `HX-Push-Url` to the matching `/research?...` URL. Empty states name why: no stored games, no
lines, every game started, or nothing matches. A targeted game with lines but no props offers `Load props`, which posts
`POST /research/games/{id}/props`, runs `SyncGameProps` for that one game with the usual credit floors, and answers the
refreshed explorer partial with `HX-Push-Url`. It is the only paid research route; a non-HTMX post is refused by the
CSRF guard before it reaches the handler.

`GET /research/bet` matches the query's selection to one cached row with `db.SameSelection`, so it ignores `line` and
`odds`, and answers 404 with a short reason when the game is unknown, has kicked off, or the selection is not on the
board. The page shows the summary with grade, odds, and implied chance; cached home-book, consensus, and best prices
with a de-vigged edge when both sides are cached; the full analyzer breakdown; and the research for its market. Player
props reuse the player sheet scoped to the bet's market and line; game markets (`SPREAD`, `ML`, `TOTAL`, `TEAM_TOTAL`)
render `TeamBetContextView` with nfelo's model spread and win chances against the market, both teams' EPA ranks, venue,
weather, and key injuries. Up to five related bets follow, same player first. `?from=` restores the explorer on the back
link.

`+ Add to slip` appears for each `OPEN` slip whose next step is `pick` and whose slate includes the game. Each button
posts `POST /slips/{id}/legs` with `from=research` and the bet page as `return`; an optional `?slip_id=` narrows it.

`GET /research/games/{id}` reads cached injuries and rosters and renders injuries filtered to key positions (`QB`, `RB`,
`WR`, `TE`, `LT`, `OT`, `C`, `CB`, `S`), plus sportsbook prop names against their matched athlete.
`POST /research/matches` saves a manual match from `odds_name`, `team`, and `athlete_id`, removes conflicting non-manual
matches, backfills `player_id` and `team` on matching odds and legs, and returns the refreshed game research partial.

`GET /research/players/{espnID}` reads the cached game log and renders the matchup strip, last-5 and season hit rates,
the last-5 range and median, and the opposing-defense chart. Both charts share a dotted O/U line when available, with
per-bar `Def #N` and `vs POS #N` badges, and fresh nflverse data adds L3 usage tiles. If `market` is omitted it resolves
from `?leg_id=` or stored odds; market tabs use each market's own stored O/U line unless `?line=` is set.

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

`GET /analyzer/guide` renders the Grading & Analyzer Guide behind `RequireUser` and `Scope`, with an optional `?from=`
back-link path. `GET /docs/grading` serves the same page as an alias inside Docs.

The page renders `web/templates/analyzer_guide.html`. It displays the active grading engine badge (`v2`), the four v2
grading pillars and weights (`analyzer.DefaultV2Config`), uncertainty badge rules, `A`–`F` letter bands and legacy v1
rubric reference (`analyzer.GradeRubric()`), pick-board badge descriptions, and every active warning rule
(`analyzer.RuleDocs()`) grouped by category with stable anchors (`#pillars`, `#leg-grades`, `#slip-grade`,
`#suggestions`, `#rules`, and `#rule-<id>`).

An optional `?from=` query parameter renders a `"← Back"` link when it passes `safeReturnPath`, which accepts only a
relative path: one leading `/`, no `//`, no backslash or control character, and no `scheme:` before the first path
slash. Absolute URLs are rejected. See [Analyzer](../concepts/analyzer.md) and [Analyzer rules](analyzer-rules.md).

### Analyzer results

`GET /analyzer/results` renders global or market-wide Grading Results behind `RequireUser`; `GET /docs/evals` serves the
same page as an alias. Invalid `?season=` or `?source=` answers `400 Bad Request`. The handler renders
`web/templates/analyzer_results.html` and reads from `eval.Cache` keyed by `(mode, 0, season)`. In `slips` mode, it
aggregates settled slips across all groups and invalidates whenever `SetLegResult` or `AutoGradeLegs` commits a
settlement change for any group. In `market` mode, it replays deduplicated pre-kickoff consensus lines across `FINAL`
games and invalidates via `InvalidateMarket` when `syncScoreboard` records a newly `FINAL` game or `syncNFLverse`
commits changes. Sandbox stage transitions or resets clear the whole cache.

Query parameters:

- `?source=`: `slips` (default, `User slips` across all groups) or `market` (`Market-wide` pre-kickoff consensus lines
  on `FINAL` games). Invalid values return `400 Bad Request`. Preserved on all season and engine filter URLs. In
  `market` mode, `Slip Calibration (A-F)`, `SGP Correlation Lift`, and `Slip Factor Attribution` are hidden (replaced
  with a note linking back to `User slips`), and `Pre-Kickoff Data Coverage` adds `Sharp benchmarks` and
  `nflverse L5 fallback` tiles.
- `?season=`: `all` (or `0`) for all seasons, or a positive season year. Defaults to the latest season with graded slips
  across all groups (`GradedSeasons`) in `slips` mode, or the latest season with `FINAL` games (`FinalSeasons`) in
  `market` mode, falling back to `Seasons` or `all`. Unreadable or non-positive values return `400 Bad Request`.
- `?engine=`: selects which registered `GradingEngine` to inspect (`v2` or `v1`, defaulting to `ActiveEngine.Name()`).
  Legacy `?rubric=` is still accepted as a fallback alias when `?engine=` is omitted.
- `?from=`: optional safe return path validated by `safeReturnPath`, preserved on source/season/engine pills and the
  docs switcher's links. See [Analyzer](../concepts/analyzer.md).

### Docs hub and glossary

| Method + path        | Purpose                                                                         | Response |
| -------------------- | ------------------------------------------------------------------------------- | -------- |
| `GET /docs`          | Docs hub: quick jumps, seven "How systems work" explainers, and common questions | Full page; optional `?from=` back-link path. |
| `GET /docs/glossary` | Searchable glossary of betting and app terms                                    | Full page; optional `?q=` search and `?from=` back-link path. |

Both routes render behind `RequireUser` and `Scope`, set `page.Tab = TabDocs`, and share a docs section switcher
(`Overview`, `Glossary`, `Grading & Rules`, `Eval Results`) rendered from the `docs-nav` partial. The switcher forwards
`?from=` to every destination. The `Grading & Rules` and `Eval Results` sections link to `/docs/grading` and
`/docs/evals`, the aliases of the two analyzer screens above; those screens render the same switcher and Docs tab with
their own section marked current. An optional `?from=` renders a `"← Back"` link when it passes `safeReturnPath`, the
same check the analyzer screens use.

`GET /docs` builds its content from static Go structs in `internal/web/docs.go` and reads no database. Sections carry
stable anchors `#sys-grading`, `#sys-odds`, `#sys-badges`, `#sys-live`, `#sys-autograde`, `#sys-sources`, and
`#sys-export`; other screens deep-link to them, so the IDs stay fixed.

`GET /docs/glossary` filters a static `[]GlossaryEntry` slice in memory by name, category, summary, or example. Each
entry renders at `#term-<slug>`, such as `#term-implied-prob` or `#term-epa`. Filtering runs no database query and
fetches nothing. See [Docs hub](../concepts/docs-hub.md).

### Profile

| Method + path                  | Purpose                                                   | Response |
| ------------------------------ | --------------------------------------------------------- | -------- |
| `GET /me`                      | Profile: home book, the group member list, and the per-device notifications card | Full page |
| `POST /me`                     | Save `home_book` (`draftkings`, `fanduel`, or blank for No preference); 422 on other values | 303 to `/me` |
| `POST /users/{id}/active`      | Turn a group member on or off from `active`                | 303 to `/me` |
| `POST /push/subscription`      | Store or reassign this device's Web Push subscription (`endpoint`, `keys.p256dh`, `keys.auth`) | 204; 400 for bad JSON or a refused endpoint; 404 when VAPID is off |
| `DELETE /push/subscription`    | Remove this device's endpoint when it belongs to the caller (`endpoint`) | 204; 404 when VAPID is off |
| `POST /push/test`              | Send the test notification to the caller's own devices     | 204, even with no stored device; 404 when VAPID is off |

`POST /users/{id}/active` answers 422 for "You can't turn yourself off." and for a value that is not on or off. A user
outside the active group is 404.

The three push routes require `HX-Request: true` like every other write, and an expired HTMX session gets
`HX-Redirect: /login`. The endpoint must be `https` on a known browser push host, and each person keeps at most 10; an
11th replaces the oldest. See [Notifications](../concepts/notifications.md) and [Push setup](../runbooks/push-setup.md).

### Bug reports

| Method + path      | Purpose                                                                      | Response |
| ------------------ | ---------------------------------------------------------------------------- | -------- |
| `POST /bug-reports` | Save a bug report (`description`, optional `page_url`, `screen_size`, `user_agent`, and base64 `screenshot` data URL) and optionally file a GitHub issue | 200 `bug-report-success` partial, or 400 `bug-report-error` partial on validation failure. |

`POST /bug-reports` caps request bodies at 2 MiB and accepts `image/jpeg`, `image/png`, or `image/webp` `data:` URLs for
`screenshot`. It persists the report in SQLite before attempting GitHub issue creation, so the user gets a 200
confirmation even when GitHub is unconfigured or unreachable.

## Themes

### `pc_theme` cookie

Every full-page handler reads the `pc_theme` cookie and sets `Page.Theme` before rendering `web/templates/layout.html`.
Handlers that know the signed-in user call `web.ThemeForRequest(r, user.Name)`; sign-in pages call
`web.ThemeFromRequest(r)`. The cookie is an unsigned UI preference validated against an allowlist in
`internal/web/theme.go`:

| `pc_theme` value | Label | `color-scheme` | `theme-color` | Notes |
| ---------------- | ----- | -------------- | ------------- | ----- |
| `paper-turf` | Paper & Turf | `dark` | `#0f1e16` | Default. Deep stadium felt chrome with cream ticket cards. |
| `redzone` | RedZone | `dark` | `#090a0c` | Obsidian broadcast dark with turf green and gold accents. |
| `almanac` | 90s Almanac | `light` | `#fff8e7` | Warm cream sports almanac with crisp white cards and ink borders. |
| `connor-mode` | Connor Mode | `light` | `#e40303` | Temporary joke theme. Full-saturation pride flags, parade-poster type, and unicorns. |

Missing or unrecognized cookie values fall back to `paper-turf`. `layout.html` renders
`data-theme="{{.ActiveTheme.ID}}"` on `<html>` along with `<meta name="color-scheme">` and `<meta name="theme-color">`,
so SSR pages paint with the chosen theme immediately and without flicker.

<!-- BEGIN Connor Mode -->
Temporary themes register themselves from their own file. Connor Mode is a temporary joke theme. A page for a person
whose first name is `Connor` falls back to `connor-mode` instead. That personal default lives in `connorPersonalTheme`
(`internal/web/connor_mode.go`) and re-applies on every request, so it is not a one-time choice. A `pc_theme` cookie
that predates Connor Mode is ignored for Connor unless the browser also carries the `pc_theme_chosen=1` marker, which
the theme switcher writes. That keeps an old saved theme from hiding his default while still letting him switch away on
purpose. Everyone else keeps their existing `pc_theme` preference. Run `scripts/remove-connor-mode.sh` to delete the
theme, its tests, and its marked code blocks.
<!-- END Connor Mode -->

Clicking `[data-theme-cycle]` in the header cycles through the theme catalog in `web/static/app.js`, updates
`document.documentElement.dataset.theme` and the meta tags in place, and writes `pc_theme=<id>` with
`Path=/; Max-Age=31536000; SameSite=Lax` (adding `; Secure` over HTTPS) with no server round-trip.

### Semantic theme tokens

`web/tailwind/input.css` defines Tailwind v4 `@theme` color and font tokens backed by `[data-theme="..."]` CSS custom
properties, compiled into `web/static/app.css` via `go generate ./...`:

- **Page and chrome (`bg-page`, `text-page-ink`, `text-page-muted`, `bg-chrome`, `text-chrome-ink`, `text-chrome-muted`,
  `border-chrome-border`):** used on `<body>`, the sticky header, the bottom nav, and text sitting directly on the page
  background.
- **Card and surface (`bg-surface`, `bg-surface-2`, `text-ink`, `text-ink-muted`, `text-ink-subtle`, `border-line`,
  `border-line-strong`):** used inside cards, dialogs, tables, and form inputs. Separating page/chrome tokens from
  surface tokens lets `paper-turf` pair dark felt chrome with light ticket cards without template conditionals.
- **Accent and status (`accent`, `positive`, `warning`, `danger`, each with `-soft` and `-ink` variants):** used for
  actions, focus rings, callouts, and status pills.
- **Component and Go-emitted classes:** `.ui-card`, `.ui-slip-card`, `.ui-next-card`, `.ui-odds`, `.ui-stamp`,
  `.ui-heads-up`, `.badge-*`, `.grade-*`, `.sev-*`, `.bar-*`, and `.trend-*`.

Templates under `web/templates/` and view-model helpers in `internal/web/` use these semantic classes rather than raw
`slate-*` or `sky-*` palette utilities (except per-user `db.AvatarColor` swatches).

## Responses

### Status codes

| Code | When |
| ---- | ---- |
| 200  | Pages, partials, and successful HTMX writes. Redirects and refreshes send 200 with an `HX-*` header. |
| 201  | A created resource for JSON requests, such as adding a recurring schedule. |
| 204  | A successful write with no content returned, such as deleting a schedule. |
| 303  | A plain form redirect or landing redirect, such as create slip, save profile, logout, joining a group, or the one-group landing. |
| 400  | A malformed form body, a bug report validation error, a bad `slip` or `start` on the reminder post, or a failed Google round trip. |
| 401  | Missing or invalid bearer token on `POST /internal/prefill` or `POST /internal/reminders`. |
| 403  | A non-GET request without `HX-Request: true`. |
| 404  | A missing slip, leg, member, or group; a resource in another group; an unknown invite token; a selection not on the cached slate at `/research/bet`; or either internal post when its token is empty. |
| 422  | A fixable rule refusal or form error. The body is the short message shown in the form. |
| 500  | An unexpected store or render failure, a signed-in user with no group, a failed `RunPrefill` pass, or a failed reminder fire. |
| 503  | `POST /internal/prefill` in sandbox mode, or `/healthz` when SQLite is unreachable. |

Rule violations from `internal/slips` (`ErrLegLimit`, `ErrKickoffLocked`, `ErrGameOutsideScope`, `ErrLockedIn`,
`ErrLegsNotReady`, and the rest) and schedule rules (`ErrScheduleNameRequired`, `ErrScheduleNameTaken`, invalid scope,
or invalid leg count) answer 422 with a short descriptive message. The form shows it and keeps every pick allowed.

### HTMX headers

| Header        | Meaning |
| ------------- | ------- |
| `HX-Redirect` | Navigate the browser to a new page without swapping an empty body. |
| `HX-Refresh`  | Reload the current page, used after a group switch, invite reset, or leave. |
| `HX-Push-Url` | Write a full page URL into the address bar after a partial swap, used by the pick board and the research explorer. |
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
| `event: bye`             | The slip is `GRADED` or the stream hit its deadline; close without retrying, and reopen on user interaction, window focus, or tab visibility. |
| `: ping`                 | A comment every 25 seconds to keep proxies from closing the connection. |

Every event carries `data: {}`. Events are signals only, never slip data, so the page fetches its own view.

## Related

- [Slips](../concepts/slips.md): members, legs, and the editing and locking rules behind the 422s.
- [Live updates](../concepts/live-updates.md): the hub, stream deadlines, and the `EventSource` client.
- [Access](../concepts/access.md): sign-in, cookies, and the middleware.
- [Groups](../concepts/groups.md): invites, the active group, and what it scopes.
