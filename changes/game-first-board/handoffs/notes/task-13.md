# Task 13: Move the lazy props prompt and week empty states

## Done

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

## Deviations

- The design says "a game with lines but no prop rows" gets the prompt. `TestPickResultsNamesAPropGameThatIsNotLoaded`
  previously expected a prompt for a searched game with no lines, so the test now gives that game one spread row. This
  keeps the prompt honest: without lines, the week's "No lines" state already says what is missing.

## Open issues

- `board.Search` still builds `Result.Unloaded`, and `board` still tests it, but nothing in the web layer reads it now.
  The next cleanup can drop it or keep it for a non-slate caller.
- A game that has rows for the week but no rows in the current search still shows its card (the slate filters at the game
  level) and offers "Load them" if it has no props. That matches the slate's game-level search.

## Next-phase notes

- The docs task owns the prose. Update the pick board section in `docs/concepts/slips.md` to say the prompt sits on the
  card and the detail, and keep `docs/reference/routes.md` to the `/games/{id}/props` trigger and `board-updated`.
- `PickGameCardView.Unloaded` and `PickDetailView.Unloaded` are the only switch for the prompt. Any new card markup that
  keeps the "Load them" button must keep `hx-swap="none"`; the props route answers an empty body and relies on
  `board-updated` to refresh the board.
- The prompt's "lines but no props" test reads the built rows in `pickPageData` (`rowGames`), not the slate or detail
  model, so changing `board.Slates` filtering will not silently turn the prompt on for a game with no lines.
