-- Keep every successful outside response for later modeling. Bodies are keyed
-- by the hash of the uncompressed bytes and stored gzipped, so a source polled
-- on a schedule that returns the same JSON stores one body and a small row per
-- call. The app never updates or deletes these rows.
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
