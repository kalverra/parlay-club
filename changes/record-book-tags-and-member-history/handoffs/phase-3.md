# Phase 3: Phase 3: Member bet history screen and living docs

## Done

### Task 5: Add `GET /record/members/{id}` in `internal/web/slips.go` and `internal/web/record.go`, register `"record-member"` in `internal/web/render.go`, build `web/templates/record_member.html` with summary strip, clickable tags, and newest-first graded leg history, and add scoping and rendering tests in `internal/web/record_test.go`.

- Mounted `GET /record/members/{id}` in `internal/web/slips.go` and implemented `recordMember`, `RecordMemberPage`, `RecordMemberBetView`, and shared season loading/view helpers in `internal/web/record.go`.
- Registered `"record-member"` in `internal/web/render.go` and updated the template list in `internal/web/render_test.go`.
- Built `web/templates/record_member.html` with a back link to `/record`, season selector, member header with rank, 4-stat summary strip, clickable `data-tag-explain` chips with `tag-explain-dialog`, and newest-first graded leg cards linking to `/slips/{id}`.
- Added `TestRecordMemberHistoryScopingAndRendering` in `internal/web/record_test.go` covering `400` invalid ID/season, `404` out-of-group scoping, newest-first ordering across regular and postseason weeks, sole-killer/vote/market/odds rendering, and empty-season state.

### Task 6: Docs: update `docs/concepts/record-book.md` (including fixing its stale front-matter fields) and `docs/reference/routes.md` to document the expanded tag catalog, clickable tag explanations, and `GET /record/members/{id}?season=`.

- Updated `docs/concepts/record-book.md`: removed stale `sources` and `last_verified` front-matter fields, added `## Overview` with a Mermaid flow diagram, and documented `ImpliedLift`, `VotesForGroupSeason`, fail-soft `eval.Cache` leg grades, the 14-tag catalog with thresholds and deterministic ordering, `#tag-explain-dialog` clickable tag explanations, and `GET /record/members/{id}?season=`.
- Updated `docs/reference/routes.md`: added `GET /record/members/{id}?season=` to the Record Book route table, documented its `400`/`404` scoping and rendered sections, updated `GET /record` notes for votes, cached grades, tag chips, and member links, and bumped `budget.max_lines` from 620 to 650.
- Ran `good-vibes docs lint` (0 errors; 0 warnings on both updated docs) and `good-vibes docs index` (updating `docs/llms.txt`).

## Deviations

None.

## Open issues

### Task 5

Living docs update for the Record Book tags and member history screen remains for the next task in Phase 3.

## Next-phase notes

### Task 5

`GET /record/members/{id}?season=` is live and renders `web/templates/record_member.html` using `#tag-explain-dialog`.

## Files touched

- `docs/concepts/record-book.md`
- `docs/llms.txt`
- `docs/reference/routes.md`
- `internal/web/record.go`
- `internal/web/record_test.go`
- `internal/web/render.go`
- `internal/web/render_test.go`
- `internal/web/slips.go`
- `web/static/app.css`
- `web/templates/record_member.html`

## Run log

- Mode: 
- Checkpoints: task-5, task-6
- Verify: verify passed: test 400ms; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
