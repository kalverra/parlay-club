# Close-out

## Consequences

- **What got easier:** Standard game and player-prop legs grade lazily from ESPN box scores as soon as games go final,
  eliminating weekly manual grading while keeping manual overrides durable. Groups also get a season Record Book with
  streaks, units, leaderboard rankings, and personal dollar P/L.
- **What got harder:** Grading accuracy for player props depends on ESPN `/summary` `boxscore.players` key names and
  category shapes; upstream schema shifts will fall back to `NEEDS_GRADING` until the parser is updated.
- **What we would revisit:** Group `Units P/L` uses active-leg `CombinedOdds` while personal `Dollar P/L` uses each
  member's logged wager odds and boost. That distinction is labeled in the UI, but we may revisit adding an aggregate
  group dollars-staked/P&L total if groups want it.

## Alternatives considered

- **Grading rules inside `internal/slips`:** Rejected in favor of a pure `internal/grader` package with no SQLite or
  HTTP dependencies, keeping market rules fast and table-driven to test.
- **Scheduled grading jobs or a second process:** Rejected to preserve the single-instance Cloud Run free-tier model and
  lazy-sync architecture.
- **Live in-game grading:** Rejected to avoid churning partial stats and intermediate slip states before games are
  final.
- **Placing the "My wager" form inside `slip-panel`:** Rejected because live `slip-updated` SSE swaps would clobber
  in-progress wager inputs; rendered it as an adjacent `slip-wager` partial instead.

## Implementation notes

- **Wager partial placement:** Contrary to the initial design sketch, the "My wager" form sits outside `slip-panel` (as
  `slip-wager` in `web/templates/slip.html`) so live panel refreshes do not reset unsaved inputs.
- **Manual override lock and zero-waste refresh:** `AutoGradeLegs` updates only `PENDING` legs
  (`WHERE id = ? AND result = 'PENDING'`), so manual grades and `NEEDS_GRADING` legs survive subsequent refreshes, and
  final games with no pending player-prop legs skip `BoxScore` fetches.
- **DNP vs. missing box score:** Empty box scores on final games (`HasStats == false`) grade player props as
  `NEEDS_GRADING`, whereas populated box scores (`HasStats == true`) void absent players as DNP (`VOID`) and treat
  players active in any other stat group as `0`.

## Follow-ups

None.
