---
id: config
title: Configuration
type: reference
summary: Environment variables the server reads, their dev defaults, and the values it rejects in prod.
tags: [config, environment, startup, google, oauth]
sources: [internal/config/config.go, cmd/server/main.go]
last_verified: 2026-11-07
---

# Configuration

## Summary

All configuration comes from environment variables. `internal/config` parses them once at startup, and `cmd/server`
exits with status 1 when any value is invalid.

## Variables

| Variable               | Dev default              | Notes                                                                         |
| ---------------------- | ------------------------ | ----------------------------------------------------------------------------- |
| `APP_ENV`              | `dev`                    | `dev` or `prod`. Any other value is an error.                                 |
| `PORT`                 | `8080`                   | Must be 1–65535. Cloud Run sets it.                                           |
| `DB_PATH`              | `./data/parlay.db`       | The directory is created if missing; `data/` is gitignored. Relative paths resolve from the working directory. |
| `SESSION_KEY`          | fixed dev string         | Required in prod, at least 32 bytes. Signs both cookies.                       |
| `GOOGLE_CLIENT_ID`     | empty                    | The OAuth Web client ID. Empty in dev means the dev sign-in form. Required in prod. |
| `GOOGLE_CLIENT_SECRET` | empty                    | The OAuth Web client secret. Required in prod.                                |
| `BASE_URL`             | `http://localhost:{PORT}` | Public base URL without a trailing slash. The redirect URI is `BASE_URL + /auth/google/callback`. Required in prod. |
| `ODDS_API_KEY`         | empty                    | Empty disables Odds API calls.                                                |
| `LOG_LEVEL`            | `debug`                  | A zerolog level: `trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic`, `disabled`. `prod` defaults to `info`. |
| `LITESTREAM_BUCKET`    | unset                    | Read by Litestream, not by the app.                                           |

Dev falls back to `http://localhost:{PORT}` for `BASE_URL` and a fixed `SESSION_KEY`. It never invents Google values. In
prod, a missing `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `BASE_URL`, or `SESSION_KEY` fails startup instead of
falling back.

`BASE_URL` is trimmed of trailing slashes. It must match the scheme, host, and port registered on the Google client, or
sign-in stops at `redirect_uri_mismatch`. The app builds the redirect URI by appending `/auth/google/callback`.

Sign-up is open, so there is no email list to configure. Group membership is what decides who sees which slips; see
[Groups](../concepts/groups.md).

## Rejected values

The server prints every problem it finds, one per line, and exits 1. It does not start with a partial config.

| Condition                                | Message                                                       |
| ---------------------------------------- | ------------------------------------------------------------- |
| `APP_ENV` is not `dev` or `prod`         | `APP_ENV must be "dev" or "prod", got "..."`                    |
| `PORT` is not a number from 1 to 65535    | `PORT must be a number from 1 to 65535, got "..."`             |
| `SESSION_KEY` is under 32 bytes in prod  | `SESSION_KEY must be at least 32 bytes in prod`                |
| `GOOGLE_CLIENT_ID` is empty in prod      | `GOOGLE_CLIENT_ID is required in prod`                         |
| `GOOGLE_CLIENT_SECRET` is empty in prod  | `GOOGLE_CLIENT_SECRET is required in prod`                     |
| `BASE_URL` is empty in prod              | `BASE_URL is required in prod`                                 |
| `LOG_LEVEL` is not a known level         | `LOG_LEVEL is not a known level, got "..."`                    |

Dev accepts a short `SESSION_KEY` and no Google values, so a fresh checkout starts without secrets.
