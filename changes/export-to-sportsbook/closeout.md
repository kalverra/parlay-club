# Close-out

## Consequences

A fully locked slip now opens DraftKings or FanDuel with the agreed picks already on the bet slip. With two or more
linked legs, one tap opens the whole book; otherwise each linked leg gets its own tap. Every leg the book cannot take is
named with a short reason, and the whole slip copies as text, so the export never blocks and never fails silently.

The app stores each book's own outcome link and selection ID on the same `odds` row as the price. Refresh writes all
three together, so links are as fresh as prices, and the card spends no extra Odds API credits. The card reads stored
rows only and never fetches or writes.

"The same selection" now has one definition, `db.Odds.MatchesSelection`, shared by the Games tab, the slip's reference
prices, and export. The same `currentOdds` filter drops rows from an older sync everywhere, so a market a book stopped
quoting becomes `no line` instead of offering a dead link.

It gets harder when a book stops returning links: those legs fall back to the manual list. FanDuel's combined link also
depends on the `marketId` inside the stored per-leg URL, so a link-shape change affects both paths. Existing odds rows
have no backfill, so a slip locked right after deploy can show manual legs until the next lines sync.

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
