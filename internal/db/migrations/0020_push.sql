-- Browser push subscriptions. One row per device endpoint, so a person can
-- have several. endpoint is unique because an endpoint identifies one device;
-- a second person subscribing on a shared phone reassigns the row.
CREATE TABLE push_subscriptions (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  endpoint   TEXT NOT NULL UNIQUE,
  p256dh     TEXT NOT NULL,              -- browser public key, base64url
  auth       TEXT NOT NULL,              -- browser auth secret, base64url
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);
CREATE INDEX push_subscriptions_user ON push_subscriptions(user_id, created_at);

-- push_log dedupes sends. A row is reserved before the send, so a failed or
-- retried push is never repeated. kind is 'graded' or 'reminder', slip_id names
-- the slip, user_id is 0 for a slip-wide row, and key carries a per-kind
-- discriminator such as the kickoff time.
CREATE TABLE push_log (
  kind    TEXT NOT NULL,
  slip_id INTEGER NOT NULL,
  user_id INTEGER NOT NULL DEFAULT 0,
  "key"   TEXT NOT NULL DEFAULT '',
  sent_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
  PRIMARY KEY (kind, slip_id, user_id, "key")
);
