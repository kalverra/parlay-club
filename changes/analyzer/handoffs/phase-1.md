# Phase 1: Engine and correlation rules

## Done

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

## Deviations

- The design says each rule is `func(ctx) []Warning`; the implementation uses the concrete `func(*analysis) []Warning`.
  Same shape, less indirection.
- Messages are assembled from small helpers (`selection`, `describe`, `humanJoin`, verb helpers) around `fmt.Sprintf`
  rather than one long format string per rule. The content and tone match the plan.
- `stats` is accepted but not read yet (`_ = stats`). No Phase 1 rule needs player stats, and a nil map is fine.

## Open issues

- Package coverage is 87.8%, below the design's 95% bar. The threshold rows are covered; the gap is in message and guard
  branches (formatting variants, favorites by moneyline, missing player IDs). Phase 2 must lift coverage to 95% while
  adding the weather, key-number, and chalk rules.
- `favorite` handles both `SPREAD` and `ML`, but `stack_clock_drain` only considers `SPREAD` legs (per the plan). The
  moneyline branch is currently unused and only partly tested.
- `AnalyzeSlip` drops any leg whose `GameID` is absent from the games map. Phase 3 must pass a game entry for every leg
  it expects to analyze.
- `hook` and the handler test that expects both `blowout_passing` and `hook` are still ahead in Phases 2 and 3.

## Next-phase notes

- Add new rules to the `rules` slice in `internal/analyzer/analyzer.go`; the final sort already owns severity and slip
  order. Keep rule IDs in `rules.go` stable.
- Reuse `a.matching`, `a.warning`, the `selection`/`describe` helpers, and the over predicates so the new rules read like
  the existing ones.
- Weather rules should skip indoor games and games with a nil `Weather`; `Game.Indoor` and `Game.Weather` are already in
  the types.
- Append Rule Sets B and C plus `chalk` to `docs/reference/analyzer-rules.md`, and delete those sections from
  `plan/analyzer.md`.
- Phase 3's web mapping should populate `Owner`, `GameID`, `Market`, `Team`, `PlayerID`/`PlayerName`, `Side`, `Line`,
  and `Odds`, and key the games map by the same `GameID`.

## Files touched

- `docs/llms.txt`
- `docs/reference/analyzer-rules.md`
- `internal/analyzer/analyzer.go`
- `internal/analyzer/analyzer_test.go`
- `internal/analyzer/rules.go`
- `plan/analyzer.md`

## Run log

- Mode: 
- Checkpoints: task-1, task-2
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
