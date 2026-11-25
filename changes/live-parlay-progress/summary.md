---
slug: live-parlay-progress
run: 20261125-004945-live-parlay-progress
outcome: done
generated: 2026-11-25T01:23:22Z
---

# Run summary: live-parlay-progress

## Outcome

- Status: done
- Branch: live-parlay-progress
- Close-out: written
- Close: pending
- Final verdict: agy@gemini-4-neon-medium approved (issues: 0)

## Totals

- Tasks: 10 of 10
- Phases: 3
- Checkpoints: 10
- Duration: 30m33s
- Cost: $—
- Fresh input tokens: 1783166
- Cache read tokens: 15546263
- Cache write tokens: 0
- Output tokens: 254284

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-medium | 17583713 | $— |

## Phases

### Phase 1: Capture live game clock and box scores from ESPN

Tasks:

- [x] Extend `internal/ingest/espn` (`scoreboard.go`, `summary.go`, `boxscore.go`) to parse live `period`, `displayClock` / break detail, and combined `Summary` (`injuries` + `boxscore`) from one `/summary` request, with unit tests for pre-game, live regulation, halftime, overtime, and final payloads.
- [x] Add `internal/db/migrations/0018_live_game_state.sql` (`period`, `clock`, `boxscore_json` on `games`) and update `internal/db/games.go` (`Game`, `UpsertGame`, `SetGameSummary`, `LiveGameIDsWithSlipLegs`) with migration and store tests verifying idempotent change detection.
- [x] Update `internal/refresh` (`rules.go`, `research.go`, `refresh.go`, `sources.go`) so `espn:summary:{gameID}` refreshes every 2 minutes for `LIVE` games with active slip legs, persists live `boxscore_json` alongside `injuries_json`, and publishes `week:{weekID}` / `slip:{id}` on changes, with staleness and publish tests.
- [x] Docs: update `docs/reference/data-sources.md`, `docs/reference/schema.md`, and `docs/concepts/data-refresh.md` for live clock/period fields, `boxscore_json`, and live summary cadence.

Phase totals: 4 checkpoints, 13m25s, $—, 4818645 input / 74550 output / 4378657 cache read / 0 cache write tokens

Checkpoints: task-1, task-2, task-3, task-4 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Extend `internal/ingest/espn` (`scoreboard.go`, `summary.go`, `boxscore.go`) to parse live `period`, `displayClock` / break detail, and combined `Summary` (`injuries` + `boxscore`) from one `/summary` request, with unit tests for pre-game, live regulation, halftime, overtime, and final payloads.

- Added `Period *int` and `Clock *string` to `espn.Game` in `internal/ingest/espn/scoreboard.go`, populated only for `Live` games (`nil` for `Scheduled` and `Final`) with break normalization for `"Halftime"` and `"End of ..."` strings.
- Added `GameSummary` and `Client.Summary(ctx, eventID)` in `internal/ingest/espn/summary.go` to parse both `InjuriesJSON` and `BoxScore` from a single `GET /summary?event={id}` request while keeping `SummaryInjuries` and `BoxScore` intact.
- Added unit tests in `internal/ingest/espn/scoreboard_test.go` and `internal/ingest/espn/summary_test.go` covering pre-game, live regulation, halftime, end-of-period break, overtime, and final payloads.

### Task 2: Add `internal/db/migrations/0018_live_game_state.sql` (`period`, `clock`, `boxscore_json` on `games`) and update `internal/db/games.go` (`Game`, `UpsertGame`, `SetGameSummary`, `LiveGameIDsWithSlipLegs`) with migration and store tests verifying idempotent change detection.

- Added `internal/db/migrations/0018_live_game_state.sql` adding nullable `period`, `clock`, and `boxscore_json` columns to `games`.
- Extended `db.Game`, `gameColumns`, `scanGame`, and `UpsertGame` in `internal/db/games.go` to store and idempotently update `Period` and `Clock` while preserving `BoxScoreJSON` and `InjuriesJSON`.
- Added `Store.SetGameSummary(ctx, gameID, injuriesJSON, boxscoreJSON)` with NULL-safe change detection (preserving any existing `boxscore_json` when `boxscoreJSON` is `nil`) and `Store.LiveGameIDsWithSlipLegs(ctx, weekID)` returning `LIVE` game IDs in the week referenced by at least one slip leg.
- Added migration and store tests in `internal/db/migrate_test.go`, `internal/db/migrate_internal_test.go`, and `internal/db/games_test.go` covering schema version 18, `Period`/`Clock` upsert idempotency, `SetGameSummary` change detection, and `LiveGameIDsWithSlipLegs` filtering.

### Task 3: Update `internal/refresh` (`rules.go`, `research.go`, `refresh.go`, `sources.go`) so `espn:summary:{gameID}` refreshes every 2 minutes for `LIVE` games with active slip legs, persists live `boxscore_json` alongside `injuries_json`, and publishes `week:{weekID}` / `slip:{id}` on changes, with staleness and publish tests.

- Added `summaryLiveAge = 2 * time.Minute` and updated `summaryStale` / `summaryAgeFor` in `internal/refresh/rules.go` and `internal/refresh/refresh.go` so `espn:summary:{gameID}` refreshes every 2 minutes only for `LIVE` games with active slip legs, keeping the 15-minute game-day and 1-hour idle windows otherwise.
- Added an optional `summaryClient` interface in `internal/refresh/refresh.go` and updated `syncSummary` in `internal/refresh/research.go` to call `Summary` when available (falling back to `SummaryInjuries` for narrow test doubles), persist marshaled `boxscore_json` alongside `injuries_json` for `LIVE` games via `SetGameSummary`, and publish `week:{weekID}` and `slip:{id}` on changes.
- Updated `slateGame` in `internal/refresh/sources.go` to map `Period` and `Clock` from `espn.Game` so `syncScoreboard` persists live quarter/clock updates and publishes on changes.
- Added staleness, persistence, and publish tests in `internal/refresh/rules_test.go`, `internal/refresh/research_test.go`, and `internal/refresh/publish_test.go`.

### Task 4: Docs: update `docs/reference/data-sources.md`, `docs/reference/schema.md`, and `docs/concepts/data-refresh.md` for live clock/period fields, `boxscore_json`, and live summary cadence.

- Updated `docs/reference/data-sources.md` to document ESPN scoreboard `period` and `clock` fields (populated only while `LIVE`) and `Client.Summary` returning both `InjuriesJSON` and `BoxScore` from one `/summary` request.
- Updated `docs/reference/schema.md` for migration `0018_live_game_state.sql` (`period`, `clock`, and `boxscore_json` on `games`), bumping the fresh database version to 18 and noting `SetGameSummary` preservation semantics.
- Updated `docs/concepts/data-refresh.md` for the 2-minute `espn:summary:{gameID}` live cadence on `LIVE` games with active slip legs, `boxscore_json` persistence, and `week:{weekID}` / `slip:{id}` publishing on summary changes.

### Phase 2: Build deterministic live progress and hit probability engine

Tasks:

- [x] Export shared player lookup and market stat extraction from `internal/grader` and create `internal/progress` with game-clock parsing (`RemainingFraction`) and live evaluation for game/team markets (`SPREAD`, `ML`, `TOTAL`, `TEAM_TOTAL`), with unit tests covering kickoff anchoring, halftime, OT, missing clock, and monotonic early cash/bust.
- [x] Add live evaluation for all player prop markets (`PASS_YDS`, `RUSH_YDS`, `REC_YDS`, `RUSH_ATT`, `RECEPTIONS`, `KICKER_PTS`, `PASS_TDS`, `ANYTIME_TD`), `OTHER`/unlinked fallbacks, and full-parlay aggregation (`EvaluateSlip`) in `internal/progress`, with tests covering yardage non-monotonicity, absent players during live games, pushes/voids, and busted parlays.
- [x] Docs: document the live progress and hit probability formulas in `docs/reference/odds-math.md`.

Phase totals: 3 checkpoints, 8m30s, $—, 4282624 input / 74835 output / 3843634 cache read / 0 cache write tokens

Checkpoints: task-5, task-6, task-7 Review fixes: 0 Integration checks: 0

#### Done

### Task 5: Export shared player lookup and market stat extraction from `internal/grader` and create `internal/progress` with game-clock parsing (`RemainingFraction`) and live evaluation for game/team markets (`SPREAD`, `ML`, `TOTAL`, `TEAM_TOTAL`), with unit tests covering kickoff anchoring, halftime, OT, missing clock, and monotonic early cash/bust.

- Exported `FindPlayer`, `PlayerMarketStat`, and `AnytimeTDCount` from `internal/grader` so live progress evaluation and final auto-grading share one canonical player lookup and stat mapping.
- Created `internal/progress` with `RemainingFraction` and `ClockLabel` handling scheduled, final, regulation `MM:SS`, halftime, end-of-period, overtime, and missing/unparseable live clocks.
- Implemented deterministic Normal-model `EvaluateLeg` evaluation for `SPREAD`, `ML`, `TOTAL`, and `TEAM_TOTAL` anchored to `parlay.ImpliedProb(leg.Odds)`, including settled-state handling and monotonic early cash/bust for `TOTAL` and `TEAM_TOTAL`.
- Added unit tests in `internal/progress/progress_test.go` covering clock parsing, kickoff anchoring, late-game convergence, monotonic early cash/bust, and settled/fallback states.

### Task 6: Add live evaluation for all player prop markets (`PASS_YDS`, `RUSH_YDS`, `REC_YDS`, `RUSH_ATT`, `RECEPTIONS`, `KICKER_PTS`, `PASS_TDS`, `ANYTIME_TD`), `OTHER`/unlinked fallbacks, and full-parlay aggregation (`EvaluateSlip`) in `internal/progress`, with tests covering yardage non-monotonicity, absent players during live games, pushes/voids, and busted parlays.

- Added live player prop evaluation in `internal/progress/props.go` for `PASS_YDS`, `RUSH_YDS`, `REC_YDS`, `RUSH_ATT`, `RECEPTIONS`, `KICKER_PTS`, `PASS_TDS`, and `ANYTIME_TD`, plus `OTHER` and unlinked fallbacks in `internal/progress/progress.go`.
- Handled non-monotonic yardage props (`PASS_YDS`, `RUSH_YDS`, `REC_YDS`) as `StateLive` until final, monotonic early cash/bust for `RUSH_ATT`, `RECEPTIONS`, `KICKER_PTS`, `PASS_TDS`, and `ANYTIME_TD`, absent players in populated live box scores as `0`, and unpopulated box scores (`box == nil || !box.HasStats`) as `"Live · stats pending"` at pre-game probability.
- Implemented `EvaluateSlip(legs []LegProgress) SlipProgress` in `internal/progress/slip.go`, multiplying live probabilities across counting legs, excluding `PUSH`/`VOID` legs, averaging leg progress, and collapsing busted parlays to `0%`.
- Added unit tests in `internal/progress/props_test.go` covering kickoff anchoring for all 8 player prop markets, yardage non-monotonicity, monotonic early settlement, absent players, pending box scores, unlinked legs, pushes/voids, and busted/cashed parlays.

### Task 7: Docs: document the live progress and hit probability formulas in `docs/reference/odds-math.md`.

- Updated `docs/reference/odds-math.md` front matter, summary, and API table to cover `internal/progress` alongside `internal/parlay`.
- Documented `RemainingFraction` and `ClockLabel` game-clock parsing rules, regulation/OT/halftime clamping, and missing-clock fallbacks.
- Documented `EvaluateLeg` anchoring, settled/scheduled/unavailable/pending-box-score states, Normal-model formulas (`SPREAD`, `ML`, `TOTAL`, `TEAM_TOTAL`, and yardage/volume props with market CVs and monotonic early settlement), and Poisson models (`ANYTIME_TD`, `PASS_TDS`).
- Documented `EvaluateSlip` parlay aggregation, `PUSH`/`VOID` exclusion, progress averaging, and tone rules.

### Phase 3: Render live progress bars and hit chances in the UI

Tasks:

- [x] Wire `SlipLiveView` and `LegLiveView` in `internal/web/slips.go` and render the parlay live progress card and per-leg progress bars with live hit chances in `web/templates/partials.html`, with handler and HTML tests covering live, cashed, busted, scheduled, and SSE-refreshed states.
- [x] Wire compact live parlay progress bars and live hit chances into `SpotlightSlipCard` in `internal/web/home.go` and `web/templates/week.html`, with overview handler tests and phone-width layout checks.
- [x] Docs: update `docs/concepts/slips.md` to document live parlay and leg progress bars and live hit chance display.

Phase totals: 3 checkpoints, 8m2s, $—, 6303783 input / 81186 output / 5609118 cache read / 0 cache write tokens

Checkpoints: task-8, task-9, task-10 Review fixes: 0 Integration checks: 0

#### Done

### Task 8: Wire `SlipLiveView` and `LegLiveView` in `internal/web/slips.go` and render the parlay live progress card and per-leg progress bars with live hit chances in `web/templates/partials.html`, with handler and HTML tests covering live, cashed, busted, scheduled, and SSE-refreshed states.

- Added `SlipLiveView` and `LegLiveView` to `internal/web/slips.go`, decoding `BoxScoreJSON` once per `LIVE`/`FINAL` game and evaluating legs and slips via `progress.EvaluateLeg` and `progress.EvaluateSlip`.
- Rendered the parlay live progress card (`data-slip-live-progress`, `role="progressbar"`, live vs. pre-game hit chance, and status summary) and per-leg progress strips (`data-leg-live-progress`, stat/clock readout, and live hit chance) in `web/templates/partials.html`, and regenerated `web/static/app.css`.
- Added `TestSlipHubLiveProgressStates` and `TestSlipPanelSSERefreshesLiveProgress` in `internal/web/slips_test.go` covering scheduled, live, cashed, busted, and SSE-refreshed `#slip-panel` states.

### Task 9: Wire compact live parlay progress bars and live hit chances into `SpotlightSlipCard` in `internal/web/home.go` and `web/templates/week.html`, with overview handler tests and phone-width layout checks.

- Added `Live SlipLiveView` to `SpotlightSlipCard` in `internal/web/home.go` and populated it in `buildGroupOverviewPage` via `evaluateSlipProgress` and `slipLiveView`.
- Rendered a compact single-row live progress bar and live hit percentage (`data-spotlight-live-progress`, `role="progressbar"`, status badge, truncated summary, and `Live <X>%`) inside spotlight cards in `web/templates/week.html`.
- Added unit assertions in `internal/web/home_overview_test.go` and overview handler/HTML/phone-width layout tests (`TestGroupWeekSpotlightLiveProgress`) in `internal/web/home_test.go` covering scheduled, live, busted, and cashed states.
- Regenerated `web/static/app.css` via `go generate ./...`.

### Task 10: Docs: update `docs/concepts/slips.md` to document live parlay and leg progress bars and live hit chance display.

- Updated `docs/concepts/slips.md` front matter (`summary` and `tags`) to include live progress tracking.
- Documented spotlight card compact live progress bars and `Live X%` display on `GET /groups/{id}/week`.
- Added a `## Live progress` section covering when progress bars appear, the slip hub parlay card (`#slip-panel`), per-leg progress strips, `internal/progress` deterministic hit-chance and early-settlement rules, and automatic `slip-updated` SSE refreshes.

## Final gate

- Rounds used: 0; verify failures: 0
- Final verify: passed
- Open issues: none
