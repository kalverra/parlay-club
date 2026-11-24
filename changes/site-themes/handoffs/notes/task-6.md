## Done

- Migrated `web/templates/record.html`, `web/templates/research.html`, `web/templates/analyzer_guide.html`, and `web/templates/analyzer_results.html` to semantic theme tokens (`ui-card`, `bg-surface`, `bg-surface-2`, `text-ink`, `text-ink-muted`, `text-ink-subtle`, `text-page-muted`, `border-line`, `border-line-strong`, `bg-accent`, `text-positive-ink`, `text-danger-ink`, `text-warning-ink`), with `font-mono tabular-nums` on stats/records/odds and `.ui-stamp` on the Parlay Terrorist badge.
- Migrated `research-game` and `research-player` in `web/templates/partials.html` to semantic tokens, `font-mono tabular-nums` stat tiles and chart labels, and `border-warning`/`bg-warning-soft`/`text-warning-ink` reference lines.
- Updated exact-class assertions in `internal/web/research_test.go` to match the new semantic `Target:`, `Spread`, `O/U`, and `border-dashed border-warning` markup.
- Verified with `go generate ./...` and `go test -short ./internal/web/...`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

In `web/templates/partials.html`, `group-list` and `game-props` still use `slate`/`sky` utilities and remain for the remaining-templates migration task.

## Commit

Migrate record, research, and analyzer templates to semantic tokens

Replace hardcoded slate, sky, emerald, rose, and amber utility classes on the Record Book, Research, Analyzer Guide, and Grading Results screens with semantic theme tokens so all three site themes render with consistent contrast and typography.
