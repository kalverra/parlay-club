# Phase 3: Phase 3: Pick-board leg suggestions and research usage view

## Done

### Task 7: Extend `internal/board` with `BoardSignals` (`Smash matchup`, `High volume`, `Role bump`, `Fits slip` badges) and `SortMatchup` sorting, with table-driven board tests.

- Added `BoardSignals` (`TeamEPA`, `PlayerUsage`, `Injuries`, `SlipLegs`), `Row.Badge`, `PlayerGroup.Tags`, `Query.Signals`, and `ApplySignals` in `internal/board`.
- Implemented the four suggestion badges (`Smash matchup`, `High volume`, `Role bump`, `Fits slip`) with priority ordering and a 2-tag cap on `PlayerGroup.Tags`.
- Added `SortMatchup` (`"matchup"`, label `"Matchup"`) to `DetailSorts` and wired signal-strength + opponent defense EPA rank sorting into `sortLineGroups` and `sortPlayers`.
- Added table-driven tests for all four badges, tag priority/capping, fail-soft nil/empty inputs, and `SortMatchup` ordering in `internal/board/signals_test.go`.

### Task 8: Wire `BoardSignals` into `internal/web/pick.go`, surface `Snap %`, `Target share`, `WOPR`, and opponent defense rank in `internal/web/research.go` and templates, and verify mobile-width rendering and HTMX partial swaps in web tests.

- Extracted shared nflverse loaders (`loadFreshNFLverseTeamProfiles`, `loadNFLversePlayerWeeks`) in `internal/web/analyzer.go` and wired `board.BoardSignals` (`TeamEPA`, `PlayerUsage`, `Injuries`, `SlipLegs`) into `internal/web/pick.go` (`pickPageData` and `pickPairRow`) so both `pick-detail` and HTMX `pick-line-group` swaps populate `PlayerGroup.Tags` and `Row.Badge`.
- Added `SnapPct`, `TargetShare`, `WOPR`, `OppDefLabel`, and `OppDefRank` to `PlayerResearchView` in `internal/web/research.go` via `populateNFLverseResearch`, switching opponent defense rank between pass and rush defense based on the active market.
- Updated `web/templates/partials.html` to render player suggestion tags, row badges in `pick-option`, and the 2-column mobile-first usage/defense stat grid in `research-player`.
- Added `TestPickBoardWiresSignalsBadgesSortAndHTMXSwaps` in `internal/web/pick_test.go` and `TestResearchPlayerSurfacesNFLverseMetricsAndHTMX` in `internal/web/research_test.go` covering fresh signals, `SortMatchup`, HTMX partial swaps, mobile-width grid/wrap layout, and fail-soft stale nflverse behavior.

### Task 9: Docs: update `docs/concepts/slips.md` for pick-board suggestion badges, `Matchup` sort, and research usage context.

- Updated `docs/concepts/slips.md` (`## The pick board` and `## Research`) to document `BoardSignals`, the four pick-board suggestion badges (`Smash matchup`, `Fits slip`, `Role bump`, `High volume`) and their 2-tag player-card cap, the `matchup` sort option, and the 2-column player research usage/defense grid (`Snap % (L3)`, `Target share`, `WOPR`, `Opp Pass Def` / `Opp Rush Def`).
- Updated `docs/reference/routes.md` (`### Pick board` and `### Research`) to document `sort=matchup` in the board query contract and the nflverse usage/defense metrics on `GET /research/players/{espnID}`.
- Verified `good-vibes docs lint` passes with 0 errors.

## Deviations

### Task 8

Also wired `board.ApplySignals` into `pickPairRow` in `internal/web/pick.go` so HTMX `POST /slips/{id}/legs` (`from=board`) partial swaps preserve `Row.Badge` on the updated `pick-line-group`.

### Task 9

Also updated `docs/reference/routes.md` for `sort=matchup` and the `GET /research/players/{espnID}` usage tiles, as noted in Task 8's next-phase notes.

## Open issues

None.

## Next-phase notes

### Task 7

- `internal/web/pick.go` can pass `Signals: board.BoardSignals{TeamEPA: ..., PlayerUsage: ..., Injuries: ..., SlipLegs: ...}` in `board.Query` when calling `board.Detail`.
- `BoardSignals.Injuries` is keyed by ESPN team abbreviation (`map[string][]espn.Injury`) and expects `Status: "Out"` for `Role bump` detection.

### Task 8

Task 9 (docs & verification) can update `docs/reference/routes.md` and `docs/concepts/slips.md` for the pick-board `sort=matchup` / badge behavior and research usage tiles (`Snap % (L3)`, `Target share`, `WOPR`, opponent defense rank).

### Task 9

All Phase 3 tasks are complete; ready for final change verification / closeout.

## Files touched

- `docs/concepts/slips.md`
- `docs/reference/routes.md`
- `internal/board/board.go`
- `internal/board/signals.go`
- `internal/board/signals_test.go`
- `internal/web/analyzer.go`
- `internal/web/pick.go`
- `internal/web/pick_test.go`
- `internal/web/research.go`
- `internal/web/research_test.go`
- `web/templates/partials.html`

## Run log

- Mode: 
- Checkpoints: task-7, task-8, task-9
- Verify: verify passed: no checks ran; skipped: lint (no matching files), test (no matching files)
- Rounds used: 1 of 2; verify failures: 0
