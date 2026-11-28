# Task 6

## Done

- Replaced `researchBetHref` with a shared `betResearchHref(sel db.SelectionKey, from string, slipID int64) string` helper in `internal/web/research.go` that encodes `game_id`, `market`, `side`, `team`, `player_id`, `player_name`, `from`, and optional `slip_id`, returning `""` for missing game/market or `OTHER` legs.
- Wired `betResearchHref` into `PickRowView` (`pickRowView`, `pickRowFromLeg`), `LegHighlightView` (`highlightsView` and `legHighlightView` for both single-leg and parlay highlights), `LegView` (`legResearchURL`), `ResearchCardView` (`researchCards`), and related bets (`relatedBetCards`, preserving `slip_id`).
- Updated `legResearchURL` in `internal/web/slips.go` to link active non-`OTHER` legs to `/research/bet` when their game has not kicked off and fall back to `/research` player/game sheets for kicked-off or `OTHER` legs.
- Added compact `Research ->` links to `pick-option`, `pick-ou-option`, and `highlights-strip` (single legs and parlay legs) in `web/templates/partials.html`, plus related bet cards in `web/templates/research_bet.html`, keeping links outside `<form>` and `<button>` elements.
- Updated `TestResearchLinksOnSlipLegs` (covering upcoming, `OTHER`, and kicked-off legs) and added `TestBetResearchHrefSharedAcrossSurfaces` in `internal/web/research_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

`betResearchHref` is ready for the remaining Phase 4 tasks (`GET /bets/add-sheet` and `#bet-add-dialog`).

## Commit

Share betResearchHref across all bet tiles

Every bet surface—pick-board options, A-grade highlights, active slip legs, research explorer cards, and related bets—now links to `/research/bet` through one helper with `from` and `slip_id` context, while kicked-off and `OTHER` slip legs fall back to player or game research sheets.
