# Task 4

## Done

- Replaced the free-text `home_book` input in `web/templates/profile.html` with a segmented DraftKings / FanDuel / No preference radio selector and updated `web/static/app.css` via `go generate ./...`.
- Added `validateHomeBook` and `normalizeSelectableHomeBook` in `internal/web/slips.go` so `POST /me` accepts `""`, `"draftkings"`, or `"fanduel"` (rejecting other values with `422`) and `GET /me` displays legacy unknown keys as `"No preference"`.
- Added `exportBooksForHomeBook` in `internal/web/export.go` and updated `exportView` to order the export card's sportsbooks with the viewer's `homeBook` first.
- Updated `TestProfilePageRendersTheHomeBookForm` and added `TestUpdateProfileValidatesHomeBookAndNormalizesLegacyKeys` in `internal/web/slips_test.go`, plus `TestExportCardOrdersBooksWithHomeBookFirst` in `internal/web/export_test.go`.
- Verified `good-vibes verify generate`, `good-vibes verify lint`, and `go test -short ./internal/web/...` pass.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Remaining work can rely on `home_book` being `""`, `"draftkings"`, or `"fanduel"` for any newly saved profile, with legacy unknown keys normalized to `""` on display.

## Commit

Add DK/FD profile selector and home-book export order

Replace the free-text home book input with a DraftKings, FanDuel, or No preference selector validated on save. Ordering the export card with the viewer's home book first keeps their primary sportsbook as the top tap target on mobile.
