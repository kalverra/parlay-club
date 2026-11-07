# Phase 1: Archive transport and store

## Done

- `0003_api_archive.sql` adds `api_bodies` and `api_responses` plus the `key, fetched_at` and `source, fetched_at`
  indexes. `api_responses.status` is constrained to 2xx. Both migration tests now pin version 3 and expect the new
  tables.
- `internal/ingest/archive` has the seam:
  - `Transport` wraps the base `RoundTripper`, archives only fully read 2xx bodies up to 8 MiB, and restores the same
    bytes for the client.
  - Non-2xx responses, transport errors, body read errors, and oversize bodies pass through unrecorded. A sink error is
    logged and dropped, so the archive never changes what the client sees.
  - Odds responses read `X-Requests-Remaining` and `X-Requests-Last`; other sources leave both columns NULL.
  - `WithKey` / `KeyFrom` carry the `sync_state` label in the request context. Without a label the key falls back to
    `host + path`, and `source` comes from the key prefix or the known host.
  - `redactURL` replaces the `apiKey` value with `REDACTED` and keeps every other query parameter.
- `db.Store.SaveAPIResponse` hashes the uncompressed body, gzips it, inserts it with `INSERT OR IGNORE`, and inserts the
  response row in the same transaction. It satisfies `archive.Sink`.
- The read API is `db.Store.ListAPIResponses` (literal key prefix plus inclusive `Since`/`Until`, oldest first) and
  `db.Store.APIResponseBody` (decompress plus size check, missing hash wraps `db.ErrAPIBodyNotFound`).
- Tests cover success, non-2xx, transport error/timeout, sink failure, oversize, body dedupe and change, redaction
  including a redirect, credit headers, both label fallbacks, metadata filtering, and a byte-for-byte body round trip
  against a temp DB.
- Docs add the tables and an archive section to `docs/reference/schema.md`, add `docs/concepts/archive.md`, and
  regenerate `docs/llms.txt`.

## Deviations

- **Redaction is stronger than the design.** It replaces every other appearance of the same `apiKey` value in the
  stored URL, not just the `apiKey` parameter. A redirect that copies the key into another parameter therefore cannot
  leak it. No interface or schema change.
- **Oversize and read-error handling keeps the client whole.** The transport reads at most `cap + 1` bytes so it can
  tell "exactly at the cap" from "over", then puts those bytes back in front of the untouched remainder. The design only
  said oversize bodies pass through; this is the mechanism that makes it safe.
- **A labelled key's prefix is lowercased** when deriving `source`. Today's keys are already lowercase, so this only
  keeps the stored `source` values consistent.
- **The timeout test is a stub transport** returning `context.DeadlineExceeded`, not a deliberately slow server. It
  exercises the same pass-through path without a timing-dependent test.

## Open issues

- **The transport is not wired yet.** `cmd/server/main.go` still passes `nil` clients, so each ingest client builds its
  own `http.Client`. Production archives nothing until phase 2, and `docs/concepts/archive.md` already describes the
  `cmd/server` wiring as if it exists. Phase 2 must make that true.
- **The Odds client makes two calls with one `sync_state` key.** `Events` is the free event list and `Lines` is the paid
  lines call, but both currently report under `odds:lines`. Phase 2 should choose two archive labels (for example
  `odds:events` and `odds:lines`) if the archive should distinguish them. This is not in the design's label examples.
- **`odds:props:{game}` is an example only.** No props call exists yet; that arrives with a later change.
- **Bodies are stored raw.** Only URLs are redacted. None of the three sources echoes credentials in its body today, but
  the design's "secrets in bodies" risk still stands if that changes.
- **The archive has no size guard or cleanup.** That is deliberate for this phase. `api_bodies` rows cannot orphan while
  both inserts stay in one transaction, and rows are append-only.

## Next-phase notes

- Build one ingest `*http.Client` in `cmd/server` (with its current timeout) whose transport is
  `archive.NewTransport(http.DefaultTransport, store, nil)`, and pass it to all three clients. Do not touch the client
  in `internal/auth`; add the OAuth-unrecorded test there or in the wiring test.
- Set `archive.WithKey` at every refresh call site, using the existing constants: `refresh.ScoreboardSource`,
  `refresh.OddsLinesSource`, `refresh.WeatherSource(game.ID)`. If the events call gets its own key, define it beside
  those constants so the archive and `sync_state` stay joinable.
- Add the end-to-end fake-server test: one refresh pass over ESPN, weather, and Odds leaves one archive row per HTTP
  call, parsed rows and `sync_state` stay unchanged, and reopening the same game inside the cache window adds one props
  row, not two.
- Update the remaining docs in phase 2: the data-source doc, the Refresh section of the architecture doc, and the
  runbook size check. `docs/concepts/archive.md` already has the growth query and needs no rewrite beyond confirming the
  wiring.
- Later migrations must bump `latestSchemaVersion` in `internal/db/migrate_test.go` and the version assertion in
  `migrate_internal_test.go`.

## Files touched

- `docs/concepts/archive.md`
- `docs/llms.txt`
- `docs/reference/schema.md`
- `internal/db/archive.go`
- `internal/db/archive_read.go`
- `internal/db/archive_test.go`
- `internal/db/migrate_internal_test.go`
- `internal/db/migrate_test.go`
- `internal/db/migrations/0003_api_archive.sql`
- `internal/ingest/archive/labels.go`
- `internal/ingest/archive/redact.go`
- `internal/ingest/archive/transport.go`
- `internal/ingest/archive/transport_test.go`

## Run log

- Mode: 
- Checkpoints: task-1, task-2, task-3
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
