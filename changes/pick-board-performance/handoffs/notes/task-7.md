## Done

- Verified existing prod-scale benchmarks in `internal/board/board_test.go` (`BenchmarkBuild`, `BenchmarkSlates`, `BenchmarkDetail`) seeded with 14 games, 15,500 odds rows, and 2,100 roster rows.
- Added prod-scale benchmarks in `internal/db/sync_test.go`: `BenchmarkOddsForWeek` (15,500 rows across 14 games), `BenchmarkUpsertOdds` (1,200-row prop batch), and `BenchmarkConcurrentReadsDuringWrite` (parallel `OddsForWeek` reads during continuous `UpsertOdds` writes).
- Added prod-scale handler benchmarks in `internal/web/pick_test.go`: `BenchmarkPickBoard`, `BenchmarkPickDetail`, and `BenchmarkPickSlate` covering full SQLite load, board build, and HTML template rendering.
- Added `[tasks.bench]` to `mise.toml` running `go test -run='^$' -bench=. -benchmem ./internal/board ./internal/db ./internal/web`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Run `mise run bench` for baseline ns/op and allocs/op numbers across `internal/board`, `internal/db`, and `internal/web`.
