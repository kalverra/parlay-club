---
id: archive
title: Response archive
type: concept
summary: How every successful external API response is kept raw in SQLite, how the source key labels it, and how to read the stored metadata and gzipped bodies.
tags: [archive, raw-responses, api, sqlite, http, transport, gzip, sha256, labels, odds, credits]
sources: [internal/ingest/archive/transport.go, internal/ingest/archive/labels.go, internal/ingest/archive/redact.go, internal/db/archive.go, internal/db/archive_read.go, internal/db/migrations/0003_api_archive.sql]
last_verified: 2026-11-09
---

# Response archive

## Summary

Every successful call to ESPN, Open-Meteo, or Parlay API is saved before the client parses it. The archive keeps the
raw bytes, not a guess at which fields a model will want later. It lives in the same SQLite file as the rest of the app,
so Litestream backs it up with everything else.

## Where it sits

`archive.Transport` is an `http.RoundTripper` that wraps the ingest HTTP client's base transport in `cmd/server`. The
clients in `internal/ingest/{espn,weather,odds}` do not know it exists and still never touch the database.

```text
refresh → ingest client → archive.Transport → network → outside API
                               ↓ 2xx
                       api_responses + api_bodies
```

The transport reads a successful body into memory, puts the same bytes back for the client, and hands one `Record` to
`Store.SaveAPIResponse`. It must wrap the base transport, not sit beside it, so no client can forget a call.

`internal/auth` builds its own client and stays unwrapped. OAuth tokens never reach the archive.

## What is kept

A response is archived only when all of these hold:

- the request reached the source and returned a 2xx status;
- the body could be read;
- the uncompressed body is at most 8 MiB.

Everything else passes through unrecorded: non-2xx responses, transport errors, timeouts, unreadable bodies, and bodies
over the cap. The client sees its original result in every case.

Each kept call writes one `api_responses` row with:

| Column              | Value                                                              |
| ------------------- | ------------------------------------------------------------------ |
| `source`            | `espn`, `weather`, `odds`, or the host for an unknown source.     |
| `key`               | The label from the request context, or `host + path` as a fallback. |
| `method`            | The HTTP method.                                                   |
| `url`               | The request URL with the odds API's `apiKey` value replaced by `REDACTED`. |
| `status`            | The 2xx code.                                                      |
| `fetched_at`        | When the response finished being read, in RFC 3339 UTC.            |
| `duration_ms`       | Time from sending the request to finishing the body read.          |
| `body_sha256`       | Hex SHA-256 of the uncompressed body.                              |
| `credits_remaining` | `x-requests-remaining`, odds only; `NULL` otherwise.               |
| `credits_used`      | `x-requests-last`, odds only; `NULL` otherwise.                    |

The body goes to `api_bodies`, gzipped, keyed by the hash of the uncompressed bytes. Identical payloads share one body
row, so a scoreboard polled every ten minutes adds small response rows and no duplicate bytes. A changed payload adds a
second body row.

Only the `apiKey` value is redacted. Other query parameters stay, because `markets`, `regions`, and `week` are what make
the URL meaningful. Bodies are stored as the source sent them; none of the three sources echoes credentials today.

Rows are append-only. The app never updates or deletes an archived response or body, and there is no retention job.

## Labels

The transport cannot tell `odds:props:401` from `odds:lines` by looking at the URL, so `refresh` puts the key in the
request context:

```go
ctx = archive.WithKey(ctx, "odds:props:401")
```

The key is the same string `sync_state.source` uses. `source` comes from the part before the first colon, lowercased.
Without a label, the transport falls back to `host + path` and derives `source` from the host: `espn.com`,
`open-meteo.com`, and `parlay-api.com` (or the older `the-odds-api.com`) map to `espn`, `weather`, and `odds`; any other
host becomes its own source.

Labelled rows join cleanly with `sync_state`. Fallback rows are still kept but are harder to group, so every client
should set a label. The constants `archive.SourceESPN`, `archive.SourceWeather`, and `archive.SourceOdds` name the three
sources in code.

## How to query it

Nothing in a page, route, or request path reads the archive. The read API exists for tests and later modeling work.
`db.Store` offers:

- `ListAPIResponses(ctx, db.APIResponseFilter{KeyPrefix, Since, Until})` returns metadata rows, oldest first, never a
  body. A zero filter keeps everything. `KeyPrefix` compares the literal prefix, so `%` and `_` in a key are not
  wildcards, and `Since` and `Until` are inclusive.
- `APIResponseBody(ctx, sha)` returns one body decompressed and checks it against the stored uncompressed size. A
  missing hash wraps `db.ErrAPIBodyNotFound`.

```go
list, err := store.ListAPIResponses(ctx, db.APIResponseFilter{
    KeyPrefix: "odds:props:",
    Since:     weekStart,
    Until:     weekEnd,
})
body, err := store.APIResponseBody(ctx, list[0].BodySHA256)
```

Plain SQL works too, and is the easiest path for analysis against a Litestream restore. List the metadata without
touching bodies:

```sql
SELECT id, source, key, url, status, fetched_at, duration_ms,
       credits_remaining, credits_used, body_sha256
FROM api_responses
WHERE key LIKE 'odds:props:%'
  AND fetched_at >= '2026-11-01T00:00:00Z'
ORDER BY fetched_at;
```

Join in the stored size to see what a slice of history costs:

```sql
SELECT r.key, count(*) AS calls, sum(b.size) AS uncompressed_bytes
FROM api_responses r
JOIN api_bodies b ON b.sha256 = r.body_sha256
GROUP BY r.key
ORDER BY uncompressed_bytes DESC;
```

Bodies are gzip blobs. In Go, use `APIResponseBody` so the decoding and size check happen for you. From a shell, write
the blob to a file and decompress it:

```sh
sqlite3 -noheader app.db \
  "SELECT body FROM api_bodies WHERE sha256 = 'the-hash'" > body.gz
gunzip -c body.gz > response.json
```

The stored body is the exact bytes the source returned, so it can be replayed through the same client parser.

## Growth

Bodies are gzipped and deduplicated, so a season of lines, props, and weather should stay in the low hundreds of MB.
Watch `SELECT sum(size) FROM api_bodies` and the database file size at the end of the first season. The
[archive size check](../runbooks/archive-size.md) has the thresholds and the prune steps. The archive rides the
replicated database file, so any growth is also replication traffic.
