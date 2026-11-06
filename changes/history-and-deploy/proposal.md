---
slug: history-and-deploy
status: designed
created: "2026-11-01"
depends_on: [analyzer]
---

# History, manual grading, and deploy

## Context

The app only helps if it is online on Sunday, and the group already has past parlays worth tracking.

There is no way to deploy, to keep data across restarts, to record results, or to load the group's past slips.

## Goals and non-goals

**Goals:**

- Deploy the app to Cloud Run inside the free tier before Sunday, Nov 8.
- Keep the SQLite database across restarts and deploys with Litestream backups to Cloud Storage.
- Grade legs by hand and recompute each slip's result.
- Create slips for past weeks and load the group's history from a file.
- Run CI on every change and block deploys during game windows.

**Non-goals:**

- Auto-grading from box scores or the Record Book. `grading-record-book` adds those.
- Scheduled jobs or a second Cloud Run instance.
- More paid infrastructure than the ~$20 GCP safety net covers.
- Sportsbook integrations or money handling.

## Decision

Deploy to Cloud Run inside the free tier, with the database backed up to Cloud Storage and deploys blocked during game
windows. Add hand grading of legs with automatic slip results, slips for past weeks, and a history file that loads the
group's past parlays. This is the last change before the Nov 8 target.

Heads up on `groups` and `google-oauth`: this change ships the single club first, with the history file creating users
by name. Once Google sign-in lands, those users must be matched to real accounts, and `groups` will load the history
into the club's first group. The history loader should stay easy to point at a group and a person later.
