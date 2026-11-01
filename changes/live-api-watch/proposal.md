---
slug: live-api-watch
status: proposed
created: 2026-11-01
---

# Live API watch

## Why

ESPN's endpoints are unofficial and can change without notice. A format change would break the app on a Sunday morning.

## Problem

Tests use saved responses, so nothing notices when a live API changes shape.

## Decision

Run the live API tests weekly in CI and on demand, so a format change shows up midweek instead of on game day.
