---
slug: record-book-tags-and-member-history
run: 20261128-022107-record-book-tags-and-member-history
outcome: done
generated: 2026-11-28T02:40:26Z
---

# Run summary: record-book-tags-and-member-history

## Outcome

- Status: done
- Branch: record-book-tags-and-member-history
- Close-out: written
- Close: pending
- Final verdict: agy@gemini-4-neon-low approved (issues: 0)

## Totals

- Tasks: 6 of 6
- Phases: 3
- Checkpoints: 6
- Duration: 16m11s
- Cost: $—
- Fresh input tokens: 456016
- Cache read tokens: 6084100
- Cache write tokens: 0
- Output tokens: 74762

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-medium | 6614878 | $— |

## Phases

### Phase 1: Pure Record Book tag engine and season data wiring

Tasks:

- [x] Add `SuiteReport.LegGrades(engineName string) map[int64]string` in `internal/eval/metrics.go` and `Store.VotesForGroupSeason(ctx, groupID, season)` in `internal/db/record.go`, with unit tests covering engine selection, ungraded/market filtering, and group/season/graded-slip vote scoping.
- [x] Extend `internal/record` (`record.go` and new `tags.go`) with `Leg.ID`, `Leg.Grade`, `Leg.NetVotes`, `MemberRow.ImpliedLift`, active leg streaks, and the 14 deterministic praise/insult tags with plain-English rules and member-specific explanations; add boundary and streak unit tests in `internal/record/record_test.go`.

Phase totals: 2 checkpoints, 5m2s, $—, 0 input / 0 output / 0 cache read / 0 cache write tokens

Checkpoints: task-1, task-2 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Add `SuiteReport.LegGrades(engineName string) map[int64]string` in `internal/eval/metrics.go` and `Store.VotesForGroupSeason(ctx, groupID, season)` in `internal/db/record.go`, with unit tests covering engine selection, ungraded/market filtering, and group/season/graded-slip vote scoping.

- Added `SuiteReport.LegGrades(engineName string) map[int64]string` in `internal/eval/metrics.go`, selecting the matching engine report (falling back to `ActiveRubric`, then `Reports[0]`) and filtering out market legs, non-positive leg IDs, and ungraded/blank bands.
- Added `Store.VotesForGroupSeason(ctx, groupID, season)` in `internal/db/record.go` to return net `SUM(value)` vote tallies keyed by leg ID for `GRADED` slips in the given group and season.
- Added `TestSuiteReportLegGrades` in `internal/eval/metrics_test.go` and `TestVotesForGroupSeasonScopesAndSums` in `internal/db/record_test.go`.

### Task 2: Extend `internal/record` (`record.go` and new `tags.go`) with `Leg.ID`, `Leg.Grade`, `Leg.NetVotes`, `MemberRow.ImpliedLift`, active leg streaks, and the 14 deterministic praise/insult tags with plain-English rules and member-specific explanations; add boundary and streak unit tests in `internal/record/record_test.go`.

- Extended `record.Leg` with `ID`, `Grade`, and `NetVotes`, and `record.MemberRow` with `ImpliedLift`, `CurrentWinStreak`, `CurrentLossStreak`, and `Tags` in `internal/record/record.go`.
- Sorted legs stably by `ID` within each graded slip so chronological active win/loss leg streaks are deterministic while skipping `PUSH` and `VOID` legs.
- Added `internal/record/tags.go` with `TagTone`, `Tag`, and the 14 deterministic praise and insult tags ordered with praise first and `parlay_terrorist` first among insults, each carrying a plain-English `Rule` and member-specific `Explanation`.
- Added unit tests in `internal/record/record_test.go` covering `ImpliedLift`, active leg streaks across `PUSH`/`VOID` legs, minimum-sample and threshold boundaries for all 14 tags, and deterministic tag ordering.

### Phase 2: Record Book leaderboard tags, clickable explanations, and member links

Tasks:

- [x] Wire `EvalCache.GetOrCompute` leg grades (fail-soft) and `VotesForGroupSeason` into `GET /record` in `internal/web/record.go`, populate `RecordMemberView.UserID`, `ImpliedLift`, and `Tags`, and link each member row to `/record/members/{id}?season=` without nesting tag buttons inside links.
- [x] Render interactive praise/insult tag chips in `web/templates/record.html`, add the shared `tag-explain-dialog` bottom sheet in `web/templates/partials.html` with its delegated click handler in `web/static/app.js`, and add handler tests in `internal/web/record_test.go`.

Phase totals: 2 checkpoints, 4m39s, $—, 0 input / 0 output / 0 cache read / 0 cache write tokens

Checkpoints: task-3, task-4 Review fixes: 0 Integration checks: 0

#### Done

### Task 3: Wire `EvalCache.GetOrCompute` leg grades (fail-soft) and `VotesForGroupSeason` into `GET /record` in `internal/web/record.go`, populate `RecordMemberView.UserID`, `ImpliedLift`, and `Tags`, and link each member row to `/record/members/{id}?season=` without nesting tag buttons inside links.

- Wired `Store.VotesForGroupSeason` and fail-soft `EvalCache.GetOrCompute` leg grades (`SuiteReport.LegGrades`) into `GET /record` in `internal/web/record.go` (and updated `buildGroupRecordSnapshot` in `internal/web/home.go` for the expanded `buildRecordBook` signature).
- Added `RecordTagView` and populated `RecordMemberView.UserID`, `ImpliedLift`, and `Tags` in `buildRecordPage`.
- Updated `web/templates/record.html` to link each member row (both mobile cards and desktop table) to `/record/members/{id}?season=` and render `.Tags` as sibling `<button type="button" data-tag-explain ...>` chips outside the `<a>` link.
- Updated `internal/web/record_test.go` to verify member links, tag button attributes, non-nesting of tag buttons inside links, `VotesForGroupSeason` tag output, and fail-soft behavior when eval grades are unavailable.

### Task 4: Render interactive praise/insult tag chips in `web/templates/record.html`, add the shared `tag-explain-dialog` bottom sheet in `web/templates/partials.html` with its delegated click handler in `web/static/app.js`, and add handler tests in `internal/web/record_test.go`.

- Added the shared `tag-explain-dialog` `<dialog class="leg-sheet">` partial in `web/templates/partials.html` with tone pill, title, member-specific explanation, rule box, and close button.
- Included `{{template "tag-explain-dialog"}}` in `web/templates/record.html` alongside the existing interactive `[data-tag-explain]` praise/insult tag chips.
- Added delegated `[data-tag-explain]`, `[data-close-tag-explain]`, and backdrop click handling in `web/static/app.js` to populate and toggle `#tag-explain-dialog`.
- Updated `internal/web/record_test.go` to verify `#tag-explain-dialog`, `data-tag-rule`, `data-close-tag-explain`, and that every `tag-explain-*` ID referenced in `web/static/app.js` renders on `GET /record`.

### Phase 3: Member bet history screen and living docs

Tasks:

- [x] Add `GET /record/members/{id}` in `internal/web/slips.go` and `internal/web/record.go`, register `"record-member"` in `internal/web/render.go`, build `web/templates/record_member.html` with summary strip, clickable tags, and newest-first graded leg history, and add scoping and rendering tests in `internal/web/record_test.go`.
- [x] Docs: update `docs/concepts/record-book.md` (including fixing its stale front-matter fields) and `docs/reference/routes.md` to document the expanded tag catalog, clickable tag explanations, and `GET /record/members/{id}?season=`.

Phase totals: 2 checkpoints, 5m51s, $—, 5717046 input / 60610 output / 5350797 cache read / 0 cache write tokens

Checkpoints: task-5, task-6 Review fixes: 0 Integration checks: 0

#### Done

### Task 5: Add `GET /record/members/{id}` in `internal/web/slips.go` and `internal/web/record.go`, register `"record-member"` in `internal/web/render.go`, build `web/templates/record_member.html` with summary strip, clickable tags, and newest-first graded leg history, and add scoping and rendering tests in `internal/web/record_test.go`.

- Mounted `GET /record/members/{id}` in `internal/web/slips.go` and implemented `recordMember`, `RecordMemberPage`, `RecordMemberBetView`, and shared season loading/view helpers in `internal/web/record.go`.
- Registered `"record-member"` in `internal/web/render.go` and updated the template list in `internal/web/render_test.go`.
- Built `web/templates/record_member.html` with a back link to `/record`, season selector, member header with rank, 4-stat summary strip, clickable `data-tag-explain` chips with `tag-explain-dialog`, and newest-first graded leg cards linking to `/slips/{id}`.
- Added `TestRecordMemberHistoryScopingAndRendering` in `internal/web/record_test.go` covering `400` invalid ID/season, `404` out-of-group scoping, newest-first ordering across regular and postseason weeks, sole-killer/vote/market/odds rendering, and empty-season state.

### Task 6: Docs: update `docs/concepts/record-book.md` (including fixing its stale front-matter fields) and `docs/reference/routes.md` to document the expanded tag catalog, clickable tag explanations, and `GET /record/members/{id}?season=`.

- Updated `docs/concepts/record-book.md`: removed stale `sources` and `last_verified` front-matter fields, added `## Overview` with a Mermaid flow diagram, and documented `ImpliedLift`, `VotesForGroupSeason`, fail-soft `eval.Cache` leg grades, the 14-tag catalog with thresholds and deterministic ordering, `#tag-explain-dialog` clickable tag explanations, and `GET /record/members/{id}?season=`.
- Updated `docs/reference/routes.md`: added `GET /record/members/{id}?season=` to the Record Book route table, documented its `400`/`404` scoping and rendered sections, updated `GET /record` notes for votes, cached grades, tag chips, and member links, and bumped `budget.max_lines` from 620 to 650.
- Ran `good-vibes docs lint` (0 errors; 0 warnings on both updated docs) and `good-vibes docs index` (updating `docs/llms.txt`).

## Final gate

- Rounds used: 0; verify failures: 0
- Final verify: passed
- Open issues: none

## Ship

- Sync: none
- Recipe: ask
