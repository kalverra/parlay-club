# Phase 3: Combined links

## Done

- Both books were checked on a phone on 2026-11-07 and both accept several selections in one URL:
  - DraftKings puts comma-separated sids in one `outcomes` parameter:
    `https://sportsbook.draftkings.com/?outcomes={sid1},{sid2}`.
  - FanDuel indexes percent-encoded `marketId`/`selectionId` pairs:
    `https://sportsbook.fanduel.com/addToBetslip?marketId%5B0%5D=...&selectionId%5B0%5D=...`.
- `internal/export` registers a `Combiner` for each book. `Plan` looks one up by normalized book key; a book without a
  format would stay on per-leg taps.
  - Fewer than two linked legs keep the per-leg list, so one tap stays one tap.
  - `Combine` returns `ok = false` if any linked leg has no `sid`, or, for FanDuel, no `marketId` in its stored link.
    The plan then leaves `Combined` empty and every per-leg link stays available.
  - DraftKings escapes each sid on its own, so a prop sid's `#` becomes `%23` while the comma stays the separator.
  - FanDuel reads the market ID from the stored per-leg link and the selection ID from `odds.sid`.
- Tests cover the verified DraftKings format, a prop sid with `#`, both FanDuel fields, the fallback for a missing `sid`
  or `marketId`, fewer than two linked legs, and manual legs never reaching a `Combiner`.
- Docs: `docs/concepts/export.md` records both verified formats with the checked date and the fallback rules.
  `plan/data-sources.md` and `docs/reference/routes.md` describe the combined card.

## Review fixes folded into this phase

- `POST /slips/{id}/lock` and `/unlock` now send `Hx-Trigger: slip-updated`, so the acting person's export card updates
  with their own response even when the SSE stream is closed.
- The export card drops rows from an earlier sync with `currentOdds`, matching the Games tab and the slip's reference
  prices. A market a book stopped quoting no longer offers a dead link.
- A moved total or prop prints its bare line (`47.5`); only a spread keeps the signed form (`-3`, `+3.5`).
- Selection matching and its helpers now live in `db`: `db.Odds.MatchesSelection`, `db.SameLine`, `db.NormalizeBook`,
  `db.OptionalString`, and `db.StringValue`. Export, the Games tab, reference prices, and refresh share one definition
  of "the same selection" and one book key.
- Handler tests cover the moved total's unsigned line, a stale row ignored before planning, and the lock/unlock refresh
  trigger.
- The `PlanWithCombiner` comment now describes its current role: tests inject a fake or nil `Combiner` independently of
  the default registry.

## Deviations

- **The FanDuel market ID is parsed from the stored per-leg link.** The design gives `sid` to a combiner, but FanDuel
  needs both a market ID and a selection ID. Adding a third column would duplicate data already in `link`; parsing the
  one field keeps the schema unchanged, and a missing value falls back to per-leg taps.
- **The combined URL needs two linked legs.** A single link is already one tap, so a one-selection "combined" URL adds
  nothing and would hide the more direct stored link.
- **The two formats are the only verified ones.** Other books are absent from the registry rather than guessed.

## Open issues

- **A phone check is not an automated test.** The combiners are tested against saved sids and link shapes, but a
  sportsbook can change its URL scheme without notice. The `live-api-watch` change should check one link shape.
- **Existing odds rows have NULL links and sids until the next sync.** A locked slip can show manual legs until
  `odds:lines` refreshes.
- **Props have no `odds` rows until `props-research` lands.** A prop leg shows `no line` for both books today.
- **The Odds API terms are still unconfirmed for storing and showing bookmaker links.**

## Files touched

- `changes/export-to-sportsbook/design.md`
- `docs/concepts/export.md`
- `docs/llms.txt`
- `docs/reference/routes.md`
- `internal/db/odds.go`
- `internal/export/export.go`
- `internal/export/export_test.go`
- `internal/refresh/sources.go`
- `internal/web/export.go`
- `internal/web/export_test.go`
- `internal/web/slips.go`
- `plan/data-sources.md`

## Run log

- Mode:
- Checkpoints: task-7, task-8
- Verify: `go test ./...`, `go vet ./...`, `golangci-lint run ./...` passed; `good-vibes check` and
  `good-vibes docs lint` passed
- Rounds used: 1 of 2; verify failures: 0
