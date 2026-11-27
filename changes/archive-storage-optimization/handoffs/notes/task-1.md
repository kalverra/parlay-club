## Done

- Added `newBulkClient(ingestClient *http.Client) *http.Client` in `cmd/server/main.go`, preserving the ingest timeout (`ingestTimeout` or `ingestClient.Timeout`) with a plain `http.DefaultTransport` (no `archive.Transport`).
- Wired `nfelo.NewClient` and `nflverse.NewClient` in `newProdServices` with `newBulkClient(ingestClient)` so `nfelo` and `nflverse` responses are no longer written to `api_responses` or `api_bodies`, while `User-Agent: parlay-club` continues to be set by each client.
- Updated `TestRefreshArchivesEveryIngestCall` in `cmd/server/main_test.go` with `fakeNfelo` and `fakeNFLverse` servers to assert that `espn`, `weather`, and `odds` are archived while `nfelo` and `nflverse` are not, and that `nfelo`/`nflverse` sync states and parsed `nflverse_team_weeks` rows are still populated.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Task 2 can now add the forward migration pruning historical `nflverse` and `nfelo` rows from `api_responses` and orphaned `api_bodies` without new rows being re-archived by `cmd/server`.

## Commit

Skip raw HTTP archiving for nflverse and nfelo

Bulk season datasets from nflverse and nfelo already upsert directly into their parsed tables and are never read from the raw archive by eval replay. Wiring them with a plain HTTP client in cmd/server stops ~70 MiB of duplicate raw CSV payloads from accumulating in SQLite while keeping ESPN, weather, and odds archiving intact.
