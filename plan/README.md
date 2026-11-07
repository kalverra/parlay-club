# Sunday Parlay Club: Plan

A small web app where a group of friends builds a shared NFL parlay each week. Everyone sees new legs live, gets
friendly heads-ups about bad pairings, and tracks results over the season. It runs as one Go binary on Cloud Run inside
the free tier, with about $20 of GCP credits as the safety net.

## How this plan works with good-vibes

`plan/` holds the **target design**: behavior that is not built yet. `docs/` holds **living docs**: the system as it is
now. Work moves from one to the other through good-vibes changes.

1. Each section under "Changes" below is one good-vibes change, built in order. A change starts as a proposal in
   `changes/<slug>/proposal.md`, then gets a design in `changes/<slug>/design.md`.
2. A change's `design.md` cites the `plan/` sections it builds, rather than copying them. Its Verification section comes
   from the acceptance checks below. Its TODO phases come from the deliverables, with each phase a short list of
   vertical tasks that ship behavior and tests together.
3. Each phase's `Docs:` task moves the parts it built out of `plan/` and into living docs under `docs/` (concepts and
   reference, with front matter), then deletes them from `plan/`. `plan/` shrinks as the app grows; when the last change
   closes, it is empty.
4. When a plan doc is silent or wrong, pick the simplest option that fits [AGENTS.md](../AGENTS.md), record it in the
   change's design, and fix the plan doc.

Every change is done when its Verification checks pass and the verify commands in `good-vibes.toml` are green.

## Plan docs

| Doc                                    | Covers                                                                |
| -------------------------------------- | --------------------------------------------------------------------- |
| [product.md](product.md)               | Grading, Record Book, screens, and links to the living docs           |
| [architecture.md](architecture.md)     | Repo layout, routes, live updates, refresh, deploy                    |
| [data-sources.md](data-sources.md)     | ESPN, Open-Meteo, The Odds API, matching, fixtures                    |
| [analyzer.md](analyzer.md)             | Warning types and every rule with exact triggers                      |

## Changes

Each change's `design.md` is its contract; the entries below are summaries. Changes 1–5 ship by **Sunday, Nov 8, 2026**.
Changes 6–8 follow; their designs are written after 1–5 land.

`history-and-deploy` runs interactively because the user runs the GCP setup. The others run with
`good-vibes implement <slug> --style orchestrated`.

### 1. `foundation`

Server, database with the full schema, passcode and user picker, phone-first page shell. Done; decision:
[docs/decisions/foundation.md](../docs/decisions/foundation.md).

### 2. `slips-live`

Shared slips, members, hand-entered legs, locking, odds math, payout, live updates over SSE. Design:
[changes/slips-live/design.md](../changes/slips-live/design.md).

### 3. `slate-and-lines`

ESPN slate, Open-Meteo weather, Odds API game lines, lazy refresh, Games tab with quick-add. Design:
[changes/slate-and-lines/design.md](../changes/slate-and-lines/design.md).

### 4. `analyzer`

Rules engine (all rules except `median_trap`) and the banner under the slip. Design:
[changes/analyzer/design.md](../changes/analyzer/design.md).

### 5. `history-and-deploy`

Hand grading, past-week slips, history file, container with Litestream, CI, deploy to Cloud Run. Runs interactively.
Design: [changes/history-and-deploy/design.md](../changes/history-and-deploy/design.md).

### 6. `props-research`: Props and research

Read: data-sources.md (props, rosters, game logs, player matching), analyzer.md (`median_trap`), product.md (Screens:
Research tab).

Deliverables:

- Per-game props on demand, ESPN rosters, player matching with `player_matches`, ESPN game logs.
- Quick-add prop pills on an expanded game card; player name autocomplete in the leg form.
- `median_trap` rule.
- Research tab: last-5 hit bars against the line with median and mean marked, and key injuries.

Acceptance:

- Player matching tests: suffixes, punctuation, and a same-name player on another team.
- `median_trap` threshold tests.
- Opening the same game twice within the cache window spends credits once.

### 7. `grading-record-book`: Auto-grading and Record Book

Read: product.md (Grading, Record Book), data-sources.md (box scores).

Deliverables:

- `internal/grader`: grades legs from ESPN box scores when games go final; lazy, via refresh.
- Record Book tab: group record, units P/L, streaks, leaderboard, Parlay Terrorist badge.
- Optional wager entry per member per slip and dollar P/L.

Acceptance:

- Grader tests for every market in product.md "Grading", using saved box scores, including VOID for a player who did not
  play and PUSH on an exact line.
- Record Book tests over a fixed set of graded slips.

### 8. `live-api-watch`: Live API watch

Deliverables: `.github/workflows/live-apis.yml`, weekly, runs the live tests without `-short`.

Acceptance: the workflow runs green by manual dispatch.
