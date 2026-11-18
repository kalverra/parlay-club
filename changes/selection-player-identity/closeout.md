---
draft: true
---

# Close-out

## Consequences

- **Easier:** Sportsbook name quirks, market labels, and moved-line rules now change in one place (`internal/db`) and apply across prop ingest, the pick board, `Fits slip` signals, game research, slip reference pricing, sportsbook export, and post-game grading.
- **Easier:** Game research resolves prop players in memory from preloaded rosters and matches instead of issuing a SQL query per row, and form-submitted legs share `(TEAM)` hint disambiguation and collision safety with prop ingest.
- **Harder:** `internal/db` now carries display-formatting functions (`MarketLabel`, `FormatSelection`, `BuildLegSelection`) alongside persistence and identity rules, and callers must pick between strict (`BuildLegSelection`) and graceful (`FormatSelection`) entry points.
- **Revisit:** If display copy diverges further between compact board rows and other views, extract the formatting functions out of `internal/db` into a separate package so `internal/db` stays focused on storage and identity.

## Alternatives considered

- **Patch each caller in place:** Fixing `(TEAM)` tag stripping, accent folding, and half-point line tolerances separately in `board`, `grader`, `web`, and `export` was rejected because new sportsbook formats or market rules would keep drifting across six packages.
- **One selection formatter for all callers:** Sharing a single `FormatSelection` function without a strict variant was rejected because board rows must render partial odds when a book omits a line, while slip form parsing relies on an empty return value to reject incomplete custom legs.
- **Keep raw SQL in `Store.ResolvePlayerMatch`:** Leaving the SQL matcher in place for form submissions while only switching game research to `BatchPlayerResolver` was rejected because form-submitted legs would still drop `(TEAM)` team hints and lack ambiguous-match guards.

## Implementation notes

- **Research market labels:** Replacing the local `marketLabel` helper in `internal/web/research.go` with `db.MarketLabel` changed research view labels from long names (`"Passing Yards"`, `"Anytime Touchdown"`, `"Kicker Points"`) to the shared short labels (`"Pass Yds"`, `"Anytime TD"`, `"Kicking Pts"`).
- **Ambiguous auto-matches:** When two roster players on the game's teams share a normalized name and the odds feed omits a `(TEAM)` tag, `BatchPlayerResolver` refuses to cache an automatic match; resolving those players still requires a manual override in `player_matches`.
- **Moved-line fallback:** `db.MatchBookOdds` prefers an exact line match and otherwise returns the first matching row for that book, which can pick a non-standard line if a book posts multiple lines for the same selection without an exact match.

## Follow-ups

None.
