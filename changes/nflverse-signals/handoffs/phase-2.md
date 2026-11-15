# Phase 2: Phase 2: Analyzer `nflverse` rules, leg/parlay grades, and in-app docs page

## Done

### Task 4: Extend `internal/analyzer` (`Game.HomeEPA`/`AwayEPA`, `PlayerStats.Usage`) with `tough_matchup`, `shrinking_role`, `pace_clash`, and `td_spike_risk` rules, deterministic `ScoreLeg` and `ScoreSlip` (`A`–`F` grades with `GradeFactor` breakdowns), and `RuleDocs`/`GradeRubric` metadata, plus boundary and 1:1 rule-coverage tests.

- Extended `analyzer.Game` with `HomeEPA`/`AwayEPA` (`*TeamDefenseProfile`, plus `TeamEPA`/`OpponentEPA` helpers) and `analyzer.PlayerStats` with `Usage` (`*PlayerUsage` and `PlayerWeekUsage`).
- Implemented the four new deterministic `nflverse` analyzer rules in `internal/analyzer/rules.go`: `tough_matchup`, `shrinking_role`, `pace_clash`, and `td_spike_risk`.
- Added deterministic `ScoreLeg` and `ScoreSlip` in `internal/analyzer/grade.go` returning `A`–`F` grades, `GradeFactor` breakdowns, category caps (`Form & usage`, `Matchup & environment`, `Line & price`), slip correlation/drag adjustments, and a sharp-friend slip summary.
- Added `RuleDocs()`, `GradeRubric()`, and `RuleIDs()` in `internal/analyzer/docs.go` and `internal/analyzer/analyzer.go` to keep the upcoming `/analyzer/guide` page locked to live engine thresholds.
- Added boundary, grade-band, fail-soft, determinism, and 1:1 rule-coverage tests in `internal/analyzer/nflverse_grade_test.go`.

### Task 5: Wire `nflverse` DB lookups into `internal/web/analyzer.go` and `internal/web/slips.go`, render clickable slip/leg grade pills with inline factor breakdowns and `#rule-<id>` links in `web/templates/partials.html` (and `#pick-tray`), and serve `GET /analyzer/guide` (`web/templates/analyzer_guide.html`) with handler render tests.

- Wired fresh `nflverse` team EPA/pace profiles and player usage lookups (`nflverseForSlip`, `buildTeamDefenseProfiles`, `buildPlayerUsage`) into `internal/web/analyzer.go`, `internal/web/slips.go`, and `internal/web/pick.go` with a 7-day freshness guard (`nflverseFreshFor`) and fail-soft fallback on missing or stale data.
- Added deterministic slip and leg `GradeView`/`GradeFactorView` rendering in `web/templates/partials.html` for both `#slip-panel` and `#pick-tray`, including clickable `<details>` factor breakdowns, `"How grading works"` link, `"See full rubric"` links, and `#rule-<id>` `"Why?"` links on every warning card.
- Added `GET /analyzer/guide` (`analyzerGuide` in `internal/web/analyzer.go`, `web/templates/analyzer_guide.html`, registered in `internal/web/render.go`) backed directly by `analyzer.RuleDocs()` and `analyzer.GradeRubric()` with `#leg-grades`, `#slip-grade`, `#suggestions`, and `#rule-<id>` anchors plus safe `?from=` back-link support.
- Regenerated `web/static/app.css` via `go generate ./...` and added handler render tests in `internal/web/analyzer_test.go` covering `/analyzer/guide` rule/anchor coverage, unsafe `from` rejection, fresh vs. stale `nflverse` wiring, and `#slip-panel` / `#pick-tray` grade pills and links.

### Task 6: Docs: update `docs/concepts/analyzer.md`, `docs/reference/analyzer-rules.md`, and `docs/reference/routes.md` with the new rules, leg/slip grade rubric, and `/analyzer/guide` route.

- Updated `docs/concepts/analyzer.md` with a Mermaid overview diagram, leg/slip `A`–`F` grading (`ScoreLeg`/`ScoreSlip`), clickable factor breakdowns in `#slip-panel` and `#pick-tray`, the `/analyzer/guide` page, and `nflverse` team EPA / player usage inputs with their 7-day freshness guard.
- Updated `docs/reference/analyzer-rules.md` with `tough_matchup`, `shrinking_role`, `pace_clash`, and `td_spike_risk` triggers and examples, plus the full `A`–`F` leg and slip grading rubric (`GradeRubric()`).
- Updated `docs/reference/routes.md` with `GET /analyzer/guide` (including `RuleDocs()`/`GradeRubric()` backing, anchor IDs, and `?from=` `safeReturnPath` behavior).
- Ran `good-vibes docs lint` (0 errors) and `good-vibes docs index` (updated `docs/llms.txt`).

## Deviations

### Task 5

- Also wired `nflverseForSlip` and `GradeView` into `internal/web/pick.go` (`dockFrom`) so `#pick-tray` renders the same clickable leg/slip grade pills, inline factor breakdowns, and `#rule-<id>` links as `#slip-panel`.

### Task 6

- Removed retired `sources` and `last_verified` front-matter fields from the three updated docs so `good-vibes docs lint` passes with 0 errors.

## Open issues

### Task 5

- None.

### Task 6

- None.

## Next-phase notes

### Task 4

- `TeamDefenseProfile` expects league ranks `1..32` (`1` = toughest defense / highest offense / fastest pace / highest pass rate; `0` = unset) and `PlayerWeekUsage` expects `0..1` fractions for `OffensePct`, `TargetShare`, and `AirYardsShare`.
- `RuleIDs()` and `RuleDocs()` are 1:1 tested; any future rule added to `rules` in `internal/analyzer/analyzer.go` must also be added to `RuleDocs()` in `internal/analyzer/docs.go`.

### Task 5

- Phase 3 (`internal/board`, `internal/web/pick.go`, `internal/web/research.go`) can reuse `nflverseSource`, `nflverseFreshFor`, and `buildTeamDefenseProfiles` / `buildPlayerUsage` patterns in `internal/web/analyzer.go` when loading `TeamEPA` and `PlayerUsage` for the pick board and research views.

### Task 6

- Phase 3 will add pick-board badges (`Smash matchup`, `High volume`, `Role bump`, `Fits slip`), `SortMatchup`, and research tab `nflverse` metrics; update `docs/reference/routes.md` (pick board query contract / research section) and `docs/concepts/slips.md` if needed in Phase 3's docs task.

## Files touched

- `docs/concepts/analyzer.md`
- `docs/llms.txt`
- `docs/reference/analyzer-rules.md`
- `docs/reference/routes.md`
- `internal/analyzer/analyzer.go`
- `internal/analyzer/docs.go`
- `internal/analyzer/grade.go`
- `internal/analyzer/nflverse_grade_test.go`
- `internal/analyzer/rules.go`
- `internal/web/analyzer.go`
- `internal/web/analyzer_test.go`
- `internal/web/pick.go`
- `internal/web/render.go`
- `internal/web/render_test.go`
- `internal/web/slips.go`
- `web/static/app.css`
- `web/templates/analyzer_guide.html`
- `web/templates/partials.html`

## Run log

- Mode: 
- Checkpoints: task-4, task-5, task-6
- Verify: verify passed: no checks ran; skipped: lint (no matching files), test (no matching files)
- Rounds used: 1 of 2; verify failures: 0
