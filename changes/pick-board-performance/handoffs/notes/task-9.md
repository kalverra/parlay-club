# Task 9: Docs: update `docs/guides/local-dev.md` with benchmark and `pprof` / `Server-Timing` debugging commands

## Done

- Added a **Benchmarks and profiling** section to `docs/guides/local-dev.md` documenting `mise run bench`, targeted `go test -bench` runs, non-prod `Server-Timing` headers and request/SQLite slow-path logs, and non-prod `/debug/pprof/` `go tool pprof` commands.
- Removed the retired `sources` and `last_verified` front-matter fields from `docs/guides/local-dev.md` so `good-vibes docs lint` passes with zero errors, and regenerated `docs/llms.txt` via `good-vibes docs index`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Phase 4 is ready to close after the binary runs `lint` and `test` gates.
