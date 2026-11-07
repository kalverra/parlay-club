---
id: routes
title: Routes
type: reference
summary: Every HTTP route the server serves, the middleware in front of them, and the status codes, HTMX headers, and SSE frames handlers return.
tags: [routes, http, handlers, middleware, htmx, sse]
sources: [cmd/server/main.go, internal/auth/handlers.go, internal/groups/handlers.go, internal/web/routes.go, internal/web/games.go, internal/web/slips.go, internal/live/stream.go]
last_verified: 2026-11-07
---

# Routes

## Summary

One `http.ServeMux` holds every route, using Go 1.22 method patterns. Handlers stay thin: they parse the request, call
`internal/slips` or the database store, and render a page or partial. Routes still to be built are listed in the
[architecture plan](../../plan/architecture.md).

## Middleware

All routes except `/healthz`, `/static/`, `/login`, `POST /logout`, the Google sign-in routes, and the dev sign-in
routes need a signed-in user. Those public routes are registered first. Everything else runs through three wrappers,
outermost first. See [Access](../concepts/access.md) for cookies and sign-in.

| Middleware    | Rejects                                                        | Response                          |
| ------------- | -------------------------------------------------------------- | --------------------------------- |
| `CSRF`        | A non-GET request without `HX-Request: true`                   | 403                               |
| `RequireUser` | A missing, tampered, expired, inactive, or unknown session      | `/login`, or `HX-Redirect` for HTMX |
| `Scope`       | Nothing for a valid session; a user with no group gets a 500    | —                                 |

`CSRF` exempts two plain forms, `POST /logout` and `POST /dev/login`. Every other write must come from HTMX.

`Scope` picks the active group and puts it on the request context. Handlers read it instead of taking a group id in the
path. A slip or leg outside the active group answers 404, the same as a missing one.

## Public routes

| Method + path                       | Purpose                                                                 | Notes |
| ----------------------------------- | ----------------------------------------------------------------------- | ----- |
| `GET /healthz`                      | Liveness probe                                                          | Always 200 `ok`. |
| `GET /static/{path...}`             | Embedded CSS, JS, and other assets                                      | One-year cache. A missing or escaping path is 404. |
| `GET /login`                        | Sign-in page                                                            | Shows the Google button, plus a dev link when Google is off. |
| `POST /logout`                      | Clear the session and active-group cookies                              | 303 to `/login`. |
| `GET /auth/google/start`            | Start the Google round trip                                             | Registered only when a Google client is configured. Redirects to Google. |
| `GET /auth/google/callback`         | Finish Google sign-in                                                   | Bad `state`, code exchange, or identity answers 400 with the sign-in page. |
| `GET /dev/login`, `POST /dev/login` | Local sign-in form and submit                                           | Registered only when `APP_ENV=dev` and `GOOGLE_CLIENT_ID` is empty. |

## Authenticated routes

### Shell and tabs

| Method + path     | Purpose                                                                                |
| ----------------- | -------------------------------------------------------------------------------------- |
| `GET /{$}`        | Slip tab: the newest slip of the latest week, or the create-slip form when there is none |
| `GET /games`      | Games tab: the latest week's board with quick-add pills                                 |
| `GET /research`   | Research tab placeholder                                                                |
| `GET /record`     | Record Book placeholder                                                                 |

The slip handler file also serves `/me` and the slip and leg routes below. `/research` and `/record` render their empty
state until later changes fill them in. The Games tab is described under [Games](#games).

### Groups

| Method + path                          | Purpose                                                     | Response |
| -------------------------------------- | ----------------------------------------------------------- | -------- |
| `GET /groups`                          | Group list, create form, invite links, and leave buttons     | Full page |
| `POST /groups`                         | Create a group from `name` and make it active                | 303 to `/groups` |
| `POST /groups/switch`                  | Make the group in `group_id` active                          | `HX-Refresh` |
| `POST /groups/{id}/reset-invite`       | Mint a new invite token and kill the old one                 | `HX-Refresh` |
| `POST /groups/{id}/leave`              | Leave the group, keeping the person's legs                   | 303 to `/groups` |
| `GET /join/{token}`                    | Join page naming the group and its active members            | Full page; unknown token 404 |
| `POST /join/{token}`                   | Join the group idempotently and make it active               | 303 to `/` |

A signed-out visitor is sent to `/login` with the invite path kept in `pc_oauth`, so sign-in returns them to the join
page. Only a `/join/` path is accepted as a return target.

`POST /groups/{id}/reset-invite` and `POST /groups/{id}/leave` answer 404 when the caller is not in that group, so a
stranger cannot learn which groups exist.

### Slips

All slip routes require the slip to belong to the active group.

| Method + path                              | Purpose                                                         | Request and response |
| ------------------------------------------ | --------------------------------------------------------------- | -------------------- |
| `GET /slips/new`                           | Create-slip page                                                | Optional `?week=`; unknown week 404. Full page. |
| `POST /slips`                              | Create the slip from `week_id`, `name`, `legs_per_member`, and `member_id` | 303 to `/slips/{id}`; a fixable error is 422. |
| `GET /slips/{id}`                          | Full slip page                                                  | Full page, with the payout form at the default stake. |
| `GET /slips/{id}/panel`                    | Members, legs, odds, lock state, and banner                     | Panel partial. Refreshes on `slip-updated`. |
| `GET /slips/{id}/payout`                   | Payout calculator                                               | Optional `?stake=` and `?boost=`; defaults 10 and 0. Payout partial. |
| `GET /slips/{id}/activity`                 | Activity feed                                                   | Activity partial. Refreshes on `slip-updated`. |
| `POST /slips/{id}/members`                 | Add a group member from `user_id`                               | Panel partial. |
| `DELETE /slips/{id}/members/{userID}`      | Remove a member                                                 | Panel partial. |
| `PATCH /slips/{id}`                        | Change `name` or `legs_per_member`                              | Panel partial. |
| `GET /slips/{id}/legs/new`                 | Empty leg form with the game picker                             | Optional `?game_id=` and `?quick=`; leg form partial. |
| `POST /slips/{id}/legs`                    | Add the signed-in member's leg                                  | Panel partial, or the `leg-added` partial for a quick add; `Hx-Trigger: leg-saved`. |
| `GET /legs/{id}/edit`                      | Edit form filled from the leg                                   | Leg form partial. |
| `PUT /legs/{id}`                           | Save the leg's pick fields                                      | Panel partial with `Hx-Trigger: leg-saved`. |
| `DELETE /legs/{id}`                        | Delete the leg                                                  | Panel partial. |
| `POST /slips/{id}/lock`                    | Record the signed-in member's lock in                           | Panel partial. |
| `POST /slips/{id}/unlock`                  | Clear every member's lock                                       | Panel partial. |
| `GET /slips/{id}/events`                   | Live updates for the slip and its week                          | `text/event-stream`; see below. |

The leg form takes `market_type`, `selection`, `team`, `player_name`, `player_id`, `side`, `line`, `odds`, and
`game_id`. The game picker lists the week's games that have not kicked off; a past week lists them all and allows no
game. When `selection` is blank, the handler builds the display text from the market fields. An unreadable number is 422
with "Odds need to be a number.", "Line needs to be a number.", or a similar short message.

`?quick=1` turns the form into a quick add: the target becomes the game card's `#quick-add-{gameID}` element, and a
successful `POST` answers with the `leg-added` partial instead of the slip panel.

`GET /slips/{id}/events` serves one connection for one slip. It subscribes to `slip:{id}` and `week:{weekID}`, sends
`slip-updated` and `board-updated` signals, pings every 25 seconds, and ends with `bye` at its deadline. The slip page
sets `data-live-url` so the client opens the stream. See [Live updates](../concepts/live-updates.md).

### Games

`GET /games` renders the latest week's board. It reads the week's games and each game's odds rows. Rows from an older
sync are dropped, so a card never mixes prices from two fetches.

Each card shows the matchup, the kickoff as a `<time datetime>` the client formats in the browser's zone, the two
spreads, and the over and under. A selection shows the viewer's home-book price when that book carries it, and the
median across books otherwise. A live or final game also shows its score and a status chip: Upcoming, Live, or Final.
Weather badges are 🏠 Indoor, 💨 wind mph (amber from 15, red from 20), and 🌧 at 0.1 in/hr of precipitation or more.

The "lines as of" note under the board uses the newest odds fetch time on the games, or the `odds:lines` row in
`sync_state` when that is newer. A week with no games shows the empty state. A missing store or week renders the same
empty page, not an error.

Expanding a card shows the quick-add forms. Each one posts to `POST /slips/{id}/legs` with `quick=1` and the game,
market, team, side, line, and price prefilled, and swaps the "Added {selection}" partial into that card. The pills
appear only when the newest slip in the week is `OPEN`. With no slip, the card links to `/slips/new`; with a locked
slip, it says picks cannot be added. "Custom leg" opens the same leg form with `game_id` set and `quick=1`, so a saved
custom leg lands in the card too.

### Profile

| Method + path                  | Purpose                                                   | Response |
| ------------------------------ | --------------------------------------------------------- | -------- |
| `GET /me`                      | Profile: home book, plus the group member list             | Full page |
| `POST /me`                     | Save `home_book`; blank means the median price             | 303 to `/me` |
| `POST /users/{id}/active`      | Turn a group member on or off from `active`                | 303 to `/me` |

`POST /users/{id}/active` answers 422 for "You can't turn yourself off." and for a value that is not on or off. A user
outside the active group is 404.

## Responses

### Status codes

| Code | When |
| ---- | ---- |
| 200  | Pages, partials, and successful HTMX writes. Redirects and refreshes send 200 with an `HX-*` header. |
| 303  | A plain form redirect, such as create slip, save profile, logout, or joining a group. |
| 400  | A malformed form body, or a failed Google round trip. |
| 403  | A non-GET request without `HX-Request: true`. |
| 404  | A missing slip, leg, member, or group; a resource in another group; or an unknown invite token. |
| 422  | A fixable rule refusal or form error. The body is the short message shown in the form. |
| 500  | An unexpected store or render failure, or a signed-in user with no group. |

Rule violations from `internal/slips` (`ErrLegLimit`, `ErrKickoffLocked`, `ErrLockedIn`, `ErrLegsNotReady`, and the
rest) answer 422 with the message from [Slips](../concepts/slips.md). The form shows it and keeps every pick allowed.

### HTMX headers

| Header        | Meaning |
| ------------- | ------- |
| `HX-Redirect` | Navigate the browser to a new page without swapping an empty body. |
| `HX-Refresh`  | Reload the current page, used after a group switch, invite reset, or leave. |
| `HX-Trigger`  | Fire a client event. `leg-saved` closes the leg form's bottom sheet. |

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
