# Task 1: Unify name normalization and `(TEAM)` stripping

## Done

- Added `StripPlayerTag(name string) (cleaned, teamHint string)` in `internal/db/players.go` to strip trailing parenthetical tags and extract 2-4 letter uppercase team hints.
- Updated `NormalizePlayerName` in `internal/db/players.go` to strip trailing parenthetical tags via `StripPlayerTag` and fold common Latin accents via `plainRune` before punctuation and suffix removal.
- Delegated `internal/board.normalize` directly to `db.NormalizePlayerName` and removed the duplicate `foldAccents` and `plainRune` helpers from `internal/board/board.go`.
- Added unit tests for `StripPlayerTag` and accented/tagged names in `internal/db/players_test.go`, and board normalization/grouping tests in `internal/board/board_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `db.StripPlayerTag` and `db.NormalizePlayerName` are ready for batch prop resolution in `internal/refresh/research.go` and `internal/db/players.go`.
- Note that `StripPlayerTag` trims any non-empty trailing parenthetical `(...)` (e.g. `(Out)`, `(mia)`) from `cleaned`, while only returning `teamHint` when the inner token is 2-4 uppercase ASCII letters.
