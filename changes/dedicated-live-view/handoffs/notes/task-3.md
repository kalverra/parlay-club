## Done

- Updated `docs/concepts/slips.md` to add `live` (`StepLive`) to the hub step table and document when a locked,
  non-graded slip transitions from `export` to `live` versus staying in `export`.
- Updated `docs/concepts/slips.md` live progress section for per-leg game-elapsed progress, before-vs-current
  probability delta and trend, and slip-level upcoming leg counts.
- Updated `docs/reference/odds-math.md` to document `GameProgressPct`, `ProbDelta`, `ProbTrend` (`0.005` deadband),
  `ScheduledCount`, updated live `Summary` formatting, and the `internal/progress` API table entries.
- Verified with `good-vibes docs lint` (`0 error(s)`).

## Deviations

None.

## Open issues

None.

## Next-phase notes

`docs/concepts/slips.md` is at its `585`-line budget; Phase 2 and Phase 3 doc updates to `slips.md` should either
tighten existing prose or update `budget.max_lines` in front matter.
