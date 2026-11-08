---
slug: analyzer
run: 20261108-020451-analyzer
outcome: done
generated: 2026-11-08T02:25:00Z
---

# Run summary: analyzer

## Outcome

- Status: done
- Branch: analyzer
- Final verdict: agy@gemini-4-argon-medium approved (issues: 0)

## Totals

- Tasks: 6 of 6
- Phases: 3
- Checkpoints: 6
- Duration: 18m48s
- Cost: $0.81
- Input tokens: 13347307
- Output tokens: 165155
- Cache read tokens: 11545098

## Phases

### Phase 1: Engine and correlation rules

Tasks:

- [x] Add analyzer types, the rule runner, stable sorting, the shared leg and game helpers, and blowout_passing, trailing_rb, total_contradiction, stack_clock_drain, and stack_shootout with table tests covering every threshold row in plan/analyzer.md
- [x] Docs: add docs/reference/analyzer-rules.md with Rule Set A; delete it from plan/analyzer.md

Phase totals: 2 checkpoints, 6m11s, $0.26, 2978604 input / 58674 output / 2393418 cache tokens

Checkpoints: task-1, task-2 Review fixes: 0 Integration checks: 0

#### Done

- Added `internal/analyzer` with the `Severity`, `Warning`, `Leg`, `Game`, `Weather`, and `PlayerStats` types from
  `plan/analyzer.md`, plus `AnalyzeSlip(legs, games, stats) []Warning`.
- `AnalyzeSlip` skips `OTHER` legs, legs with no `GameID`, and legs whose game is missing, then runs a fixed rule list
  and sorts output by severity (`Danger`, `Warn`, `Info`) and first leg position. Sorting is stable, and each warning's
  `LegIDs` are kept in slip order.
- Added the shared helpers: same game, same team, same player (ID first, name fallback), opponent, favorite, underdog,
  trailing underdog, and the passing/receiving/rushing over predicates. Message helpers build selections and join names,
  and they name owners whenever a rule involves more than one person.
- Added five rules with the thresholds from the plan:
  - `blowout_passing` (Warn): spread at −7.0 or lower plus a `PASS_YDS`/`REC_YDS` over on that team.
  - `trailing_rb` (Danger): spread at +4.5 or higher, or any underdog `ML`, plus a `RUSH_YDS`/`RUSH_ATT` over on that
    team.
  - `total_contradiction` (Danger): a `TOTAL` under plus two or more `ANYTIME_TD` or passing/receiving overs in the
    game.
  - `stack_clock_drain` (Info): favorite `SPREAD`, rush over on the favorite, and opponent `TEAM_TOTAL` under.
  - `stack_shootout` (Info): `PASS_YDS` over, a different receiver's over on the same team, and a `TOTAL` over.
- Added table tests for every threshold case in `plan/analyzer.md` for these rules, including `OTHER` legs, empty slips,
  unknown games, stable ordering, and owner naming in messages.
- Added `docs/reference/analyzer-rules.md` with Rule Set A, removed Rule Set A from `plan/analyzer.md`, and refreshed
  `docs/llms.txt`.

`go test ./...`, `good-vibes docs lint`, and `good-vibes check analyzer` pass.

### Phase 2: Weather, key numbers, chalk

Tasks:

- [x] Add wind, fake_weather, hook, total_key_number, and chalk with table tests covering every threshold row, plus OTHER legs and empty slips, and keep package coverage at or above 95%
- [x] Docs: add Rule Sets B, C, and chalk to docs/reference/analyzer-rules.md; delete them from plan/analyzer.md

Phase totals: 2 checkpoints, 8m2s, $0.36, 6339041 input / 76190 output / 5566764 cache tokens

Checkpoints: task-3, task-4 Review fixes: 0 Integration checks: 0

#### Done

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

### Phase 3: Banner

Tasks:

- [x] Map slip legs and games into analyzer inputs and render the banner: Looks clean, or cards sorted by severity with message, legs, and owners, with a handler test where BUF -7.5 (one user) and Josh Allen Over 245.5 (another) shows blowout_passing and hook to both
- [x] Docs: add docs/concepts/analyzer.md (purpose, tone, inputs); delete the banner text from plan/product.md

Phase totals: 2 checkpoints, 4m35s, $0.19, 4029662 input / 30291 output / 3584916 cache tokens

Checkpoints: task-5, task-6 Review fixes: 0 Integration checks: 0

#### Done

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

## Final gate

- Rounds used: 0; verify failures: 0
- Final verify: no commands configured
- Open issues: none
