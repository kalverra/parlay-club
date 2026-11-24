## Commit

Clean up research view, position helper, and docs

Prepared nflverse upsert statements once per transaction to prevent lock-contention timeouts under `-race`, removed the six unused legacy `PlayerResearchView` fields left over from the template migration, consolidated position normalization into `db.NormalizePosition`, and updated `docs/reference/routes.md` to match the shipped player research view.
