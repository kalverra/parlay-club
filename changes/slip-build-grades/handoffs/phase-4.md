# Phase 4: Phase 4: Post-game Process vs. Outcome badges and living docs

## Done

### Task 8: Render post-game Process vs. Outcome badges (`Clean Hit`, `Tough Beat`, `Total Bailout`, `Book Donation`) on settled slip cards (`/slips/{id}`) and Record Book views (`/record` and `/record/members/{id}`), backed by `SuiteReport.SlipGrades` and unit/handler tests.

- Added `ProcessOutcomeView` and `newProcessOutcomeView` in `internal/web/analyzer.go` mapping `analyzer.ClassifyProcessResult` tones (`positive`, `neutral`, `warn`, `bad`) to semantic stamp classes, reserving `--positive` for `Clean Hit` and settled wins.
- Wired `loadEvalGrades` (backed by `SuiteReport.SlipGrades`) into `slipPageData` (`internal/web/slips.go`) and updated `web/templates/partials.html` so settled `/slips/{id}` cards render the Process vs. Outcome stamp (`Clean Hit`, `Tough Beat`, `Total Bailout`, `Book Donation`), its summary, and result-aware `GRADED` status pills (`badge-lost`, `badge-push`, `badge-void`, or positive for `WON`).
- Wired `SuiteReport.SlipGrades` into `internal/web/record.go` (`RecordSlipView` on `RecordPage` and `SlipGrade`/`SlipProcessOutcome` on `RecordMemberBetView`) and rendered Build Grade and Process vs. Outcome badges in `web/templates/record.html` and `web/templates/record_member.html`.
- Added unit tests (`internal/web/record_unit_test.go`) and handler tests (`TestProcessOutcomeBadgesOnSettledSlipAndRecordViews` in `internal/web/record_test.go`) covering `/slips/{id}`, `/record`, and `/record/members/{id}`.

### Task 9: Docs: update `docs/concepts/analyzer.md`, `docs/reference/analyzer-rules.md`, and `docs/concepts/docs-hub.md` to document Build Grades, the 3-layer progressive disclosure UX, Process vs. Outcome badges, and the Explaining Vegas vs. Beating Vegas eval tracks, then run `good-vibes docs lint`.

- Updated `docs/concepts/analyzer.md` to document Build Grades, non-green grade color tokens (`.grade-a` / `--grade-clean` and `.grade-b` / `--grade-solid`), the 3-layer progressive disclosure UX (Glance Card, Breakdown Drawer, inline Why? popovers + Model Card), "Cleanest builds on the board" highlights, post-game Process vs. Outcome badges, and the Explaining Vegas vs. Beating Vegas eval tracks.
- Updated `docs/reference/analyzer-rules.md` with the `BuildDiagnostics` contract (subdivided `DisplayGrade`, 2-word `Descriptor`, probability provenance, Signal Pills, Context Radar, Cleanup Nudge, and `parlay.FormatImpliedReality`) and the `ClassifyProcessOutcome` / `ClassifyProcessResult` 2x2 matrix.
- Updated `docs/concepts/docs-hub.md` to document `/docs/grading` Build Grade ground rules, the two-track Explaining Vegas vs. Beating Vegas scorecards on `/docs/evals`, `SuiteReport.SlipGrades` in `eval.Cache`, and how Layer 2/3 progressive disclosure pairs with `?` help bubbles.
- Ran `good-vibes docs index` and `good-vibes docs lint`, passing with 0 errors.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 8

Living docs in `docs/` can now document the Process vs. Outcome badges alongside the 3-layer Build Grade framing and dual-track evals.

### Task 9

All Phase 4 tasks are complete; living docs now match the implemented Build Grades, 3-layer UX, Process vs. Outcome badges, and dual-track evals.

## Files touched

- `docs/concepts/analyzer.md`
- `docs/concepts/docs-hub.md`
- `docs/llms.txt`
- `docs/reference/analyzer-rules.md`
- `internal/web/analyzer.go`
- `internal/web/record.go`
- `internal/web/record_test.go`
- `internal/web/record_unit_test.go`
- `internal/web/slips.go`
- `web/static/app.css`
- `web/templates/partials.html`
- `web/templates/record.html`
- `web/templates/record_member.html`

## Run log

- Mode: 
- Checkpoints: task-8, task-9
- Verify: verify passed: test 400ms; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
