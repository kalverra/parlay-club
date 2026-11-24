## Done

- Replaced raw Tailwind classes in `internal/web/slips.go` (`resultClass`) with semantic `badge-won`, `badge-lost`, `badge-push`, `badge-void`, and `badge-pending` classes.
- Replaced raw Tailwind classes in `internal/web/analyzer.go` (`gradeClass`, `analyzerSeverity`) with semantic `grade-a`..`grade-f`, `grade-neutral`, `sev-danger`, `sev-warn`, and `sev-info` classes.
- Replaced raw Tailwind classes in `internal/web/research.go` (`buildBarChart` `ResultClass`, `newDefRankBadge` `badgeClass`) with semantic `bar-hit`, `bar-miss`, `bar-push`, `bar-neutral`, `badge-won`, `badge-lost`, and `badge-void` classes.
- Updated existing Tailwind class assertions in `internal/web/slips_test.go` and added semantic class assertions in `internal/web/analyzer_test.go` and `internal/web/research_test.go`.

## Deviations

- The task prompt named `severityBadge` in `internal/web/analyzer.go`, which is actually `analyzerSeverity` in that file; updated `analyzerSeverity` in place.

## Open issues

None.

## Next-phase notes

`internal/web/slips.go`, `internal/web/analyzer.go`, and `internal/web/research.go` now emit only semantic CSS classes for results, grades, severities, chart bars, and defense rank badges. Template updates in Phase 3 can style those elements with theme tokens without touching Go view helpers.

## Commit

Use semantic theme classes in Go view helpers

Replace hardcoded Tailwind color strings in slip result chips, analyzer grades and severity cards, and research bar/defense badges with the semantic CSS classes defined in Phase 1 so all three site themes style them consistently.
