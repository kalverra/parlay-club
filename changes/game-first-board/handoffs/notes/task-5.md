# Task 5 notes

## Done

- Added `GET /slips/{id}/pick/detail` (`pickDetail`) and the `pick-detail` partial. `pickPageData` now shapes the open
  game through `board.Detail` into a `PickDetailView` with tabs, sort chips, line groups, and player cards.
- The Lines tab renders Spread, Moneyline, Total, then one team total per team, with both sides in one row. A prop tab
  renders one card per player, one line per market, and Over/Under options under each line.
- The detail header and a horizontally scrolling tab bar share one sticky block; tab and sort links target
  `#pick-detail` only and each link keeps the other's resolved value in the URL.
- The detail sorts by player by default; `knownSort` now validates against `board.DetailSorts`, so `sort=line` sticks
  instead of resetting to kickoff.
- Add and remove stay inside every `.pick-row` through the unchanged `pick-option` partial. Tests cover the route, tab
  order, Lines ordering, player clustering, link preservation, tab fallbacks, empty states, the remove state, and no
  prop sync.

## Deviations

- Tabs and sorts reuse `PickChip` (`Label`, `Href`, `PushURL`, `Count`, `Active`) instead of a new tab view type, because
  `board.Tab` carries no links.
- Prop options show a short label ("Over 245.5", "Yes") because the player's name is the card heading. Lines rows keep
  the full `Row.Text`. The posted fields are unchanged, so add and remove behave the same.
- An unknown `sort` clears to empty in `pickQuery` and `board.Detail` resolves player, instead of writing
  `sort=player` into every slate card link.
- Fixed `board.marketHeading` to label `ANYTIME_TD` "Anytime TD"; the player card needs a market heading there.
- Added a `no-scrollbar` utility to `web/tailwind/input.css` and regenerated `web/static/app.css`.
- `PickGameGroup`, `groupByGame`, `pairKey`, `categoryChips`, and `sortChips` are gone; `board.Detail` owns those shapes.

## Open issues

- The pick-body's `hx-get` live refresh URL is fixed at page load. A tab or sort swap replaces only `#pick-detail`, so a
  later `board-updated` refetches the body with the old tab and sort and resets it. `SelfURL` is seeded with the
  resolved tab and sort, which narrows the window but does not close it. Give the detail and slate their own refresh
  URLs, or sync the pick-body attribute in `app.js`, in a later phase.
- `result.Rows` from `board.Search` is now unused in `pickPageData`; only `Started` and `Unloaded` are read. Phase 5 can
  trim the call if it keeps the unloaded-props prompts.
- `LineGroup.Label` is still empty for a market with no `marketLabel`; only `ANYTIME_TD` was filled in. A new market
  needs a heading or its card renders without one.
- `/pick/detail` with no open game returns the `pick-detail-empty` placeholder, not the slate. The full page still falls
  back to the slate.
- Docs are untouched here. `docs/concepts/slips.md` and `docs/reference/routes.md` still describe the flat detail rows,
  the kickoff default sort, and the category chip row; phase 2's docs task owns those updates.

## Next-phase notes

- Phase 3's pairing replaces the options inside `PickLineGroupView.Options`. The template seam is `pick-line-group`, and
  each option is still a `.pick-row`, so the add/remove targets keep working.
- `PickLineGroupView` and `PickPlayerView` are the detail seams. `pickLineGroupView` takes `prop bool` to choose the
  short prop label.
- Tab links set `cat=<tab.Category>` and never emit `team_totals`. `detailHref`/`detailPushURL` keep q, win, game, the
  resolved category, and the resolved sort; reuse them for new detail links.
- `data-category` and `data-sort` on `#pick-detail` name the resolved tab and sort, which is the easiest test hook.
- Test helpers `pickDetail` and `addTeamTotal` live in `internal/web/pick_test.go`.
