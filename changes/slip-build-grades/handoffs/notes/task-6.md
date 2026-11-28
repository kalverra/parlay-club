# Task 6

## Done

- Added `--grade-clean`, `--grade-clean-soft`, `--grade-clean-ink` (teal/cyan) and `--grade-solid`, `--grade-solid-soft`, `--grade-solid-ink` (slate/blue) tokens across all four themes (`paper-turf`, `redzone`, `almanac`, `connor-mode`) and exposed them in `@theme` in `web/tailwind/input.css`.
- Remapped `.grade-a` to `--grade-clean*` and `.grade-b` to `--grade-solid*`, keeping `.grade-c` on `--warning*`, `.grade-d`/`.grade-f` on `--danger*`, and `.grade-neutral` on `--line-strong`/`--surface-2`/`--ink-muted` so no grade badge class maps to `--positive`.
- Regenerated `web/static/app.css` via `go generate ./...`.
- Added `TestBuildGradeThemeTokensAndClasses` in `web/embed_test.go` to assert non-empty build-grade tokens in all four themes and verify that no `.grade-*` rule in `web/tailwind/input.css` or `web/static/app.css` references `--positive`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `--color-grade-clean*` and `--color-grade-solid*` are registered in `@theme` in `web/tailwind/input.css` so Task 7 templates and view models can also use `bg-grade-clean*`, `text-grade-clean-ink`, `bg-grade-solid*`, and `text-grade-solid-ink` utilities when replacing any remaining `--positive` build-grade accents.

## Commit

Replace green grade tokens with cool build tokens

High build grades previously shared the green positive palette with settled wins, implying a guaranteed winning bet rather than clean slip structure. Separating build grades onto cool teal and slate-blue tokens across every theme reserves positive green strictly for actual settled outcomes.
