---
slug: live-api-watch
status: proposed
created: "2026-11-01"
depends_on: [grading-record-book]
---

# Live API watch

## Context

ESPN's endpoints are unofficial and can change without notice. A format change would break the app on a Sunday morning.

Tests use saved responses, so nothing notices when a live API changes shape.

## Goals and non-goals

**Goals:**

- Run the live API tests weekly in CI and on manual dispatch.
- Run them without `-short` so the real endpoints are exercised.
- Make a format change show up midweek instead of on game day.

**Non-goals:**

- Replacing the fixture tests or calling live APIs on every pull request.
- Monitoring app uptime, latency, or Odds API credit use.
- Sending alerts beyond the workflow's own status.

## Decision

Run the live API tests weekly in CI and on demand, so a format change shows up midweek instead of on game day.
