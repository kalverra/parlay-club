---
slug: bet-browsing-and-research-ux
status: implementing
created: "2026-11-27"
accepted_by:
    - Adam Hamrick
accepted_at: "2026-11-28T01:35:32Z"
area: board-and-research
complexity: medium
risk: medium
---

# Bet browsing, slip O/U clarity, and research UX

## Context

Browsing props and reviewing slips on a phone takes too much scrolling and leaves common actions out of reach. Five
related friction points slow down picking and research today:

1. **Expanded groups flood the screen.** Every category, player, or grade group on the pick board opens expanded by
   default. Getting from Passing to Rushing or TDs means scrolling past dozens of player cards.
2. **Over/Under bets waste vertical space and blur together on slips.** Most props and totals are Over/Under pairs. On
   the pick board, each side renders a full multi-line button with repeated text, grade popovers, and status lines. On
   the slip hub, Over and Under are buried inside plain text like `Josh Allen Over 245.5 Pass Yds`, so scanning a slip's
   lean takes effort.
3. **Category sorting defaults to alphabetical player name.** Opening a category shows bench players and low-volume
   depth options ahead of stars and featured lines. Grade is available, but there is no popularity or visibility sort
   that puts headline players and active markets first.
4. **The bet research screen leads with a wall of book prices.** The first major card below the header is a multi-book
   "Book prices" table covering books the club does not use. Because the club places bets on FanDuel and DraftKings,
   visual summaries, grade breakdown, and player or matchup context get pushed below the fold.
5. **Research and Add actions are inconsistent across bet surfaces.** Slip legs link to general player or game sheets
   instead of the specific bet research view. Pick-board tiles and A-grade highlights let you add a bet, but have no
   direct link to research that exact bet. Research explorer cards and related bets link to research, but offer no way
   to add the bet from the card—and when browsing outside a specific slip, there is no unified picker to choose which
   eligible slip should receive the leg.

```mermaid
flowchart TD
  PB["Pick Board<br/>all groups expanded<br/>alphabetical default<br/>tall O/U buttons"]
  SLIP["Slip Hub<br/>plain-text Over/Under legs"]
  RES_SHEET["Player / Game Sheet"]
  EXPLORE["Research Explorer<br/>and Related Bets"]
  BET["Bet Research Screen<br/>1. Multi-book price list<br/>2. Grade factors<br/>3. Player / Team visuals"]
  PB -->|"Add only (no bet research link)"| SLIP
  SLIP -->|"Links to player/game sheet"| RES_SHEET
  EXPLORE -->|"Research only (no card Add)"| BET
```

## Goals and non-goals

### Goals

- **Collapse pick-board groups by default.** Tapping a group header expands or collapses it so a phone viewport shows
  every group at a glance and stays fast to navigate.
- **Make Over/Under bets compact to browse and unmistakable on slips.** Show the shared market and line once on the pick
  board with tight side-by-side Over and Under tap targets, and give Over/Under picks clear visual badges on the slip
  hub and pick tray.
- **Sort categories by popularity/visibility first, then grade.** Default category ordering to a deterministic
  popularity/visibility rank built from cached stats, market breadth, and role signals, with Grade as the second sort
  option.
- **Lead bet research with digestible visuals and FD/DK context.** Reorder the bet research screen from highest-signal
  graphics and summaries at the top (grade, hit-rate visuals, matchup context, and a compact FD/DK comparison) down to
  detailed factor lists and collapsed or lower-priority multi-book prices at the bottom.
- **Put Research and Add on every bet tile and mention.** Every bet card, option, highlight, slip leg, and related bet
  offers a link to that exact bet's research view and an Add action—adding straight to the active slip when picking for
  one, or prompting to choose among eligible open slips when browsing outside a slip context.

### Non-goals

- Fetching new external APIs or spending additional Odds API or Parlay API credits. All popularity sorting, visuals, and
  summaries use cached SQLite data only.
- Changing analyzer grading formulas, warning rules, or dual-book (FD + DK) pick eligibility.
- Changing slip locking, voting, live progress, auto-grading, or sportsbook export rules.
- Supporting sportsbooks beyond FanDuel and DraftKings for primary price highlights.

## Decision

Unify bet presentation and navigation around five coordinated UX changes: collapsible pick-board groups, compact O/U
controls with clear slip badges, popularity-first category sorting, a visual-first bet research hierarchy, and a shared
Research + Add action pair with an eligible-slip chooser.

### Before

```mermaid
flowchart LR
  subgraph Browse["Pick Board & Highlights"]
    G1["Expanded Groups<br/>(Player A–Z default)"] --> T1["Tall O/U Buttons<br/>Add only"]
  end
  subgraph Research["Bet Research Screen"]
    R1["Book Prices List (all books)"] --> R2["Grade & Factors"] --> R3["Player / Team Visuals"]
  end
  subgraph Slip["Slip Hub"]
    S1["Plain-text O/U leg"] -->|"Player/Game sheet"| R3
  end
  T1 --> S1
```

### After

```mermaid
flowchart LR
  subgraph Browse["Pick Board & Highlights"]
    G2["Collapsed Groups<br/>(tap to expand)<br/>Sort: Popular -> Grade"] --> T2["Compact O/U Row<br/>Research + Add"]
  end
  subgraph Research["Bet Research Screen"]
    V1["Visual Summary &<br/>FD/DK Snapshot"]
    V2["Player / Matchup Charts"]
    V3["Grade Factors &<br/>Other Books (below fold)"]
    V1 --> V2 --> V3
  end
  subgraph Slip["Slip Hub"]
    S2["Clear Over/Under Badge<br/>Research bet + Add"]
  end
  T2 -->|"Research bet"| V1
  S2 -->|"Research bet"| V1
  T2 -->|"Active slip"| S2
  V1 -->|"No active slip -><br/>Eligible slip picker"| S2
```

### Why this approach

- **Collapsed groups by default** beat infinite scroll or pagination because users still see every category and count in
  one viewport without extra server round trips, and tapping opens only what they care about.
- **Shared compact O/U rows and explicit O/U badges** beat shrinking fonts on the existing two-card layout because
  hoisting the shared market/line out of both buttons cuts duplicate text in half while keeping phone tap targets
  comfortable.
- **Deterministic popularity/visibility default sort (with Grade second)** beats alphabetical player sorting because
  headline starters and high-volume props surface first without requiring manual filtering, while Grade stays one tap
  away. Computing popularity from cached volume, role, and market signals keeps the app deterministic and costs zero API
  credits.
- **Visual-first bet research with FD/DK upfront** beats removing multi-book prices outright because users get the
  headline visual read and their actual books immediately, while sharp-book or full-market detail stays available lower
  down for anyone who scrolls.
- **Universal Research + Add with an eligible-slip chooser** beats silently guessing a target slip or hiding Add on
  research surfaces because every bet mention becomes actionable while respecting each slip's slate scope, open status,
  and remaining leg slots.

### Complexity and risk

- **Complexity (`medium`):** Coordinating shared bet-tile controls, HTMX swap targets, collapsible group state,
  popularity ranking, and eligible-slip selection spans the board, slips, and research layers and requires keeping
  selection identity and phone-width ergonomics consistent across every bet surface.
- **Risk (`medium`):** Changes touch core picking, slip reading, and research screens where visual hierarchy, mobile tap
  targets, and HTMX interactions are only partially exercised by server-rendered HTML tests, though no persistent schema
  or external API contracts change.
