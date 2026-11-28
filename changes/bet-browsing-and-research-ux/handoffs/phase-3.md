# Phase 3: Phase 3: Visual-first bet research hierarchy with FD/DK focus

## Done

### Task 5: Extract explicit FanDuel and DraftKings quotes (ordered with `homeBook` first) and visual pillar summary data on `BetResearchView` in `internal/web/research.go`, and reorder `web/templates/research_bet.html` to lead with the FD/DK snapshot, visual grade/warnings, and player/team graphics while moving all-book prices into a collapsed `<details>` below the fold, with tests in `internal/web/research_test.go`.

- Added `SideBadge` and `SideClass` to `BetResearchView` via `legSideBadge` and `SnapshotBooks` / `BookCount` to `BetPricingView` in `internal/web/research.go` via `extractSnapshotBooks` (ordering FanDuel and DraftKings with `homeBook` first).
- Reordered `web/templates/research_bet.html` to lead with the bet summary card (with `side-badge`), the FD vs DK snapshot + de-vig edge + 4-pillar visual bars + analyzer warnings, and player/team graphics, while moving the analyzer factor breakdown and a collapsed `<details>` (`Compare all N books`) below the fold.
- Added `TestResearchBetVisualHierarchyAndFDDKSnapshot` in `internal/web/research_test.go` verifying section order, `homeBook`-first FD/DK snapshot ordering, O/U side badge, pillar bars, and collapsed all-book details.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 5

Phase 4 can now wire the universal `betResearchHref` and `#bet-add-dialog` (`GET /bets/add-sheet`) onto the "More in this game" related bet cards in `web/templates/research_bet.html` as well as pick-board options, highlights, slip legs, and research explorer cards.

## Files touched

- `internal/web/research.go`
- `internal/web/research_test.go`
- `web/static/app.css`
- `web/templates/research_bet.html`

## Run log

- Mode: 
- Checkpoints: task-5
- Verify: verify passed: generate reused, changed: web/static/app.css, lint reused, test reused; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
