---
id: adr-pick-board-performance
title: Speed up pick board and page loads
type: decision
summary: Pre-index roster lookups per board build, move pick-time odds refreshes off the request path via SSE, and enable concurrent SQLite WAL readers so page loads stay fast.
area: board-and-research
---

# Speed up pick board and page loads

## Context

During Sunday launch, clicking links to explore parlays on the pick board took 5–22 seconds per request, stacked into
multi-minute stalls, and caused 13 Cloud Run `429` aborts ("waiting too long for an available instance").

Production logs and a restored copy of the prod database (`15,543` odds rows, `2,082` roster rows) show three causes
piling up on the single `1` vCPU Cloud Run instance:

1. **O(N × M) normalization per request:** Building the pick board groups `15,543` odds rows into ~2,000+ selections and
   linearly scans all `2,082` roster players for every selection, normalizing player names ~8 million times on every
   page or HTMX partial load. Even `pick/slate` (zero external network calls) took `5.1s` solo and `11.1s–11.5s`
   alongside `pick/detail`. Four users loading `/pick` together took `20.9s–22.7s` each.
2. **Synchronous external syncs on clicks:** Opening the board, clicking a game, or switching tabs runs stale-odds
   refreshes inline on the HTTP request (blocking up to `10s` per timeout).
3. **Single-connection SQLite contention:** All reads and background sync writes share one pooled SQLite connection, so
   background refreshes stall page reads.

```mermaid
flowchart LR
  Click["User click / HTMX swap"] --> Sync["Inline stale-odds sync (up to 10s)"]
  Sync --> DB["Single SQLite connection"]
  DB --> Scan["Board build: ~2000 groups x 2082 roster scans"]
  Scan --> CPU["1 vCPU pegged (5s-22s)"]
  CPU --> Abort["Cloud Run queues -> 429 aborts"]
```

## Goals and non-goals

**Goals:**

- Make pick-board page loads, game switches, tab changes, and search respond in tens of milliseconds under multi-user
  Sunday traffic.
- Never block user navigation on external API calls or background sync writes.
- Eliminate Cloud Run `429` queue aborts caused by CPU saturation on the single instance.

**Non-goals:**

- Scaling out to multiple Cloud Run instances (single-instance SQLite and SSE hub remain the architecture).
- Introducing external caches such as Redis or Memcached.
- Changing API credit floors or Sunday pre-fill rules.

## Decision

Pre-index roster lookups once per board build instead of scanning and normalizing the full roster for every selection,
move pick-time odds refreshes off the synchronous request path so clicks always render from stored data and push live
updates over SSE, and allow concurrent SQLite WAL readers so reads do not stall behind background sync writes.

```mermaid
flowchart LR
  subgraph Before
    BClick["HTTP request"] --> BSync["Blocking API sync"]
    BSync --> BPool["1 SQLite conn"]
    BPool --> BLoop["O(N x M) roster scan"]
  end

  subgraph After
    AClick["HTTP request"] --> ARead["Concurrent WAL read"]
    ARead --> AIndex["O(1) pre-indexed roster map"]
    AClick -. "if stale" .-> ABg["Async refresh -> SSE update"]
  end
```

Raising Cloud Run CPU/instance limits or adding a full-page cache alone would either break the single-instance SQLite
design or hide an O(N × M) hotspot that worsens as the week's odds grow. Fixing the algorithmic scan and separating
reads from syncs solves the root cause within the free-tier footprint.

The `effort` rating is `medium` because the work spans board indexing, request-path/async refresh coordination, SSE
triggers, and SQLite read/write connection behavior across multiple packages while preserving credit and single-instance
invariants.

## Consequences

**Easier:**

- Pick-board loads, game switches, tab changes, and search queries run against an O(1) pre-normalized `rosterIndex` and
  scoped queries (`pickSlate` skips `dockFrom` and `Detail`; `pickDetail` loads only the open game's odds and team
  rosters).
- Page reads run concurrently on a `query_only=ON` SQLite WAL read pool without blocking behind background `UpsertOdds`
  writes, and automatic pick-time refreshes run asynchronously and push updates over SSE.
- `mise run bench` and non-prod `Server-Timing` / `/debug/pprof/` tooling make regressions in SQLite, board building, or
  template rendering straightforward to measure against prod-scale data.

**Harder:**

- Pick-board views can briefly show cached odds while a background refresh completes and triggers an SSE `board-updated`
  swap, rather than waiting inline for fresh upstream data.
- File-backed `db.Store` instances open a second `*sql.DB` read pool, so callers must call `Store.Close()` to avoid
  leaking connections (tests using `:memory:` share the single writer handle).

**What we would revisit:**

- If roster or odds volume grows an order of magnitude beyond current NFL week sizes, revisit per-request `board.Build`
  versus sharing the user-independent portion of the board across users.

## Alternatives considered

- **Raising Cloud Run CPU/instance limits:** Rejected because scaling instances breaks the single-instance SQLite and
  in-memory SSE hub architecture, and more CPU only hides an O(N × M) hotspot that worsens as odds rows grow.
- **Full-page or full-board in-memory caching:** Rejected because `homeBook` and `mine` legs vary per user and SSE
  updates would still invalidate cached boards; pre-indexing the roster makes fresh per-request builds fast with zero
  cache staleness bugs.
- **External caches (Redis or Memcached):** Rejected to stay within the single-instance free-tier footprint.

## Implementation notes

- `KickPickLines` and `KickPickGame` take `ctx context.Context` (matching the rest of `web.Refresher`) and detach
  request cancellation via `context.WithoutCancel` while still respecting service shutdown via `s.stopCtx`.
- `Store.NewStore` inspects `PRAGMA database_list` to decide whether to open a separate read pool; `:memory:` databases
  fall back to the single writer handle because SQLite in-memory databases are per-connection unless named with shared
  cache.

## Follow-ups

None.
