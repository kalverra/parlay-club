-- ESPN NFL rosters and athlete gamelogs for props research and matching.
CREATE TABLE roster_players (
  athlete_id   TEXT PRIMARY KEY,
  team         TEXT NOT NULL,
  espn_team_id TEXT NOT NULL,
  full_name    TEXT NOT NULL,
  norm_name    TEXT NOT NULL,
  position     TEXT NOT NULL DEFAULT '',
  updated_at   TEXT NOT NULL
);
CREATE INDEX roster_players_team_norm ON roster_players(team, norm_name);

CREATE TABLE player_gamelogs (
  athlete_id   TEXT PRIMARY KEY,
  games_played INTEGER NOT NULL DEFAULT 0,
  stats_json   TEXT NOT NULL DEFAULT '{}',
  updated_at   TEXT NOT NULL
);
