# Phase 2 handoff: Rules and slip view

## Done

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

## Deviations

- The design's "phone-width check" is a structural check, not a browser screenshot. The repo has no headless browser
  or layout test setup, so the chip's placement was verified from the rendered markup: it sits in the existing
  `flex flex-wrap` meta row beside the odds and result, it is `whitespace-nowrap` so its own text cannot break, and the
  selection truncates through the row's `min-w-0` / `truncate`. The handler test pins that row class. A person may
  still want a one-minute look in a real browser before shipping.
- The docs step named in the phase names only `docs/reference/analyzer-rules.md` and `docs/concepts/analyzer.md`.
  `docs/concepts/slips.md` also changed because it now describes the leg row. It is in the phase's Files touched list.
- The handler test covers a final game in addition to the design's live game. The guard is the same status check, and
  the extra case makes it explicit that a finished game cannot show a projection.
- The design says the chip "renders beside the odds". It is inside the existing meta row next to the odds and the
  result chip rather than in a new element, so the wrap at 360 px works without widening the card.

## Open issues

- `qb_drag` reads the level of nfelo's pre-game QB adjustment. It cannot detect a game-day quarterback change, and the
  message stays accurate only if nfelo keeps that column up to date.
- `internal/web` reads `NfeloForGame` once per game on every slip render. That is fine for a week's slate today, but a
  batch read is the next step if render time grows or a page starts showing up in profiles.
- The 72 hour guard treats any `SyncState` error, not just a missing row, as "no model". That keeps the page working
  when SQLite has a transient problem, but the chip and model sentences go quiet without any visible signal. The sync
  failure log and archive remain the way to notice.
- If nfelo stops publishing or changes a column, every model sentence disappears after 72 hours rather than showing
  stale data. That is intended, and it means a quiet banner is not proof that the rule code is healthy.
- Carried over from Phase 1: the nfelo repo has no license, the column meanings are inferred, the fixture is frozen at
  2026-11-09, and the CSV is fetched every 12 hours. A person should settle permission with the author before shipping.
- `good-vibes check nfelo-signals` reports 0 errors and 10 stale-doc warnings for docs unrelated to this phase (they
  include `cmd/server/main.go`, which Phase 1 touched). They predate this phase and were left alone.

## Next-phase notes

- Phase 2 is the last phase in the design's TODO. The next step is the close-out: run the close skill, have a person
  review it, then run `good-vibes done nfelo-signals`. Do not start new implementation work under this change.
- Any future rule that needs the model should call `Game.ModelSpread` or `Game.ModelWinProb` instead of reading
  `Nfelo` fields directly, so the away-team sign and probability flips stay in one place.
- Keep the freshness guard on `sync_state` for `nfelo:projections`. `nfelo_games.updated_at` only moves when a value
  changes and is not a fetch timestamp.
- The handler tests seed rows with `UpsertNfeloGames` plus `RecordSync`, then render `/slips/{id}/panel` through the
  existing `newSlipFixture`, `addGame`, and `addStoredLeg` helpers. Reuse that setup for any future chip or banner
  change.
- The model chip is preformatted in `internal/web`, so the template only checks `{{if .Model}}`. Keep formatting out of
  `partials.html`.

## Files touched

- `docs/concepts/analyzer.md`
- `docs/concepts/slips.md`
- `docs/llms.txt`
- `docs/reference/analyzer-rules.md`
- `internal/analyzer/analyzer.go`
- `internal/analyzer/analyzer_test.go`
- `internal/analyzer/rules.go`
- `internal/web/analyzer.go`
- `internal/web/analyzer_test.go`
- `internal/web/slips.go`
- `web/templates/partials.html`

## Run log

- Mode: 
- Checkpoints: task-4, task-5, task-6
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
