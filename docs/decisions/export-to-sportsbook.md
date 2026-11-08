---
id: adr-export-to-sportsbook
title: Export locked slips to DraftKings and FanDuel
type: decision
summary: Once a slip is fully locked in, the slip page shows an export panel with a button for each sportsbook.
last_verified: "2026-11-07"
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

## Consequences

A fully locked slip now opens DraftKings or FanDuel with the agreed picks already on the bet slip. With two or more
linked legs, one tap opens the whole book; otherwise each linked leg gets its own tap. Every leg the book cannot take is
named with a short reason, and the whole slip copies as text, so the export never blocks and never fails silently.

The app stores each book's own outcome link, selection ID, and event ID on the same `odds` row as the price. Refresh
writes all four together, so links are as fresh as prices, and the card spends no extra Odds API credits. The card reads
stored rows only and never fetches or writes.

"The same selection" now has one definition, `db.Odds.MatchesSelection`, shared by the Games tab, the slip's reference
prices, and export. The same `currentOdds` filter drops rows from an older sync everywhere, so a market a book stopped
quoting becomes `no line` instead of offering a dead link.

It gets harder when a book stops returning links: those legs fall back to the manual list. FanDuel's combined link also
depends on the `marketId` inside the stored per-leg URL, and DraftKings' depends on the book's stored event ID and only
covers legs from one event, so a slip that spans games keeps per-leg taps. A link-shape change affects both paths.
Existing odds rows have no backfill, so a slip locked right after deploy can show manual legs until the next lines sync.

The main thing to revisit is the Odds API terms for storing and showing bookmaker links. After that, whether other books
deserve a `Combiner`, and how props behave once they have odds rows.

## Alternatives considered

- **Copy the slip as text and let people search by hand.** Rejected: that is today's pain, and it does not remove the
  retyping or the wrong-line risk.
- **Integrate with each sportsbook directly.** Rejected by the product rules: the app tracks bets; people place them.
- **Build every book's link from `sid`s alone.** Rejected: it guesses URL formats and breaks silently. Only formats
  checked on a phone shipped, and a bad format can be removed from the registry.
- **Fetch fresh links when the export button is pressed.** Rejected: it spends scarce Odds API credits on every tap. The
  links come from the lazy sync the app already runs.
- **Store links in a separate `book_links` table.** Rejected: a link belongs to the same outcome row as its price, and
  one upsert already refreshes both.
- **Block export until every leg can be linked.** Rejected: the product is friendly and never blocking. The manual list
  shows what is left.
- **Add a market ID column for FanDuel.** Rejected: the market ID is already in the stored per-leg link. Parsing it
  keeps the schema unchanged, and a missing value falls back to per-leg taps rather than dropping a pick.
- **Use a one-selection "combined" URL.** Rejected: it is no better than the direct stored link, so a book needs two
  linked legs before the combined button appears.

## Implementation notes

- **The design said "the two odds calls", but only the lines client exists.** `includeLinks=true&includeSids=true`
  therefore ship on `Lines` only. The shared outcome struct carries `link` and `sid`, so the future props client needs
  only the same two query parameters.
- **The plan type is `BookPlan`, and the card is a sibling partial.** Go cannot have a type and function both named
  `Plan`, so the design's `Plan` type became `BookPlan`. The card is `slip-export`, placed after `#slip-panel`, not
  nested inside it. Lock and unlock responses carry `Hx-Trigger: slip-updated`, so the person who acts sees the card
  change even when the SSE stream is closed.
- **The card uses the newest sync, not the cache window.** It plans only from rows at each game's newest `fetched_at`,
  matching the Games tab and reference prices. Current rows past the cache window still show, because the book corrects
  the price when the leg opens. A line moved to a total or prop prints bare (`47.5`); only a spread keeps its sign.
- **Links are nullable columns written beside the price.** Migration `0004` adds `link` and `sid` to `odds`.
  `UpsertOdds` updates both in the same statement and clears them when a book drops them. `db.OptionalString` is the
  bridge from a blank API value to SQL NULL.
- **No live API test ships.** The probe and both format checks were run by hand on 2026-11-07 against one NFL game: the
  flags cost no extra credits, and DraftKings and FanDuel returned links on every game-line and probed prop outcome. A
  sportsbook can change its URL scheme without notice.
- **Known limits:** existing rows have NULL links until the next lines sync; props have no `odds` rows until
  `props-research` lands; only DraftKings and FanDuel have verified formats; and the Odds API terms for storing and
  showing bookmaker links are still unconfirmed.

## Follow-ups

- Confirm with a person that storing and showing DraftKings and FanDuel deep links fits the Odds API plan terms.
- Add one link-shape check to the `live-api-watch` change so a broken sportsbook URL scheme is caught before a user taps
  it.
- When `props-research` lands, send `includeLinks=true&includeSids=true` on the props call and add a saved-fixture test
  for a linked prop and a combined prop slip.
- Run a phone-width visual pass on the export card with every leg linked, some manual, and none linked, and record any
  layout fix in `docs/concepts/export.md`.
- Decide whether a book that returns no links should keep its block in the card or collapse it, once real usage shows
  how often that happens.
