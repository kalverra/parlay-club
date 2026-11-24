# Task 1: Rename Thursday scope label to Thursday Slate

## Done

- Updated `db.SlateScopeThursday.Label()` in `internal/db/slips.go` from `"Thursday Night"` to `"Thursday Slate"`.
- Updated the static `<option value="thursday">` text in `web/templates/slip.html` and `web/templates/week.html` to `"Thursday Slate"`, and updated the schedule name placeholder in `web/templates/week.html` to `"e.g. Thursday Slip"`.
- Updated `TestSlateScopeLabel` in `internal/db/slips_test.go`, `TestCreateSlipFormSlateScopeSelection` in `internal/web/slips_test.go`, and added `"Thursday Slate"` scope badge assertions to `TestGroupWeekRendersMultipleSlips` and `TestGroupWeekAutoCreatesMultipleScheduledSlipsAndRendersView` in `internal/web/home_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `internal/web/home_test.go` and `internal/web/slips_test.go` still use `"Thursday Night"` as test slip/schedule *names* in existing fixtures; those are user-facing slip names, not scope labels.

## Commit

Rename Thursday scope label to Thursday Slate

The Thursday slate scope matches all Thursday Eastern kickoffs, including Thanksgiving afternoon games, not just Thursday Night Football. Renaming the label to Thursday Slate makes the scope badge and selector accurate for any Thursday slate.
