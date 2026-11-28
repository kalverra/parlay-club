# Phase 2: Phase 2: Record Book leaderboard tags, clickable explanations, and member links

## Done

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

## Deviations

None.

## Open issues

### Task 3

- The `<dialog id="tag-explain-dialog">` partial and `web/static/app.js` click handler are not wired yet (scheduled for the next task in Phase 2), and `/record/members/{id}` is not mounted yet (Phase 3).

## Next-phase notes

### Task 3

- `RecordMemberView` now exposes `UserID`, `ImpliedLift` (formatted `%+.1f pts` or `""`), and `Tags` (`[]RecordTagView` with `Key`, `Label`, `Tone`, `IsPraise`, `Rule`, `Explanation`).
- `web/templates/record.html` already emits `data-tag-explain`, `data-tag-label`, `data-tag-tone`, `data-tag-rule`, and `data-tag-explanation` on each tag button, ready for `#tag-explain-dialog` and `app.js`.

### Task 4

- Phase 3 (`web/templates/record_member.html`) can include `{{template "tag-explain-dialog"}}` and emit `<button type="button" data-tag-explain data-tag-label="..." data-tag-tone="..." data-tag-rule="..." data-tag-explanation="...">` chips directly; `web/static/app.js` already handles them via document-level delegation.

## Files touched

- `internal/web/home.go`
- `internal/web/record.go`
- `internal/web/record_test.go`
- `web/static/app.css`
- `web/static/app.js`
- `web/templates/partials.html`
- `web/templates/record.html`

## Run log

- Mode: 
- Checkpoints: task-3, task-4
- Verify: verify passed: generate reused, lint 2.8s, test 8.4s; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
