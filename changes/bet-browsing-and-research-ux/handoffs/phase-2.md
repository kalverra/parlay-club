# Phase 2: Phase 2: Compact O/U pick rows and clear O/U slip badges

## Done

### Task 3: Add `IsOU`, `SharedLine`, and hoisted `SplitLine` fields to `PickLineGroupView` in `internal/web/pick.go` and render compact side-by-side Over/Under controls in `web/templates/partials.html`, verifying add/switch/remove/swap markup in `internal/web/pick_test.go`.

- Added `IsOU`, `SharedLine`, and hoisted `SplitLine` to `PickLineGroupView` in `internal/web/pick.go` and populated them in `pickLineGroupView` for paired `OVER`/`UNDER` line groups.
- Added compact side-by-side Over/Under controls (`pick-ou-option`) in `web/templates/partials.html` under `pick-line-group`, rendering one header (`Label · SharedLine` + hoisted `SplitLine` badge), directional `▲ Over` / `▼ Under` buttons with checkmark and `aria-pressed="true"` when picked, sibling `<details data-leg-grade>` popovers, and `closest .pick-line-group` HTMX swap targets across add, switch, remove, drift sync, and at-limit swap states.
- Updated existing O/U pick-board assertions and added `TestPickBoardCompactOUControlsAndHoistedSplitLine` in `internal/web/pick_test.go`.

### Task 4: Add structured `SideBadge` and `SideClass` fields to `LegView` and `PickTrayLegView` in `internal/web/slips.go` and `internal/web/pick.go` and render prominent `OVER`/`UNDER` visual badges on `#slip-panel` and `#pick-tray` in `web/templates/partials.html` (with semantic theme styles in `web/tailwind/` and `go generate ./...`), covered by tests in `internal/web/slips_test.go` and `internal/web/pick_test.go`.

- Added `SideBadge` and `SideClass` fields to `LegView` in `internal/web/slips.go` and `TrayLegView` in `internal/web/pick.go`, populated via `legSideBadge` for `OVER` and `UNDER` legs (empty for `SPREAD`, `ML`, `ANYTIME_TD`, and `OTHER`).
- Added semantic `.badge-over` and `.badge-under` styles in `web/tailwind/input.css` using theme tokens (`--positive*` and `--warning*`) and regenerated `web/static/app.css` via `go generate ./...`.
- Added the `side-badge` partial in `web/templates/partials.html` and rendered it on `#slip-panel` and `#pick-tray` leg rows.
- Added `TestSlipPanelRendersSideBadgesForOULegs` in `internal/web/slips_test.go` and `TestPickTrayRendersSideBadgesForOULegs` in `internal/web/pick_test.go`.

## Deviations

### Task 3

- None.

### Task 4

- The existing pick tray leg struct in `internal/web/pick.go` is named `TrayLegView` rather than `PickTrayLegView`; added `SideBadge` and `SideClass` to `TrayLegView` directly to preserve existing references in `PickDockView`, `PickSwapView`, and `internal/web/analyzer_test.go`.

## Open issues

### Task 3

- None.

### Task 4

- None.

## Next-phase notes

### Task 3

- Task 4 can now add `SideBadge` and `SideClass` to `LegView` and `PickTrayLegView` on `#slip-panel` and `#pick-tray`.

### Task 4

- Phase 3 can reuse `legSideBadge` and the `side-badge` partial / `.badge-over` and `.badge-under` classes on `BetResearchView` and `web/templates/research_bet.html`.

## Files touched

- `internal/web/pick.go`
- `internal/web/pick_test.go`
- `internal/web/slips.go`
- `internal/web/slips_test.go`
- `web/static/app.css`
- `web/tailwind/input.css`
- `web/templates/partials.html`

## Run log

- Mode: 
- Checkpoints: task-3, task-4
- Verify: verify passed: generate 400ms, lint 4.7s, test 11.6s; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
