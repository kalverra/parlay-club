---
id: config
title: Configuration
type: reference
summary: Environment variables the server reads, their dev defaults, and the values it rejects in prod.
tags: [config, environment, startup]
sources: [internal/config/config.go, cmd/server/main.go]
last_verified: 2026-11-01
---

# Configuration

## Summary

All configuration comes from environment variables. `internal/config` parses them once at startup, and `cmd/server`
exits with status 1 when any value is invalid.

## Variables

| Variable            | Dev default        | Notes                                                                    |
| ------------------- | ------------------ | ------------------------------------------------------------------------ |
| `APP_ENV`           | `dev`              | `dev` or `prod`. Any other value is an error.                            |
| `PORT`              | `8080`             | Must be 1–65535. Cloud Run sets it.                                      |
| `DB_PATH`           | `./data/parlay.db` | The directory is created if missing; `data/` is gitignored. Relative paths resolve from the working directory. |
| `PASSCODE`          | `dev`              | Required in prod.                                                        |
| `SESSION_KEY`       | fixed dev string   | Required in prod, at least 32 bytes.                                     |
| `ODDS_API_KEY`      | empty              | Empty disables Odds API calls.                                           |
| `LOG_LEVEL`         | `debug`            | A zerolog level: `trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic`, `disabled`. `prod` defaults to `info`. |
| `LITESTREAM_BUCKET` | unset              | Read by Litestream, not by the app.                                      |

An empty variable counts as unset. Dev defaults never apply in prod for `PASSCODE` and `SESSION_KEY`, so a missing
secret fails startup instead of falling back.

## Rejected values

The server prints every problem it finds, one per line, and exits 1. It does not start with a partial config.

| Condition                             | Message                                                    |
| ------------------------------------- | ---------------------------------------------------------- |
| `APP_ENV` is not `dev` or `prod`      | `APP_ENV must be "dev" or "prod", got "..."`                |
| `PORT` is not a number from 1 to 65535 | `PORT must be a number from 1 to 65535, got "..."`          |
| `PASSCODE` is empty in prod           | `PASSCODE is required in prod`                              |
| `SESSION_KEY` is under 32 bytes in prod | `SESSION_KEY must be at least 32 bytes in prod`           |
| `LOG_LEVEL` is not a known level      | `LOG_LEVEL is not a known level, got "..."`                 |

Dev accepts a short `SESSION_KEY`; only prod enforces the 32-byte minimum.
