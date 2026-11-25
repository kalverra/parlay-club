-- Add live game period, clock, and cached box score JSON columns on games.
ALTER TABLE games ADD COLUMN period INTEGER;
ALTER TABLE games ADD COLUMN clock TEXT;
ALTER TABLE games ADD COLUMN boxscore_json TEXT;
