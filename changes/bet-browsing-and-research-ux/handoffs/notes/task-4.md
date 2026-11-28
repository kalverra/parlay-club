# Task 4

## Done

- Added `SideBadge` and `SideClass` fields to `LegView` in `internal/web/slips.go` and `TrayLegView` in `internal/web/pick.go`, populated via `legSideBadge` for `OVER` and `UNDER` legs (empty for `SPREAD`, `ML`, `ANYTIME_TD`, and `OTHER`).
- Added semantic `.badge-over` and `.badge-under` styles in `web/tailwind/input.css` using theme tokens (`--positive*` and `--warning*`) and regenerated `web/static/app.css` via `go generate ./...`.
- Added the `side-badge` partial in `web/templates/partials.html` and rendered it on `#slip-panel` and `#pick-tray` leg rows.
- Added `TestSlipPanelRendersSideBadgesForOULegs` in `internal/web/slips_test.go` and `TestPickTrayRendersSideBadgesForOULegs` in `internal/web/pick_test.go`.

## Deviations

- The existing pick tray leg struct in `internal/web/pick.go` is named `TrayLegView` rather than `PickTrayLegView`; added `SideBadge` and `SideClass` to `TrayLegView` directly to preserve existing references in `PickDockView`, `PickSwapView`, and `internal/web/analyzer_test.go`.

## Open issues

- None.

## Next-phase notes

- Phase 3 can reuse `legSideBadge` and the `side-badge` partial / `.badge-over` and `.badge-under` classes on `BetResearchView` and `web/templates/research_bet.html`.

## Commit

Add O/U side badges to slip panel and pick tray

Over and Under legs on `#slip-panel` and `#pick-tray` previously rendered as plain selection text, making directional picks harder to scan at a glance. Adding structured `SideBadge` and `SideClass` fields with semantic theme-aware badge styles gives O/U legs distinct `▲ OVER` and `▼ UNDER` pills while leaving non-O/U markets clean.
