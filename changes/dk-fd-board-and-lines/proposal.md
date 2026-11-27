---
slug: dk-fd-board-and-lines
status: implementing
created: "2026-11-27"
accepted_by:
    - Adam Hamrick
accepted_at: "2026-11-27T18:48:56Z"
area: board-and-research
complexity: medium
risk: medium
---

# DK and FD dual-book board and split-line display

## Context

Club members only place bets on DraftKings and FanDuel, but the pick board and research explorer still treat every book
in the odds feed equally. Any selection quoted by BetMGM, BetRivers, Bovada, or a DFS feed appears as pickable, even
when DraftKings or FanDuel does not offer it. Members pick those props and only discover the gap after locking the slip,
when export drops them to manual entry.

Line display also breaks when books disagree. Without a home book set (or when the home book is missing), the board
takes the arithmetic median of stored lines. When DraftKings posts `2.5` and FanDuel posts `3.0`, the app displays
`2.75 O/U`—a line that exists on neither book. Even in Week 12 production slips, DraftKings and FanDuel frequently
differed by a point or two on the same player prop (for example, DeVonta Smith at `68.5` on DK vs `66.5` on FD, or
Patrick Mahomes at `229.5` on DK vs `228.5` on FD). Meanwhile, setting a home book in Profile still asks users to type a
raw bookmaker key into a free-text input.

```mermaid
flowchart LR
  Odds["Stored odds (all books)"] --> Board["Pick board & explorer (any book)"]
  Board -->|"median across books"| Avg["Synthetic average line (e.g. 2.75 O/U)"]
  Board -->|"single-book prop"| Dead["Un-exportable pick on DK or FD"]
  Profile["Profile free-text book key"] --> Board
```

## Goals and non-goals

**Goals:**

- Show only selections available on **both** DraftKings and FanDuel on all pickable views (pick board, game detail,
  research explorer).
- Carry both DraftKings and FanDuel lines and prices on every pickable selection, and never average differing lines into
  a synthetic number.
- Display differing lines clearly at phone width (for example, `DK 2.5 (-115) · FD 3.0 (-110)`) when DraftKings and
  FanDuel disagree, while keeping identical lines compact.
- Optimize home-book and slip UX around the two-book assumption: replace the free-text profile input with a DraftKings /
  FanDuel picker, highlight the viewer's preferred book first, and show clear DK/FD context on picks.
- Keep all stored books intact in the database so sharp-book grading and historical evals continue to work unchanged.

**Non-goals:**

- Removing non-DK/FD books from ingestion or storage.
- Adding retail support or export for a third sportsbook.
- Blocking manual `OTHER` legs or changing how past slips are graded.

## Decision

Keep all books in storage for sharp-book grading, but filter user-facing pickable boards to the DraftKings ∩ FanDuel
intersection, attach both books' quotes to each selection, and render split lines explicitly whenever the two books
disagree.

```mermaid
flowchart LR
  subgraph Before
    B1["All stored books"] --> B2["Any-book pick board"]
    B2 --> B3["Median line averaging (2.75 O/U)"]
    B2 --> B4["Free-text home book"]
  end
  subgraph After
    A1["All stored books"] -->|"sharp books kept"| A2["Analyzer & eval grading"]
    A1 -->|"DK ∩ FD filter"| A3["Dual-book board & explorer"]
    A3 --> A4["Explicit DK vs FD line display"]
    A3 --> A5["DK / FD profile picker & home-first UX"]
  end
```

This beats filtering out non-DK/FD books at ingest time, which would break sharp-book grading (Pinnacle, Circa, LowVig,
BetOnline) and market-consensus benchmarks. It also beats picking an arbitrary single book's line when DK and FD differ,
which hides real point differences that matter to whoever places the bet on the other book.

- **Complexity: medium** — Spans board selection grouping, dual-book quote modeling, pick/explorer/slip/profile UI, and
  the invariant that sharp books remain untouched for grading and evals.
- **Risk: medium** — Tightening selection filtering and changing board/template line rendering affects every pick screen
  on mobile, where unit tests cover the pure grouping logic but phone layout and edge-case player-name matching need
  careful review.
