# Phase 2: Plan and show the export

## Done

- `internal/export` is a pure planning package. `Plan` and `PlanWithCombiner` turn a locked slip's legs and stored odds
  rows into a `BookPlan` per book, with every leg in exactly one of `Linked` or `Manual`.
  - The match uses the same key as the `odds` primary key: game, book, market type, team, player, and side, plus an
    exact line where NULL matches only NULL.
  - The kickoff check runs before the odds lookup. A started game is manual even when its row still has a link.
  - The manual reasons are `no line`, `no link`, `line moved`, and `game started`. A moved line carries the book's
    current line so the card can show it.
  - Price drift alone still links. An empty or missing link is `no link`, not an error.
  - A non-`LOCKED` slip gets an empty plan.
  - Table tests cover every market type on both books, every manual reason, each part of the odds key, price drift, the
    exactly-one-list property, a verified and an unverified `Combiner`, and open/graded slips.
- `GET /slips/{id}/export` renders the `slip-export` partial. The slip page includes the same partial after the slip
  panel, and the wrapper refreshes on `slip-updated`, so an unlock removes the card on the next event.
- The card shows one block per book. A book with a combined URL gets one "Open in ..." button; otherwise each linked leg
  is a numbered tap with its selection and the book's price. Manual legs sit under their book with a short friendly
  note. "Links as of" comes from the newest `odds.fetched_at` among the slip's games, and "Copy slip as text" writes one
  line per leg with member, selection, and the price at lock.
- Links open with `target="_blank"` and `rel="noopener"`. Selections and notes render through `html/template`, so a
  hand-typed `OTHER` leg is escaped in both the card and the copy text.
- The export path only reads `slips`, `legs`, `odds`, and the week's games. It makes no Odds API call and no database
  write.
- Docs: `docs/concepts/export.md` covers the data source, the planning rules, the reasons, combined links, the card,
  staleness, and the related docs. The route and its behavior are in `docs/reference/routes.md`, and `docs/llms.txt`
  includes the new concept.
- `go test ./...`, `go vet ./...`, and `golangci-lint run ./...` pass. `good-vibes docs lint` reports no errors for the
  new doc.

## Deviations

- **The route is documented in `docs/reference/routes.md`, not `plan/architecture.md`.** The architecture doc's Routes
  section now lists only routes still to come and points built routes at `docs/reference/routes.md`, so the export route
  went where every other built route lives.
- **The plan type is `BookPlan`, and the function is `Plan`.** Go cannot have a function and a type share a name, so the
  design's `type Plan struct` became `BookPlan`. The fields match the design.
- **The card is a sibling partial on the slip page, not a child of `#slip-panel`.** The design calls it an export panel
  on the slip panel; it is instead the `slip-export` partial placed right after the panel. Both refresh on
  `slip-updated`, but the export card keeps its own HTMX target and can disappear on unlock without waiting for the
  panel's member data.
- **No `Combiner` is registered.** Both books still show per-leg taps, which is the design's complete fallback until a
  multi-selection format is proven on a phone. `PlanWithCombiner` and the `combiner` seam are in place for Phase 3.
- **The card's note uses stored odds rows only.** It does not consult `sync_state` the way the Games board's note can.
  The design names `odds.fetched_at`, so the card follows the link rows themselves.

## Open issues

- **No multi-selection format is known.** Phase 3 still needs the hands-on phone check before any `Combiner` is written.
  The two known link shapes each hold one selection.
- **DraftKings prop sids contain `#`,** which The Odds API sends URL-encoded as `%23`. A future `Combiner` must encode
  stored sids instead of concatenating them.
- **Props have no `odds` rows yet.** Until `props-research` lands, a prop leg has no row for either book and shows the
  `no line` note. A game-line leg whose book omitted a link shows `no link`. Both are expected, not errors.
- **Existing odds rows keep NULL links until the next `odds:lines` sync.** There is no backfill, so a slip locked right
  after deploy can show manual legs until that refresh runs.
- **The phone-width visual pass is not recorded in an automated test.** The card is a single-column, full-width layout
  with no fixed widths, and the handler tests assert its markup. A hands-on check with all legs linked, some manual, and
  none linked is still worth doing before close.
- **The Odds API terms are still unconfirmed for storing and showing bookmaker links.** A person should confirm this
  before shipping.

## Next-phase notes

- Phase 3 only needs to add a verified `Combiner`. Put it in the `combiners` map keyed by `draftkings` or `fanduel`, or
  call `PlanWithCombiner` directly. Do not guess a format; return `ok = false` when the phone check fails.
- `LegLink` carries `Sid`, and `PlanWithCombiner` only passes linked legs to the combiner. Manual legs never reach it.
- The template already switches on `OpenURL`, so a non-empty `Combined` becomes the single "Open in ..." button with no
  template change. Keep the per-leg list for books without a combiner.
- The export card reads `db.Odds.Link` directly. Treat `nil` or blank as manual. `Sid` is only for a combiner.
- Keep the `every leg appears in exactly one of Linked or Manual` test as the safety net when adding a `Combiner`; it is
  what catches a leg silently disappearing from the card.
- When a format is verified, record it with the checked date in `docs/concepts/export.md` under "Verified formats", and
  update the route doc only if the route or its response changes.

## Files touched

- `docs/concepts/export.md`
- `docs/llms.txt`
- `docs/reference/routes.md`
- `internal/export/export.go`
- `internal/export/export_test.go`
- `internal/web/export.go`
- `internal/web/export_test.go`
- `internal/web/render.go`
- `internal/web/slips.go`
- `web/templates/partials.html`
- `web/templates/slip.html`

## Run log

- Mode:
- Checkpoints: task-4, task-5, task-6
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
