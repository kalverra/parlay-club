## Commit

Fix prop enrichment merge, prop-leg helper, and docs

Normalize player names in `oddsRowKey` so `enrichGameProps` merges unresolved provider name variants instead of leaving unlinked duplicates. Share `db.IsPropLeg` across packages to keep the prop-market rule in one place, and trim duplicate refresh details from `docs/reference/data-sources.md`.
