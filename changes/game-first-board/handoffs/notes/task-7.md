# Task 7 notes

## Done

- Added `board.Pair(game, rows) []LineGroup`, the exported keyed fold that puts both sides of one line in a single
  group.
- `lineGroups` now calls `Pair` and only adds the detail's sort, so the detail and any later pair-row refresh share one
  pairing rule.
- Option order is fixed: away then home for spread and moneyline, Over then Under for totals and props, and the single
  Yes for `ANYTIME_TD`. Field-level tiebreaks keep duplicate-rank options stable regardless of input order.
- Added tests for Over/Under on a total and a prop, away/home spread and moneyline, the single anytime-TD side, one
  stored side, and deterministic option order under reversed input.

## Deviations

- None. The design names pairing as a keyed fold but does not give `Pair` a signature; it mirrors `lineGroups` minus the
  sort: `Pair(game db.Game, rows []Row) []LineGroup`.

## Open issues

- `Pair` keeps groups in first-seen order. `lineGroups` sorts them afterwards for the detail; a caller that uses `Pair`
  directly must sort or pick the group it wants.
- `Pair` does not filter by `game.ID`; callers pass rows already scoped to one game, as `Detail` does.
- `sortOptions` now compares side, team, line, then text after the rank. That is only a tiebreak for degenerate input,
  but it means the option order is total rather than merely stable.

## Next-phase notes

- Use `Pair(game, rows)` to rebuild the refreshed row after add, update, or delete. Match the updated leg on market,
  team, player, and line, the same fields `lineKey` uses; group options already carry `Picked` from `board.Build`.
- `Pair` returns `[]LineGroup`, so the handler can find the one group by `Market`, `Team`, `Player`, and `Line` or by
  the first option that matches the posted selection.
- The template seam is still `pick-line-group`; each option remains a `.pick-row`.
- No docs changed in this task. Pairing and the update response belong to the next task's docs step.
