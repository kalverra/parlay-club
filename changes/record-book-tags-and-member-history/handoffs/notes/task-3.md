# Task 3 Notes

## Done

- Wired `Store.VotesForGroupSeason` and fail-soft `EvalCache.GetOrCompute` leg grades (`SuiteReport.LegGrades`) into `GET /record` in `internal/web/record.go` (and updated `buildGroupRecordSnapshot` in `internal/web/home.go` for the expanded `buildRecordBook` signature).
- Added `RecordTagView` and populated `RecordMemberView.UserID`, `ImpliedLift`, and `Tags` in `buildRecordPage`.
- Updated `web/templates/record.html` to link each member row (both mobile cards and desktop table) to `/record/members/{id}?season=` and render `.Tags` as sibling `<button type="button" data-tag-explain ...>` chips outside the `<a>` link.
- Updated `internal/web/record_test.go` to verify member links, tag button attributes, non-nesting of tag buttons inside links, `VotesForGroupSeason` tag output, and fail-soft behavior when eval grades are unavailable.

## Deviations

None.

## Open issues

- The `<dialog id="tag-explain-dialog">` partial and `web/static/app.js` click handler are not wired yet (scheduled for the next task in Phase 2), and `/record/members/{id}` is not mounted yet (Phase 3).

## Next-phase notes

- `RecordMemberView` now exposes `UserID`, `ImpliedLift` (formatted `%+.1f pts` or `""`), and `Tags` (`[]RecordTagView` with `Key`, `Label`, `Tone`, `IsPraise`, `Rule`, `Explanation`).
- `web/templates/record.html` already emits `data-tag-explain`, `data-tag-label`, `data-tag-tone`, `data-tag-rule`, and `data-tag-explanation` on each tag button, ready for `#tag-explain-dialog` and `app.js`.

## Commit

Wire eval leg grades, votes, tags, and member links into Record Book

Surfacing earned praise/insult tags and member drill-down links on `/record` lets group members see who is hot, cold, or fading the group without leaving the leaderboard. Fail-soft eval grade loading keeps the Record Book available even when pre-kickoff replay data is missing or errors.
