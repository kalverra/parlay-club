# Task 4

## Done

- Migrated `web/templates/layout.html` to semantic page (`bg-page`, `text-page-ink`), chrome (`bg-chrome`, `text-chrome-ink`, `text-chrome-muted`, `border-chrome-border`), and surface tokens (`bg-surface`, `bg-surface-2`, `text-ink`, `text-ink-muted`, `border-line`, `bg-accent`, `text-accent-ink`).
- Migrated `web/templates/home.html` subtitle to `text-page-muted` while preserving `<h1>` markup expected by `layout_test.go` and `routes_test.go`.
- Migrated `web/templates/week.html` slip cards, schedule cards, form controls, status badges, and buttons to `.ui-card` and semantic surface, accent, positive, and danger tokens.
- Updated `web/templates/avatar.html` initials fallback text from `text-slate-950` to `text-zinc-950` so initials stay legible on per-user avatar swatches across all themes.
- Migrated `web/templates/media.html` team logos and player portraits to `bg-surface-2`, `text-ink`, `text-ink-muted`, and `ring-line`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

The remaining templates (`slip.html`, `pick.html`, `record.html`, `research.html`, `groups.html`, `login.html`, etc.) can now rely on `layout.html` providing `bg-page text-page-ink` on `<body>` and semantic chrome/surface tokens throughout the shell.

## Commit

Migrate shell and week templates to semantic tokens

Replace hardcoded slate and sky Tailwind utilities in the shared layout, home, week, avatar, and media templates with semantic page-chrome, surface, and accent tokens so all three themes render with consistent contrast.
