-- Up and down votes on a leg. A vote is a tally that changes nothing about the
-- slip or the leg, so it never writes an activity entry. Clearing a vote deletes
-- the row; deleting a leg cascades.
CREATE TABLE leg_votes (
  leg_id     INTEGER NOT NULL REFERENCES legs(id) ON DELETE CASCADE,
  user_id    INTEGER NOT NULL REFERENCES users(id),
  value      INTEGER NOT NULL CHECK (value IN (-1, 1)),
  updated_at TEXT NOT NULL,
  PRIMARY KEY (leg_id, user_id)
);
