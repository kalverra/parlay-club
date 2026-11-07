---
id: adr-groups
title: Groups and solo play
type: decision
summary: Make the group the boundary for everything people share.
last_verified: "2026-11-07"
---

# Groups and solo play

## Context

Today the app is one club. Every user sees every slip, the record book is one leaderboard, and the list of who may sign
in is a line of config. That fits one group of friends. It does not fit a friend who wants to run their own club with
their own people, or who just wants to track their own parlays alone.

Google sign-in (`google-oauth`) makes each person a stable user and keeps the question of which club they belong to open
on purpose. This change answers it. The other planned changes (shared slips, slate and lines, grading and the record
book) were written for one club, so they assume a single shared pool of slips and members.

## Goals and non-goals

**Goals:**

- Any friend can start their own group and invite others to it.
- Any friend can play solo, with no group to set up.
- A person can belong to more than one group.
- Slips, the record book, and live updates stay inside a group. Nobody sees another group's picks.
- Sign-in no longer depends on a config list of emails. Getting in is easy; seeing a group's data takes an invite.
- Everything works at phone width and stays friendly, with no admin screens or lectures.

**Non-goals:**

- Roles or permissions inside a group. Every member can do what every other member can. A group is a boundary for who
  sees what, not a permission system.
- Public groups, discovery, or search for groups to join.
- Moving a slip between groups, or merging groups.
- Billing, limits per user, or any money handling.
- A cross-group leaderboard or global stats.

## Decision

Make the group the boundary for everything people share. A solo player is simply a group of one, created for them
automatically, so there is one code path instead of a solo mode and a group mode. Membership links a person to a group.
Slips, seasons, the record book, and the live-update channel hang off a group, and every read and write is checked
against the caller's membership. Joining is by an invite link that a member shares in their own chat, which matches how
these friends already talk. We build no email sending and no member management beyond leaving a group.

Game lines, weather, and props are the same for every group, so they stay global and shared. That matters because Odds
API credits are scarce. A second group must not double the credit use; the cache already in place serves everyone. The
single-instance and idle-stream limits stay as they are, with live updates scoped per group.

This beats keeping one club and adding filters, because filters leak data and every query would need to remember them.
It beats a per-group database, because the app runs as one instance on SQLite and per-group files would split the shared
cache and make migrations painful. Open sign-up replaces the email list, so the cost guard moves to the shared cache and
the credit floors rather than a gate at the door. The effort is hard: nearly every table and query gains a group, the
live-update hub needs scoping, the existing club's data must be moved into a first group without loss, and the designs
of the changes still waiting must be reconciled with it.

## Consequences

Groups are now the boundary for everything people share. A solo player is a group of one, so there is one code path, and
any solo group can become a shared club by minting an invite. Sign-up is open; an invite is what adds someone to a group
that already has people in it. The old email allow list, its config parser, and the gate are gone.

The membership check lives in one place. `groups.Scope` runs after `RequireUser`, resolves the active group from the
signed `pc_group` cookie or the person's first membership, and handlers read it with `groups.From`. A new authenticated
route inherits the scope instead of repeating it.

It is easier to add a screen that reads shared data and harder to add one that leaks. The scope only tells a handler
which group the browser chose; it cannot make a query filter by group. A route mounted outside `RequireUser`, a query
that forgets `slips.group_id`, or a slip-by-id handler that trusts `pc_group` would expose another group's data. The
route-table isolation test that would catch this cannot be written yet because no slip routes exist.

A group can become empty. Two members who each belong to another group can both leave, and the group keeps its slips and
a working invite link with nobody to look at them. Invite links remain bearer secrets, so a leaked link is enough to
join until someone resets it. Open sign-up plus lazy sync also means strangers can trigger line and prop fetches; the
shared cache and credit floors bound the cost, and a stranger sees only their solo group.

## Alternatives considered

- **Keep one club and filter views.** Every query would have to remember the filter, and one omission leaks. Rejected in
  favor of a boundary the schema enforces.
- **One database file per group.** This splits the shared odds cache, multiplies migrations, and works against the
  single-instance, single-SQLite design. Rejected.
- **Put the group in every URL.** It is the most explicit option, but it rewrites every route in the waiting designs and
  makes the active-group cookie pointless. Rejected.
- **Roles and permissions inside a group.** Nobody asked for them. A group is a visibility boundary, not an
  authorization system. Rejected.
- **Email invites or member management screens.** Friends already share links in their own chat, so a bearer link avoids
  SMTP, delivery failures, and an admin surface. Rejected.
- **Keep the email allow list and add groups on top.** This keeps two gates for one question. Open sign-up replaces the
  list, and the cost guard moves to the shared cache and credit floors. Rejected.

## Implementation notes

The design assumed `slips-live`, `slate-and-lines`, and `history-and-deploy` had shipped. They had not: this branch has
no `internal/slips`, `internal/live`, or `internal/seed`, and no slip routes. What landed here is the schema,
`groups.Scope`, the `pc_group` cookie, open sign-up, the group store and handlers, and the screens. Threading `Actor`
through slips, the slip-by-id membership guard, the `group:{id}` topic, membership-checked streams, group filters on the
home list and Record Book, seeding the club group, and the isolation, same-name, live, and credit tests move to the
changes that build that code. Those changes should build group-aware from the start rather than adapting later.

The schema was edited in place and nothing was deployed, so nothing was migrated. `slips.group_id` is `NOT NULL` with no
backfill; an existing `data/parlay.db` must be deleted and rebuilt, which the local development guide now says. A
deployed database with data would have needed a real migration.

Removing the gate also removed the name-based history link, because `gate.LinkName` was its only caller. `auth.SignIn`
now resolves a subject, then an email, then creates a user. The history seed must therefore set an email or create users
before their first sign-in; a seeded name alone can no longer be claimed by a Google account.

`Scope` resolves membership with one `ForUser` call and checks the cookie against that list, rather than calling
`IsMember` and then `ForUser` for the fallback. It also writes the fallback group back to the cookie so the choice
sticks. Both differ from the design and save a query per request.

Smaller deviations a maintainer will notice:

- Logout clears `pc_group` as well as the session, so a shared browser forgets the active group.
- A signed-in person with no group gets a 500. `SignIn` always creates one, so this is a real inconsistency.
- `groups.Scope` takes `UserFrom` and a small `GroupCookie` interface instead of importing `internal/auth`, which keeps
  `internal/groups` as the membership seam and lets tests pass stubs.
- The signed-out invite redirect lives in `auth.RequireUser`, not the join handler, because the handler never runs for a
  signed-out visitor.
- Creating a group makes it active. The design did not say which group becomes active.
- Reset, leave, and switch return `HX-Refresh` instead of swapping a fragment, because the group list, the active badge,
  and the top bar all change together.
- The join page lists active members only, since inactive users cannot sign in.
- Invite links are built from the request host and `X-Forwarded-Proto`, not `BASE_URL`. A proxy that drops the header
  would produce `http` links on an HTTPS page.
- `/groups` calls `ForUser` even though `Scope` already loaded the same memberships, so a page view costs one extra
  query.
- Joining lands on `/`, which has no slip list yet, so a new member sees the joined group's slips only once `slips-live`
  lands.
- Phone-width fit was checked by structure and template tests, not in a browser.
- `docs/concepts/slips.md` and `docs/concepts/live-updates.md` do not exist on this branch, so the group rules they were
  meant to receive live in `docs/concepts/groups.md` for now.

## Follow-ups

1. **Land the deferred group-scoped slip work in the waiting changes.** `slips-live`, `slate-and-lines`, and
   `history-and-deploy` should thread `Actor{UserID, GroupID}` or `groups.From` through slip methods, guard every
   slip-by-id route and SSE subscription by membership with a 404 for non-members, publish `group:{id}` on slip create
   and rename, filter the home list and Record Book by the active group, and seed "Sunday Parlay Club" and its members.
   Bring the route-table isolation, same-name-two-groups, live, and credit tests with that code.
2. **Reconcile the waiting designs.** `changes/slips-live/design.md`, `changes/slate-and-lines/design.md`, and
   `changes/history-and-deploy/design.md` still carry "Groups later" notes, and two of them say not to add a group
   column or table. That is now false. Rewrite those notes to take the group from the actor, and make sure
   `grading-record-book` does the same.
3. **Decide what happens to an empty group.** Choose one of: delete a group when its last member leaves, offer to hand
   it over, or document that an ownerless group keeps its slips and a live invite until someone joins. Right now the
   third is the silent behavior.
4. **Fold groups into the slip and live docs.** When `docs/concepts/slips.md` and `docs/concepts/live-updates.md` land,
   move the group rules from `docs/concepts/groups.md` into them and link back, then run `good-vibes docs index`.
5. **Make invite links independent of the proxy.** Build the copied link from `BASE_URL`, or verify that every deploy
   path sends `X-Forwarded-Proto`, so an invite never says `http` on an HTTPS page.
6. **Reuse the memberships `Scope` already loaded on `/groups`.** An exported accessor on the group context would drop
   one query per page view. Small, and only worth doing with another reason to touch the context.
