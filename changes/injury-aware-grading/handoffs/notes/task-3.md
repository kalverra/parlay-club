## Done

- Updated `docs/reference/schema.md` for `0014_roster_injuries.sql` (version 14), adding `injury_status` and `injury_date` to the `roster_players` DDL, summary, migrations list, tables summary, and conventions.
- Updated `docs/reference/data-sources.md` under ESPN `Roster` to document `InjuryStatus`, `InjuryDate`, status-priority selection (`Out` > `Doubtful` > `Questionable` > `Injured Reserve` > first non-empty), and clearing on empty `injuries`.
- Shortened both docs' front-matter `summary` fields to pass `good-vibes docs lint` and ran `good-vibes docs index`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

Phase 1 is complete; Phase 2 can build `espn.MergeTeamInjuries`, `analyzer.MergeGameInjuries`, and the `web.boardSignalsForGame` / `web.loadGameResearch` integrations on top of the documented `roster_players` injury columns.
