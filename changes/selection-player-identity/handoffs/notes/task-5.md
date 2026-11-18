## Done

- Updated [docs/concepts/slips.md](docs/concepts/slips.md) to document unified `db.SameSelection`, `db.SamePlayer`, `db.SameLineGroup`, `db.MatchBookOdds` (used by both sportsbook export and slip reference odds), shared selection formatting (`db.MarketLabel`, `db.FormatSelection`, `db.BuildLegSelection`), in-memory game research resolution via `db.BatchPlayerResolver`, and grader player-name normalization (`db.IsPlayerMarket`, `db.StripPlayerTag`, `db.NormalizePlayerName`).
- Updated [docs/reference/data-sources.md](docs/reference/data-sources.md) under **Player matching** to document `db.BatchPlayerResolver` as the single player-matching engine across prop ingest (`mapPropsToOdds`), game research (`loadGameResearch`), and `Store.ResolvePlayerMatch`, plus `ResolvedPlayer` metadata (`Position`, `HeadshotURL`) and board/grader alignment.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- All Phase 3 tasks (including docs) are complete.
