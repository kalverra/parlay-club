# Task 5: Docs: update `docs/concepts/data-refresh.md` and `docs/concepts/slips.md` for `grade` and auto-grading behavior

## Done

- Updated `docs/concepts/data-refresh.md` to include the `grade` source in the pass flow and staleness table, and added an `## Auto-grading` section covering `PendingLegsOnFinalGames`, game-only score optimization, singleflight + per-game 5-minute `BoxScore` error backoff under `SummarySource(gameID)`, and `slips.Service.AutoGradeLegs`.
- Updated `docs/concepts/slips.md` (`## Grading`) to replace the future-tense auto-grader note with the implemented `grade` + `AutoGradeLegs` behavior, `WHERE result = 'PENDING'` override protection, `NEEDS_GRADING` handling, `GradedBy` (`"Auto"` vs member name) attribution, amber chip styling, and the five-option manual override menu.
- Updated frontmatter `sources` and `last_verified: 2026-11-14` in both docs and verified with `good-vibes docs lint` (`ok`).

## Deviations

None.

## Open issues

None.

## Next-phase notes

- No code changes were made in this task; all docs reflect the Phase 2 implementation in `internal/refresh/grade.go`, `internal/slips/rules.go`, `internal/web/slips.go`, and `web/templates/partials.html`.
