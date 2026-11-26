-- reminder_tasks remembers the slip start each open slip's reminder was
-- scheduled for. A row keeps a refresh pass cheap: it reads one row per slip
-- instead of enqueuing the same Cloud Tasks job again. A stale task is harmless
-- because the reminder endpoint rechecks the slip and its start.
CREATE TABLE reminder_tasks (
  slip_id       INTEGER PRIMARY KEY REFERENCES slips(id) ON DELETE CASCADE,
  start_utc     TEXT NOT NULL,              -- RFC 3339 UTC; the slip's first kickoff
  scheduled_at  TEXT NOT NULL,              -- RFC 3339 UTC; start minus the reminder lead
  updated_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);
