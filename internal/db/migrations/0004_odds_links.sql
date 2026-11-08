-- Keep the sportsbook's own deep link and selection ID on the same row as the
-- price. Both are nullable: The Odds API returns them per outcome only when
-- the request asks for them and the book supplies them, and a missing value is
-- normal. Refreshing a price refreshes the link with it.
ALTER TABLE odds ADD COLUMN link TEXT;
ALTER TABLE odds ADD COLUMN sid TEXT;
