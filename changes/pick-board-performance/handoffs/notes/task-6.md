## Done

- Updated `docs/reference/schema.md` (`## Connection settings`) to document the single-writer handle (`SetMaxOpenConns(1)`, `_txlock=immediate`), the file-backed `query_only=ON` read pool (`SetMaxOpenConns(4)`, `SetMaxIdleConns(4)`) opened by `db.NewStore`, `:memory:` handle sharing, and `Store.Close`.
- Updated `docs/concepts/data-refresh.md` (`## What kicks a sync` and `## Odds credits`) to document the non-blocking `KickPickLines` and `KickPickGame` pick-time refresh flow (`context.WithoutCancel`, `s.wg`, `singleflight` via `SyncPickLines`/`SyncPickGame`, shutdown cancellation, and `board-updated` week publication on change) alongside synchronous `ForcePickRefresh`, `SyncGameProps`, and research syncs.
- Verified documentation with `good-vibes docs lint`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

None.
