---
slug: grading-record-book
run: 20261114-144025-grading-record-book
outcome: done
generated: 2026-11-14T15:34:57Z
---

# Run summary: grading-record-book

## Outcome

- Status: done
- Branch: grading-record-book
- Final verdict: agy@gemini-4-argon-high approved (issues: 0)

## Totals

- Tasks: 10 of 10
- Phases: 4
- Checkpoints: 10
- Duration: 45m32s
- Cost: $—
- Fresh input tokens: 1337183
- Cache read tokens: 12460025
- Cache write tokens: 0
- Output tokens: 189288

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-medium | 13986496 | $— |

## Phases

### Phase 1: ESPN BoxScore parser and pure auto-grader

Tasks:

- [x] Add `Client.BoxScore` (`internal/ingest/espn`) parsing `boxscore.players[]` by key name and handling DNP vs empty box scores, alongside fixture tests using `summary_week9_featured.json` and `summary_empty.json`
- [x] Create `internal/grader` implementing all 13 markets in `plan/product.md` (`SPREAD`, `ML`, `TOTAL`, `TEAM_TOTAL`, props, `ANYTIME_TD`, `KICKER_PTS`, `OTHER`) with table-driven tests for wins, losses, pushes, DNPs, and missing stats

Phase totals: 2 checkpoints, 6m7s, $—, 1316188 input / 29632 output / 1097438 cache read / 0 cache write tokens

Checkpoints: task-1, task-2 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Add `Client.BoxScore` (`internal/ingest/espn`) parsing `boxscore.players[]` by key name and handling DNP vs empty box scores, alongside fixture tests using `summary_week9_featured.json` and `summary_empty.json`

- Added `BoxScore`, `PlayerBoxStats`, `PassingStats`, `RushingStats`, `ReceivingStats`, `KickingStats`, and `(*Client).BoxScore(ctx, eventID)` in `internal/ingest/espn/boxscore.go`, parsing `boxscore.players[]` by `keys[]` name and merging categories per player.
- Updated `parseMadeAttempts` in `internal/ingest/espn/gamelog.go` to split on both `/` (`2/2`) and `-` (`2-2`), and added `FieldGoalsMade*3 + ExtraPointsMade` fallback when `totalKickingPoints` is omitted.
- Added fixture and unit tests in `internal/ingest/espn/boxscore_test.go` covering `summary_week9_featured.json`, `summary_empty.json`, `summary.json`, reordered keys, kicking fallback, `OtherTDs`, and HTTP errors.

### Task 2: Create `internal/grader` implementing all 13 markets in `plan/product.md` (`SPREAD`, `ML`, `TOTAL`, `TEAM_TOTAL`, props, `ANYTIME_TD`, `KICKER_PTS`, `OTHER`) with table-driven tests for wins, losses, pushes, DNPs, and missing stats

- Created `internal/grader/grader.go` with `LegInput`, `GameScore`, `IsPlayerMarket`, and `GradeLeg` implementing all 13 markets from `plan/product.md` (`SPREAD`, `ML`, `TOTAL`, `TEAM_TOTAL`, `PASS_YDS`, `PASS_TDS`, `RUSH_YDS`, `RUSH_ATT`, `REC_YDS`, `RECEPTIONS`, `ANYTIME_TD`, `KICKER_PTS`, `OTHER`) with no DB or HTTP dependencies.
- Implemented player lookup by `PlayerID` first, then normalized `PlayerName` (stripping punctuation and generational suffixes, narrowed by `Team` when set), distinguishing `HasStats == false` (`NEEDS_GRADING`) from absent players in populated box scores (`VOID` / DNP) and active players with `0` in unlogged categories.
- Added table-driven unit tests in `internal/grader/grader_test.go` covering wins, losses, pushes, DNPs, missing stats, kicking fallback (`FieldGoalsMade*3 + ExtraPointsMade`), `ANYTIME_TD` (`YES`/`NO`, excluding `PassingTDs`), and `IsPlayerMarket`.

### Phase 2: Lazy `grade` refresh and manual override UI

Tasks:

- [x] Add `PendingLegsOnFinalGames` in `internal/db`, `slips.Service.AutoGradeLegs` (`WHERE result = 'PENDING'`, `graded_by = "auto"`, system activity, `slip:{id}` notify), and the `grade` source in `internal/refresh` (with game-only score optimization, error backoff, and integration tests proving manual overrides survive re-runs)
- [x] Surface `GradedBy` and `NEEDS_GRADING` amber styling on `LegView` in `internal/web/slips.go` and `web/templates/partials.html`, with handler/render tests
- [x] Docs: update `docs/concepts/data-refresh.md` and `docs/concepts/slips.md` for `grade` and auto-grading behavior

Phase totals: 3 checkpoints, 16m6s, $—, 1974282 input / 44448 output / 1646157 cache read / 0 cache write tokens

Checkpoints: task-3, task-4, task-5 Review fixes: 0 Integration checks: 0

#### Done

### Task 3: Add `PendingLegsOnFinalGames` in `internal/db`, `slips.Service.AutoGradeLegs` (`WHERE result = 'PENDING'`, `graded_by = "auto"`, system activity, `slip:{id}` notify), and the `grade` source in `internal/refresh` (with game-only score optimization, error backoff, and integration tests proving manual overrides survive re-runs)

- Added `PendingFinalLeg`, `Store.PendingLegsOnFinalGames`, and `Store.SetLegResultIfPending` (`WHERE id = ? AND result = 'PENDING'`) in `internal/db/legs.go` with unit tests in `internal/db/legs_test.go`.
- Added `slips.AutoGraderName` (`"auto"`), `slips.AutoGrade`, and `slips.Service.AutoGradeLegs` in `internal/slips/rules.go`, recomputing slip results/statuses via `gradeSlip`, writing system activity (`user_id = NULL`), and publishing `slip:{id}` only for slips with updated legs, with unit tests in `internal/slips/grade_test.go`.
- Added `GradeSource` (`"grade"`), `BoxScoreClient`, `SlipGrader`, and `grade.go` in `internal/refresh` with game-only score optimization (skipping `BoxScore` when a final game has no pending player-prop legs), per-game 5-minute error backoff, and singleflight + archive-keyed `BoxScore` fetches.
- Added integration tests in `internal/refresh/grade_test.go` covering game-only optimization, player-prop grading and zero-waste re-runs, manual override durability across re-runs, and per-game `BoxScore` error backoff.

### Task 4: Surface `GradedBy` and `NEEDS_GRADING` amber styling on `LegView` in `internal/web/slips.go` and `web/templates/partials.html`, with handler/render tests

- Added `GradedBy` to `LegView` and populated it in `legViews` via `legGradedBy` (`internal/web/slips.go`), mapping `slips.AutoGraderName` (`"auto"`) to `"Auto"` and preserving member names.
- Updated `resultClass` in `internal/web/slips.go` to return `border-amber-700 bg-amber-950 text-amber-300` for `NEEDS_GRADING`, and added `resultLabel` (registered in `templateFuncs` in `internal/web/render.go`) to render `"Needs grading"` for `NEEDS_GRADING` and `"{RESULT} · {GradedBy}"` when graded.
- Updated both the interactive `<summary>` chip and read-only `<span>` chip in `web/templates/partials.html` to render `{{resultLabel .Result .GradedBy}}`, keeping the five override options (`PENDING`, `WON`, `LOST`, `PUSH`, `VOID`) in the menu.
- Added `TestSlipPanelRendersGradedByAndNeedsGradingAmberChip` in `internal/web/slips_test.go` verifying `Auto` vs member attribution, `NEEDS_GRADING` amber styling and label, the five override menu values, and manual override updating the chip.

### Task 5: Docs: update `docs/concepts/data-refresh.md` and `docs/concepts/slips.md` for `grade` and auto-grading behavior

- Updated `docs/concepts/data-refresh.md` to include the `grade` source in the pass flow and staleness table, and added an `## Auto-grading` section covering `PendingLegsOnFinalGames`, game-only score optimization, singleflight + per-game 5-minute `BoxScore` error backoff under `SummarySource(gameID)`, and `slips.Service.AutoGradeLegs`.
- Updated `docs/concepts/slips.md` (`## Grading`) to replace the future-tense auto-grader note with the implemented `grade` + `AutoGradeLegs` behavior, `WHERE result = 'PENDING'` override protection, `NEEDS_GRADING` handling, `GradedBy` (`"Auto"` vs member name) attribution, amber chip styling, and the five-option manual override menu.
- Updated frontmatter `sources` and `last_verified: 2026-11-14` in both docs and verified with `good-vibes docs lint` (`ok`).

### Phase 3: Per-member wager logging

Tasks:

- [x] Add `internal/db/wagers.go`, `slips.Service.SetWager` / `DeleteWager`, `PUT` / `DELETE /slips/{id}/wagers/me` in `internal/web`, and the **"My wager"** form in `partials.html`, covered by DB, service, and HTTP tests
- [x] Docs: update `docs/reference/routes.md` and `docs/concepts/slips.md` for wager endpoints and behavior

Phase totals: 2 checkpoints, 10m23s, $—, 1316188 input / 29632 output / 1097438 cache read / 0 cache write tokens

Checkpoints: task-6, task-7 Review fixes: 0 Integration checks: 0

#### Done

### Task 6: Add `internal/db/wagers.go`, `slips.Service.SetWager` / `DeleteWager`, `PUT` / `DELETE /slips/{id}/wagers/me` in `internal/web`, and the **"My wager"** form in `partials.html`, covered by DB, service, and HTTP tests

- Added `internal/db/wagers.go` (`Wager`, `UpsertWager`, `DeleteWager`, `WagerForUser`, `WagersForSlip`, `WagersForGroupSeason`) and DB tests in `internal/db/wagers_test.go`.
- Added `slips.Service.SetWager` and `slips.Service.DeleteWager` in `internal/slips/wagers.go` enforcing group membership, finite `Stake > 0`, valid American `Odds` (`<= -100` or `>= 100`), `BoostPct >= 0`, trimmed `Book <= 40` runes, and live `slip:{id}` notifications, covered by `internal/slips/wagers_test.go`.
- Added `PUT /slips/{id}/wagers/me` and `DELETE /slips/{id}/wagers/me` handlers, `WagerView`, and `wagerView` in `internal/web/slips.go`, registered `"slip-wager"` in `internal/web/render.go`, and added HTTP tests in `internal/web/wagers_test.go`.
- Added the `slip-wager` **"My wager"** HTMX partial in `web/templates/partials.html` and included it in `web/templates/slip.html` below `slip-payout`.

### Task 7: Docs: update `docs/reference/routes.md` and `docs/concepts/slips.md` for wager endpoints and behavior

- Updated `docs/reference/routes.md` (`last_verified: 2026-11-14`, `sources`, and `tags`) to document `PUT /slips/{id}/wagers/me` and `DELETE /slips/{id}/wagers/me`, including request fields, group-membership access, 422 validation rules, and the `slip-wager` partial response.
- Updated `docs/concepts/slips.md` (`sources`, `tags`, `## The hub`, typed errors table, and a new `## Wagers` section) to describe the **My wager** form (`slip-wager`), its placement outside `slip-panel`, validation/default rules, live `slip:{id}` publishing, and `ErrWager*` errors.
- Verified documentation front matter and links with `good-vibes docs lint` (`ok`).

### Phase 4: Record Book calculation and `/record` screen

Tasks:

- [x] Create `internal/record` and season query methods in `internal/db` for `GRADED` slips, streaks, leaderboard (`<6` leg threshold, avg leg odds, Parlay Terrorist badge), and wager dollar P/L, with unit tests covering ties, pushes, and group isolation
- [x] Wire `GET /record?season=` in `internal/web` and replace `web/templates/record.html` with the phone-first Record Book view, verify `sandbox` week 9 stages (`AllClear` / `SundayComplete`), and cover routes/templates with tests
- [x] Docs: update `docs/reference/routes.md` and `docs/` (`good-vibes docs lint`, `good-vibes docs index`) for the Record Book screen and grading/wager specs

Phase totals: 3 checkpoints, 12m56s, $—, 9190550 input / 85576 output / 8618992 cache read / 0 cache write tokens

Checkpoints: task-8, task-9, task-10 Review fixes: 0 Integration checks: 0

#### Done

### Task 8: Create `internal/record` and season query methods in `internal/db` for `GRADED` slips, streaks, leaderboard (`<6` leg threshold, avg leg odds, Parlay Terrorist badge), and wager dollar P/L, with unit tests covering ties, pushes, and group isolation

- Added season Record Book queries in `internal/db/record.go` (`SeasonsForGroup`, `GradedSeasonsForGroup`, `GradedSlipsForGroupSeason`, `GradedLegsForGroupSeason`, and `RecordUsersForGroupSeason`).
- Added unit tests in `internal/db/record_test.go` covering chronological week ordering, `GRADED`-only filtering, group/season isolation, and retaining users who left a group after authoring a leg on a graded slip.
- Created pure `internal/record` package (`internal/record/record.go`) computing group W-L-P record, net `UnitsPL`, chronological `LongestWin`/`LongestLoss` streaks with `PUSH` resets, member leaderboard (`MinDecidedLegs = 6` threshold, arithmetic-mean decimal leg odds converted to American, `ParlayTerrorist` tie handling, and wager `DollarPL` distinguishing `nil` unlogged from `$0.00`).
- Added unit tests in `internal/record/record_test.go` covering empty/ungraded filtering, out-of-order streak calculation with `PUSH` resets, `<6` decided legs threshold, `VOID` exclusion from average odds, multi-loser vs solo-loser Parlay Terrorist ties, and wager dollar P/L edge cases.

### Task 9: Wire `GET /record?season=` in `internal/web` and replace `web/templates/record.html` with the phone-first Record Book view, verify `sandbox` week 9 stages (`AllClear` / `SundayComplete`), and cover routes/templates with tests

- Wired `GET /record?season=` in `internal/web/record.go` and `internal/web/slips.go` (removing the old static `pageRoutes` shell handler in `internal/web/routes.go`), defaulting to the latest season with `GRADED` slips (or the active group's latest season / latest week / current year) and rejecting invalid `?season=` values with `400 Bad Request`.
- Replaced `web/templates/record.html` with the phone-first Record Book view (season selector, empty states, 2x2/4-col summary cards for W-L-P record, Units P/L, win/loss streaks, and leaderboard with mobile cards below `sm:` and a table at `sm:+`, Parlay Terrorist badge, and explicit subtext distinguishing active-leg Units P/L from logged-wager Dollar P/L) and rebuilt `web/static/app.css`.
- Added `internal/web/record_test.go` covering empty states, invalid season handling, default season selection, `?season=` switching, leaderboard/Parlay Terrorist/Dollar P/L rendering, and group isolation, and updated `internal/web/layout_test.go`.
- Verified `sandbox` week 9 stages (`AllClear` and `SundayComplete`) end-to-end with `TestSandboxRecordBookReflectsGradedStages` in `cmd/server/sandbox_test.go`.

### Task 10: Docs: update `docs/reference/routes.md` and `docs/` (`good-vibes docs lint`, `good-vibes docs index`) for the Record Book screen and grading/wager specs

- Updated [routes.md](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/grading-record-book/docs/reference/routes.md) to replace the `/record` placeholder with the `GET /record?season=` route specification, season fallback order, `400` validation behavior, and `sources` front matter (`internal/web/record.go`, `internal/record/record.go`, `web/templates/record.html`).
- Created [record-book.md](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/grading-record-book/docs/concepts/record-book.md) documenting season selection, group scoping, chronological win/loss streaks with `PUSH` resets, active-leg Units P/L, the member leaderboard (`MinDecidedLegs = 6` threshold, avg leg odds, Parlay Terrorist ties, and personal wager Dollar P/L), and verified that [slips.md](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/grading-record-book/docs/concepts/slips.md) and [data-refresh.md](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/grading-record-book/docs/concepts/data-refresh.md) already cover auto-grading and wagers.
- Ran `good-vibes docs index` (updating `docs/llms.txt`) and `good-vibes docs lint` (passed `ok`).

## Final gate

- Rounds used: 0; verify failures: 0
- Final verify: no commands configured
- Open issues: none
