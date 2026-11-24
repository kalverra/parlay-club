-- Add opponent metadata on player_gamelogs and matchup/stat columns on nflverse_player_weeks.
ALTER TABLE player_gamelogs ADD COLUMN games_json TEXT NOT NULL DEFAULT '[]';

ALTER TABLE nflverse_player_weeks ADD COLUMN opponent    TEXT    NOT NULL DEFAULT '';
ALTER TABLE nflverse_player_weeks ADD COLUMN player_name TEXT    NOT NULL DEFAULT '';
ALTER TABLE nflverse_player_weeks ADD COLUMN position    TEXT    NOT NULL DEFAULT '';
ALTER TABLE nflverse_player_weeks ADD COLUMN pass_att    INTEGER NOT NULL DEFAULT 0;
ALTER TABLE nflverse_player_weeks ADD COLUMN pass_yds    INTEGER NOT NULL DEFAULT 0;
ALTER TABLE nflverse_player_weeks ADD COLUMN pass_tds    INTEGER NOT NULL DEFAULT 0;
ALTER TABLE nflverse_player_weeks ADD COLUMN rush_yds    INTEGER NOT NULL DEFAULT 0;
ALTER TABLE nflverse_player_weeks ADD COLUMN rush_tds    INTEGER NOT NULL DEFAULT 0;
ALTER TABLE nflverse_player_weeks ADD COLUMN receptions  INTEGER NOT NULL DEFAULT 0;
ALTER TABLE nflverse_player_weeks ADD COLUMN rec_yds     INTEGER NOT NULL DEFAULT 0;
ALTER TABLE nflverse_player_weeks ADD COLUMN rec_tds     INTEGER NOT NULL DEFAULT 0;
ALTER TABLE nflverse_player_weeks ADD COLUMN kicker_pts  INTEGER NOT NULL DEFAULT 0;

CREATE INDEX idx_nflverse_player_weeks_opp_pos
  ON nflverse_player_weeks (season, season_type, opponent, position, week_num DESC);
