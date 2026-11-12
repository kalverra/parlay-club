# Phase 5: Phase 5: Finish search, sorts, and lazy prompts

## Done

### Task 12: Scope `q` to the open game in the detail, add the odds and line sorts and their tests, and keep the slate search at matchup, team, venue, and roster player.

- Confirmed `board.Detail` keeps only the open game's rows and applies `q` to them, so the detail route, its tab links,
  and its sort links all carry one scoped search.
- Confirmed `board.DetailSorts` names player, odds, and line; `Detail` resolves kickoff, category, and unknown sorts to
  player. The web layer renders those three chips and `knownSort` accepts `sort=line`.
- Confirmed the slate search stays game-level through `gameMatchesQuery`: matchup, team, venue, and roster player, never
  a row's text. Added matchup and full-name assertions to `TestSlatesFilterByWindowAndQuery`.
- Added `TestPickDetailSortsByOddsAndLine`: player is alphabetical, odds puts the shortest price first, and line puts the
  highest number first, asserted from the rendered player cards.
- Added `TestPickDetailSearchStaysInsideTheOpenGame`: the open game's rows match `q`, another game's player does not
  leak in, and a query that matches nothing shows the empty state.

### Task 13: Move the unloaded-props prompt onto the slate and the open detail, move the week empty states to the slate, and add the test that browsing never syncs while "Load them" syncs one game.

- Moved "Load them" from the slate's standalone prompt list onto each game card. The card is now a wrapper with the
  game link plus a bordered footer that names the game and loads its props.
- Added the same prompt to the open detail. The detail's button carries the open game's id, so loading props there
  refreshes `#pick-body` through `board-updated` and the game stays open.
- A prompt appears only for a game that already has stored lines but no prop rows. A week with no odds at all keeps the
  single "No lines for this week yet." state and shows no per-game button. The no-games, no-lines, all-started, and
  no-match states stay on the slate.
- Removed `PickSlateView.Unloaded` and its template loop; `PickGameCardView` now carries `SlipID`, `Unloaded`, and
  `PickDetailView` carries `Unloaded`.
- Kept `TestPickBoardNeverSyncsPropsOnItsOwn` and gave it two unloaded games: browsing every board route still spends no
  credits, and one "Load them" syncs exactly that game. Added `TestPickLoadPropsPromptSitsOnTheCardAndTheDetail` (one
  prompt on the card and one in the detail) and `TestPickGameWithNoLinesShowsTheWeekStateInsteadOfAPropPrompt`.

### Task 14: Docs: update `docs/reference/routes.md` and the pick board section in `docs/concepts/slips.md`, and correct the tray-at-`lg` sentence in `plan/product.md`.

- Rewrote the lazy-props paragraph in `docs/concepts/slips.md`: "Load them" sits on the slate card and again in the open
  detail, appears only for a game with stored lines but no prop rows, keeps the one week-level "No lines" state when the
  week has no odds, and refreshes the body in place through `board-updated`.
- Added the detail's own empty state to the pick-board section: a search that matches nothing says so inside the detail,
  and with no search the detail says the game's lines have not synced.
- Updated `docs/reference/routes.md`: `board.Search` no longer claims to name unloaded games; the handler marks a card
  unloaded when its game has lines but no props; the `/games/{id}/props` paragraph says the prompt sits on the card and
  in the detail; the `HX-Trigger` row says `tray-updated` refreshes the dock and tray.
- Corrected the `plan/product.md#Screens` sentence: the pick board puts its slate and detail in two columns at `lg`,
  with a fixed dock at the bottom at every width.
- Passed `good-vibes docs lint`, regenerated `docs/llms.txt`, and ran `good-vibes check` (0 errors).

## Deviations

### Task 12

None. The production behavior described by this task had already landed in tasks 4 and 5 (`board.Detail`,
`DetailSorts`, the detail chips, and the slate search), so this turn added the missing test coverage and verified the
behavior instead of rewriting it.

### Task 13

- The design says "a game with lines but no prop rows" gets the prompt. `TestPickResultsNamesAPropGameThatIsNotLoaded`
  previously expected a prompt for a searched game with no lines, so the test now gives that game one spread row. This
  keeps the prompt honest: without lines, the week's "No lines" state already says what is missing.

### Task 14

None. Most of the pick-board prose had already landed in earlier tasks, so this turn corrected the remaining stale
paragraphs instead of rewriting the section.

## Open issues

### Task 12

- `board.Sorts` (kickoff, player, category, odds) is now used only by `TestSorts`. The slate has no sort chips, and
  the detail uses `DetailSorts`. Nothing breaks; the next cleanup can drop it or give it a caller.
- A game-level search such as a venue or a roster player with no props matches the game card but no row, so opening that
  game shows "Nothing matches that search." The design says the detail matches rows only, so this is the intended
  behavior, not a bug.

### Task 13

- `board.Search` still builds `Result.Unloaded`, and `board` still tests it, but nothing in the web layer reads it now.
  The next cleanup can drop it or keep it for a non-slate caller.
- A game that has rows for the week but no rows in the current search still shows its card (the slate filters at the game
  level) and offers "Load them" if it has no props. That matches the slate's game-level search.

### Task 14

- `board.Search` still builds `Result.Unloaded` and `board` still tests it, but nothing in the web layer reads it. The
  routes doc now describes the card flag the handler actually uses, so the field is undocumented until a cleanup drops
  it or gives it a caller.
- `good-vibes check` reports 10 stale-doc warnings from earlier dates. None come from this change.

## Next-phase notes

### Task 12

- The lazy-props task does not need to touch search or sort. Every route stay is credit-free: `TestPickBoardNeverSyncsPropsOnItsOwn`
  covers the board, search, filter, sort, detail, dock, and tray paths.
- Keep `q` as one parameter. The slate reads it game-level through `gameMatchesQuery`; the detail reads it row-level
  through `rowMatches`. Changing either rule needs both board tests and the two web tests above.
- The detail's empty state distinguishes no rows from no search match with `.Search`, so a new empty state should keep
  that switch intact.

### Task 13

- The docs task owns the prose. Update the pick board section in `docs/concepts/slips.md` to say the prompt sits on the
  card and the detail, and keep `docs/reference/routes.md` to the `/games/{id}/props` trigger and `board-updated`.
- `PickGameCardView.Unloaded` and `PickDetailView.Unloaded` are the only switch for the prompt. Any new card markup that
  keeps the "Load them" button must keep `hx-swap="none"`; the props route answers an empty body and relies on
  `board-updated` to refresh the board.
- The prompt's "lines but no props" test reads the built rows in `pickPageData` (`rowGames`), not the slate or detail
  model, so changing `board.Slates` filtering will not silently turn the prompt on for a game with no lines.

### Task 14

- The pick-board section of `docs/concepts/slips.md` and the pick-board section of `docs/reference/routes.md` now agree
  on the prop prompt. Keep its switch tied to "lines but no props"; a game with no odds stays the week-level state.
- If `#pick-detail` later gets its own refresh URL, update the "Game selection replaces `#pick-body`" paragraph in
  `slips.md` and the design's stale-self-URL risk sentence to match.

## Files touched

- `docs/concepts/slips.md`
- `docs/reference/routes.md`
- `internal/board/board_test.go`
- `internal/web/pick.go`
- `internal/web/pick_test.go`
- `plan/product.md`
- `web/templates/partials.html`

## Run log

- Mode: 
- Checkpoints: task-12, task-13, task-14
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
