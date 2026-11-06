---
slug: export-to-sportsbook
status: proposed
created: 2026-11-05
effort: medium
depends_on: [slips-live, slate-and-lines]
---

# Export locked slips to DraftKings and FanDuel

## Context

When every member has locked in, the group has a finished parlay, and then someone has to build it by hand in a
sportsbook app. They read each leg off the slip, search for the game, find the market, and tap the right side, for every
member's legs. On a phone, with kickoff close, this is slow and easy to get wrong. A wrong line or a missed leg means
the bet placed is not the bet the group agreed on.

The app already knows exactly what each leg is. It should hand that to the sportsbook, not make a person retype it.

## Goals and non-goals

**Goals:**

- When all members are locked in, show one clear action per sportsbook: open DraftKings, open FanDuel.
- Tapping it opens the sportsbook app (or site) with the legs already on the bet slip, ready to place.
- Legs that cannot be linked are named, so the person adds just those by hand. Nothing fails silently.
- Costs no extra Odds API credits beyond what lazy sync and the cache already fetch.
- Works at phone width, where the tap lands in the sportsbook app.

**Non-goals:**

- Placing bets, logging in to a sportsbook, or handling money. The app hands off; a person places the bet.
- Other sportsbooks. Two to start.
- Linking legs the app has no line for. Hand-typed legs fall back to plain text.
- Blocking anything. If a link is stale or the odds moved, the sportsbook shows its own price and the person decides.

## Decision

Once a slip is fully locked in, the slip page shows an export panel with a button for each sportsbook. Each button opens
a deeplink that pre-fills the bet slip with the slip's legs. The links come from the Odds API. Its `includeLinks` option
returns bookmaker links to events, markets, and betslips "if available", and `includeSids` returns the sportsbook's own
ids, which the docs say help build links for mobile apps. We ask for them on the fetches we already make, so the cache
holds them. A leg gets a link only when it matches a game and market the app already fetched. Any leg without a match is
listed under the button with a short note, plus a copy-as-text option for the whole slip, so the export is never
all-or-nothing.

The alternative is to copy the legs as text and let people search by hand. That is where we are today, and it does not
fix the pain. Another is a direct sportsbook integration, which the product rules rule out. A deeplink is the lightest
thing that removes the retyping and keeps the person in control of placing the bet.

Effort is medium. The UI is small, but the design must confirm how each sportsbook builds its bet-slip links, where the
selection identifiers come from without spending scarce credits, and how a link behaves when a line has moved or a game
has started.

## Open risks

The Odds API docs name both options but do not describe the response fields, say whether links can be missing, or say
whether they cost extra credits. They show no DraftKings or FanDuel link examples, and they say nothing about player
props. The first design task is a one-call live test against a real game to see the actual fields, on both books, for a
game line and a prop. If betslip links turn out to be missing for props or one book, the export covers only what links
and lists the rest by name. If the options cost extra credits, the design must budget for it before any code.
