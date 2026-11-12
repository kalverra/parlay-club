-- Recurring slip schedules for a group. When resolving a group's active week,
-- EnsureWeekSlips creates slips for each active schedule.
CREATE TABLE group_slip_schedules (
  id              INTEGER PRIMARY KEY,
  group_id        INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
  name            TEXT NOT NULL,
  slate_scope     TEXT NOT NULL DEFAULT 'sunday_main',
  legs_per_member INTEGER NOT NULL DEFAULT 2 CHECK (legs_per_member BETWEEN 1 AND 5),
  is_active       INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0, 1)),
  created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
  UNIQUE (group_id, name)
);

-- Add slate_scope and schedule_id to slips to isolate deadlines and link recurring slips.
-- Existing slips default to sunday_main. One-off slips have schedule_id NULL.
ALTER TABLE slips ADD COLUMN slate_scope TEXT NOT NULL DEFAULT 'sunday_main';
ALTER TABLE slips ADD COLUMN schedule_id INTEGER REFERENCES group_slip_schedules(id) ON DELETE SET NULL;

CREATE INDEX idx_slips_schedule_id ON slips(schedule_id);

-- Backfill existing groups with the default active Sunday slip schedule.
INSERT INTO group_slip_schedules (group_id, name, slate_scope, legs_per_member, is_active)
SELECT id, 'Main Sunday Slip', 'sunday_main', 2, 1
FROM groups;

-- Link each group's existing newest slip to the backfilled schedule so EnsureWeekSlips
-- matches it instead of creating a duplicate.
UPDATE slips
SET schedule_id = (
  SELECT s.id
  FROM group_slip_schedules s
  WHERE s.group_id = slips.group_id AND s.name = 'Main Sunday Slip'
)
WHERE id IN (
  SELECT s1.id
  FROM slips s1
  WHERE s1.id = (
    SELECT s2.id FROM slips s2
    WHERE s2.group_id = s1.group_id
    ORDER BY s2.week_id DESC, s2.created_at DESC, s2.id DESC
    LIMIT 1
  )
);
