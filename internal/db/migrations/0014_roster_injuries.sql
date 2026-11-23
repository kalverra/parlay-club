-- Add roster injury status and date columns on roster_players.
ALTER TABLE roster_players ADD COLUMN injury_status TEXT NOT NULL DEFAULT '';
ALTER TABLE roster_players ADD COLUMN injury_date   TEXT NOT NULL DEFAULT '';
