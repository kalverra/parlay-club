## Done

- Updated [live-updates.md](docs/concepts/live-updates.md) with the `15m` live / `3m` idle server SSE deadlines, `GRADED` slip `event: bye` short-circuit and omitted `data-live-url`, and the client's `3m` inactivity pause, `pagehide` teardown, and reopen refresh behavior.
- Updated [persistence.md](docs/concepts/persistence.md) with `deploy/litestream.yml`'s `sync-interval: 10s`, compaction `levels` (`5m`, `1h`, `24h`), `snapshot` (`interval: 24h`, `retention: 24h`), and `deploy/setup.sh` clearing GCS soft-delete.
- Updated [gcp-setup.md](docs/guides/gcp-setup.md) to document `--clear-soft-delete` on the Litestream GCS bucket during initial setup, idempotent re-runs, and spend expectations.
- Removed retired `sources` and `last_verified` front-matter keys from `docs/concepts/live-updates.md` and `docs/concepts/persistence.md` so `good-vibes docs lint` passes with `0 error(s)`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

All Phase 2 tasks are complete.
