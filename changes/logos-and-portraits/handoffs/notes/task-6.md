# Task 6 Notes

## Done

- Wired `HeadshotURL` and `Initials` onto `GamePlayerPropsView` in [internal/web/games.go](../../../internal/web/games.go) and rendered `player-portrait` in `game-props` player headers in [web/templates/partials.html](../../../web/templates/partials.html).
- Wired `HomeLogo`/`AwayLogo` onto `GameResearchView`, `LogoURL` onto `TeamInjuriesView` (falling back to game team logos when missing from `injuries_json`), and `HeadshotURL`/`Initials` onto `InjuryView` and `PropMatchView` in [internal/web/research.go](../../../internal/web/research.go).
- Wired `HeadshotURL`, `Initials`, and `TeamLogo` (via `Store.TeamLogos`) onto `PlayerResearchView` in [internal/web/research.go](../../../internal/web/research.go), and rendered matchup logos, injury team/player media, matched player portraits, and player research header portrait/logo in [web/templates/partials.html](../../../web/templates/partials.html).
- Regenerated Tailwind CSS (`go generate ./web`) and added handler/render tests in [internal/web/games_test.go](../../../internal/web/games_test.go) and [internal/web/research_test.go](../../../internal/web/research_test.go).

## Deviations

None.

## Open issues

None.

## Next-phase notes

- All research and standalone props views now use the `media.html` helpers (`team-logo`, `player-portrait`, `matchup-logos`).
- Next task can verify `app.js` fallback behavior, phone-width layout, and living documentation updates.
