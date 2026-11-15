## Done

- Extracted shared nflverse loaders (`loadFreshNFLverseTeamProfiles`, `loadNFLversePlayerWeeks`) in `internal/web/analyzer.go` and wired `board.BoardSignals` (`TeamEPA`, `PlayerUsage`, `Injuries`, `SlipLegs`) into `internal/web/pick.go` (`pickPageData` and `pickPairRow`) so both `pick-detail` and HTMX `pick-line-group` swaps populate `PlayerGroup.Tags` and `Row.Badge`.
- Added `SnapPct`, `TargetShare`, `WOPR`, `OppDefLabel`, and `OppDefRank` to `PlayerResearchView` in `internal/web/research.go` via `populateNFLverseResearch`, switching opponent defense rank between pass and rush defense based on the active market.
- Updated `web/templates/partials.html` to render player suggestion tags, row badges in `pick-option`, and the 2-column mobile-first usage/defense stat grid in `research-player`.
- Added `TestPickBoardWiresSignalsBadgesSortAndHTMXSwaps` in `internal/web/pick_test.go` and `TestResearchPlayerSurfacesNFLverseMetricsAndHTMX` in `internal/web/research_test.go` covering fresh signals, `SortMatchup`, HTMX partial swaps, mobile-width grid/wrap layout, and fail-soft stale nflverse behavior.

## Deviations

Also wired `board.ApplySignals` into `pickPairRow` in `internal/web/pick.go` so HTMX `POST /slips/{id}/legs` (`from=board`) partial swaps preserve `Row.Badge` on the updated `pick-line-group`.

## Open issues

None.

## Next-phase notes

Task 9 (docs & verification) can update `docs/reference/routes.md` and `docs/concepts/slips.md` for the pick-board `sort=matchup` / badge behavior and research usage tiles (`Snap % (L3)`, `Target share`, `WOPR`, opponent defense rank).
