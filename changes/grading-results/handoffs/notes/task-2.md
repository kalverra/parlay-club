## Done

- Added deterministic synthetic fixture `internal/eval/testdata/benchmark_dataset.json` (4 weeks, 24 slips, 72 legs spanning spreads, totals, moneylines, and player props across all A-F bands with weather, injury, nfelo, nflverse, stack, and conflict signals).
- Extended `cmd/eval/main.go` with `--dataset`, `--all`, and `--format markdown` output covering single-rubric, suite, and comparison reports.
- Added CLI tests in `cmd/eval/main_test.go` covering `--dataset`, `--all`, `--format markdown`, and error cases (`--db` vs `--dataset` mutual exclusivity, `--all` with compare flags, unsupported `--format`, and missing dataset).

## Deviations

None.

## Open issues

None.

## Next-phase notes

`cmd/eval --dataset internal/eval/testdata/benchmark_dataset.json --all --format markdown` is ready to be wired into `.github/workflows/ci.yml`.
