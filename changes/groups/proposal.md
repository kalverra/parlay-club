---
slug: groups
status: implementing
created: "2026-11-05"
depends_on:
    - google-oauth
effort: hard
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
