## Done

- Mounted `GET /record/members/{id}` in `internal/web/slips.go` and implemented `recordMember`, `RecordMemberPage`, `RecordMemberBetView`, and shared season loading/view helpers in `internal/web/record.go`.
- Registered `"record-member"` in `internal/web/render.go` and updated the template list in `internal/web/render_test.go`.
- Built `web/templates/record_member.html` with a back link to `/record`, season selector, member header with rank, 4-stat summary strip, clickable `data-tag-explain` chips with `tag-explain-dialog`, and newest-first graded leg cards linking to `/slips/{id}`.
- Added `TestRecordMemberHistoryScopingAndRendering` in `internal/web/record_test.go` covering `400` invalid ID/season, `404` out-of-group scoping, newest-first ordering across regular and postseason weeks, sole-killer/vote/market/odds rendering, and empty-season state.

## Deviations

None.

## Open issues

Living docs update for the Record Book tags and member history screen remains for the next task in Phase 3.

## Next-phase notes

`GET /record/members/{id}?season=` is live and renders `web/templates/record_member.html` using `#tag-explain-dialog`.

## Commit

Add member bet history page to Record Book

Give group members a drill-down view of a teammate's season summary, earned tags, and newest-first graded leg history while enforcing active-group scoping.
