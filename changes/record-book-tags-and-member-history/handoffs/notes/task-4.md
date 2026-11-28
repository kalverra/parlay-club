## Done

- Added the shared `tag-explain-dialog` `<dialog class="leg-sheet">` partial in `web/templates/partials.html` with tone pill, title, member-specific explanation, rule box, and close button.
- Included `{{template "tag-explain-dialog"}}` in `web/templates/record.html` alongside the existing interactive `[data-tag-explain]` praise/insult tag chips.
- Added delegated `[data-tag-explain]`, `[data-close-tag-explain]`, and backdrop click handling in `web/static/app.js` to populate and toggle `#tag-explain-dialog`.
- Updated `internal/web/record_test.go` to verify `#tag-explain-dialog`, `data-tag-rule`, `data-close-tag-explain`, and that every `tag-explain-*` ID referenced in `web/static/app.js` renders on `GET /record`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Phase 3 (`web/templates/record_member.html`) can include `{{template "tag-explain-dialog"}}` and emit `<button type="button" data-tag-explain data-tag-label="..." data-tag-tone="..." data-tag-rule="..." data-tag-explanation="...">` chips directly; `web/static/app.js` already handles them via document-level delegation.

## Commit

Add shared tag explanation dialog and click handler

Interactive tag chips on the Record Book leaderboard need a mobile-safe bottom sheet so members can tap any praise or insult tag to see why it was earned without clipping inside tables or narrow viewports.
