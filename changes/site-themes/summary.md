---
slug: site-themes
run: 20261124-154423-site-themes
outcome: done
generated: 2026-11-24T16:11:29Z
---

# Run summary: site-themes

## Outcome

- Status: done
- Branch: site-themes
- Close-out: written
- Close: pending
- Final verdict: agy@gemini-4-neon-low approved (issues: 0)

## Totals

- Tasks: 8 of 8
- Phases: 4
- Checkpoints: 8
- Duration: 24m47s
- Cost: $—
- Fresh input tokens: 1239823
- Cache read tokens: 10123823
- Cache write tokens: 0
- Output tokens: 324884

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-medium | 11688530 | $— |

## Phases

### Phase 1: Theme registry, cookie plumbing, and CSS token foundation

Tasks:

- [x] Add `internal/web/theme.go` (`ThemeID`, `Theme`, allowlist for `paper-turf`, `redzone`, `almanac`, `ThemeCookie`, `ThemeFromRequest`) and `Page.Theme` / `Page.ActiveTheme()` in `internal/web/page.go`; wire `ThemeFromRequest(r)` in `internal/web`, `internal/auth`, `internal/groups`, and `cmd/server/sandbox.go`, with unit tests covering default, valid, and invalid cookies plus `layout_test.go` SSR assertions.
- [x] Define the semantic `@theme` tokens, three `[data-theme="..."]` palettes (`paper-turf`, `redzone`, `almanac`), and shared component/badge classes in `web/tailwind/input.css`; add Google Fonts links, `data-theme`/`color-scheme`/`theme-color` attributes, and the compact top-bar `[data-theme-cycle]` button in `web/templates/layout.html`; implement instant theme cycling and `pc_theme` cookie persistence in `web/static/app.js`; regenerate `web/static/app.css`.

Phase totals: 2 checkpoints, 5m23s, $—, 615694 input / 15119 output / 539236 cache read / 0 cache write tokens

Checkpoints: task-1, task-2 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Add `internal/web/theme.go` (`ThemeID`, `Theme`, allowlist for `paper-turf`, `redzone`, `almanac`, `ThemeCookie`, `ThemeFromRequest`) and `Page.Theme` / `Page.ActiveTheme()` in `internal/web/page.go`; wire `ThemeFromRequest(r)` in `internal/web`, `internal/auth`, `internal/groups`, and `cmd/server/sandbox.go`, with unit tests covering default, valid, and invalid cookies plus `layout_test.go` SSR assertions.

- Added `internal/web/theme.go` with `ThemeCookie`, `ThemeID`, `Theme`, the three-theme allowlist (`paper-turf`, `redzone`, `almanac`), and `ThemeFromRequest`.
- Added `Page.Theme` and `Page.ActiveTheme()` to `internal/web/page.go`, defaulting to `paper-turf` when empty or unrecognized.
- Wired `ThemeFromRequest` into all full-page builders in `internal/web`, `internal/auth`, `internal/groups`, and `cmd/server/sandbox.go`.
- Wired `data-theme`, `color-scheme`, and `theme-color` attributes in `web/templates/layout.html` so SSR reflects the active theme.
- Added cookie/fallback unit tests in `internal/web/theme_test.go` and SSR layout assertions in `internal/web/layout_test.go`.

### Task 2: Define the semantic `@theme` tokens, three `[data-theme="..."]` palettes (`paper-turf`, `redzone`, `almanac`), and shared component/badge classes in `web/tailwind/input.css`; add Google Fonts links, `data-theme`/`color-scheme`/`theme-color` attributes, and the compact top-bar `[data-theme-cycle]` button in `web/templates/layout.html`; implement instant theme cycling and `pc_theme` cookie persistence in `web/static/app.js`; regenerate `web/static/app.css`.

- Defined semantic `@theme` color and font tokens, three `[data-theme="..."]` palettes (`paper-turf`, `redzone`, `almanac`), and shared component/badge classes (`.ui-card`, `.ui-slip-card`, `.ui-slip-divider`, `.ui-next-card`, `.ui-odds`, `.ui-stamp`, `.ui-heads-up`, `.badge-*`, `.grade-*`, `.sev-*`, `.bar-*`, `.trend-*`) in `web/tailwind/input.css`.
- Added Google Fonts `<link>` tags (`Barlow Condensed`, `IBM Plex Mono`, `Space Grotesk`) and the compact top-bar `[data-theme-cycle]` switcher button (icon-only on narrow phones, short label from `sm` up) in `web/templates/layout.html`.
- Implemented instant theme cycling (`paper-turf` -> `redzone` -> `almanac`) updating `data-theme`, `color-scheme`, `theme-color`, button labels, and `pc_theme` cookie persistence in `web/static/app.js`.
- Added `[data-theme-cycle]` assertions in `internal/web/layout_test.go` and regenerated `web/static/app.css`.

### Phase 2: Semantic status, grade, and result roles in Go view builders

Tasks:

- [x] Replace raw Tailwind class strings in `internal/web/slips.go` (`resultClass`), `internal/web/analyzer.go` (`gradeClass`, `severityBadge`), and `internal/web/research.go` (`ResultClass`, `badgeClass`) with semantic CSS classes defined in Phase 1, and update the corresponding assertions in `internal/web/slips_test.go`, `internal/web/analyzer_test.go`, and `internal/web/research_test.go`.

Phase totals: 1 checkpoints, 2m20s, $—, 1226146 input / 25083 output / 1082138 cache read / 0 cache write tokens

Checkpoints: task-3 Review fixes: 0 Integration checks: 0

#### Done

### Task 3: Replace raw Tailwind class strings in `internal/web/slips.go` (`resultClass`), `internal/web/analyzer.go` (`gradeClass`, `severityBadge`), and `internal/web/research.go` (`ResultClass`, `badgeClass`) with semantic CSS classes defined in Phase 1, and update the corresponding assertions in `internal/web/slips_test.go`, `internal/web/analyzer_test.go`, and `internal/web/research_test.go`.

- Replaced raw Tailwind classes in `internal/web/slips.go` (`resultClass`) with semantic `badge-won`, `badge-lost`, `badge-push`, `badge-void`, and `badge-pending` classes.
- Replaced raw Tailwind classes in `internal/web/analyzer.go` (`gradeClass`, `analyzerSeverity`) with semantic `grade-a`..`grade-f`, `grade-neutral`, `sev-danger`, `sev-warn`, and `sev-info` classes.
- Replaced raw Tailwind classes in `internal/web/research.go` (`buildBarChart` `ResultClass`, `newDefRankBadge` `badgeClass`) with semantic `bar-hit`, `bar-miss`, `bar-push`, `bar-neutral`, `badge-won`, `badge-lost`, and `badge-void` classes.
- Updated existing Tailwind class assertions in `internal/web/slips_test.go` and added semantic class assertions in `internal/web/analyzer_test.go` and `internal/web/research_test.go`.

### Phase 3: Migrate shell, slip, and pick-board templates to semantic tokens

Tasks:

- [x] Migrate `web/templates/layout.html`, `home.html`, `week.html`, `avatar.html`, and `media.html` from hardcoded `slate`/`sky` utilities to semantic page-chrome and card tokens, keeping `internal/web/layout_test.go` and `routes_test.go` passing.
- [x] Migrate `web/templates/slip.html`, `pick.html`, and `partials.html` (Next banner, slip ticket card, member legs, mono odds chips, vote pills, analyzer heads-up, payout/wager, pick slate/detail/dock, and modal dialogs) to semantic tokens and component classes, update exact-class assertions in `internal/web/pick_test.go` and `analyzer_test.go`, and regenerate `web/static/app.css`.

Phase totals: 2 checkpoints, 8m6s, $—, 3121914 input / 85166 output / 2799151 cache read / 0 cache write tokens

Checkpoints: task-4, task-5 Review fixes: 0 Integration checks: 0

#### Done

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

### Phase 4: Migrate secondary screens and verify zero raw palette leakage

Tasks:

- [x] Migrate `web/templates/record.html`, `research.html`, `analyzer_guide.html`, and `analyzer_results.html` to semantic tokens (including tabular mono stats, Parlay Terrorist stamp badge, and hit/miss bar classes) and update exact-class assertions in `internal/web/research_test.go`.
- [x] Migrate `web/templates/groups.html`, `profile.html`, `join.html`, `login.html`, `dev-login.html`, and `sandbox.html` to semantic tokens, verify zero leftover raw `slate-*`/`sky-*` classes in `web/templates/` and `internal/web/` (excluding avatar swatches), and regenerate `web/static/app.css`.
- [x] Docs: document the `pc_theme` cookie and semantic theme token convention in `docs/reference/routes.md` (or the relevant UI/reference doc) and run `good-vibes docs lint`.

Phase totals: 3 checkpoints, 8m14s, $—, 5698622 input / 177746 output / 5084631 cache read / 0 cache write tokens

Checkpoints: task-6, task-7, task-8 Review fixes: 0 Integration checks: 0

#### Done

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

## Final gate

- Rounds used: 0; verify failures: 0
- Final verify: passed
- Open issues: none
