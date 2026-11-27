---
id: adr-archive-storage-optimization
title: Trim raw archive bloat from nflverse and nfelo
type: decision
summary: Exclude bulk static/season datasets (`nflverse` and `nfelo`) from `api_bodies` archiving (keeping their parsed tables updated in place) and run a one-time cleanup/vacuum of historical `nflverse` and `nfelo` archive rows.
area: odds-and-refresh
touched:
    - cmd/server
    - internal/db
    - internal/db/migrations
related:
    - archive
    - archive-size
    - data-sources
    - schema
---

# Trim raw archive bloat from nflverse and nfelo

## Context

The production SQLite database (`backups/parlay-2026-11-27.db`) has grown to 90.2 MiB, and Litestream's GCS replica
directory is 2.7 GiB because every Cloud Run cold start restores and re-uploads a full ~90 MiB snapshot.

84% of the database (75.8 MiB) sits in `api_bodies`, the append-only raw HTTP response archive. Every ingest client
shares `archive.Transport`, which gzips and permanently appends every 2xx response up to 8 MiB keyed by SHA-256. Each
time nflverse updates its season CSVs or nfelo updates projections, the bytes change and a brand-new multi-MB copy is
appended forever:

- `nflverse:season:2026`: **60.0 MiB** gzipped (217.6 MB raw across 32 unique bodies) — 67% of the entire database.
- `nfelo:projections`: **10.3 MiB** gzipped (46.0 MB raw across 20 unique bodies) — 11% of the entire database.
- All parsed app tables combined (`odds`, `nflverse_player_weeks`, `games`, `slips`, `legs`, `roster_players`) take only
  ~14 MiB.

Neither `nflverse` nor `nfelo` raw bodies are read anywhere in production or offline evals. Production routes never read
`api_bodies`, and offline eval replay only reads ESPN (`summary`, `roster`, `gamelog`) and Odds (`lines`, `props`) from
the archive while reading nflverse directly from its week-keyed parsed tables. Because Cloud Run runs with 512 MiB of
RAM and holds `/data/parlay.db` in memory, letting unused raw archives grow risks hitting the memory ceiling and slows
down every cold start.

```mermaid
flowchart LR
  NFLV["nflverse CSVs (60 MiB) & nfelo (10.3 MiB)"] --> Transport["archive.Transport (append forever)"]
  ESPN["ESPN & Odds (4 MiB)"] --> Transport
  Transport --> Bodies["api_bodies (75.8 MiB / 84% of DB)"]
  Bodies -->|"read by eval replay"| Eval["Offline evals (ESPN & Odds only)"]
  NFLV -->|"parsed & upserted in place"| Tables["nflverse_* tables (3 MiB)"]
```

## Goals and non-goals

**Goals:**

- Stop archiving raw HTTP bodies for sources that are already parsed into week-keyed tables and never replayed from raw
  bytes (`nflverse` and `nfelo`), or retain at most the single latest body if debugging needs it.
- Prune existing orphaned/historical `nflverse` and `nfelo` rows from `api_responses` and `api_bodies` and reclaim space
  (bringing the production database from ~90 MiB down to ~15–20 MiB).
- Preserve full append-only archiving for ESPN (`scoreboard`, `summary`, `roster`, `gamelog`) and Odds (`lines`,
  `props`, `theoddsapi`) so offline eval replay and link debugging keep working unchanged.

**Non-goals:**

- Removing the raw response archive for ESPN or Odds API responses.
- Moving the archive to a separate database or adding a background cron job.
- Changing how `nflverse_*` or `nfelo` parsed tables are structured or queried.

## Decision

Exclude bulk static/season datasets (`nflverse` and `nfelo`) from `api_bodies` archiving (keeping their parsed tables
updated in place) and run a one-time cleanup/vacuum of historical `nflverse` and `nfelo` archive rows.

```mermaid
flowchart LR
  subgraph Before
    B1["nflverse & nfelo syncs"] --> B2["Append new raw copy to api_bodies forever"]
    B3["ESPN & Odds syncs"] --> B2
    B2 --> B4["90+ MiB DB & 2.7 GiB GCS replica"]
  end
  subgraph After
    A1["nflverse & nfelo syncs"] --> A2["Upsert parsed tables only (skip raw archive)"]
    A3["ESPN & Odds syncs"] --> A4["Append to api_bodies (for evals & debugging)"]
    A4 --> A5["~15-20 MiB DB & ~5x smaller cold starts"]
  end
```

This beats raising Cloud Run memory or adding a scheduled pruning job, which both pay ongoing complexity and cost to
store raw CSV snapshots that nothing in the app or eval suite ever reads. It also beats replacing `nflverse` in-place in
`api_bodies`, because even a single latest copy of all five nflverse files still adds unnecessary megabytes to the
in-memory SQLite file when the parsed tables already hold every field the app uses.

- **Complexity: low** — Localized to archive transport wiring/filtering and a straightforward cleanup path, with clear
  existing invariants for which sources eval replay reads.
- **Risk: low** — No production feature reads `api_bodies`, offline evals are covered by automated replay tests, and
  excluding only `nflverse` and `nfelo` leaves ESPN and Odds archives untouched.

## Overview

```mermaid
flowchart LR
  subgraph Before
    B1["nflverse & nfelo syncs"] --> B2["archive.Transport"]
    B3["ESPN, Weather & Odds syncs"] --> B2
    B2 --> B4["api_responses + api_bodies (90.2 MiB DB)"]
  end
  subgraph After
    A1["nflverse & nfelo syncs"] --> A2["Plain HTTP client (newBulkClient)"]
    A2 --> A3["Parsed tables only"]
    A4["ESPN, Weather & Odds syncs"] --> A5["archive.Transport"]
    A5 --> A6["api_responses + api_bodies (historical bulk rows pruned)"]
  end
```

## Consequences

- **Easier:** Cloud Run cold starts and Litestream GCS restores handle a much smaller SQLite database instead of a 90+ MiB file, keeping memory usage well below the 512 MiB limit.
- **Easier:** Client construction in `cmd/server/main.go` controls which sources skip raw archiving without coupling `archive.Transport` to source-name policy.
- **Harder:** Raw HTTP responses for `nflverse` and `nfelo` can no longer be inspected or replayed from `api_bodies`; only the latest parsed rows remain in their tables.
- **Revisit:** Revisit if offline evals ever need raw `nflverse` or `nfelo` byte replay, or if another high-churn bulk source is added and needs `newBulkClient`.

## Alternatives considered

- **Raise Cloud Run memory or add a scheduled pruning job:** Rejected because both add ongoing cost and operational complexity to store raw CSV snapshots that nothing in production or evals reads.
- **Keep only the latest `nflverse` and `nfelo` body in `api_bodies`:** Rejected because even a single copy of the bulk CSVs adds unnecessary megabytes to the in-memory SQLite database when parsed tables already hold every field the app uses.
- **Filter excluded sources inside `archive.Transport`:** Rejected because `archive.Transport` would still buffer multi-MB responses before discarding them and would bake source-name policy into a generic HTTP wrapper.

## Implementation notes

- The design used `0024_prune_bulk_archive.sql` as an example name; the migration shipped as `0022_prune_bulk_archive.sql`.
- Deleting rows in SQLite marks pages as reusable without shrinking the file on disk, and `VACUUM` cannot run inside a migration transaction. The database file and Litestream snapshots will not shrink until a manual `VACUUM` runs.

## Follow-ups

- Run `sqlite3 "$DB_PATH" "VACUUM;"` in production during a quiet maintenance window after deploying migration `0022_prune_bulk_archive.sql` to reclaim freed SQLite pages and shrink Litestream snapshots.
