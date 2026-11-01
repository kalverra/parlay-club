# Schema

SQLite via `modernc.org/sqlite`. Connection pragmas are in architecture.md "Database access".

## Migrations

- Files: `internal/db/migrations/NNNN_name.sql`, embedded, applied in order inside one transaction each.
- The applied version is `PRAGMA user_version`. A migration runs only when its number is greater.
- `0001_init.sql` creates every table below, including ones later changes use, so early changes need no migrations.
  Later schema changes add new files; never edit an applied file.

## Tables

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

## Notes

- Timestamps are RFC 3339 UTC text. Parse them with `time.RFC3339`.
- Booleans are `INTEGER` 0/1.
- A leg's kickoff lock comes from `games.kickoff_utc`; it is not stored.
- The slip's reference odds are computed when rendered (product.md "Odds and payout"), not stored.
- `slips.status` and `slips.result` are written by `internal/slips` whenever locks or leg results change.
