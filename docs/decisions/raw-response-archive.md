---
id: adr-raw-response-archive
title: Archive every external response
type: decision
summary: Add one append-only archive of raw responses in SQLite, written by a single seam that every outside call passes through.
last_verified: "2026-11-07"
---

# Archive every external response

## Context

The app reads three outside sources: ESPN, the weather service, and the Odds API. Each fetch gets parsed into game,
line, and weather rows. The parsed rows overwrite the old ones, and `sync_state` keeps only the last fetch time and the
odds credits left. Nothing keeps what the source actually said.

So the history is gone. We can't see how a line moved over a week, what a prop paid at 10 a.m. versus kickoff, or what
the weather forecast looked like two days out. We can't replay a past week against a new analyzer rule. We can't test a
model on real data, because we never saved any.

Odds API data hurts most. It costs scarce credits (500 a month), and every call we throw away is paid for once and never
reused. The other sources are free, but their past responses can't be fetched again. ESPN and the weather service only
show the present.

## Goals and non-goals

**Goals:**

- Keep every successful response from every outside source, forever, in the app's own database.
- Store each one raw and unchanged, with enough labels to find it later: which source, which game or week, when it was
  fetched, and the request that made it.
- Make saving automatic. A new client gets archiving by being a client, with no extra step to forget.
- Make the archive easy to query by source, key, and time, so a later modeling or backtest job can read it without
  touching the app's live tables.
- Keep the archive off every request path. A page load never reads it, and a failed write never breaks a sync.

**Non-goals:**

- Building the models, backtests, or replay tools. This change only keeps the data.
- Parsing the archive into new tables. Raw is the point; later work derives what it needs.
- Showing archived data in the app, or changing the analyzer.
- Pruning, compaction, or export. We'll decide those if size becomes a problem.
- Archiving Google OAuth traffic. It holds no market data and carries secrets.

## Decision

Add one append-only archive of raw responses in SQLite, written by a single seam that every outside call passes through.
Each row holds the source key, the request's identifying inputs, the fetch time, the HTTP status, and the response body
as received. The same source keys that `sync_state` uses become the archive's labels, so the two stay easy to join. The
Odds API credit count from the response headers goes in the row too, so we can see what each fetch cost.

The seam wraps the clients' shared HTTP layer, so the three clients in `slate-and-lines` and the later ones in
`props-research` and `grading-record-book` are archived without each one remembering to do it. Writes happen after the
response is in hand and the sync has what it needs. If the write fails, we log it and move on, because friendly and
never blocking applies to the sync too. Bodies are compressed to keep the file small. An NFL season of lines, props, and
weather is a few hundred megabytes at worst, which fits the database's disk.

The obvious alternatives are worse. Saving files to disk or a bucket splits the data across places and breaks the
single-instance, SQLite-only shape. Parsing into rich history tables now guesses what a future model needs and throws
away fields we haven't learned to want. Logging responses leaves them unstructured and unqueryable. A raw, labeled,
append-only table costs little today and keeps every option open.

Effort is medium. The idea is simple, but the seam must sit under all clients without bending them, handle failures
quietly, and settle storage and schema choices that later modeling will depend on.

## Consequences

Every successful ingest call now leaves a raw, labeled copy in the app's SQLite file. A later backtest or model can read
the bytes a source actually sent, not just the parsed rows that overwrote them, and can see what each Odds API call
cost. New ingest clients inherit the archive by using the shared `cmd/server` client, so forgetting is hard.

The cost is a larger database that Litestream replicates in full, one more small write on the sync path, and a new
schema and package to maintain. The archive is also only as joinable as its labels: a client that skips `WithKey` still
gets rows, but under `host + path` keys that do not line up with `sync_state`. Retention is still manual, and bodies are
kept raw, so a future source that echoes credentials in its body would need a new redaction step.

We would revisit three things after a season of real data: whether the size thresholds in the runbook are right, whether
the free Odds event list should have its own key, and whether any source needs body-level redaction.

## Alternatives considered

- **Files or a bucket.** Rejected because it splits the data off the single SQLite file and adds a second thing to back
  up. The archive rides Litestream with the rest of the app.
- **A `Sink` call inside each client.** Rejected because every future client would have to remember it. The HTTP
  transport makes archiving a property of being an ingest client.
- **Parsed history tables.** Rejected because they guess which fields a future model needs and drop the ones we have not
  learned to want. Raw bytes keep every option open.
- **An async writer or queue.** Not chosen. A synchronous insert is small next to a network call, and a queue adds a
  second failure mode and a place to lose rows. A failed insert is logged and dropped so it never changes the client's
  result.
- **An in-app switch or retention job.** Rejected for now because it adds config and code for a problem we do not have
  yet. The runbook covers manual pruning.
- **A separate `odds:events` key.** Deferred. The free event list and paid lines share one `sync_state` row and one
  credit balance per pass, so one key keeps the archive joinable. The URL path still distinguishes the two calls.
- **Substring matching for key prefixes.** Rejected after review because `substr(key, ...)` bypasses the key index. The
  read API now uses a half-open range, which keeps prefix queries on the B-tree as the archive grows.
- **A silent no-op for a nil sink.** Rejected. The archive is the transport's purpose, so `NewTransport` panics at
  construction rather than quietly archiving nothing in production.
- **Testing the OAuth guard inside `internal/auth`.** Rejected because the test owned a store the provider never saw, so
  it passed by construction. The guard now lives in `cmd/server`, where the ingest client and provider are wired.

## Implementation notes

- `refresh` labels the context once per source, not once per call site. Both Odds calls therefore archive under
  `odds:lines`; that is a deliberate change from the design's per-call examples and is documented in
  `docs/concepts/data-refresh.md`.
- The OAuth guard test needed a seam to point the provider at a local issuer. `newProviderForIssuer` is that seam;
  production still calls `newProvider`, which uses Google. The test first proves the ingest client's archive is live,
  then proves the token exchange adds no row.
- The archive reads at most 8 MiB plus one byte so it can tell an exactly-at-cap body from an oversize one, and puts the
  bytes it read back in front of the client's body. A body over the cap is not saved.
- Only the URL's `apiKey` value is redacted, including when a redirect copies it into another parameter. Bodies are
  stored exactly as received.
- No props call exists yet, so nothing archives under `odds:props:{game}` today. The label shape is ready for the change
  that adds props.
- The archive write happens before the parsed rows and `sync_state` update. The end-to-end test relies on that order; a
  future sync change that reorders it can observe a partial pass.
- The read API is not on any page or request path. It exists for tests and later modeling, and plain SQL against a
  Litestream restore works too.
- `api_bodies` rows cannot orphan while the body insert and response insert stay in one transaction. Rows are
  append-only; the app has no update or delete path.

## Follow-ups

- Add props archiving under `odds:props:{game}` when the props client lands, and extend the wiring test's key counts.
- Decide after a season whether to split the Odds archive into `odds:events` and `odds:lines`, and update the constants,
  wiring test, and docs together if so.
- Add an automated retention or prune plan once `docs/runbooks/archive-size.md` thresholds are hit in production.
- Extend redaction to response bodies if any source starts echoing credentials, with a test that covers the real shape.
- Build the first modeling or backtest reader against the archive, and use it to confirm the key and time filters are
  enough.
- Consider a size or growth alert tied to the runbook queries so the thresholds do not depend on someone remembering to
  check.
