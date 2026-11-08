# Phase 1: Probe and store links

## Done

- The live probe ran on 2026-11-07 against Cincinnati Bengals at Atlanta Falcons (event
  `dc56d980f527ffeef89940133b211b13`, kickoff 2026-11-08T14:30:00Z), once with and once without
  `includeLinks=true&includeSids=true`:
  - `h2h,spreads,totals` cost 3 credits either way (`x-requests-last: 3`); `player_pass_yds` cost 1 either way.
  - DraftKings and FanDuel returned `link` and `sid` on every game-line outcome (6 of 6 each) and every probed prop
    outcome (4 of 4 each). Other books vary and often omit them.
  - The fields sit on each outcome; the market object keeps both NULL. Without the flags the fields are absent.
- Saved the two responses to `internal/ingest/odds/testdata/odds_game_links.json` and `odds_prop_links.json`, with the
  query and credit headers recorded in `testdata/README.md`.
- `plan/data-sources.md` gained a "Deep-link flags" section with the cost, per-book coverage, and both link shapes;
  `docs/reference/data-sources.md` gained the flags and `Link`/`Sid` rows in the `Line` table.
- `0004_odds_links.sql` adds nullable `link` and `sid` TEXT columns to `odds`. `db.Odds` carries `Link` and `Sid` as
  `*string`, and `oddsColumns`, `scanOdds`, and `UpsertOdds` include them.
- `UpsertOdds` writes both columns in the same statement as `price`, updates them under the same `WHERE`, and treats a
  vanished link as a change, so a refresh sets them to NULL when the book drops one. A temp-DB test covers insert, no-op
  re-upsert, price+link change, other-book isolation, and link removal.
- The odds client sends both flags on `Lines` and copies each outcome's `link` and `sid` onto its `Line`; `refresh` maps
  a blank value to `nil` so a missing link stores NULL.
- Docs: `docs/reference/schema.md` records the columns, the version-4 migration, and the refresh note; `docs/llms.txt`
  is regenerated.

## Deviations

- **The design says "the two odds calls", but only one exists.** There is no props client yet; `props-research` is still
  proposed. So `Lines` is the only call sending the flags today.
- **The shared outcome struct still carries the prop fields.** `outcomeResponse` gained `Link` and `Sid`, and a test
  decodes the saved prop fixture through it. The future props client inherits both fields; it only needs to add the same
  two query parameters.
- **`odds.Line` uses `string`, `db.Odds` uses `*string`.** The client treats an absent field as empty; the store keeps
  NULL explicit. `refresh.optionalString` is the bridge. This is the smallest seam and matches the existing nullable
  columns.
- **The probe measured one event and one prop market.** Zero extra cost held for both; the design's "if the flags cost
  credits, stop" branch was not needed.

## Open issues

- **No multi-selection format is known.** Every Probe link holds exactly one selection: DraftKings
  `https://sportsbook.draftkings.com/?outcomes={sid}`, FanDuel
  `https://sportsbook.fanduel.com/addToBetslip?marketId={marketId}&selectionId={sid}`. Phase 3 still needs the phone
  check before any `Combiner` is written.
- **DraftKings prop sids contain `#`.** In the link it arrives URL-encoded (`%23`). Any code that composes a URL from
  stored sids must encode them, not concatenate raw.
- **Market- and book-level links exist and are not stored.** For books that ship no outcome link, the book-level link
  points at an event page, not a betslip. The parser deliberately reads only the outcome value; do not fall back to the
  book-level one when a leg is planned.
- **Existing odds rows keep NULL links until the next sync.** There is no backfill; a locked slip created right after
  deploy can show manual legs until `odds:lines` refreshes.
- **Props have no `odds` rows at all yet.** Until `props-research` lands, a prop leg has no row for either book, so
  Phase 2 will exercise the "no line"/"no link" manual path for props rather than a linked prop.
- **The Odds API terms are still unconfirmed.** A person should confirm that storing and showing bookmaker links fits
  the plan before shipping.

## Next-phase notes

- `export.Plan` can read `db.Odds.Link` directly. Treat `nil` as "no link" and route the leg to `Manual`. `Sid` is only
  needed if Phase 3 adds a `Combiner`; the per-leg path taps `Link`.
- The odds rows written today are game lines only. A prop leg with no row must render the plain manual reason, not an
  error, and the same holds for a game-line leg whose book omitted the link.
- `plan/data-sources.md` and `docs/reference/data-sources.md` now agree on the flags, cost, and link shapes. Keep
  `docs/reference/data-sources.md` as the living doc and update `last_verified` on any re-probe.
- Phase 2 should write `docs/concepts/export.md` and add `GET /slips/{id}/export` to `plan/architecture.md` as the
  design's TODO says.
- Later migrations must bump `latestSchemaVersion` in `internal/db/migrate_test.go` and the version assertion in
  `migrate_internal_test.go`; this phase moved both to 4.

## Files touched

- `docs/llms.txt`
- `docs/reference/data-sources.md`
- `docs/reference/schema.md`
- `internal/db/migrate_internal_test.go`
- `internal/db/migrate_test.go`
- `internal/db/migrations/0004_odds_links.sql`
- `internal/db/odds.go`
- `internal/db/sync_test.go`
- `internal/ingest/odds/client_test.go`
- `internal/ingest/odds/events.go`
- `internal/ingest/odds/lines.go`
- `internal/ingest/odds/lines_test.go`
- `internal/ingest/odds/links_internal_test.go`
- `internal/ingest/odds/testdata/README.md`
- `internal/ingest/odds/testdata/odds_game_links.json`
- `internal/ingest/odds/testdata/odds_prop_links.json`
- `internal/refresh/sources.go`
- `internal/refresh/sources_test.go`
- `plan/data-sources.md`

## Run log

- Mode:
- Checkpoints: task-1, task-2, task-3
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
