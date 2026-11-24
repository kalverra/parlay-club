# Phase 3: Phase 3: Migrate shell, slip, and pick-board templates to semantic tokens

## Done

### Task 4: Migrate `web/templates/layout.html`, `home.html`, `week.html`, `avatar.html`, and `media.html` from hardcoded `slate`/`sky` utilities to semantic page-chrome and card tokens, keeping `internal/web/layout_test.go` and `routes_test.go` passing.

- Migrated `web/templates/layout.html` to semantic page (`bg-page`, `text-page-ink`), chrome (`bg-chrome`, `text-chrome-ink`, `text-chrome-muted`, `border-chrome-border`), and surface tokens (`bg-surface`, `bg-surface-2`, `text-ink`, `text-ink-muted`, `border-line`, `bg-accent`, `text-accent-ink`).
- Migrated `web/templates/home.html` subtitle to `text-page-muted` while preserving `<h1>` markup expected by `layout_test.go` and `routes_test.go`.
- Migrated `web/templates/week.html` slip cards, schedule cards, form controls, status badges, and buttons to `.ui-card` and semantic surface, accent, positive, and danger tokens.
- Updated `web/templates/avatar.html` initials fallback text from `text-slate-950` to `text-zinc-950` so initials stay legible on per-user avatar swatches across all themes.
- Migrated `web/templates/media.html` team logos and player portraits to `bg-surface-2`, `text-ink`, `text-ink-muted`, and `ring-line`.

### Task 5: Migrate `web/templates/slip.html`, `pick.html`, and `partials.html` (Next banner, slip ticket card, member legs, mono odds chips, vote pills, analyzer heads-up, payout/wager, pick slate/detail/dock, and modal dialogs) to semantic tokens and component classes, update exact-class assertions in `internal/web/pick_test.go` and `analyzer_test.go`, and regenerate `web/static/app.css`.

- Migrated `web/templates/slip.html` and `web/templates/pick.html` (slip creation form, empty state, pick board header, and modal sheet containers) to `.ui-card` and semantic page, chrome, surface, and accent tokens.
- Migrated `web/templates/partials.html` slip, dialog, highlights, and pick-board templates (`slip-panel`, `slip-export`, `slip-activity`, `slip-payout`, `slip-wager`, `leg-sheet`, `leg-added`, `leg-form`, `pick-option`, `pick-body`, `pick-slate`, `pick-detail`, `pick-line-group`, `pick-detail-empty`, `pick-tray`, `pick-dock`, `pick-swap`, `bug-report-*`, and `highlights-strip`) to semantic tokens and component classes (`.ui-next-card`, `.ui-slip-card`, `.ui-card`, `.ui-odds`).
- Wrapped `pick-detail` game lines in a `.ui-card` container so `pick-line-group` labels use `text-ink-muted` consistently across both game lines and player prop cards.
- Updated exact-class assertions in `internal/web/pick_test.go` and `internal/web/analyzer_test.go` and regenerated `web/static/app.css` via `go generate ./...`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 4

The remaining templates (`slip.html`, `pick.html`, `record.html`, `research.html`, `groups.html`, `login.html`, etc.) can now rely on `layout.html` providing `bg-page text-page-ink` on `<body>` and semantic chrome/surface tokens throughout the shell.

### Task 5

In `web/templates/partials.html`, `group-list`, `game-props`, `research-game`, and `research-player` still use `slate`/`sky` utilities and remain for the research/remaining-templates migration task.

## Files touched

- `internal/web/analyzer_test.go`
- `internal/web/pick_test.go`
- `web/static/app.css`
- `web/templates/avatar.html`
- `web/templates/home.html`
- `web/templates/layout.html`
- `web/templates/media.html`
- `web/templates/partials.html`
- `web/templates/pick.html`
- `web/templates/slip.html`
- `web/templates/week.html`

## Run log

- Mode: 
- Checkpoints: task-4, task-5
- Verify: verify passed: generate 300ms, lint 3s, test 4.7s; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
