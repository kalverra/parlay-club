## Done

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

## Deviations

None.

## Open issues

None.

## Next-phase notes

Live templates and `app.js` reload-on-flip are in place for the remaining documentation and verification tasks.
