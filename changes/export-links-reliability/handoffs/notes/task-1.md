# Task 1

## Done

- Updated `Store.UpsertOdds` in `internal/db/odds.go` so `ON CONFLICT DO UPDATE` preserves stored `link` and `sid` when incoming values are `NULL` on an unchanged line (`odds.line IS excluded.line`), clears stale `link` and `sid` when the line moves, and preserves `event_sid` via `COALESCE(excluded.event_sid, odds.event_sid)`.
- Updated the `WHERE` clause in `Store.UpsertOdds` so incoming `NULL` link/sid/event_sid values on an unchanged row do not trigger spurious updates.
- Added `TestStoreUpsertOddsLinkPreservation` in `internal/db/odds_test.go` covering unchanged-line no-op, unchanged-line link preservation (both numeric and `nil` lines), moved-line `link`/`sid` clearing with `event_sid` preservation, and non-null overwrite; updated `TestUpsertOddsStoresAndRefreshesLinksWithPrices` in `internal/db/sync_test.go` to match the link-preserving contract.

## Deviations

None.

## Open issues

None.

## Next-phase notes

`Store.UpsertOdds` now assumes `NULL` means "no link supplied" and preserves existing links only when `odds.line IS excluded.line`. Callers that want to replace a non-preferred link (such as a non-`42.` FanDuel link) must supply the preferred non-null link in the merged row.

## Commit

Preserve odds deep links on NULL upserts

Subsequent Parlay-only refreshes emit NULL link, sid, and event_sid values
that previously overwrote enriched links in SQLite. Keeping stored links on
unchanged lines and clearing only line-specific link and sid on line moves
prevents link loss while avoiding stale deep links.
