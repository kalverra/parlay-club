# Task 4: Docs for canonical player normalization and `(TEAM)` tag stripping

## Done

- Updated [data-sources.md](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/canonical-player-identity/docs/reference/data-sources.md) to document `StripPlayerTag` `(TEAM)` tag stripping, accent folding in `NormalizePlayerName`, per-game batch prop resolution (`mapPropsToOdds`, `RosterForTeams`, `PlayerMatchesForTeams`, `UpsertPlayerMatches`), canonical `odds.player_name`/`odds.team` rewriting, unmatched casing unification, and same-book collision deduplication.
- Updated [schema.md](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/canonical-player-identity/docs/reference/schema.md) to document migration `0013_canonical_players.sql` and its `backfillCanonicalPlayersTx` hook, bump the fresh database version to 13, and document canonical `odds.player_name`/`odds.team` and `roster_players.norm_name` conventions.
- Cleared three pre-existing `names-path` lint errors on [schema.md](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/canonical-player-identity/docs/reference/schema.md) so `good-vibes docs lint` passes with 0 errors.

## Deviations

Fixed three pre-existing `names-path` references in `docs/reference/schema.md` (`internal/db.Open` and `internal/db/migrations/NNNN_name.sql`) that `good-vibes docs lint` promoted from warnings to errors once the file was modified.

## Open issues

None.

## Next-phase notes

- Phase 2 documentation is complete and `good-vibes docs lint` passes with 0 errors.
