## Done

- Migrated `web/templates/groups.html`, `web/templates/profile.html`, `web/templates/join.html`, `web/templates/login.html`, `web/templates/dev-login.html`, and `web/templates/sandbox.html` to semantic theme tokens (`ui-card`, `bg-surface`, `bg-surface-2`, `text-ink`, `text-ink-muted`, `text-ink-subtle`, `text-page-ink`, `text-page-muted`, `border-line`, `border-line-strong`, `bg-accent`, `text-accent-ink`, `border-accent`, `bg-accent-soft`, `border-danger`, `bg-danger-soft`, `text-danger-ink`, `border-warning`, `bg-warning-soft`, `text-warning-ink`, `border-positive`, `bg-positive-soft`, `text-positive-ink`).
- Migrated the remaining `group-list` and `game-props` blocks in `web/templates/partials.html` to semantic tokens and `font-mono tabular-nums` prop prices.
- Verified with `rg` that zero raw `slate-*`/`sky-*` classes remain in `web/templates/` or `internal/web/` outside `bg-sky-500` avatar swatch assertions in `internal/web/layout_test.go` and `internal/web/routes_test.go`.
- Regenerated `web/static/app.css` via `go generate ./...` and verified `go test -short ./...` passes.

## Deviations

None.

## Open issues

None.

## Next-phase notes

All templates in `web/templates/` and handlers in `internal/web/` are now free of raw `slate-*`/`sky-*` palette classes (excluding avatar swatches).

## Commit

Migrate secondary templates and partials to semantic tokens

Replace the remaining hardcoded slate and sky utility classes across secondary screens and shared partials so every page renders legibly under all three themes. Regenerate the compiled stylesheet and verify no raw palette classes remain outside avatar swatches.
