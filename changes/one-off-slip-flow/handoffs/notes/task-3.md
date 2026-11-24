# Task 3

## Done

- Added `HasExistingSlips bool` to `SlipCreateView` in `internal/web/slips.go` and updated `createView` to load `SlipsForWeek` once, set `HasExistingSlips`, and pass the slice to `nextSlipName`.
- Updated `web/templates/slip.html` so weeks with existing slips show *"Add a slip for {{.Page.WeekLabel}}"* and *"Pick a slate, legs per member, and who's in."*, while empty weeks keep the *"Start ... / No slip ... yet"* copy.
- Wrapped the *"Or add a past week"* `<fieldset>` in `web/templates/slip.html` inside a `<details data-past-week>` that opens only when `len(.Create.Weeks) == 0` or `.Create.Season != 0 || .Create.WeekNum != 0`.
- Added `TestCreateFormStates` in `internal/web/slips_test.go` and `<details data-past-week>` state assertions in `internal/web/home_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

None.

## Commit

Collapse past-week fields on create-slip form

Existing-slip weeks previously showed "No slip for Week X yet" copy and an expanded past-week fieldset above Name, Slate scope, and Members. Adding `HasExistingSlips` and collapsing the past-week fieldset into `<details>` keeps the primary slip fields above the fold on phone screens and surfaces accurate copy.
