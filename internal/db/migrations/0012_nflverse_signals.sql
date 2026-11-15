-- nflverse player ID crosswalk (ESPN <-> GSIS <-> PFR) and weekly team/player
-- efficiency and usage signals.

CREATE TABLE nflverse_player_ids (
  espn_id    TEXT PRIMARY KEY,
  gsis_id    TEXT NOT NULL,
  pfr_id     TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL                  -- RFC 3339 UTC
);

CREATE INDEX idx_nflverse_player_ids_gsis ON nflverse_player_ids (gsis_id);

CREATE TABLE nflverse_team_weeks (
  season               INTEGER NOT NULL,
  season_type          INTEGER NOT NULL,    -- 2 = regular, 3 = postseason
  week_num             INTEGER NOT NULL,
  team                 TEXT    NOT NULL,    -- ESPN abbreviation
  off_pass_epa         REAL    NOT NULL,
  off_rush_epa         REAL    NOT NULL,
  def_pass_epa_allowed REAL    NOT NULL,
  def_rush_epa_allowed REAL    NOT NULL,
  plays_per_game       REAL    NOT NULL,
  pass_rate            REAL    NOT NULL,
  updated_at           TEXT    NOT NULL,    -- RFC 3339 UTC
  PRIMARY KEY (season, season_type, week_num, team)
);

CREATE TABLE nflverse_player_weeks (
  espn_id         TEXT    NOT NULL,
  season          INTEGER NOT NULL,
  season_type     INTEGER NOT NULL,         -- 2 = regular, 3 = postseason
  week_num        INTEGER NOT NULL,
  team            TEXT    NOT NULL,         -- ESPN abbreviation
  offense_snaps   INTEGER NOT NULL,
  offense_pct     REAL    NOT NULL,
  target_share    REAL    NOT NULL,
  air_yards_share REAL    NOT NULL,
  wopr            REAL    NOT NULL,
  carries         INTEGER NOT NULL,
  targets         INTEGER NOT NULL,
  total_epa       REAL    NOT NULL,
  rz_opps         INTEGER NOT NULL DEFAULT 0,
  updated_at      TEXT    NOT NULL,         -- RFC 3339 UTC
  PRIMARY KEY (espn_id, season, season_type, week_num)
);
