-- Explicit eligible games for one-off custom slips.
CREATE TABLE slip_games (
  slip_id INTEGER NOT NULL REFERENCES slips(id) ON DELETE CASCADE,
  game_id TEXT NOT NULL REFERENCES games(id),
  PRIMARY KEY (slip_id, game_id)
);

CREATE INDEX idx_slip_games_slip_id ON slip_games(slip_id);
