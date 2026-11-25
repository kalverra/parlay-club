-- Add v2 expected-opportunity, carry-share, and CPOE columns on nflverse_player_weeks.
ALTER TABLE nflverse_player_weeks ADD COLUMN exp_pass_yds   REAL NOT NULL DEFAULT 0;
ALTER TABLE nflverse_player_weeks ADD COLUMN exp_rush_yds   REAL NOT NULL DEFAULT 0;
ALTER TABLE nflverse_player_weeks ADD COLUMN exp_rec_yds    REAL NOT NULL DEFAULT 0;
ALTER TABLE nflverse_player_weeks ADD COLUMN exp_receptions REAL NOT NULL DEFAULT 0;
ALTER TABLE nflverse_player_weeks ADD COLUMN exp_pass_tds   REAL NOT NULL DEFAULT 0;
ALTER TABLE nflverse_player_weeks ADD COLUMN exp_rush_tds   REAL NOT NULL DEFAULT 0;
ALTER TABLE nflverse_player_weeks ADD COLUMN exp_rec_tds    REAL NOT NULL DEFAULT 0;
ALTER TABLE nflverse_player_weeks ADD COLUMN carry_share    REAL NOT NULL DEFAULT 0;
ALTER TABLE nflverse_player_weeks ADD COLUMN cpoe           REAL NOT NULL DEFAULT 0;
