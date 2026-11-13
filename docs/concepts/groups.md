---
id: groups
title: Groups
type: concept
summary: How groups split people into shared clubs, how invites and leaving work, how recurring slip schedules automate weekly slips, and how the active group scopes what a person sees.
tags: [groups, membership, invites, solo, scope, cookies, schedules]
sources: [internal/groups/groups.go, internal/groups/handlers.go, internal/db/groups.go, internal/db/schedules.go, internal/db/migrations/0001_init.sql, internal/db/migrations/0009_multi_slip_events.sql, internal/auth/signin.go, internal/auth/cookies.go, internal/auth/middleware.go, internal/web/home.go, internal/web/slips.go, internal/web/schedules.go, internal/slips/slips.go, cmd/server/main.go, web/templates/home.html, web/templates/groups.html, web/templates/join.html, web/templates/layout.html, web/static/app.js]
last_verified: 2026-11-13
---

# Groups

## Summary

A group is one club of people who share slips. It is the boundary for everything people see and change together. A
person can belong to more than one group, and every person starts in a group of one so solo play needs no separate mode.
There are no roles: every member can invite, reset the invite link, leave, and configure recurring slip schedules.

Weeks, games, odds, player matches, and `sync_state` stay global. Every group reads the same lines through the shared
cache, so a second group spends no extra Odds API credits.

## Membership

Three tables hold the boundary:

- `groups` holds the name, the invite token, and when the group was created. Names are not unique, so two clubs can both
  call themselves "Sunday Parlay Club".
- `group_members` pairs a group with a user and records when they joined. The pair is the primary key.
- `group_slip_schedules` holds recurring slip schedules for the group, defining the name, slate scope, legs per member,
  and active status. Schedule names are unique per group.

`auth.SignIn` creates a new user and a group named "Just me" in one transaction. A known Google subject or email keeps
the user and the groups they already have. Anyone with a Google account can sign up; an invite is what adds someone to a
group that already has people in it. See [Access](access.md) for the sign-in flow.

`slips` carries `group_id NOT NULL`, and its unique key is `(group_id, week_id, name)`. A slip with the same name and
week in two groups succeeds in both. Each person's legs stay theirs after they leave a group.

## Recurring slip schedules

Groups configure recurring slip schedules (`group_slip_schedules`) to automate their weekly rituals:

- Each schedule defines a name, a slate scope (`thursday`, `sunday_main`, or `full_week`), a leg limit per member (1–5,
  defaulting to 2), and an active toggle (`is_active`). Names are unique within a group.
- Every group starts with a default active schedule: `Main Sunday Slip` (`sunday_main`, 2 legs). Existing groups were
  backfilled with this schedule in migration 0009.
- When resolving a group's active week, `EnsureWeekSlips` checks for slips matching each active schedule. If any are
  missing for that week, it creates them and attaches all active group members. Slips created from a schedule store its
  `schedule_id`.
- Members can toggle schedules on or off, edit their slate scope or leg count, add new schedules, or delete them.
  Pausing a schedule stops auto-creation for future weeks without modifying existing slips.
- Members can also create ad-hoc one-off slips for the current week without a schedule (`schedule_id` is null). These do
  not recur in future weeks.

## The active group

The browser carries a signed `pc_group` cookie holding a group ID. `groups.Scope`, mounted after `RequireUser`, resolves
the request's group:

1. Read `pc_group`.
2. Load the person's memberships with `ForUser`, oldest membership first.
3. Use the cookie's group only when it is in that list.
4. Otherwise fall back to the first group and write it back to the cookie, so the fallback sticks.

A missing, forged, expired, or stale cookie is only a fallback. It never grants access. A signed-in person with no group
gets a 500, because `SignIn` always creates one and a missing membership is a real inconsistency.

Handlers read the chosen group with `groups.From` and pass its ID into queries. They never build a group filter
themselves. The top bar reads `groups.Options`, which lists the person's groups from what `Scope` already loaded, with
the active one marked.

Opening `/groups/{id}/week` makes that group active and then opens the group's newest slip of the latest week, or its
create-slip form. The route refuses a group the person does not belong to with a 404. Opening a slip from another of
their groups switches the active group the same way, so a shared link always lands in the right club.

Logging out clears `pc_group` with the session, so a shared browser forgets the choice. Leaving the active group moves
the cookie to another group.

## Invites

Every group has one `invite_token`: 16 random bytes as 32 lowercase hex characters. It is a bearer secret. The `/groups`
page shows the full link with a copy button. Resetting the link mints a new token and kills the old one immediately.

The link is `BASE_URL + /join/{token}` when `BASE_URL` is set. Without it, the app falls back to the request's host and
`X-Forwarded-Proto`. That fallback is right on a direct HTTP request, but a proxy that rewrites `Host` (Cloudflare in
front of Cloud Run) makes the request host an internal hostname. `BASE_URL` is what keeps a pasted link on the public
domain.

- `GET /join/{token}` shows "Join {group}?" and the group's active members. It names the group and the people, nothing
  else.
- A signed-out visitor is sent to `/login` with the invite path kept in the signed `pc_oauth` cookie. Only a `/join/`
  path is accepted as a post-sign-in destination, so the return path can never send a browser off-site.
- `POST /join/{token}` adds the membership and makes the group active. Joining twice changes nothing.
- An unknown or reset token gets a 404.

`POST /groups/{id}/reset-invite` and `POST /groups/{id}/leave` check that the caller belongs to the group in the path,
so a stranger cannot learn which groups exist.

## Leaving

`POST /groups/{id}/leave` removes only the membership row. The person's legs stay on the group's slips, and the rest of
the group keeps its history. Leaving your only group is refused with "You can't leave your only group." A non-member
gets a 404.

## What a group scopes

The group is the filter for shared data:

- The landing page lists every group the person belongs to. The slip screens and the Record Book read the active group.
- A route that names a group or slip requires membership, whatever `pc_group` says. A non-member gets a 404, the same as
  a missing slip. A member opening a slip from another of their groups switches the active group.
- Recurring slip schedules belong to the group; listing or modifying schedules requires membership in that group.
- Live streams check membership before subscribing, so a stream never carries another group's slip. `slip:{id}` topics
  stay private; `week:{weekID}` topics are signals with no group data.

The Record Book is per group: its record, leaderboard, streaks, and badge count only the active group's slips, and a
solo player gets a record of one. Grading stays global, because a game result is the same for everyone.

The slip routes, live streams, and seeding that use these rules ship with the changes that build them. This change lands
the schema, the scope check, and the group screens.

## Screens

`/` is the landing page. It lists one card per group, oldest first, each showing the group name and the viewer's next
step on that group's newest slip of the latest week ("Pick 1 more", "Waiting on Ben", "Ready to place", or "No slip
yet"). Each card opens that group's week. A person with one group skips the list: `/` sends them straight to
`/groups/{id}/week`. The list holds no live stream; the browser refetches it when the tab becomes visible.

`/groups` manages the caller's groups. Each card shows the invite link with copy and reset, a leave button when the
person has more than one group, and a note when they do not. A create form names and starts a new group.

The top bar hides the switcher while a person has one group and shows that group's name as a link to `/groups`. With
more than one group it shows the active name; tapping it opens a bottom sheet of groups, and picking one sets `pc_group`
and reloads the page.

## Trade-offs

- No roles or permissions. Every member can invite, reset the link, leave, and configure recurring schedules. A group is
  a boundary for what people see, not a permission system.
- Invite links are bearer secrets. 128-bit tokens, one-click reset, and a join page that shows only names limit the
  harm.
- Invite links trust `BASE_URL` over request headers, so a rewriting proxy cannot leak an internal hostname into a
  pasted link. The cost is that a request on another hostname still gets the configured one.
- Open sign-up means a stranger can reach the app. The shared cache and the credit floors bound the cost, and a stranger
  sees only their own solo group.
- You cannot leave your last group, so there is always somewhere for a slip to live.
- There is no discovery, search, or public group list. Joining takes a link a member shares.
- A group is not a database. All groups share one SQLite file, one cache, and one instance.

## Related

- [Access](access.md): Google sign-in, the session cookie, and the `pc_oauth` return path.
- [Database schema](../reference/schema.md): the `groups`, `group_members`, `group_slip_schedules`, and `slips` tables.
- [Routes](../reference/routes.md): group and slip schedule HTTP endpoints.
- [Configuration](../reference/config.md): the values startup requires.
- [Architecture](../../plan/architecture.md): routes, middleware, and the single-instance constraints.
- [Product](../../plan/product.md): slips, screens, and the Record Book.
