## Done

- Added executable `scripts/eval-prod.sh` to validate `litestream` and `LITESTREAM_BUCKET`/`LITESTREAM_REPLICA_URL`, restore a snapshot into a temporary directory under `./data/` using `deploy/litestream.yml`, clean up the temp directory via `trap` on `EXIT`/`INT`/`TERM`, and run `go run ./cmd/eval --db "$tmpdb" "$@"` (defaulting to `--all`).
- Added `[tasks.eval-prod]` in `mise.toml` invoking `./scripts/eval-prod.sh`.
- Verified `sh -n scripts/eval-prod.sh`, error exits when `litestream` or replica env vars are missing, temp-dir cleanup after a failed mock `litestream restore`, and `mise tasks ls` registration.

## Deviations

None.

## Open issues

None.

## Next-phase notes

Task 7 can update the living docs under `docs/` (including `docs/ops/evals.md` or relevant ops/analyzer docs) and run `good-vibes docs lint`.

## Commit

Add eval-prod task and snapshot eval script

Provide a single local command to restore a read-only Litestream production snapshot into a temporary directory and run `cmd/eval` against it without leaving database files behind.
