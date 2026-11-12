# Task 5

## Done

- Threaded `AwayLogo` and `HomeLogo` onto `PickGameCardView` and `PickDetailView`, and `Initials`, `HeadshotURL`, and `TeamLogo` onto `PickPlayerView` in [internal/web/pick.go](../../../internal/web/pick.go).
- Extended `LegView` with `PlayerHeadshot`, `PlayerInitials`, `TeamLogo`, `MatchupAwayLogo`, and `MatchupHomeLogo`, and preloaded player portraits and team logos in `slipPageData` (`legMedia` / `applyLegMedia`) in [internal/web/slips.go](../../../internal/web/slips.go) using `RosterPlayersByIDs`, week games, and `TeamLogos` fallback.
- Updated `slip-panel`, `pick-slate`, and `pick-detail` templates in [web/templates/partials.html](../../../web/templates/partials.html) to render `matchup-logos`, `player-portrait`, and `team-logo` with graceful fallbacks and plain-text rendering for unlinked legs, and regenerated [web/static/app.css](../../../web/static/app.css).
- Added handler/render tests `TestPickBoardRendersLogosAndPortraits` in [internal/web/pick_test.go](../../../internal/web/pick_test.go) and `TestSlipPanelRendersLegLogosAndPortraits` in [internal/web/slips_test.go](../../../internal/web/slips_test.go).

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Next tasks can wire `GamePlayerPropsView` (`internal/web/games.go`) and research views (`GameResearchView`, `PlayerResearchView` in `internal/web/research.go`) using the same `media.html` helpers and `Initials` helper.
