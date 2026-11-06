-- Add Google identity columns and drop the unique name rule, because Google
-- display names collide. SQLite cannot drop a column constraint, so rebuild the
-- table with the 12-step method. The migration runner already wraps this file in
-- a transaction; deferring foreign-key checks lets the child tables point at
-- users throughout the swap. The old rows wait in a temp table and return after
-- the rename, which is how the deferred checks are satisfied.
PRAGMA defer_foreign_keys = ON;

CREATE TEMP TABLE users_0002_backup AS
SELECT id, name, avatar_color, home_book, active, created_at
FROM users;

CREATE TABLE users_new (
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

DELETE FROM users;
DROP TABLE users;

ALTER TABLE users_new RENAME TO users;

INSERT INTO users (id, name, avatar_color, home_book, active, created_at)
SELECT id, name, avatar_color, home_book, active, created_at
FROM users_0002_backup;

DROP TABLE users_0002_backup;

CREATE INDEX users_name ON users(name COLLATE NOCASE);
