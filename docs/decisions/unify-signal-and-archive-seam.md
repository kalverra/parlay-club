---
id: adr-unify-signal-and-archive-seam
title: Unify live and historical signal loading
type: decision
summary: Share pure injury-merging and signal helpers between live loading and eval replay, use canonical archive keys, and scope replay queries to games on settled slips.
touched:
    - internal/analyzer
    - internal/db
    - internal/eval
    - internal/refresh
area: evals
---

# Unify live and historical signal loading

## Context

Live grading loads player usage, team defense profiles, and merged ESPN injuries through a deepened signal loader.
Historical eval replay re-implements a parallel version of that pipeline against pre-kickoff archived payloads,
duplicating injury merging, player-ID extraction, and postseason season-type rules.

The replay module also hardcodes raw archive key prefixes as string literals instead of sharing the canonical key
builders used during background refresh. If a refresh source key changes, historical replay silently loses archive
coverage without failing a test.

Finally, replay does unnecessary work on every cache miss or CLI run: it scans and parses archived odds payloads for
every game on a week's slate (even games with zero slips) just to compute a coverage boolean, fetches slips and legs
with per-slip queries, and loads every historical archive row before kickoff only to pick the newest one in memory.

```mermaid
flowchart TD
  Refresh["Background Refresh"] -->|"canonical source keys"| Archive["Raw Response Archive"]
  Live["Live Signal Loader"] --> LiveHelpers["Injury Merge + Player IDs + Season Types"]
  Replay["Eval Replayer"] -->|"hardcoded key strings"| Archive
  Replay --> DupHelpers["Duplicated Injury Merge + Player IDs + Season Types"]
  Replay -. "parses all 16 games/wk for a bool" .-> Archive
```

## Goals and non-goals

**Goals:**

- Share canonical archive source key builders between background refresh and historical replay so key formats have one
  source of truth.
- Share pure signal-assembly helpers (injury merging, player-ID deduplication, and postseason season-type handling)
  between live grading and historical replay while keeping the analyzer free of archive decompression and raw HTTP
  parsing.
- Add a direct latest-before-timestamp archive lookup at the storage seam and batch slip/leg reads during replay.
- Scope historical archive reads to games actually referenced by settled slips, and stop parsing full odds payloads just
  to report coverage presence.

**Non-goals:**

- Moving raw archive decompression or pre-kickoff replay into the live analyzer module.
- Falling back to live overwritten tables when pre-kickoff archive rows are missing (leak-free rules stay strict).
- Changing any analyzer scoring rules, weights, or calibration formulas.

## Decision

### Before

```mermaid
flowchart LR
  Live["Live Signal Loader"] --> L1["Live Helpers"]
  Replay["Eval Replayer"] --> R1["Copied Helpers + Hardcoded Keys"]
  Replay --> R2["Full-Slate Archive Scans + N+1 Queries"]
```

### After

```mermaid
flowchart LR
  Live["Live Signal Loader"] --> Shared["Shared Pure Signal Helpers"]
  Replay["Eval Replayer (Slip-Scoped)"] --> Shared
  Refresh["Background Refresh"] --> Keys["Canonical Archive Keys + Latest-Before Query [new]"]
  Replay --> Keys
```

Place canonical archive key builders and a latest-before-kickoff lookup at the storage seam, share pure injury-merging
and player/season assembly helpers across live and historical callers, and scope replay archive reads to games touched
by settled slips.

This beats pulling historical archive replay into the live analyzer module because live grading stays decoupled from raw
archive formats while two real adapters (live tables and pre-kickoff archive) share one signal-assembly seam with zero
drift.

`complexity: medium` fits because the change spans the storage archive seam, background refresh key naming, live signal
helpers, and leak-free historical replay while preserving strict pre-kickoff cutoff invariants. `risk: low` fits because
all behavior is deterministic and covered by in-memory SQLite replay tests that verify pre- and post-kickoff leakage
guards and identical graded outputs.

## Overview

```mermaid
flowchart LR
  Live["Live Signal Loader"] --> Shared["Shared Pure Signal Helpers"]
  Replay["Eval Replayer (Slip-Scoped)"] --> Shared
  Replay --> Batch["Batched Week Slips and Legs"]
  Refresh["Background Refresh"] --> Archive["Canonical Archive Keys and Latest-Before Query"]
  Replay -->|"metadata-only odds + game-scoped"| Archive
```

## Consequences

- **Easier:** Changing or adding archive source keys without silent replay drift, since refresh and eval share canonical
  `Archive*Key` builders in `internal/db`.
- **Easier:** Keeping live and historical injury merging, player-ID deduplication, and postseason season-type rules
  aligned through shared pure helpers in `internal/analyzer`.
- **Easier:** Replaying multi-game slates with less I/O and CPU, because slip/leg reads are batched, archive reads skip
  unreferenced games, and odds coverage uses metadata-only checks cached per kickoff.
- **Harder:** Callers of `SlipsWithLegsForWeek` must pass a valid `db.SlipStatus` (or empty string) rather than
  arbitrary status strings.
- **Revisit:** `LatestAPIResponseBefore` queries `fetched_at <= before` at 1-second RFC3339 resolution, so strict
  `< kickoff` callers still adjust the cutoff timestamp and re-check `FetchedAt.Before(kickoff)` in Go.

## Alternatives considered

- **Moving pre-kickoff archive replay into `internal/analyzer`:** Rejected because it would couple the live analyzer to
  archive decompression and raw ESPN/odds HTTP JSON parsers. Sharing pure assembly helpers in `internal/analyzer` keeps
  the boundary clean.
- **Extracting a new `internal/archivekeys` package:** Rejected as a shallow package. `internal/db` already owns
  `api_responses` reads and writes and is imported by both `internal/refresh` and `internal/eval` without an import
  cycle.

## Implementation notes

- `NFLverseSeasonTypes` returns `[]int`, not `[]string` as the design originally wrote.
- `SlipsWithLegsForWeek` accepts `db.SlipStatus` instead of `string` and validates non-empty values with
  `SlipStatus.Valid()`.
- `api_responses` enforces `CHECK (status BETWEEN 200 AND 299)` at the schema level, so non-2xx rows cannot be inserted
  via `SaveAPIResponse`; `LatestAPIResponseBefore` still includes `status >= 200 AND status < 300` in SQL.
- Because `LatestAPIResponseBefore` uses `fetched_at <= ?` at 1-second RFC3339 resolution, `eval` adjusts the cutoff
  before querying and verifies `resp.FetchedAt.Before(kickoff)` to preserve strict pre-kickoff leakage guards.

## Follow-ups

None.
