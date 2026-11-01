---
slug: foundation
status: implementing
created: "2026-11-01"
---

# Foundation

## Why

Three friends build a shared NFL parlay every Sunday morning over group chat. Picks get lost in the thread, nobody sees
the combined odds, and there is no record of who picked what. There is no app yet: the repo holds only tooling config.

## Problem

Nothing can be built until there is a running server that stores data, knows who each friend is, and keeps strangers
out.

## Decision

Build a bootable web server with a local database, a shared group passcode, and a "who are you?" picker that remembers
each friend on their device. It ships with an empty phone-first page shell that later changes fill in. No accounts or
passwords.
