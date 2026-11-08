-- The book's own ID for the event. DraftKings needs it in the event-page
-- combined link (/event/{event_sid}?outcomes=...), and it comes from the
-- bookmaker block rather than from any outcome. Nullable: a book may omit it.
ALTER TABLE odds ADD COLUMN event_sid TEXT;
