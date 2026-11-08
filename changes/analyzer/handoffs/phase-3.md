# Phase 3: Banner

## Done

- Added `internal/web/analyzer.go`:
  - `analyzerView` maps the slip's legs and the week's games into `analyzer.Leg` and `analyzer.Game`, keying the games
    map by the same `GameID` each leg carries.
  - `analyzerLeg` carries ID, owner name, game, market, team, player ID and name, side, line, and odds.
  - `analyzerGame` carries IDs, home and away teams, the indoor flag, and weather (wind, gusts, precipitation,
    temperature) when the game has a forecast.
  - `analyzerSeverity` maps `Danger`, `Warn`, and `Info` to headings and Tailwind classes.
- `internal/web/slips.go` loads `GamesForWeek` in `slipPageData`, adds `Analyzer` to `SlipView`, and fills it from the
  legs, games, and member names.
- `web/templates/partials.html` renders the analyzer section inside the slip panel: a green "Looks clean" when the
  warning list is empty, otherwise cards sorted by the analyzer's output with severity, message, and a chip per leg
  showing owner and selection. Each card carries `data-rule` for stable test hooks.
- `internal/web/analyzer_test.go` covers both banner states:
  - Two owners, BUF -7.5 for one and Josh Allen Over 245.5 for the other: `blowout_passing` and `hook` both render,
    `blowout_passing` comes first, both owners and selections appear, and "Looks clean" is absent.
  - Empty slip: "Looks clean" renders and no rule cards appear.
- Added `docs/concepts/analyzer.md` covering purpose, tone, the banner, inputs, skipped legs, and stable output order;
  deleted the analyzer banner bullet from `plan/product.md`; regenerated `docs/llms.txt`.
- `go test ./...` passes, `golangci-lint run ./...` reports 0 issues, and `internal/analyzer` coverage stays at 97.8%.

## Deviations

- The design said the banner refreshes with "no new wiring" because it lives in the slip panel. The panel only listened
  for `slip-updated`, so the edit adds `board-updated` to the panel's `hx-trigger`. That is a one-attribute change, not
  a new SSE path, but it is wiring the design did not call out.
- The design still described rules as `func(ctx) []Warning`; the implementation has always used
  `func(*analysis) []Warning`. The design's approach now matches the code.
- The banner does not re-sort warnings. It trusts the analyzer's stable severity-then-position order, so the handler test
  can assert rule order directly.
- Leg chips print `Owner · Selection`; the concept doc's tone example uses possessive prose ("Adam's BUF -7.5 + Sam's
  Josh Allen Over 245.5"). The messages follow the plan; only the chips use the shorter separator.

## Open issues

- No line or odds range validation exists in the leg form. The analyzer reads whatever was entered, so a nonsensical
  spread or odds value flows straight into a warning. This was already a listed risk and is still open.
- `stats` is passed as `nil` and remains unread; `median_trap` still waits for `props-research`.
- Legs with no `GameID`, or a `GameID` missing from the week's games, stay silent in the banner. That is the engine's
  documented behavior, but it means a hand-entered leg with no game contributes no warning.
- `favorite`'s `ML` branch is still unused by any rule, and `hook` still ignores moneyline legs that carry a stray line.

## Next-phase notes

- Phase 3 was the last phase. The remaining work is the close-out: write `changes/analyzer/closeout.md`, review
  `docs/concepts/analyzer.md` and `docs/reference/analyzer-rules.md` against the Tone section, then run
  `good-vibes done analyzer`.
- `board-updated` now re-renders the whole slip panel on every board refresh, not just the banner. This is intended for
  weather changes but also means the panel does more work on each board update.
- The banner's Tailwind classes are already present in `web/static/app.css`; no CSS regeneration is needed for this
  phase.

## Files touched

- `docs/concepts/analyzer.md`
- `docs/llms.txt`
- `internal/web/analyzer.go`
- `internal/web/analyzer_test.go`
- `internal/web/slips.go`
- `plan/product.md`
- `web/templates/partials.html`

## Run log

- Mode: 
- Checkpoints: task-5, task-6
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
