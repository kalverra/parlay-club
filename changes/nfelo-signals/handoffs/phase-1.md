# Phase 1 handoff: Sync nfelo projections

## Done

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

## Deviations

- The design said the team-code test checks every output code against the venue map's teams. The weather venue map
  holds venue IDs and coordinates, not team codes, so the test uses a hardcoded set of the 32 ESPN abbreviations.
  Updated the Approach and the team-code risk to match.
- The fixture is larger than the design's estimate: 1,551 raw data rows through 2026 week 9, not about 1,550 overall
  from 2021. Only 1,449 rows carry every required cell, because rows with a blank QB name or adjustment are dropped
  as designed. The docs say "about 1,550 rows", which describes the file rather than the parsed count.
- The design's sample week 9 game, BUF −7.5 against MIA, is not in the saved fixture. The real 2026 week 9 has MIN
  at home against BUF, with Carson Wentz at a −33.88 Elo adjustment and Josh Allen at +12.90. The sample slip stays
  illustrative; Phase 2 tests should seed their own rows.
- The design left the fixture check as future work in the Approach. Replaced that sentence with the confirmed result,
  with no change to the column choice or any task text.

## Open issues

- The nfelo repo has no license and the site says "All rights reserved." A person should ask the author for
  permission before this ships; the agent did not.
- The whole CSV is about 0.6 MB and is fetched every 12 hours. Unchanged bodies are stored once in the archive, but
  check the archive-size runbook after the first week.
- The column meaning is inferred, not documented by nfelo. The fixture guard passes today. If the model spread starts
  equaling the market line on most games, suspect an upstream column change; header-name parsing would not catch a
  meaning change.
- The fixture is frozen at 2026-11-09. Tests assert exact counts and a few values from it, so refreshing the fixture
  later means updating those expectations.
- Only games matched on season, week, home, and away get a `nfelo_games` row. An unmapped team code or a missing
  game leaves no row, which Phase 2 must treat as "no model data" rather than stale data.

## Next-phase notes

- Read projections with `db.NfeloForGame(ctx, gameID)`. It reports a missing row as `ok == false` with a nil error.
  A batch read is unnecessary until per-leg reads prove noisy.
- Do not use `nfelo_games.updated_at` as "last fetched": it only moves when a value changes. The 72 hour freshness
  guard must read `sync_state` for `nfelo:projections`.
- `HomeSpread` is from the home side and `HomeWinProb` is the home team's probability. Flip the spread sign for an
  away-team leg and invert the probability for an away `ML` leg in this order: leg side, then compare.
- Useful real rows from the fixture: MIN vs BUF in 2026 week 9 gives a live `qb_drag` case (Carson Wentz, about
  −1.36 points), and SF at LV plus WSH hosting LAR exercise both mapped team codes.
- No analyzer or web code changed in this phase. `analyzer.Game` has no `Nfelo` field and `LegView` has no `Model`
  field yet.

## Files touched

- `cmd/server/main.go`
- `docs/concepts/data-refresh.md`
- `docs/llms.txt`
- `docs/reference/data-sources.md`
- `docs/reference/schema.md`
- `internal/db/migrate_internal_test.go`
- `internal/db/migrate_test.go`
- `internal/db/migrations/0006_nfelo_games.sql`
- `internal/db/nfelo.go`
- `internal/db/nfelo_test.go`
- `internal/ingest/nfelo/client.go`
- `internal/ingest/nfelo/projections.go`
- `internal/ingest/nfelo/projections_test.go`
- `internal/ingest/nfelo/teams.go`
- `internal/ingest/nfelo/teams_test.go`
- `internal/ingest/nfelo/testdata/README.md`
- `internal/ingest/nfelo/testdata/historic_projected_spreads.csv`
- `internal/refresh/nfelo.go`
- `internal/refresh/nfelo_test.go`
- `internal/refresh/refresh.go`
- `internal/refresh/rules.go`
- `plan/data-sources.md`

## Run log

- Mode: 
- Checkpoints: task-1, task-2, task-3
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
