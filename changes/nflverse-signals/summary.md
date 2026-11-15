---
slug: nflverse-signals
run: 20261115-124122-nflverse-signals
outcome: done
generated: 2026-11-15T13:51:38Z
---

# Run summary: nflverse-signals

## Outcome

- Status: done
- Branch: nflverse-signals
- Final verdict: claude@claude-sonnet-5-5 approved (issues: 0)

## Totals

- Tasks: 9 of 9
- Phases: 3
- Checkpoints: 9
- Duration: 55m40s
- Cost: $—
- Fresh input tokens: 758941
- Cache read tokens: 17644194
- Cache write tokens: 0
- Output tokens: 152047

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-medium | 18555182 | $— |

## Phases

### Phase 1: Ingest, SQLite storage, refresh wiring, and sandbox fixtures

Tasks:

- [x] Build `internal/ingest/nflverse` CSV clients (`Players`, `TeamStats`, `PlayerStats`, `SnapCounts`) with ESPN team-code mapping and header-based parsing, plus tests against real saved CSV fixtures.
- [x] Add `0012_nflverse_signals.sql`, `internal/db/nflverse.go` store queries/upserts, `internal/refresh` weekly sync source (`nflverse:season:{year}`), `internal/sandbox/registry.go` + `reset.go` + `migrate_test.go` wiring, and end-to-end refresh tests.
- [x] Docs: update `docs/reference/data-sources.md`, `docs/reference/schema.md`, `docs/concepts/data-refresh.md`, and `docs/reference/sandbox-replay.md` for `nflverse` sources and tables.

Phase totals: 2 checkpoints, 8m55s, $—, 0 input / 0 output / 0 cache read / 0 cache write tokens

Checkpoints: task-1, task-3 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Build `internal/ingest/nflverse` CSV clients (`Players`, `TeamStats`, `PlayerStats`, `SnapCounts`) with ESPN team-code mapping and header-based parsing, plus tests against real saved CSV fixtures.

- Built `internal/ingest/nflverse` (`client.go`, `teams.go`, `players.go`, `team_stats.go`, `player_stats.go`, `snap_counts.go`) with header-based CSV parsing, `User-Agent: parlay-club`, 30s default timeout, and injected `*http.Client`.
- Implemented `Players`, `TeamStats`, `PlayerStats`, and `SnapCounts` returning plain structs (`Player`, `TeamWeek`, `PlayerWeek`, `SnapCount`) with ESPN team-code normalization (`LA`→`LAR`, `WAS`→`WSH`, plus historical `OAK`→`LV`, `SD`→`LAC`, `STL`→`LAR`, `JAC`→`JAX`) and opponent-row joining in `TeamStats`.
- Saved real 2026 `nflverse-data` CSV fixtures in `internal/ingest/nflverse/testdata/` (`stats_team_week_2026.csv`, `stats_player_week_2026.csv`, `snap_counts_2026.csv`, `players.csv`, and `README.md`).
- Added unit and fixture tests in `internal/ingest/nflverse/nflverse_test.go` covering all 4 endpoints, all 32 ESPN team codes, reordered columns, postseason game types, missing columns, blank optional cells, and bad numeric errors.

### Task 3: Docs: update `docs/reference/data-sources.md`, `docs/reference/schema.md`, `docs/concepts/data-refresh.md`, and `docs/reference/sandbox-replay.md` for `nflverse` sources and tables.

- Updated `docs/reference/data-sources.md` with the `nflverse` client (`Players`, `TeamStats`, `PlayerStats`, `SnapCounts`), 30s default timeout, header-based parsing, season-type and ESPN team-code normalization, and 8 MiB archive behavior.
- Updated `docs/reference/schema.md` for `0012_nflverse_signals.sql` (schema version 12), adding `nflverse_player_ids`, `nflverse_team_weeks`, and `nflverse_player_weeks` to the summary, migrations, tables, DDL, and global table conventions.
- Updated `docs/concepts/data-refresh.md` for `nflverse:season:{year}` in `freeSourcesFor`, its staleness/skip rules (6h Tue/Wed/game days, 24h otherwise; skipped when all games are final and last sync > 12h after latest kickoff), and post-commit `week:{weekID}` publishing.
- Updated `docs/reference/sandbox-replay.md` with the 4 `nflverse` week 9 fixtures (104 total across 4 stages: 61 captured, 43 derived), reset table list, startup sync, and test coverage, and ran `good-vibes docs lint` and `good-vibes docs index` (`docs/llms.txt`).

### Phase 2: Analyzer `nflverse` rules, leg/parlay grades, and in-app docs page

Tasks:

- [x] Extend `internal/analyzer` (`Game.HomeEPA`/`AwayEPA`, `PlayerStats.Usage`) with `tough_matchup`, `shrinking_role`, `pace_clash`, and `td_spike_risk` rules, deterministic `ScoreLeg` and `ScoreSlip` (`A`–`F` grades with `GradeFactor` breakdowns), and `RuleDocs`/`GradeRubric` metadata, plus boundary and 1:1 rule-coverage tests.
- [x] Wire `nflverse` DB lookups into `internal/web/analyzer.go` and `internal/web/slips.go`, render clickable slip/leg grade pills with inline factor breakdowns and `#rule-<id>` links in `web/templates/partials.html` (and `#pick-tray`), and serve `GET /analyzer/guide` (`web/templates/analyzer_guide.html`) with handler render tests.
- [x] Docs: update `docs/concepts/analyzer.md`, `docs/reference/analyzer-rules.md`, and `docs/reference/routes.md` with the new rules, leg/slip grade rubric, and `/analyzer/guide` route.

Phase totals: 3 checkpoints, 19m55s, $—, 0 input / 0 output / 0 cache read / 0 cache write tokens

Checkpoints: task-4, task-5, task-6 Review fixes: 0 Integration checks: 0

#### Done

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

### Phase 3: Pick-board leg suggestions and research usage view

Tasks:

- [x] Extend `internal/board` with `BoardSignals` (`Smash matchup`, `High volume`, `Role bump`, `Fits slip` badges) and `SortMatchup` sorting, with table-driven board tests.
- [x] Wire `BoardSignals` into `internal/web/pick.go`, surface `Snap %`, `Target share`, `WOPR`, and opponent defense rank in `internal/web/research.go` and templates, and verify mobile-width rendering and HTMX partial swaps in web tests.
- [x] Docs: update `docs/concepts/slips.md` for pick-board suggestion badges, `Matchup` sort, and research usage context.

Phase totals: 4 checkpoints, 26m51s, $—, 18403135 input / 152047 output / 17644194 cache read / 0 cache write tokens

Checkpoints: task-7, task-8, task-9, final Review fixes: 0 Integration checks: 0

#### Done

### Task 7: Extend `internal/board` with `BoardSignals` (`Smash matchup`, `High volume`, `Role bump`, `Fits slip` badges) and `SortMatchup` sorting, with table-driven board tests.

- Added `BoardSignals` (`TeamEPA`, `PlayerUsage`, `Injuries`, `SlipLegs`), `Row.Badge`, `PlayerGroup.Tags`, `Query.Signals`, and `ApplySignals` in `internal/board`.
- Implemented the four suggestion badges (`Smash matchup`, `High volume`, `Role bump`, `Fits slip`) with priority ordering and a 2-tag cap on `PlayerGroup.Tags`.
- Added `SortMatchup` (`"matchup"`, label `"Matchup"`) to `DetailSorts` and wired signal-strength + opponent defense EPA rank sorting into `sortLineGroups` and `sortPlayers`.
- Added table-driven tests for all four badges, tag priority/capping, fail-soft nil/empty inputs, and `SortMatchup` ordering in `internal/board/signals_test.go`.

### Task 8: Wire `BoardSignals` into `internal/web/pick.go`, surface `Snap %`, `Target share`, `WOPR`, and opponent defense rank in `internal/web/research.go` and templates, and verify mobile-width rendering and HTMX partial swaps in web tests.

- Extracted shared nflverse loaders (`loadFreshNFLverseTeamProfiles`, `loadNFLversePlayerWeeks`) in `internal/web/analyzer.go` and wired `board.BoardSignals` (`TeamEPA`, `PlayerUsage`, `Injuries`, `SlipLegs`) into `internal/web/pick.go` (`pickPageData` and `pickPairRow`) so both `pick-detail` and HTMX `pick-line-group` swaps populate `PlayerGroup.Tags` and `Row.Badge`.
- Added `SnapPct`, `TargetShare`, `WOPR`, `OppDefLabel`, and `OppDefRank` to `PlayerResearchView` in `internal/web/research.go` via `populateNFLverseResearch`, switching opponent defense rank between pass and rush defense based on the active market.
- Updated `web/templates/partials.html` to render player suggestion tags, row badges in `pick-option`, and the 2-column mobile-first usage/defense stat grid in `research-player`.
- Added `TestPickBoardWiresSignalsBadgesSortAndHTMXSwaps` in `internal/web/pick_test.go` and `TestResearchPlayerSurfacesNFLverseMetricsAndHTMX` in `internal/web/research_test.go` covering fresh signals, `SortMatchup`, HTMX partial swaps, mobile-width grid/wrap layout, and fail-soft stale nflverse behavior.

### Task 9: Docs: update `docs/concepts/slips.md` for pick-board suggestion badges, `Matchup` sort, and research usage context.

- Updated `docs/concepts/slips.md` (`## The pick board` and `## Research`) to document `BoardSignals`, the four pick-board suggestion badges (`Smash matchup`, `Fits slip`, `Role bump`, `High volume`) and their 2-tag player-card cap, the `matchup` sort option, and the 2-column player research usage/defense grid (`Snap % (L3)`, `Target share`, `WOPR`, `Opp Pass Def` / `Opp Rush Def`).
- Updated `docs/reference/routes.md` (`### Pick board` and `### Research`) to document `sort=matchup` in the board query contract and the nflverse usage/defense metrics on `GET /research/players/{espnID}`.
- Verified `good-vibes docs lint` passes with 0 errors.

## Final gate

- Rounds used: 2; verify failures: 0
- Final verify: no commands configured
- Open issues: none
