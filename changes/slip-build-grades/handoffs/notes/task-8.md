## Done

- Added `ProcessOutcomeView` and `newProcessOutcomeView` in `internal/web/analyzer.go` mapping `analyzer.ClassifyProcessResult` tones (`positive`, `neutral`, `warn`, `bad`) to semantic stamp classes, reserving `--positive` for `Clean Hit` and settled wins.
- Wired `loadEvalGrades` (backed by `SuiteReport.SlipGrades`) into `slipPageData` (`internal/web/slips.go`) and updated `web/templates/partials.html` so settled `/slips/{id}` cards render the Process vs. Outcome stamp (`Clean Hit`, `Tough Beat`, `Total Bailout`, `Book Donation`), its summary, and result-aware `GRADED` status pills (`badge-lost`, `badge-push`, `badge-void`, or positive for `WON`).
- Wired `SuiteReport.SlipGrades` into `internal/web/record.go` (`RecordSlipView` on `RecordPage` and `SlipGrade`/`SlipProcessOutcome` on `RecordMemberBetView`) and rendered Build Grade and Process vs. Outcome badges in `web/templates/record.html` and `web/templates/record_member.html`.
- Added unit tests (`internal/web/record_unit_test.go`) and handler tests (`TestProcessOutcomeBadgesOnSettledSlipAndRecordViews` in `internal/web/record_test.go`) covering `/slips/{id}`, `/record`, and `/record/members/{id}`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

Living docs in `docs/` can now document the Process vs. Outcome badges alongside the 3-layer Build Grade framing and dual-track evals.
