# Phase 2: Wire it under the clients

## Done

- `cmd/server` now builds one ingest `*http.Client` in `newIngestClient`: a 10 second timeout and
  `archive.NewTransport(http.DefaultTransport, store, nil)` as its transport. All three ingest clients share it, so the
  clients stay database-free and cannot forget a call.
- `refresh.Service.refresh` labels every source's context with `archive.WithKey(ctx, src.key)`, using the same string it
  writes to `sync_state`. No call site needs its own label.
- `TestRefreshArchivesEveryIngestCall` drives one real refresh pass through real ESPN, weather, and Odds clients over
  fake servers. It asserts:
  - the ingest client's transport is `*archive.Transport` and its timeout is 10 seconds;
  - one archive row per outside call: ESPN 1, weather 1, Odds 2 (the free `/events` list plus the paid `/odds` call);
  - every row has the expected key and source, a 2xx status, a non-empty body that decompresses, and no API key in its
    URL;
  - Odds rows carry the credit headers (490/0 for events, 489/1 for lines) and the other sources leave both columns NULL;
  - the parsed game, weather, and odds rows are the ones the clients wrote, and every `sync_state` row still has the
    refresh instant and the paid-lines credit balance.
- `TestGoogleProviderExchangeLeavesTheArchiveEmpty` in `internal/auth` runs a real token exchange against a fake OIDC
  server and asserts `api_responses` stays empty. The auth client remains unwrapped.
- Docs: `docs/reference/data-sources.md` gains a Response archive section, `docs/concepts/data-refresh.md` gains an
  Archive section (and its summary, tags, and sources), `docs/llms.txt` regenerates, and `docs/runbooks/archive-size.md`
  gives the growth queries, the 200 MB file / 1.5 GB uncompressed thresholds, and a prune procedure.

## Deviations

- **One label per source, not per call site.** The design's next-phase notes suggested labeling each call, but `refresh`
  already knows the key for the whole source, so it sets the label once in `Service.refresh`. Every request the source
  makes inherits it. The visible effect is that both Odds calls (`/events` and `/odds`) archive under `odds:lines`.
- **The Odds calls keep one key.** Phase 1's open issue offered `odds:events` as a second label. I kept `odds:lines`
  for both calls because `sync_state` has one Odds row and one credit balance per pass, and splitting the archive key
  would make the credit-bearing rows harder to join. The two calls are distinguishable by URL path, and the fact is
  documented in `docs/concepts/data-refresh.md`.
- **`cmd/server` owns the timeout.** The ingest clients' constructors still default to 10 seconds, but the shared client
  replaces them. The constant `ingestTimeout` lives in `cmd/server` so the archive wiring has one obvious knob.
- **The wiring test stamps real time.** The transport built in `cmd/server` uses its default clock, so the test asserts
  `FetchedAt` is within a minute of now rather than injecting the fixed refresh clock. The archive's own unit tests cover
  the injected clock.

## Open issues

- **Splitting the Odds archive labels is still possible.** If later modeling wants to separate the free event list from
  paid lines, add `OddsEventsSource = "odds:events"`, label that call separately, and update the wiring test's key
  counts. Nothing else stores the key.
- **Props are not archived.** No props call exists yet. When one arrives it should use `odds:props:{game}` so it joins
  its own `sync_state` row.
- **The archive has no retention job.** `docs/runbooks/archive-size.md` covers the manual prune and the thresholds. The
  app still never deletes a row.
- **Secrets in bodies remain unhandled.** Only URLs are redacted. None of the three sources echoes credentials today, but
  this risk from phase 1 still stands.

## Next-phase notes

- `newIngestClient` is the single seam for outside traffic. Any new ingest client built elsewhere will not be archived,
  so build it through this helper or pass the shared client.
- A new source must set `archive.WithKey` through the existing `source.key`, not at the HTTP call. The transport's
  `host + path` fallback still keeps a forgotten label, but the row will not join `sync_state`.
- Keep the auth client separate. `TestGoogleProviderExchangeLeavesTheArchiveEmpty` is the guard.
- Later migrations must bump `latestSchemaVersion` in `internal/db/migrate_test.go` and the version assertion in
  `migrate_internal_test.go`.
- The end-to-end test waits for every `sync_state` key and then reads, exploiting the order in `refresh`: the archive
  row is written before the parsed rows and the sync record. Keep that order if the sync path changes, or the test can
  observe a partial pass.

## Files touched

- `cmd/server/main.go`
- `cmd/server/main_test.go`
- `docs/concepts/archive.md`
- `docs/concepts/data-refresh.md`
- `docs/llms.txt`
- `docs/reference/data-sources.md`
- `docs/runbooks/archive-size.md`
- `internal/auth/provider_test.go`
- `internal/refresh/refresh.go`

## Run log

- Mode: 
- Checkpoints: task-4, task-5
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
