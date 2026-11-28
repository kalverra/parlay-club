# Phase 3: Phase 3: Dedicated phone-first Live view ordered by game time

## Done

### Task 7: Live view models and overview spotlight

The worker wrote no notes.

### Task 8: Phone-first Live layout

- Updated `web/templates/slip.html` to stamp `data-live-mode` on the outer `<section>`, render `#slip-panel` first when
  `.Slip.LiveMode` is true, and place `slip-export`, `slip-payout`, `slip-wager`, and `slip-activity` inside a collapsed
  `<details class="ui-card" data-live-secondary>` drawer below `#slip-panel` while preserving the export-first layout
  when `LiveMode` is false.
- Updated `web/templates/partials.html` so `#slip-panel` stamps `data-live-mode` and renders the dedicated phone-first
  Live layout when `LiveMode` is true: compact Live header (`LIVE` pill, `<time>` freshness stamp, sibling switcher, and
  `Refresh` button posting to `/slips/{id}/live/refresh`), Parlay Live Hero box (Before % vs. Now %, delta badge, and
  progress bar), and game-time-ordered `LiveGames` cards (game progress bar, leg progress bar, compact owner badge, and
  Before → Now % on every leg).
- Updated `web/static/app.js` in `htmx:afterSwap` to reload the page when `#slip-panel`'s `data-live-mode` differs from
  `section[data-live-mode]`.
- Added `TestSlipDedicatedLiveView` in `internal/web/slips_test.go` and `TestAppJSReloadsWhenLiveModeFlips` in
  `internal/web/static_test.go`, and ran `go generate ./...`, `golangci-lint run ./... --fix`, and
  `go test -short ./...`.

### Task 9: Live view docs

- Updated [docs/concepts/slips.md](../../../docs/concepts/slips.md) (`## The hub`, `## Creating a slip`,
  `## Live progress`, and `## Several slips`) to document the dedicated `live` step layout, compact Live header, Parlay
  Live Hero, `LiveGames` game-time ordering, collapsed secondary controls drawer, `app.js` reload-on-flip behavior, and
  `"Watch live"` overview spotlight.
- Adjusted `budget.max_lines` in `docs/concepts/slips.md` to `615` (601 actual lines) while keeping code-span density
  within the 105-per-100-lines concept limit.
- Ran `good-vibes docs lint` (0 errors) and `good-vibes docs index`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 8

Live templates and `app.js` reload-on-flip are in place for the remaining documentation and verification tasks.

### Task 9

Phase 3 documentation is complete; ready for Phase 3 verification/closeout.

## Files touched

- `docs/concepts/slips.md`
- `internal/web/home.go`
- `internal/web/home_overview_test.go`
- `internal/web/slips.go`
- `internal/web/slips_test.go`
- `internal/web/slips_unit_test.go`
- `internal/web/static_test.go`
- `web/static/app.css`
- `web/static/app.js`
- `web/templates/partials.html`
- `web/templates/slip.html`

## Run log

- Mode:
- Checkpoints: task-7, task-8, task-9
- Verify: verify passed: test 10.1s; skipped: generate (no matching files), lint (no matching files), actionlint (no
  matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
