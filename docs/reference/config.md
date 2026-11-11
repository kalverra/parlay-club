---
id: config
title: Configuration
type: reference
summary: Environment variables the server reads, their dev defaults, the odds provider keys they configure, the offline sandbox switch, and the values it rejects in prod.
tags: [config, environment, startup, google, oauth, odds, parlay-api, the-odds-api, sandbox, offline]
sources: [internal/config/config.go, cmd/server/main.go, cmd/server/sandbox.go, internal/sandbox/*.go, internal/ingest/odds/client.go]
last_verified: 2026-11-10
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
| `ODDS_API_KEY`         | empty                    | Parlay API key. Empty disables odds calls.                                    |
| `THE_ODDS_API_KEY`     | empty                    | The Odds API key for enrichment and fallback lines. Empty disables that provider. |
| `LOG_LEVEL`            | `debug`                  | A zerolog level: `trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic`, `disabled`. `prod` defaults to `info`. |
| `SUNDAY_SANDBOX`       | empty                    | `1` runs the offline Sunday sandbox. It fixes the database at `./data/sunday-sandbox.db` and ignores `DB_PATH`. Rejected in prod. |
| `LITESTREAM_BUCKET`    | unset                    | Read by Litestream, not by the app.                                           |

Dev falls back to `http://localhost:{PORT}` for `BASE_URL` and a fixed `SESSION_KEY`. It never invents Google values. In
prod, a missing `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `BASE_URL`, or `SESSION_KEY` fails startup instead of
falling back.

`SUNDAY_SANDBOX=1` is a development switch, not a deployment mode. It always uses `./data/sunday-sandbox.db`, skips
history import and Google provider construction, and forces the dev sign-in form. Every ingest client reads the saved
week 9 responses in `internal/sandbox` through a fail-closed replay transport, so `ODDS_API_KEY` and
`THE_ODDS_API_KEY` are unused and no credits are spent. `mise run sandbox` sets the variable for the normal hot-reload
loop; see [Local development](../guides/local-dev.md).

`BASE_URL` is trimmed of trailing slashes. It must match the scheme, host, and port registered on the Google client, or
sign-in stops at `redirect_uri_mismatch`. The app builds the redirect URI by appending `/auth/google/callback`.

`ODDS_API_KEY` is the Parlay API key and `THE_ODDS_API_KEY` is the key for The Odds API. Both are optional; an empty key
disables that provider. Parlay API is the primary game-line feed. The Odds API enriches its rows with DraftKings links
and missing quotes, and serves the whole week when Parlay API is unconfigured, fails, or falls below its credit floor.
`internal/ingest/odds` defaults the primary base URL to
`https://parlay-api.com/v1/sports/americanfootball_nfl` and the fallback base URL to
`https://api.the-odds-api.com/v4/sports/americanfootball_nfl`; tests point `NewClient` at a fake server instead.

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
| `SUNDAY_SANDBOX` is true in prod         | `SUNDAY_SANDBOX is not allowed in prod`                        |
| `SUNDAY_SANDBOX` is not a boolean        | `SUNDAY_SANDBOX must be a boolean, got "..."`                  |
| `LOG_LEVEL` is not a known level         | `LOG_LEVEL is not a known level, got "..."`                    |

Dev accepts a short `SESSION_KEY` and no Google values, so a fresh checkout starts without secrets.
