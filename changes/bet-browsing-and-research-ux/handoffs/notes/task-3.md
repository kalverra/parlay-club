# Task 3

## Done

- Added `IsOU`, `SharedLine`, and hoisted `SplitLine` to `PickLineGroupView` in `internal/web/pick.go` and populated them in `pickLineGroupView` for paired `OVER`/`UNDER` line groups.
- Added compact side-by-side Over/Under controls (`pick-ou-option`) in `web/templates/partials.html` under `pick-line-group`, rendering one header (`Label · SharedLine` + hoisted `SplitLine` badge), directional `▲ Over` / `▼ Under` buttons with checkmark and `aria-pressed="true"` when picked, sibling `<details data-leg-grade>` popovers, and `closest .pick-line-group` HTMX swap targets across add, switch, remove, drift sync, and at-limit swap states.
- Updated existing O/U pick-board assertions and added `TestPickBoardCompactOUControlsAndHoistedSplitLine` in `internal/web/pick_test.go`.

## Deviations

- None.

## Open issues

- None.

## Next-phase notes

- Task 4 can now add `SideBadge` and `SideClass` to `LegView` and `PickTrayLegView` on `#slip-panel` and `#pick-tray`.

## Commit

Render compact Over/Under controls on pick board

Paired Over/Under markets repeated the line number, split-line badge, and verbose action copy inside both side buttons. Hoisting the shared and split lines to the line-group header and trimming the buttons to directional labels makes O/U rows faster to scan at phone width while preserving row-level HTMX swaps.
