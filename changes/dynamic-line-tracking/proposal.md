---
slug: dynamic-line-tracking
status: designed
created: "2026-11-16"
accepted_by:
    - Adam Hamrick
accepted_at: "2026-11-16T19:06:58Z"
depends_on:
    - canonical-player-identity
effort: hard
---

# Track prop selections dynamically when lines move

## Context

Sportsbook prop lines routinely shift by half a yard or vary slightly across books. Today, the app treats the exact line
number and raw player string as part of a pick's identity, so small line moves break both the pick board and sportsbook
export links:

- **Cross-book line differences break exports:** On prod leg 20 (`Bryce Young Over 248.5 Pass Yds`), the picker's home
  book (FanDuel) had `248.5` while DraftKings had `254.5`, BetMGM `245.5`, and BetRivers `247.5`. Export rejected
  DraftKings with `"line moved"` instead of linking to DraftKings' current line for Bryce Young Over Pass Yds.
- **Name mismatches break exports:** On prod leg 19 (`Ollie Gordon Over 17.5 Rush Att`), the saved leg recorded
  `"Ollie Gordon"` while DraftKings listed `"Ollie Gordon II"` at `17.5`, so export failed with `"no line"`.
- **Moved lines detach picks from the board:** Once a line moves after a pick is saved, exact-line matching on the board
  stops recognizing the leg as picked, breaking one-tap removal and swaps.

```mermaid
flowchart LR
  Leg["Saved leg: (raw name, market, side, exact line)"] --> Board["Board: exact line check fails -> pick looks unselected"]
  Leg --> Export["Export: exact name + line check fails -> manual fallback"]
```

## Goals and non-goals

**Goals:**

- Make a prop pick's primary identity `(game, canonical player, market, side)` — `"Player X Over/Under Stat Y"` — and
  treat the line number and odds as live, updatable values.
- Keep board selection state (picked badge, remove, swap) intact when a line moves.
- Allow sportsbook export to build working deep links to a book's current standard line for that selection while showing
  clearly when a book's current line differs from the slip's line.
- Refresh or re-sync open picks cleanly against current lines before lock-in without breaking votes unnecessarily.

**Non-goals:**

- Mutating settled or locked historical slips after kickoff or grading.
- Supporting alternate/milestone line picking (e.g., matching a 100+ yard alt line to a standard O/U pick).

## Decision

Separate selection identity (`game`, `canonical player`, `market`, `side`) from the quoted `line` and `odds`, so board
matching, leg updates, and sportsbook export look up the target selection dynamically and resolve each book's current
line, price, and deep-link selection ID.

```mermaid
flowchart LR
  subgraph Before
    BLeg["Leg keyed by raw name + exact line"] --> BMatch["Exact match only"]
    BMatch --> BFail["Breaks on 0.5 move or book variance"]
  end

  subgraph After
    ALeg["Leg keyed by canonical player + market + side"] --> ALookup["Dynamic lookup per book / board"]
    ALookup --> ABoard["Board stays synced + swappable"]
    ALookup --> AExport["Export links live book line + flags drift"]
  end
```

Simply ignoring line differences silently would mislead friends about what number they bet or graded. Keeping the picked
line for display/grading while matching and deep-linking by canonical selection (with a clear line-drift callout on
export) lets links and leg controls keep working without hiding line movement.

The `effort` rating is `hard` because selection identity is woven through leg creation, board matching, vote-clearing
rules, sportsbook link export, and auto-grading, requiring careful invariants around pre-kickoff vs locked/graded legs.
