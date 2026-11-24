## Done

- Migrated `web/templates/slip.html` and `web/templates/pick.html` (slip creation form, empty state, pick board header, and modal sheet containers) to `.ui-card` and semantic page, chrome, surface, and accent tokens.
- Migrated `web/templates/partials.html` slip, dialog, highlights, and pick-board templates (`slip-panel`, `slip-export`, `slip-activity`, `slip-payout`, `slip-wager`, `leg-sheet`, `leg-added`, `leg-form`, `pick-option`, `pick-body`, `pick-slate`, `pick-detail`, `pick-line-group`, `pick-detail-empty`, `pick-tray`, `pick-dock`, `pick-swap`, `bug-report-*`, and `highlights-strip`) to semantic tokens and component classes (`.ui-next-card`, `.ui-slip-card`, `.ui-card`, `.ui-odds`).
- Wrapped `pick-detail` game lines in a `.ui-card` container so `pick-line-group` labels use `text-ink-muted` consistently across both game lines and player prop cards.
- Updated exact-class assertions in `internal/web/pick_test.go` and `internal/web/analyzer_test.go` and regenerated `web/static/app.css` via `go generate ./...`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

In `web/templates/partials.html`, `group-list`, `game-props`, `research-game`, and `research-player` still use `slate`/`sky` utilities and remain for the research/remaining-templates migration task.
