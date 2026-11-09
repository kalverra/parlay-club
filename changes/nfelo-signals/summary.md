---
slug: nfelo-signals
run: 20261109-194154-nfelo-signals
outcome: done
generated: 2026-11-09T20:14:35Z
---

# Run summary: nfelo-signals

## Outcome

- Status: done
- Branch: nfelo-signals
- Final verdict: agy@gemini-4-argon-medium approved (issues: 0)

## Totals

- Tasks: 6 of 6
- Phases: 2
- Checkpoints: 6
- Duration: 31m5s
- Cost: $1.23
- Input tokens: 16121637
- Output tokens: 149701
- Cache read tokens: 12868926

## Phases

### Phase 1: Sync nfelo projections

Tasks:

- [x] Add `internal/ingest/nfelo` with `Projections`, the team-code map, a saved fixture of the real CSV, and tests for header-name parsing and every error case; use the fixture to confirm the spread column and fix the Approach if it is wrong
- [x] Add migration `0006_nfelo_games.sql`, the store read and upsert, and the `nfelo:projections` refresh source with a 12 hour window, the regular-season and all-kicked-off skips, backoff, and a publish on change, with the sync tests
- [x] Docs: add nfelo to `docs/reference/data-sources.md`, `docs/concepts/data-refresh.md`, and `docs/reference/schema.md`; add the source to `plan/data-sources.md` only if that file lists sources

Phase totals: 3 checkpoints, 14m21s, $0.67, 8647407 input / 73157 output / 6842646 cache tokens

Checkpoints: task-1, task-2, task-3 Review fixes: 0 Integration checks: 0

#### Done

- Added `internal/ingest/nfelo` with a `Client` that takes a base URL and an `*http.Client` (10 second default,
  `User-Agent: parlay-club`). `Projections(ctx)` fetches `output_data/historic_projected_spreads.csv`, reads every
  column by header name, ignores the nameless pandas index, drops a row with any empty required cell, and errors on a
  missing header, an unparseable number, a malformed row, or a non-200 response. Team codes map `OAK`→`LV` and
  `WAS`→`WSH`; other codes pass through.
- Saved the real CSV at `internal/ingest/nfelo/testdata/historic_projected_spreads.csv` (1,551 data rows, 1,449 with
  every required cell) with a README recording the source and fetch date. Tests cover the full fixture, a reordered
  header, each missing required header, unparseable numeric cells, empty required cells, a header-only file, an empty
  body, a malformed row, and a 404. A test also proves every team code in the fixture is one of the 32 ESPN
  abbreviations.
- Confirmed the spread column against the fixture. `home_closing_line_rounded_nfelo` equals the market close on most
  current-season rows, while `home_line_pre_regression` rarely does, so `HomeSpread` comes from
  `home_line_pre_regression` and `HomeWinProb` from `home_probability_nfelo`. A test pins the choice.
- Added migration `0006_nfelo_games.sql`: one row per game keyed by `game_id` with `ON DELETE CASCADE`, holding
  `home_spread`, `home_win_prob`, `home_qb`, `away_qb`, `home_qb_adj`, `away_qb_adj`, and `updated_at`. The migration
  tests now expect schema version 6 and the new table.
- Added `db.UpsertNfeloGames` and `db.NfeloForGame`. The upsert no-ops when every stored value matches, so
  `updated_at` moves only on a real change. The read returns `(NfeloGame, false, nil)` when a game has no row.
- Added the `nfelo:projections` refresh source with `ProjectionsClient` in `Deps` and wiring in `cmd/server/main.go`.
  It is stale after 12 hours, skips a non-regular-season week, a week with no games, and a week whose games have all
  kicked off, and turns off when the client is nil. It records `sync_state` only on a successful fetch, leaves rows
  and `fetched_at` alone when a fetch fails (so the existing five minute backoff applies), matches on season, week,
  home, and away, and publishes `week:{id}` plus the week's slips after commit only when a row changed. Current-week
  CSV rows that match no game are logged at debug.
- Sync tests cover a matching insert and publish, a second identical run that changes and publishes nothing, an
  unmatched game, both skip cases, the nil client, and a failed fetch that keeps the old row and backs off five
  minutes.
- Docs: nfelo is in `docs/reference/data-sources.md`, `docs/concepts/data-refresh.md`, and
  `docs/reference/schema.md`; `plan/data-sources.md` lists it too. Regenerated `docs/llms.txt`.

### Phase 2: Rules and slip view

Tasks:

- [x] Add `analyzer.Nfelo`, the web mapping with the `SCHEDULED` and 72 hour guards, the `hook` and `chalk` sentences, and the new `qb_drag` rule, with the table tests and the nil-input regression test
- [x] Add `LegView.Model` and the leg row chip, with a handler test and a phone-width check
- [x] Docs: add the three changes to `docs/reference/analyzer-rules.md` and nfelo as an input in `docs/concepts/analyzer.md`

Phase totals: 3 checkpoints, 16m44s, $0.56, 7474230 input / 76544 output / 6026280 cache tokens

Checkpoints: task-4, task-5, task-6 Review fixes: 0 Integration checks: 0

#### Done

- Added `analyzer.Nfelo` and the `Game.ModelSpread` / `Game.ModelWinProb` helpers. The helpers flip the home spread
  for an away leg and invert the home win probability, so rules only see a number from the leg team's side. Both
  report false when the game has no model or the team is not in the game.
- `hook` keeps its trigger and severity and appends one sentence from the cushion `L − m`: at +2 or more the model sits
  farther from the key number, at −2 or less the leg asks for more than the model does, and in between the hook stays
  live.
- `chalk` keeps its trigger and severity and appends nfelo's win probability on an `ML` leg only. A spread or prop at
  the same price keeps the plain sentence.
- Added the `qb_drag` rule at `Info`. It fires when a team's nfelo QB adjustment is −30 Elo or lower and the slip has
  that team's favorite `SPREAD` or `ML`, or an over on `PASS_YDS`, `PASS_TDS`, `REC_YDS`, `RECEPTIONS`, or
  `TEAM_TOTAL`. It gives one card per game and team, names the QB and the points, and names both owners when two
  people have legs on the team.
- `internal/web` now reads `nfelo:projections` from `sync_state` and drops the whole model when the row is missing, the
  read fails, or the last success is more than 72 hours old. `analyzerGame` attaches nfelo only to a `SCHEDULED` game,
  so a live or final game gets no model sentences and no chip.
- Added `LegView.Model` and the leg-row chip. A spread shows `nfelo -5.0` from the leg team's side; a moneyline shows
  `nfelo 76%`; totals, props, and games without fresh model data render nothing.
- Tests cover every `hook` cushion on a favorite and an underdog, `chalk` on `ML` versus `SPREAD` and a prop, the
  `qb_drag` threshold at −29.9 and −30, its markets, its quiet cases, two owners on one team, and a no-model run. A
  regression test pins the old `hook` and `chalk` text byte for byte when `Nfelo` is nil.
- Handler tests cover the week-9 style slip with fresh and scheduled rows, a four day old `sync_state` row, and live
  and final games. They also pin the chip's two formats, the away-team sign flip, the quiet total, and the wrapping
  row class.
- Docs: `docs/reference/analyzer-rules.md` documents the `hook` and `chalk` sentences plus the new `qb_drag` rule;
  `docs/concepts/analyzer.md` adds nfelo as an input and the `SCHEDULED` / 72 hour guard; `docs/concepts/slips.md`
  describes the chip and `docs/llms.txt` was regenerated.

## Final gate

- Rounds used: 0; verify failures: 0
- Final verify: no commands configured
- Open issues: none
