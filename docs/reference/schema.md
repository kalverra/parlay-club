---
id: schema
title: Database schema
type: reference
summary: SQLite tables, how migrations are tracked and applied, and the connection settings behind every query.
tags: [database, sqlite, migrations]
sources: [internal/db/migrations/*.sql, internal/db/migrate.go, internal/db/db.go]
last_verified: 2026-11-01
---

# Database schema

## Summary

One SQLite file, opened by `internal/db.Open`, holds every table the app uses. `0001_init.sql` creates all of them, so
later changes add no migrations until the schema itself changes.

## Connection settings

`internal/db.Open` sets these pragmas on every connection:

| Pragma        | Value    | Why                                               |
| ------------- | -------- | ------------------------------------------------- |
| `journal_mode` | `WAL`   | Reads continue while a write is in flight.        |
| `busy_timeout` | `5000`  | Wait for a lock instead of failing at once.       |
| `foreign_keys` | `1`     | SQLite enforces foreign keys only when asked.     |
| `synchronous`  | `NORMAL`| The safe pairing with WAL.                        |

Transactions begin with `_txlock=immediate`, and the pool is capped at one connection. Traffic is tiny, and a single
connection rules out `SQLITE_BUSY`.

`Open` creates the database directory when missing and fails if the first connection cannot be made.

## Migrations

- Files live in `internal/db/migrations/NNNN_name.sql` and are embedded in the binary.
- `db.Migrate` applies every file whose number is greater than `PRAGMA user_version`, in number order.
- Each file runs in its own transaction. The migration's statements and its `user_version` bump commit together, so a
  failed migration leaves neither behind.
- A fresh database ends at version 1 with every table below.
- Never edit an applied file. Add `0002_name.sql` and up.
- `PRAGMA user_version` on the database file is the only record of what ran.

## Tables

| Table            | Holds                                                        |
| ---------------- | ------------------------------------------------------------ |
| `users`          | Group members, their avatar color, home book, and active flag. |
| `weeks`          | NFL weeks, keyed `season*1000 + season_type*100 + week_num`.  |
| `games`          | One row per ESPN event: teams, kickoff, venue, weather, scores. |
| `odds`           | Odds API prices per game, book, market, and selection.        |
| `slips`          | One shared parlay per week, with status and result.           |
| `slip_members`   | Who is on a slip and when they locked in.                     |
| `legs`           | Picks on a slip, with market, selection, line, odds, and result. |
| `wagers`         | Optional stake, odds, and boost per member per slip.          |
| `player_matches` | Odds player name to ESPN athlete ID, per team.                |
| `activity`       | Human-readable slip history; `user_id` is NULL for system entries. |
| `sync_state`     | Last fetch time per outside data source, plus remaining odds credits. |

## DDL

The definitions below match `internal/db/migrations/0001_init.sql`, which is the source of truth.

```sql
CREATE TABLE users (
  id           INTEGER PRIMARY KEY,
  name         TEXT NOT NULL UNIQUE COLLATE NOCASE,
  avatar_color TEXT NOT NULL,                  -- Tailwind color name, e.g. 'sky'
  home_book    TEXT,                           -- Odds API bookmaker key; NULL = median
  active       INTEGER NOT NULL DEFAULT 1,
  created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE TABLE weeks (
  id          INTEGER PRIMARY KEY,             -- season*1000 + season_type*100 + week_num, e.g. 2026209
  season      INTEGER NOT NULL,
  season_type INTEGER NOT NULL,                -- 2 regular, 3 postseason
  week_num    INTEGER NOT NULL,
  UNIQUE (season, season_type, week_num)
);

CREATE TABLE games (
  id             TEXT PRIMARY KEY,             -- ESPN event ID
  week_id        INTEGER NOT NULL REFERENCES weeks(id),
  odds_event_id  TEXT UNIQUE,                  -- The Odds API event ID once matched
  home_team      TEXT NOT NULL,                -- ESPN abbreviation, e.g. 'BUF'
  away_team      TEXT NOT NULL,
  home_name      TEXT NOT NULL,                -- ESPN displayName, e.g. 'Buffalo Bills' (used for odds matching)
  away_name      TEXT NOT NULL,
  home_espn_id   TEXT NOT NULL,                -- ESPN team IDs (rosters)
  away_espn_id   TEXT NOT NULL,
  kickoff_utc    TEXT NOT NULL,                -- RFC 3339 UTC
  status         TEXT NOT NULL,                -- SCHEDULED, LIVE, FINAL
  venue_id       TEXT,
  venue_name     TEXT,
  is_indoor      INTEGER NOT NULL,
  temp_f         REAL,
  wind_mph       REAL,
  wind_gust_mph  REAL,
  precip_in      REAL,
  home_score     INTEGER,
  away_score     INTEGER,
  injuries_json  TEXT,                         -- raw ESPN injuries block
  updated_at     TEXT NOT NULL
);
CREATE INDEX games_week ON games(week_id, kickoff_utc);

CREATE TABLE odds (
  game_id     TEXT NOT NULL REFERENCES games(id),
  book        TEXT NOT NULL,                   -- Odds API bookmaker key
  market_type TEXT NOT NULL,                   -- product.md market names
  team        TEXT NOT NULL DEFAULT '',        -- ESPN abbreviation; '' for TOTAL
  player_name TEXT NOT NULL DEFAULT '',        -- '' for game markets
  side        TEXT NOT NULL,                   -- TEAM, OVER, UNDER, YES
  line        REAL,                            -- NULL for ML and ANYTIME_TD
  price       INTEGER NOT NULL,                -- American odds
  fetched_at  TEXT NOT NULL,
  PRIMARY KEY (game_id, book, market_type, team, player_name, side)
);

CREATE TABLE slips (
  id              INTEGER PRIMARY KEY,
  week_id         INTEGER NOT NULL REFERENCES weeks(id),
  name            TEXT NOT NULL,
  legs_per_member INTEGER NOT NULL DEFAULT 2 CHECK (legs_per_member BETWEEN 1 AND 5),
  status          TEXT NOT NULL DEFAULT 'OPEN',     -- OPEN, LOCKED, GRADED
  result          TEXT NOT NULL DEFAULT 'PENDING',  -- PENDING, WON, LOST, PUSH
  created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
  UNIQUE (week_id, name)
);

CREATE TABLE slip_members (
  slip_id   INTEGER NOT NULL REFERENCES slips(id) ON DELETE CASCADE,
  user_id   INTEGER NOT NULL REFERENCES users(id),
  locked_at TEXT,                              -- set by "Lock in"
  PRIMARY KEY (slip_id, user_id)
);

CREATE TABLE legs (
  id          INTEGER PRIMARY KEY,
  slip_id     INTEGER NOT NULL REFERENCES slips(id) ON DELETE CASCADE,
  user_id     INTEGER NOT NULL REFERENCES users(id),
  game_id     TEXT REFERENCES games(id),       -- NULL only on past-week slips
  market_type TEXT NOT NULL,
  selection   TEXT NOT NULL,                   -- generated display text (free text for OTHER)
  team        TEXT,                            -- ESPN abbreviation
  player_name TEXT,
  player_id   TEXT,                            -- ESPN athlete ID when matched
  side        TEXT,                            -- TEAM, OVER, UNDER, YES
  line        REAL,
  odds        INTEGER NOT NULL DEFAULT -110,
  result      TEXT NOT NULL DEFAULT 'PENDING', -- PENDING, NEEDS_GRADING, WON, LOST, PUSH, VOID
  graded_by   TEXT,                            -- 'auto' or a user name
  created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);
CREATE INDEX legs_slip ON legs(slip_id);

CREATE TABLE wagers (
  slip_id   INTEGER NOT NULL REFERENCES slips(id) ON DELETE CASCADE,
  user_id   INTEGER NOT NULL REFERENCES users(id),
  book      TEXT,
  stake     REAL NOT NULL CHECK (stake > 0),
  odds      INTEGER NOT NULL,
  boost_pct REAL NOT NULL DEFAULT 0,
  PRIMARY KEY (slip_id, user_id)
);

CREATE TABLE player_matches (
  odds_name       TEXT NOT NULL,               -- normalized name, see data-sources.md
  team            TEXT NOT NULL,               -- ESPN abbreviation
  espn_athlete_id TEXT NOT NULL,
  manual          INTEGER NOT NULL DEFAULT 0,  -- 1 = fixed by hand; never overwritten by matching
  PRIMARY KEY (odds_name, team)
);

CREATE TABLE activity (
  id         INTEGER PRIMARY KEY,
  slip_id    INTEGER NOT NULL REFERENCES slips(id) ON DELETE CASCADE,
  user_id    INTEGER REFERENCES users(id),     -- NULL for system entries (e.g. auto-grading)
  message    TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);
CREATE INDEX activity_slip ON activity(slip_id, id DESC);

CREATE TABLE sync_state (
  source                 TEXT PRIMARY KEY,     -- see data-sources.md "Sources"
  fetched_at             TEXT NOT NULL,
  odds_credits_remaining INTEGER               -- set by Odds API sources only
);
```

## Conventions

- Timestamps are RFC 3339 UTC text. Parse them with `time.RFC3339`.
- Booleans are `INTEGER` 0/1.
- A leg's kickoff lock comes from `games.kickoff_utc`; it is not stored.
- A slip's reference odds are not stored; any view that shows them computes them.
- `slips.status` and `slips.result` stay in step with lock and leg-result changes.
