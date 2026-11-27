---
slug: archive-storage-optimization
run: 20261127-192256-archive-storage-optimization
outcome: done
generated: 2026-11-27T19:34:22Z
---

# Run summary: archive-storage-optimization

## Outcome

- Status: done
- Branch: archive-storage-optimization
- Close-out: written
- Close: pending
- Final verdict: agy@gemini-4-neon-low approved (issues: 0)

## Totals

- Tasks: 3 of 3
- Phases: 1
- Checkpoints: 4
- Duration: 9m57s
- Cost: $—
- Fresh input tokens: 419158
- Cache read tokens: 2721510
- Cache write tokens: 0
- Output tokens: 32604

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-medium | 2197018 | $— |
| gemini-4-neon-low | 976254 | $— |

## Phases

### Phase 1: Exclude nflverse and nfelo from raw archiving and prune existing bloat

Tasks:

- [x] Wire `nfelo.NewClient` and `nflverse.NewClient` in `cmd/server/main.go` with a non-archiving `*http.Client` (preserving timeout and `User-Agent`), and update `cmd/server/main_test.go` (`TestRefreshArchivesEveryIngestCall`) to assert that `espn`, `weather`, and `odds` are archived while `nflverse` and `nfelo` are not.
- [x] Add a SQLite migration in `internal/db/migrations/` to delete `source IN ('nflverse', 'nfelo')` from `api_responses` and orphaned rows from `api_bodies`, with a migration test in `internal/db/migrate_test.go` confirming ESPN and Odds archive rows survive intact.
- [x] Docs: update `docs/concepts/archive.md`, `docs/reference/data-sources.md`, and `docs/runbooks/archive-size.md` to document the bulk-source exclusion and post-deploy `VACUUM` step.

Phase totals: 4 checkpoints, 9m16s, $—, 2778178 input / 29222 output / 2426677 cache read / 0 cache write tokens

Checkpoints: task-1, task-2, task-3, final Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Wire `nfelo.NewClient` and `nflverse.NewClient` in `cmd/server/main.go` with a non-archiving `*http.Client` (preserving timeout and `User-Agent`), and update `cmd/server/main_test.go` (`TestRefreshArchivesEveryIngestCall`) to assert that `espn`, `weather`, and `odds` are archived while `nflverse` and `nfelo` are not.

- Added `newBulkClient(ingestClient *http.Client) *http.Client` in `cmd/server/main.go`, preserving the ingest timeout (`ingestTimeout` or `ingestClient.Timeout`) with a plain `http.DefaultTransport` (no `archive.Transport`).
- Wired `nfelo.NewClient` and `nflverse.NewClient` in `newProdServices` with `newBulkClient(ingestClient)` so `nfelo` and `nflverse` responses are no longer written to `api_responses` or `api_bodies`, while `User-Agent: parlay-club` continues to be set by each client.
- Updated `TestRefreshArchivesEveryIngestCall` in `cmd/server/main_test.go` with `fakeNfelo` and `fakeNFLverse` servers to assert that `espn`, `weather`, and `odds` are archived while `nfelo` and `nflverse` are not, and that `nfelo`/`nflverse` sync states and parsed `nflverse_team_weeks` rows are still populated.

### Task 2: Add a SQLite migration in `internal/db/migrations/` to delete `source IN ('nflverse', 'nfelo')` from `api_responses` and orphaned rows from `api_bodies`, with a migration test in `internal/db/migrate_test.go` confirming ESPN and Odds archive rows survive intact.

- Added `internal/db/migrations/0022_prune_bulk_archive.sql` to delete `source IN ('nflverse', 'nfelo')` rows from `api_responses` and then delete orphaned `api_bodies` rows not referenced by any remaining `api_responses` row.
- Updated `latestSchemaVersion` in `internal/db/migrate_test.go` and the version assertion in `internal/db/migrate_internal_test.go` from 21 to 22.
- Added `TestMigration0022PruneBulkArchive` in `internal/db/migrate_test.go` verifying that `nflverse` and `nfelo` archive rows and their unreferenced bodies are deleted while `espn`, `odds`, `weather`, and shared bodies survive intact.

### Task 3: Docs: update `docs/concepts/archive.md`, `docs/reference/data-sources.md`, and `docs/runbooks/archive-size.md` to document the bulk-source exclusion and post-deploy `VACUUM` step.

- Updated `docs/concepts/archive.md` to document that `nfelo` and `nflverse` use a plain `*http.Client` (`newBulkClient` in `cmd/server/main.go`) without `archive.Transport`, added the required `## Overview` Mermaid diagram, and brought the front matter and fences into `good-vibes docs lint` compliance.
- Updated `docs/reference/data-sources.md` to exclude `nfelo` and `nflverse` from the raw response archive and remove stale references to archiving those bulk CSVs.
- Updated `docs/runbooks/archive-size.md` to note the bulk-source exclusion and document the one-time post-deploy `VACUUM` step after `internal/db/migrations/0022_prune_bulk_archive.sql`, while updating SQL snippets to `sh` `sqlite3` commands for `good-vibes docs lint`.

## Final gate

- Rounds used: 1; verify failures: 0
- Final verify: passed
- Open issues: none

## Ship

- Sync: none
- Recipe: ask
