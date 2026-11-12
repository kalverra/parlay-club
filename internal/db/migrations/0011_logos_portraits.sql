-- Add team logo URLs on games and player headshot URLs on roster_players.
ALTER TABLE games ADD COLUMN home_logo TEXT;
ALTER TABLE games ADD COLUMN away_logo TEXT;
ALTER TABLE roster_players ADD COLUMN headshot_url TEXT NOT NULL DEFAULT '';
