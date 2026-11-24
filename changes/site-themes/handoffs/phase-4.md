# Phase 4: Phase 4: Migrate secondary screens and verify zero raw palette leakage

## Done

### Task 6: Migrate `web/templates/record.html`, `research.html`, `analyzer_guide.html`, and `analyzer_results.html` to semantic tokens (including tabular mono stats, Parlay Terrorist stamp badge, and hit/miss bar classes) and update exact-class assertions in `internal/web/research_test.go`.

- Migrated `web/templates/record.html`, `web/templates/research.html`, `web/templates/analyzer_guide.html`, and `web/templates/analyzer_results.html` to semantic theme tokens (`ui-card`, `bg-surface`, `bg-surface-2`, `text-ink`, `text-ink-muted`, `text-ink-subtle`, `text-page-muted`, `border-line`, `border-line-strong`, `bg-accent`, `text-positive-ink`, `text-danger-ink`, `text-warning-ink`), with `font-mono tabular-nums` on stats/records/odds and `.ui-stamp` on the Parlay Terrorist badge.
- Migrated `research-game` and `research-player` in `web/templates/partials.html` to semantic tokens, `font-mono tabular-nums` stat tiles and chart labels, and `border-warning`/`bg-warning-soft`/`text-warning-ink` reference lines.
- Updated exact-class assertions in `internal/web/research_test.go` to match the new semantic `Target:`, `Spread`, `O/U`, and `border-dashed border-warning` markup.
- Verified with `go generate ./...` and `go test -short ./internal/web/...`.

### Task 7: Migrate `web/templates/groups.html`, `profile.html`, `join.html`, `login.html`, `dev-login.html`, and `sandbox.html` to semantic tokens, verify zero leftover raw `slate-*`/`sky-*` classes in `web/templates/` and `internal/web/` (excluding avatar swatches), and regenerate `web/static/app.css`.

- Migrated `web/templates/groups.html`, `web/templates/profile.html`, `web/templates/join.html`, `web/templates/login.html`, `web/templates/dev-login.html`, and `web/templates/sandbox.html` to semantic theme tokens (`ui-card`, `bg-surface`, `bg-surface-2`, `text-ink`, `text-ink-muted`, `text-ink-subtle`, `text-page-ink`, `text-page-muted`, `border-line`, `border-line-strong`, `bg-accent`, `text-accent-ink`, `border-accent`, `bg-accent-soft`, `border-danger`, `bg-danger-soft`, `text-danger-ink`, `border-warning`, `bg-warning-soft`, `text-warning-ink`, `border-positive`, `bg-positive-soft`, `text-positive-ink`).
- Migrated the remaining `group-list` and `game-props` blocks in `web/templates/partials.html` to semantic tokens and `font-mono tabular-nums` prop prices.
- Verified with `rg` that zero raw `slate-*`/`sky-*` classes remain in `web/templates/` or `internal/web/` outside `bg-sky-500` avatar swatch assertions in `internal/web/layout_test.go` and `internal/web/routes_test.go`.
- Regenerated `web/static/app.css` via `go generate ./...` and verified `go test -short ./...` passes.

### Task 8: Docs: document the `pc_theme` cookie and semantic theme token convention in `docs/reference/routes.md` (or the relevant UI/reference doc) and run `good-vibes docs lint`.

- Documented the `pc_theme` cookie allowlist (`paper-turf`, `redzone`, `almanac`), SSR `data-theme` / meta tag rendering, and client-side `[data-theme-cycle]` switching in `docs/reference/routes.md`.
- Documented the semantic theme token convention (`web/tailwind/input.css` page/chrome, surface, accent/status, and `.ui-*` / semantic badge classes) and the no-raw-palette rule in `docs/reference/routes.md`.
- Added `themes` to `docs/reference/routes.md` front-matter tags and verified `good-vibes docs lint` reports 0 errors and 0 warnings for `docs/reference/routes.md`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 6

In `web/templates/partials.html`, `group-list` and `game-props` still use `slate`/`sky` utilities and remain for the remaining-templates migration task.

### Task 7

All templates in `web/templates/` and handlers in `internal/web/` are now free of raw `slate-*`/`sky-*` palette classes (excluding avatar swatches).

### Task 8

All Phase 4 tasks are complete.

## Files touched

- `docs/reference/routes.md`
- `internal/web/research_test.go`
- `web/static/app.css`
- `web/templates/analyzer_guide.html`
- `web/templates/analyzer_results.html`
- `web/templates/dev-login.html`
- `web/templates/groups.html`
- `web/templates/join.html`
- `web/templates/login.html`
- `web/templates/partials.html`
- `web/templates/profile.html`
- `web/templates/record.html`
- `web/templates/research.html`
- `web/templates/sandbox.html`

## Run log

- Mode: 
- Checkpoints: task-6, task-7, task-8
- Verify: verify passed: test 300ms; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
