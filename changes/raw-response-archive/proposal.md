---
slug: raw-response-archive
status: designed
created: "2026-11-07"
depends_on:
    - slate-and-lines
effort: medium
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
