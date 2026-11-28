---
id: adr-pick-board-ux
title: Pick board grouping, pre-pick grades, and grade breakdown
type: decision
summary: Pick board grouping, pre-pick grades, and grade breakdown
area: board-and-research
touched:
    - internal/board
    - internal/web
    - web/static
    - web/tailwind
    - web/templates
related:
    - analyzer
    - routes
    - slips
---

# Pick board grouping, pre-pick grades, and grade breakdown

## Context

Picking bets inside an open slip has three friction points today:

1. **Fragmented single-tab browsing.** Opening a game on the slip pick board forces a single-category tab bar (`Lines`,
   `Passing`, `Rushing`, `Receiving`, `TDs`, `Kicking`). You can only view one category at a time. A quarterback's
   passing, rushing, and touchdown props live on three separate tabs, and there is no way to scan all of a game's bets
   grouped by category, by player, or by grade in one scroll.
2. **Grades stay hidden until after you pick.** Game cards on the slate already score cached rows to display grade-count
   pills (`3A`, `5B`), and the top of a game shows up to three A-grade highlights. Yet the individual bet buttons
   themselves show only odds, implied chance, and optional matchup badges—never the bet's A–F grade. You have to add a
   leg to the slip (or leave for Research) to see how it grades.
3. **Grade details blur hit chance and payout value.** When you tap a grade pill on a slip or tray, the breakdown shows
   the overall letter, 0–100 score, and four pillar bars, but omits the calibrated hit probability that the v2 engine
   already computes. A safe `-140` volume prop (~61% fair hit chance) and a `+220` touchdown swing (~35% fair hit chance
   with strong payout value) can both earn an `A` without clearly separating how likely they are to hit from how well
   they pay.

```mermaid
flowchart LR
  Slate["Game Slate (shows A/B/C counts)"] --> Game["Open Game Detail"]
  Game --> Tabs["Single-Category Tab Bar (1 tab at a time)"]
  Tabs --> Bet["Bet Tile (odds + badge, no grade)"]
  Bet -->|"tap to add"| Slip["Open Slip / Tray"]
  Slip --> Pill["A-F Grade Pill"]
  Pill --> Dropdown["Grade Dropdown (Overall + 4 Pillars, no hit chance vs. payout split)"]
```

## Goals and non-goals

**Goals:**

- Enable grouping by default inside an open game on the slip pick board, with three switchable modes: **By Category**
  (default: Game Lines, Passing, Rushing, Receiving, TDs, Kicking), **By Player** (all of a player's props across
  categories in one card, plus Game Lines), and **By Grade** (A through F, strongest plays first).
- Show each bet's A–F grade pill directly on the pick board before picking, scored from already-cached SQLite odds and
  signals with zero extra external API calls.
- Let a user tap any pre-pick grade pill to inspect its breakdown without accidentally adding, removing, or swapping the
  bet.
- Differentiate **Overall grade**, **Hit chance** (calibrated fair win probability), and **Payout value**
  (plain-language return/value read vs. fair odds) inside the grade breakdown dropdown across the pick board, pick tray,
  and slip views—free of raw "+EV %" lectures.
- Keep grouped views, pre-pick grade pills, and grade breakdowns fast to scan and easy to tap at 375px phone width.

**Non-goals:**

- Changing the v2 grading model, pillar weights, or A–F band cutoffs.
- Triggering paid odds or prop API calls to populate pre-pick grades.
- Replacing the standalone Research Explorer tab or blocking any low-grade pick.
- Persisting per-user grouping preferences in SQLite.

## Decision

Default the slip pick board's open-game view to grouped sections (`Category`, `Player`, or `Grade`), add clickable A–F
grade pills to every gradable bet before picking via the cached v2 scorer, and surface an
**Overall / Hit chance / Payout value** summary strip at the top of the shared grade breakdown dropdown.

### Before

```mermaid
flowchart TD
  Cached["Cached Odds + Signals"] --> Detail["Open Game Detail"]
  Detail --> SingleTab["One Category Tab at a Time"]
  SingleTab --> Tile["Bet Tile (no grade pill)"]
  Tile -->|"add leg"| Slip["Slip & Tray"]
  Slip --> Breakdown["Grade Dropdown: Overall + 4 Pillars"]
```

### After

```mermaid
flowchart TD
  Cached["Cached Odds + Signals"] --> Scorer["Existing v2 Scorer (reused, zero new API calls)"]
  Cached --> Detail["Open Game Detail"]
  Scorer --> Detail
  Detail --> Switcher["Group By Switcher: Category (default) | Player | Grade [new]"]
  Switcher --> Groups["Grouped Bet Sections [new]"]
  Groups --> Tile["Bet Tile: Pick Action + Separate Grade Pill [updated]"]
  Tile -->|"tap grade pill"| Breakdown["Shared Grade Dropdown: Overall | Hit Chance | Payout Value + 4 Pillars [updated]"]
  Tile -->|"tap bet"| Slip["Slip & Tray"]
  Slip --> Breakdown
```

**Why this shape:**

- **Grouping by default beats single-category tabs with a grade sort.** Adding a "Sort by Grade" chip inside the
  existing tabs would still force friends to tap across six tabs to compare a game's best plays or see a player's full
  prop menu. Defaulting to grouped sections with `Category` (default), `Player`, and `Grade` modes makes every browsing
  pattern one tap away while keeping search and sort intact.
- **One grade pill on the tile + three-way split in the dropdown beats triple badges on every bet.** Showing separate
  Overall, Hit Chance, and Payout ROI meters directly on every bet tile would crowd 375px phone screens. Keeping one
  compact A–F pill on the tile and placing the **Overall / Hit chance / Payout value** strip inside the grade dropdown
  keeps the board scannable and gives full clarity on tap.
- **Reusing v2's existing calibrated probability and Price & Vig outputs beats ad-hoc UI math.** The v2 engine already
  computes calibrated fair hit probabilities (for both individual legs and copula-adjusted slips) and evaluates offered
  odds against fair value in the Price & Vig pillar. Surfacing those exact values in plain language—such as fair hit
  chance alongside a plain payout-value read ("Good value · fair -120 vs. -110")—means zero model drift and no "+EV %"
  lecture copy, and keeps pre-pick grades identical to post-pick slip grades.

**Complexity (`medium`):** Coordinates across new grouping modes in the board layer, pre-pick leg scoring in the
pick-board handlers and partials without triggering API fetches, nested interactive grade toggles inside pickable bet
tiles, and surfacing calibrated hit chance and payout value in the shared grade breakdown across pick, tray, and slip
views.

**Risk (`medium`):** Changes the primary slip pick-board DOM, HTMX partial swaps for add/remove/switch/swap actions, and
shared grade dropdowns where phone-width layout issues or accidental form submissions from tapping a grade pill are only
partially caught by Go unit and HTML tests; scoring and grouping stay read-only over cached SQLite data with strong
deterministic test coverage.

## Overview

```mermaid
flowchart TD
  Cache["Cached SQLite Odds + Signals (zero new API calls)"] --> Scorer["board.ScoreRows (runs once per render, including at-limit swaps)"]
  Cache --> Detail["board.Detail (defaults to All categories)"]
  Scorer --> Detail
  Detail --> Switcher["Group by: Category (default) | Player | Grade + All/Category chips"]
  Switcher --> Tile["Pick Tile: Pick Button + Sibling Grade A-F Pill"]
  Tile -->|"tap Grade pill"| Popover["Full-Width grade-breakdown: Overall | Hit chance | Payout value + 4 Pillars"]
  Tile -->|"tap Bet (Category/Player)"| BoardSwap["HTMX from=board -> swaps .pick-line-group"]
  Tile -->|"tap Bet (Grade)"| DetailSwap["HTMX from=detail -> OOB swaps #pick-detail"]
```

## Consequences

- **Easier:** Browsing all bets in an open game in one scroll across `Category` (default with `All` + category chips), `Player` (all props for a player on one card plus `Game lines`), or `Grade` (`A`–`F` single-side buckets) without tab-hopping.
- **Easier:** Checking a bet's A–F grade before picking and distinguishing how often it hits (`Hit chance` from `CalibratedProb`) from how well it pays (`Payout value` from `Price & Vig`) across the pick board, tray, slip, and highlights without `+EV %` jargon.
- **Harder:** Maintaining pick-board HTMX mutations and 375px phone layout. `GroupByGrade` splits Over/Under pairs into single-side rows that require a `from=detail` OOB `#pick-detail` swap instead of `.pick-line-group` swaps, and pre-pick grade popovers must stay sibling `<details data-leg-grade data-help-popover>` elements anchored to `.pick-line-group` / `.pick-grade-row` so they neither trigger form submits nor squeeze into half-width columns.
- **Revisit:** If friends consistently switch to `Player` or `Grade` on every visit, consider persisting the `group` choice instead of keeping it URL-query-only.

## Alternatives considered

- **Single-category tabs with a Grade sort:** Rejected because friends would still have to tap across six separate tabs to compare a game's best plays or view one player's full prop menu.
- **Three badges (Overall, Hit chance, Payout value) on each bet tile:** Rejected because triple meters on every tile crowd 375px phone screens. Keeping one `Grade A–F` pill on the tile and placing the 3-column summary strip inside the breakdown popover keeps the board scannable.
- **Inline accordion inside the 2-column bet grid:** Rejected because expanding inline inside `grid-cols-2` squeezes text to ~155px at 375px width and misaligns paired Over/Under buttons. Anchoring the `<details>` panel across the full `.pick-line-group` width gives ~319px width without shifting buttons.
- **Ad-hoc UI math or raw `+EV %` copy for payout value:** Rejected in favor of reusing v2's `CalibratedProb` and `Price & Vig` thresholds (`Strong value`, `Fair value`, `Slight juice`, `Steep juice`) so pre-pick and post-pick grades never drift and tone stays friendly.
- **Removing category chips when grouping shipped:** Rejected because keeping an `All` + category filter strip alongside the `Group by` switcher preserves one-tap narrowing to a single market like `TDs` or `Kicking`.

## Implementation notes

- Adding `relative` directly to `class="pick-line-group"` in `web/templates/partials.html` would have broken existing exact-class HTML assertions, so `.pick-line-group { position: relative; }` was added in `web/tailwind/input.css` and compiled into `web/static/app.css`.
- `GroupByGrade` renders individual sides outside `.pick-line-group`, so `from=board` could not update the row in place; `from=detail` was added to `POST`, `PUT`, and `DELETE` leg handlers to OOB-swap `#pick-detail` while keeping `?group=grade` and active filters.
- `pickDetailData` previously skipped loading board signals when `remainingSlots == 0`; it now loads signals whenever an open slip has rows so swap options at the leg limit still carry pre-pick grades.
- Under `EngineV1` (which sets `CalibratedProb == 0` and omits pillars), `HitChance` and `PayoutLabel` stay empty and the breakdown falls back to the overall grade and reasons.
- Legacy `Lines` and `Players` fields on `board.GameDetail` remain populated alongside `Sections` for compatibility with existing callers and tests.

## Follow-ups

- Remove the legacy `GameDetail.Lines` and `GameDetail.Players` fields (and migrate any remaining test assertions to `DetailSection` / `PickDetailView.Sections`) once no callers depend on them.
- Propose persisting the user's preferred `group` mode (`category`, `player`, or `grade`) in a cookie or SQLite setting if usage shows people repeatedly switching away from the `Category` default.
