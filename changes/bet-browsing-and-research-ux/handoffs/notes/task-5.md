# Task 5

## Done

- Added `SideBadge` and `SideClass` to `BetResearchView` via `legSideBadge` and `SnapshotBooks` / `BookCount` to `BetPricingView` in `internal/web/research.go` via `extractSnapshotBooks` (ordering FanDuel and DraftKings with `homeBook` first).
- Reordered `web/templates/research_bet.html` to lead with the bet summary card (with `side-badge`), the FD vs DK snapshot + de-vig edge + 4-pillar visual bars + analyzer warnings, and player/team graphics, while moving the analyzer factor breakdown and a collapsed `<details>` (`Compare all N books`) below the fold.
- Added `TestResearchBetVisualHierarchyAndFDDKSnapshot` in `internal/web/research_test.go` verifying section order, `homeBook`-first FD/DK snapshot ordering, O/U side badge, pillar bars, and collapsed all-book details.

## Deviations

None.

## Open issues

None.

## Next-phase notes

Phase 4 can now wire the universal `betResearchHref` and `#bet-add-dialog` (`GET /bets/add-sheet`) onto the "More in this game" related bet cards in `web/templates/research_bet.html` as well as pick-board options, highlights, slip legs, and research explorer cards.

## Commit

Lead bet research with FD/DK snapshot and visual grade

Bet research previously buried visual player/team graphics and warnings beneath a full multi-book price table. Reordering the page around an FD/DK snapshot, four-pillar visual bars, and collapsed full-book details keeps the first screen focused on the highest-signal decision context.
