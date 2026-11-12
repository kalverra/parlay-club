---
id: schema
title: Database schema
type: reference
summary: SQLite tables, including groups and their members, legs and their votes, odds rows with sportsbook deep links, rosters and player game logs for props research, player name matching, nfelo model projections, the raw external response archive, how migrations are tracked and applied, and connection settings.
tags: [database, sqlite, migrations, groups, archive, api, raw-responses, odds, links, nfelo, projections, roster, gamelog, player-matching, injuries, votes]
sources: [internal/db/migrations/*.sql, internal/db/migrate.go, internal/db/db.go, internal/db/odds.go, internal/db/groups.go, internal/db/nfelo.go, internal/db/players.go, internal/db/votes.go, internal/db/archive.go, internal/db/archive_read.go, internal/ingest/archive/transport.go]
last_verified: 2026-11-12
---

# Database schema

## Summary

One SQLite file, opened by `internal/db.Open`, holds every table the app uses. `0001_init.sql` creates the tables,
including groups, their members, and `slips.group_id`. `0002_google_identity.sql` rebuilds `users` to add Google
identity columns and drop the unique name rule. `0003_api_archive.sql` adds the raw response archive, `api_bodies` and
`api_responses`; see [Archive](../concepts/archive.md). `0004_odds_links.sql` adds each book's deep link and selection
ID to `odds`, and `0005_odds_event_sid.sql` adds the book's own event ID. `0006_nfelo_games.sql` adds `nfelo_games`,
one model projection per game. `0007_props_research.sql` adds `roster_players` and `player_gamelogs` for props research.
`0008_leg_votes.sql` adds `leg_votes`, the up and down votes members cast on each other's legs.
`0011_logos_portraits.sql` adds `home_logo` and `away_logo` to `games` and `headshot_url` to `roster_players`.

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
- A fresh database ends at version 8 with every table below.
- `0001_init.sql` creates `groups`, `group_members`, and `slips.group_id`. A database created before groups existed has
  the old `slips` shape, so delete it and let the migrations rebuild.
- `0002_google_identity.sql` rebuilds `users` in its transaction with the SQLite 12-step method and
  `PRAGMA defer_foreign_keys = ON`, so the drop and rename keep every child row valid. Existing rows keep their `id`,
  name, color, home book, active flag, and creation time, and come back with no Google identity.
- `0003_api_archive.sql` creates the two archive tables and their two indexes. It changes no existing table.
- `0004_odds_links.sql` adds nullable `link` and `sid` columns to `odds`. On an existing database every stored price
  keeps its row and gets NULL links until the next sync fills them.
- `0005_odds_event_sid.sql` adds a nullable `event_sid` column to `odds`. It holds the bookmaker block's `sid`, the
  book's own ID for the event. Existing rows get NULL until the next sync.
- `0006_nfelo_games.sql` creates `nfelo_games`, keyed by `game_id` with `ON DELETE CASCADE` to `games`. A game without a
  matching nfelo row simply has no projection, and a game that drops out of the CSV keeps its last one. It changes no
  existing table.
- `0007_props_research.sql` creates `roster_players` and `player_gamelogs` for ESPN NFL team rosters and athlete
  game logs.
- `0008_leg_votes.sql` creates `leg_votes`, one row per member per leg they voted on, cascading from `legs`. It changes
  no existing table.
- `0011_logos_portraits.sql` adds nullable `home_logo` and `away_logo` columns to `games` and a
  `headshot_url TEXT NOT NULL DEFAULT ''` column to `roster_players`.
- Never edit an applied file. Add `0012_name.sql` and up.
- `PRAGMA user_version` on the database file is the only record of what ran.

## Tables

| Table             | Holds                                                        |
| ----------------- | ------------------------------------------------------------ |
| `groups`          | One club: its name, invite token, and creation time.          |
| `group_members`   | Which people belong to which group and when they joined.      |
| `users`           | People, their Google identity, avatar color, home book, and active flag. |
| `weeks`           | NFL weeks, keyed `season*1000 + season_type*100 + week_num`.  |
| `games`           | One row per ESPN event: teams, kickoff, venue, weather, scores. |
| `odds`            | Odds API prices and sportsbook deep links per game, book, market, and selection. |
| `nfelo_games`     | nfelo's pre-game model spread, win probability, and QB adjustments per game. |
| `slips`           | One shared parlay per group per week, with status and result.  |
| `slip_members`    | Who is on a slip and when they locked in.                     |
| `legs`            | Picks on a slip, with market, selection, line, odds, and result. |
| `leg_votes`       | Up and down votes members cast on each other's legs.          |
| `wagers`          | Optional stake, odds, and boost per member per slip.          |
| `player_matches`  | Odds player name to ESPN athlete ID, per team.                |
| `roster_players`  | ESPN NFL team rosters by athlete ID with normalized names.    |
| `player_gamelogs` | ESPN athlete game logs and market stats by athlete ID.        |
| `activity`        | Human-readable slip history; `user_id` is NULL for system entries. |
| `sync_state`      | Last fetch time per outside data source, plus remaining odds credits. |
| `api_bodies`      | Raw external response bodies, gzipped and keyed by the SHA-256 of the uncompressed bytes. |
| `api_responses`   | One row per archived external call: source, key, redacted URL, status, time, and body hash. |

## Users and identity

`users.name` is no longer unique. Google display names collide, so two people can share one, and `users_name` is a plain
case-insensitive index for lookups. A user's display name changes only by hand; sign-in never renames a linked row.

Three nullable columns hold the Google identity:

- `google_sub` is Google's stable subject and is unique. It is `NULL` for a history-import user who has not signed in.
- `email` is the verified Google address, unique and compared without case.
- `picture_url` is the Google profile picture. It refreshes on every sign-in.

`auth.SignIn` is the only writer of these columns. It links by `google_sub`, then by `email`, and otherwise creates a
user and their "Just me" group. An unlinked row keeps its history when it gains an identity. See
[Access](../concepts/access.md) for the full resolution order.

`active` still turns a person off without deleting their slips and legs. `avatar_color` still comes from the fixed
palette, and a linked user keeps the color they already have.

## Groups and membership

A group is one club. `groups.name` is not unique; `invite_token` is unique and holds 32 lowercase hex characters, or 128
random bits. Resetting an invite replaces the token, which kills the old link.

`group_members` pairs a group with a user, with `joined_at` recording the order. Leaving deletes only that row, so the
person's legs stay on the group's slips. Every signed-in person belongs to at least one group: `auth.SignIn` creates the
"Just me" group in the same transaction as a new user.

`slips.group_id` is `NOT NULL` and points at `groups`, so a slip always has a home. The unique key is
`(group_id, week_id, name)`, so two groups can both have "Week 9" in the same week. Weeks, games, odds, player matches,
and `sync_state` are global: every group reads the same slate and the same cached prices.

## DDL

The definitions below match `internal/db/migrations/`, which is the source of truth.

```sql
CREATE TABLE users (
  id           INTEGER PRIMARY KEY,
  name         TEXT NOT NULL COLLATE NOCASE,
  avatar_color TEXT NOT NULL,                  -- Tailwind color name, e.g. 'sky'
  home_book    TEXT,                           -- Odds API bookmaker key; NULL = median
  active       INTEGER NOT NULL DEFAULT 1,
  created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
  google_sub   TEXT UNIQUE,                    -- stable OpenID Connect subject
  email        TEXT UNIQUE COLLATE NOCASE,     -- verified Google email
  picture_url  TEXT                            -- Google profile picture
);
CREATE INDEX users_name ON users(name COLLATE NOCASE);

CREATE TABLE groups (
  id           INTEGER PRIMARY KEY,
  name         TEXT NOT NULL,                  -- not unique
  invite_token TEXT NOT NULL UNIQUE,           -- 128-bit random, hex
  created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE TABLE group_members (
  group_id  INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
  user_id   INTEGER NOT NULL REFERENCES users(id),
  joined_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
  PRIMARY KEY (group_id, user_id)
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
  odds_event_id  TEXT UNIQUE,                  -- matched odds provider event ID (primary wins)
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
  updated_at     TEXT NOT NULL,
  home_logo      TEXT,                         -- ESPN team logo URL (https:// only)
  away_logo      TEXT                          -- ESPN team logo URL (https:// only)
);
CREATE INDEX games_week ON games(week_id, kickoff_utc);

CREATE TABLE nfelo_games (
  game_id       TEXT PRIMARY KEY REFERENCES games(id) ON DELETE CASCADE,
  home_spread   REAL NOT NULL,                 -- home perspective; negative = home favored
  home_win_prob REAL NOT NULL,                 -- 0..1
  home_qb       TEXT NOT NULL,
  away_qb       TEXT NOT NULL,
  home_qb_adj   REAL NOT NULL,                 -- Elo points
  away_qb_adj   REAL NOT NULL,
  updated_at    TEXT NOT NULL                  -- RFC 3339 UTC
);

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
  link        TEXT,                            -- book's deep link for this outcome; NULL when none
  sid         TEXT,                            -- book's selection ID for this outcome; NULL when none
  event_sid   TEXT,                            -- book's own event ID; NULL when none
  PRIMARY KEY (game_id, book, market_type, team, player_name, side)
);

CREATE TABLE slips (
  id              INTEGER PRIMARY KEY,
  group_id        INTEGER NOT NULL REFERENCES groups(id),
  week_id         INTEGER NOT NULL REFERENCES weeks(id),
  name            TEXT NOT NULL,
  legs_per_member INTEGER NOT NULL DEFAULT 2 CHECK (legs_per_member BETWEEN 1 AND 5),
  status          TEXT NOT NULL DEFAULT 'OPEN',     -- OPEN, LOCKED, GRADED
  result          TEXT NOT NULL DEFAULT 'PENDING',  -- PENDING, WON, LOST, PUSH
  created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
  UNIQUE (group_id, week_id, name)
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

CREATE TABLE leg_votes (
  leg_id     INTEGER NOT NULL REFERENCES legs(id) ON DELETE CASCADE,
  user_id    INTEGER NOT NULL REFERENCES users(id),
  value      INTEGER NOT NULL CHECK (value IN (-1, 1)), -- 1 up, -1 down; clearing deletes the row
  updated_at TEXT NOT NULL,
  PRIMARY KEY (leg_id, user_id)
);

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

CREATE TABLE roster_players (
  athlete_id   TEXT PRIMARY KEY,
  team         TEXT NOT NULL,
  espn_team_id TEXT NOT NULL,
  full_name    TEXT NOT NULL,
  norm_name    TEXT NOT NULL,
  position     TEXT NOT NULL DEFAULT '',
  updated_at   TEXT NOT NULL,
  headshot_url TEXT NOT NULL DEFAULT ''        -- ESPN athlete headshot URL (https:// only)
);
CREATE INDEX roster_players_team_norm ON roster_players(team, norm_name);

CREATE TABLE player_gamelogs (
  athlete_id   TEXT PRIMARY KEY,
  games_played INTEGER NOT NULL DEFAULT 0,
  stats_json   TEXT NOT NULL DEFAULT '{}',
  updated_at   TEXT NOT NULL
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

CREATE TABLE api_bodies (
  sha256   TEXT PRIMARY KEY,     -- hex of the uncompressed body
  encoding TEXT NOT NULL,        -- 'gzip'
  size     INTEGER NOT NULL,     -- uncompressed bytes
  body     BLOB NOT NULL
);

CREATE TABLE api_responses (
  id                INTEGER PRIMARY KEY,
  source            TEXT NOT NULL,       -- espn | weather | odds
  key               TEXT NOT NULL,       -- sync_state source key, or host+path
  method            TEXT NOT NULL,
  url               TEXT NOT NULL,       -- apiKey redacted
  status            INTEGER NOT NULL,
  fetched_at        TEXT NOT NULL,
  duration_ms       INTEGER NOT NULL,
  body_sha256       TEXT NOT NULL REFERENCES api_bodies(sha256),
  credits_remaining INTEGER,             -- x-requests-remaining, odds only
  credits_used      INTEGER,             -- x-requests-last, odds only
  CHECK (status BETWEEN 200 AND 299)
);
CREATE INDEX api_responses_key ON api_responses(key, fetched_at);
CREATE INDEX api_responses_source ON api_responses(source, fetched_at);
```

## API archive

`api_responses` holds one row per archived external call. `body_sha256` points at `api_bodies`, where the body is
gzipped and stored once per distinct uncompressed payload. A source that polls on a schedule and returns the same JSON
adds a response row but no second body. The app inserts these rows and never updates or deletes them.

Both writes happen in one transaction, so a response row never points at a missing body. `status` only ever holds a 2xx
code, and `url` has The Odds API's `apiKey` value replaced by `REDACTED`. `credits_remaining` and `credits_used` are set
for `source = 'odds'` and stay `NULL` for the other sources. See [Archive](../concepts/archive.md) for what is kept and
how to read it.

## Conventions

- Timestamps are RFC 3339 UTC text. Parse them with `time.RFC3339`.
- Booleans are `INTEGER` 0/1.
- `api_bodies.body` is gzipped unless `encoding` says otherwise. Decompress it before treating it as JSON.
- A leg's kickoff lock comes from `games.kickoff_utc`; it is not stored.
- `odds.link`, `odds.sid`, and `odds.event_sid` belong to the outcome row, so they refresh in the same upsert as
  `price`. All stay NULL when the book returned no deep link or event ID; a missing link is normal. `event_sid` is the
  bookmaker block's own ID for the event, used by DraftKings' combined event-page link.
- `nfelo_games` holds one global row per game. A sync upserts it, and `updated_at` moves only when a stored value
  changes, so rerunning with the same CSV is a no-op.
- A slip's reference odds are not stored; any view that shows them computes them.
- `leg_votes` holds only cast votes. Clearing one deletes its row, and deleting a leg cascades. A vote changes no slip or
  leg field and writes no activity entry.
- `slips.status` and `slips.result` stay in step with lock and leg-result changes.
- `groups`, `group_members`, `slips`, and the rows that hang off a slip are scoped to one group. Weeks, games, odds,
  nfelo projections, player matches, rosters, player game logs, and `sync_state` are global.
- `roster_players` stores team roster athletes with `norm_name` for fast matching and `headshot_url` (`''` when
  missing).
- `games.home_logo` and `games.away_logo` store validated `https://` ESPN team logo URLs (`NULL` when missing).
- `player_gamelogs.stats_json` stores parsed season market values as a JSON map of float arrays, ordered most recent first.
- `player_matches` maps normalized sportsbook player names and teams to ESPN athlete IDs. A row with `manual = 1` was
  corrected by a user and is never overwritten by automated matching.
- `games.injuries_json` stores the raw ESPN summary injuries JSON array.
- An invite token is a bearer secret. Do not log it or echo it in an error.
