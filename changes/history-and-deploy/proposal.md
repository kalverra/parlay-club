---
slug: history-and-deploy
status: designed
created: "2026-11-01"
---

# History, manual grading, and deploy

## Why

The app only helps if it is online on Sunday, and the group already has past parlays worth tracking.

## Problem

There is no way to deploy, to keep data across restarts, to record results, or to load the group's past slips.

## Decision

Deploy to Cloud Run inside the free tier, with the database backed up to Cloud Storage and deploys blocked during game
windows. Add hand grading of legs with automatic slip results, slips for past weeks, and a history file that loads the
group's past parlays. This is the last change before the Nov 8 target.
