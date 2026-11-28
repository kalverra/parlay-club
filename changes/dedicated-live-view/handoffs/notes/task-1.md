## Done

- Added `StepLive` (`"live"`, label `"Live"`) to `internal/slips/next.go`.
- Updated `NextStep` to transition locked, non-graded slips to `StepLive` via `legGamesStarted` when any leg references
  a `LIVE`, `FINAL`, or at/past-kickoff game, while keeping `StepExport` for pre-kickoff and unlinked-only slips.
- Added `TestNextStepLiveTransitions` in `internal/slips/next_test.go` covering `LIVE`, `FINAL`, at-kickoff,
  past-kickoff, `everyoneLocked`, late-game-only, unlinked-only, graded, and unlocked cases.

## Deviations

None.

## Open issues

None.

## Next-phase notes

`NextStep` checks `legGamesStarted` against the already-filtered `games` slice for the slip, so callers must continue
passing the week's games. Downstream `internal/web` handlers (`slips.go`, `home.go`) will need to handle
`slips.StepLive` in later tasks.
