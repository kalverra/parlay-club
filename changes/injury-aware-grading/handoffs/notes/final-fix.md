## Commit

Fix pick board injury fixture and slips concept doc

The pick board injury tests seeded a flat JSON shape that failed ESPN summary parsing and masked the summary-plus-roster merge path. Updating the fixture to the raw ESPN summary schema exercises the real merge behavior, and updating the slips concept doc keeps the pick board and Game Research descriptions aligned with the merged injury loader.
