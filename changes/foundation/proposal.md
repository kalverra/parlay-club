---
slug: foundation
status: designed
created: "2026-11-01"
---

# Foundation

## Context

Three friends build a shared NFL parlay every Sunday morning over group chat. Picks get lost in the thread, nobody sees
the combined odds, and there is no record of who picked what. There is no app yet: the repo holds only tooling config.

Nothing can be built until there is a running server that stores data, knows who each friend is, and keeps strangers
out.

## Goals and non-goals

**Goals:**

- Run a bootable Go web server with a local SQLite database, config validation, migrations, a health check, and graceful
  shutdown.
- Keep strangers out with one shared group passcode, then let each friend pick their name so their device remembers
  them.
- Ship a phone-first page shell with the Slip, Games, Research, and Record Book tabs ready for later changes.
- Enforce session cookies and CSRF on every request that changes data.

**Non-goals:**

- Slips, games, lines, weather, the analyzer, grading, or history. Later changes build those.
- Accounts, passwords, OAuth, or per-person logins.
- Scheduled jobs or outside data fetches.

## Decision

Build a bootable web server with a local database, a shared group passcode, and a "who are you?" picker that remembers
each friend on their device. It ships with an empty phone-first page shell that later changes fill in. No accounts or
passwords.
