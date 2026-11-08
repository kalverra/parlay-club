# Phase 2: Weather, key numbers, chalk

## Done

- Added five rules to `internal/analyzer`, each running in the fixed `rules` order:
  - `wind` (Danger at sustained wind ≥ 20 mph, otherwise Warn at ≥ 15 mph or gusts ≥ 25 mph): one warning per outdoor
    game with known weather, naming every flagged `OVER` on `PASS_YDS`, `PASS_TDS`, `REC_YDS`, `RECEPTIONS`,
    `KICKER_PTS`, or `TOTAL`.
  - `fake_weather` (Info): precipitation above 0 and below 0.1 in/hr, sustained wind below 10 mph, and a `TOTAL` `UNDER`
    in that game.
  - `hook` (Warn): a `SPREAD` at exactly −3.5, −7.5, +2.5, or +6.5, with the buy-to and moneyline suggestions from the
    plan.
  - `total_key_number` (Info): a `TOTAL` within 0.5 of 37, 41, 43, 44, 47, or 51, with separate messages for a line on a
    key, half a point off one, and between two.
  - `chalk` (Warn): any leg at −230 or lower, reporting the `1 + 100/|odds|` multiplier and the `1 − |odds|/(|odds| +
    100)` bust chance.
- Added table tests for every threshold row on both sides, plus indoor games, nil weather, missing weather, a missing
  line, `OTHER` legs, empty slips, multiple warnings per rule, and owner naming. Output-order tests now cover all five
  rule sets together.
- Package coverage is 97.8%, above the design's 95% bar.
- Added Rule Sets B and C and the chalk section to `docs/reference/analyzer-rules.md`, deleted those sections and their
  test-table rows from `plan/analyzer.md`, and regenerated `docs/llms.txt`.
- `go test ./...` and `golangci-lint run ./...` pass.

## Deviations

- The design describes each rule as `func(ctx) []Warning`; the implementation keeps the Phase 1 concrete form,
  `func(*analysis) []Warning`. Same shape, less indirection. Phase 1 noted this already.
- The plan's chalk model message said "about 1 in 4 times"; the implementation and docs say "about 25% of the time" to
  match the formula-driven output. The content is unchanged and the Tone section allows rewording.
- The Phase 1 coverage gap is closed without new production code; the added cases for the weather, key-number, and chalk
  guards lifted the package from 87.8% to 97.8%.

## Open issues

- `favorite` still has an `ML` branch that no rule uses: `stack_clock_drain` only considers `SPREAD` legs, per the plan.
  Keep it unless a later rule adopts it.
- `AnalyzeSlip` still drops any leg whose `GameID` is missing from the games map. Phase 3 must key the games map by the
  same `GameID` for every leg it expects to analyze.
- `stats` remains accepted and unread. `median_trap` still waits for `props-research`.
- `hook` only fires on `SPREAD` legs with a line; a moneyline carrying a stray line stays quiet, which matches the plan.

## Next-phase notes

- Phase 3 maps slip legs and games in `internal/web` and renders the banner. Populate `Owner`, `GameID`, `Market`,
  `Team`, `PlayerID`/`PlayerName`, `Side`, `Line`, and `Odds`, and key the games map by the same `GameID`.
- The banner test with BUF −7.5 (one owner) and Josh Allen Over 245.5 (another) should see `blowout_passing` and `hook`.
  Both are `Warn` and start on the spread leg, so the stable rule order puts `blowout_passing` first.
- The panel already refreshes on `slip-updated` and `board-updated`; no new SSE wiring is needed for weather changes.
- Phase 3 docs: add `docs/concepts/analyzer.md` and delete the banner text from `plan/product.md`.

## Files touched

- `docs/llms.txt`
- `docs/reference/analyzer-rules.md`
- `internal/analyzer/analyzer.go`
- `internal/analyzer/analyzer_test.go`
- `internal/analyzer/rules.go`
- `plan/analyzer.md`

## Run log

- Mode: 
- Checkpoints: task-3, task-4
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
