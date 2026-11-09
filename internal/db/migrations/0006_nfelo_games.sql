-- nfelo's pre-game model projection for one game. One row per game; the slate
-- sync never deletes rows here, so a game that drops out of the CSV keeps its
-- last projection. updated_at only moves when a value changes.
CREATE TABLE nfelo_games (
  game_id       TEXT PRIMARY KEY REFERENCES games(id) ON DELETE CASCADE,
  home_spread   REAL NOT NULL,                 -- home perspective; negative = home favored
  home_win_prob REAL NOT NULL,                 -- 0..1
  home_qb       TEXT NOT NULL,
  away_qb       TEXT NOT NULL,
  home_qb_adj   REAL NOT NULL,                 -- Elo points
  away_qb_adj   REAL NOT NULL,                 -- Elo points
  updated_at    TEXT NOT NULL                  -- RFC 3339 UTC
);
