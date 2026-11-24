# Phase 2: Phase 2: Semantic status, grade, and result roles in Go view builders

## Done

### Task 3: Replace raw Tailwind class strings in `internal/web/slips.go` (`resultClass`), `internal/web/analyzer.go` (`gradeClass`, `severityBadge`), and `internal/web/research.go` (`ResultClass`, `badgeClass`) with semantic CSS classes defined in Phase 1, and update the corresponding assertions in `internal/web/slips_test.go`, `internal/web/analyzer_test.go`, and `internal/web/research_test.go`.

- Replaced raw Tailwind classes in `internal/web/slips.go` (`resultClass`) with semantic `badge-won`, `badge-lost`, `badge-push`, `badge-void`, and `badge-pending` classes.
- Replaced raw Tailwind classes in `internal/web/analyzer.go` (`gradeClass`, `analyzerSeverity`) with semantic `grade-a`..`grade-f`, `grade-neutral`, `sev-danger`, `sev-warn`, and `sev-info` classes.
- Replaced raw Tailwind classes in `internal/web/research.go` (`buildBarChart` `ResultClass`, `newDefRankBadge` `badgeClass`) with semantic `bar-hit`, `bar-miss`, `bar-push`, `bar-neutral`, `badge-won`, `badge-lost`, and `badge-void` classes.
- Updated existing Tailwind class assertions in `internal/web/slips_test.go` and added semantic class assertions in `internal/web/analyzer_test.go` and `internal/web/research_test.go`.

## Deviations

### Task 3

- The task prompt named `severityBadge` in `internal/web/analyzer.go`, which is actually `analyzerSeverity` in that file; updated `analyzerSeverity` in place.

## Open issues

None.

## Next-phase notes

### Task 3

`internal/web/slips.go`, `internal/web/analyzer.go`, and `internal/web/research.go` now emit only semantic CSS classes for results, grades, severities, chart bars, and defense rank badges. Template updates in Phase 3 can style those elements with theme tokens without touching Go view helpers.

## Files touched

- `internal/web/analyzer.go`
- `internal/web/analyzer_test.go`
- `internal/web/research.go`
- `internal/web/research_test.go`
- `internal/web/slips.go`
- `internal/web/slips_test.go`
- `web/static/app.css`

## Run log

- Mode: 
- Checkpoints: task-3
- Verify: verify passed: generate changed: web/static/app.css, lint 4.2s, test 4.7s; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
