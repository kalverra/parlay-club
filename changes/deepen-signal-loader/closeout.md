---
draft: true
---

# Close-out

## Consequences

- **Domain invariants are testable without HTTP:** Freshness windows (72 hours for `nfelo`, 7 days for `nflverse`), postseason-to-regular-season stat merging, 32-team EPA ranking, and rolling usage math now live in `internal/analyzer` behind `LoadSlipSignals`, `LoadGameSignals`, and `LoadPlayerResearchSignals`.
- **Single source of rolling usage math:** The analyzer, pick board badges, and player research views now share `BuildPlayerUsage` and `SummarizeRecentUsage` instead of maintaining three separate loops.
- **Batched `nfelo` reads:** `Store.NfeloForGames` loads projections for an entire slate in one `IN (...)` query, replacing the per-game query loop.
- **New package boundary to guard:** `internal/analyzer` now imports `internal/db` for its signal-loading entry points while keeping `AnalyzeSlip`, `ScoreLeg`, and `ScoreSlip` pure. Maintainers must keep DB and clock access out of the pure rule and grading functions.
- **Fallback accessors on `PlayerUsage`:** `EffectiveTargetShare`, `EffectiveSnapAvg`, `EffectiveSeasonSnapAvg`, `RecentWOPR`, and `AverageCarries` fall back to scanning `Weeks` when precomputed fields are zero. This keeps existing table-driven tests concise, at the cost of a small fallback path to maintain.

## Alternatives considered

- **Keep signal assembly in `internal/web`:** Rejected because freshness rules, postseason merging, EPA ranking, and usage aggregation were duplicated across slip, pick board, and research handlers and could not be tested without HTTP setup.
- **Rewrite all `internal/board` test literals to set precomputed `PlayerUsage` fields:** Rejected in favor of fallback-aware accessor methods on `PlayerUsage`, avoiding boilerplate in table-driven tests while routing production callers through `BuildPlayerUsage`.
- **Merge `BuildPlayerUsage` and `SummarizeRecentUsage` behind one game-count threshold:** Rejected because analyzer rules and board badges require at least 3 weeks of data (`BuildPlayerUsage`), whereas player research shows 1-to-3-week averages (`SummarizeRecentUsage`). Splitting them shares the field definitions while preserving each view's threshold.

## Implementation notes

- `design.md` named `internal/analyzer/nflverse_grade_test.go` for the new signal-loader tests; they were placed in `internal/analyzer/signals_test.go` alongside the test moved from `internal/web/analyzer_internal_test.go`.
- When a `PlayerUsage` has a true precomputed average of `0` alongside non-empty `Weeks`, the accessor methods still scan `Weeks` once.
- `proposal.md` included moving injury parsing out of the web layer, but `parseGameInjuries` remained in `internal/web/pick.go` because `design.md` scoped the `LoadGameSignals` return values to team defense profiles and player usage.

## Follow-ups

- Move `parseGameInjuries` from `internal/web/pick.go` into the domain signal loader (`LoadGameSignals` or `internal/board`) so ESPN injury JSON parsing leaves the web layer.
- Update `internal/board/signals_test.go` literals to construct usage via `BuildPlayerUsage` (or a test helper) and remove the `Weeks`-scanning fallback branches from `PlayerUsage` accessors.
