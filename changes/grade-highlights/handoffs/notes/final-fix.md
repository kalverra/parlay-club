## Commit

Reuse board signal loader and fix strip docs

Deduplicate the player ID extraction and signal loading in `slipHighlights` by calling `loadBoardSignalsForRows`. Update the pick-board detail documentation in `docs/concepts/slips.md` and `docs/reference/routes.md` so the `"Top plays in this game"` strip is described below the pinned header and tab bar, before the sort chips.
