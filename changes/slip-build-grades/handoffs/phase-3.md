# Phase 3: Phase 3: Semantic grade colors and 3-layer progressive disclosure UI

## Done

### Task 6: Replace green `.grade-a` tokens with cool/neutral build-grade tokens across all 4 themes in `web/tailwind/input.css` (reserving `--positive` strictly for settled wins), run `go generate ./...`, and verify no grade badge class maps to `--positive`.

- Added `--grade-clean`, `--grade-clean-soft`, `--grade-clean-ink` (teal/cyan) and `--grade-solid`, `--grade-solid-soft`, `--grade-solid-ink` (slate/blue) tokens across all four themes (`paper-turf`, `redzone`, `almanac`, `connor-mode`) and exposed them in `@theme` in `web/tailwind/input.css`.
- Remapped `.grade-a` to `--grade-clean*` and `.grade-b` to `--grade-solid*`, keeping `.grade-c` on `--warning*`, `.grade-d`/`.grade-f` on `--danger*`, and `.grade-neutral` on `--line-strong`/`--surface-2`/`--ink-muted` so no grade badge class maps to `--positive`.
- Regenerated `web/static/app.css` via `go generate ./...`.
- Added `TestBuildGradeThemeTokensAndClasses` in `web/embed_test.go` to assert non-empty build-grade tokens in all four themes and verify that no `.grade-*` rule in `web/tailwind/input.css` or `web/static/app.css` references `--positive`.

### Task 7: Update `internal/web/analyzer.go` and templates (`partials.html`, `research_bet.html`, `analyzer_guide.html`) to render Layer 1 (Build Grade + 2-word descriptor, human-first `Vegas Implied` `1 in X` copy, max 3 clickable Signal Pills), Layer 2 (breakdown drawer with Ground-Rule Banner, separated Market Reality vs. Slip Diagnostics with provenance tags, Context Radar `Already in the line` pills, and optional 1-line cleanup nudge), and Layer 3 (inline live-slip "Why does this matter?" popovers + 4-bullet Model Card), plus reframed highlights strip, with full web handler and template tests.

- Extended `GradeView` and view builders in `internal/web/analyzer.go` with `DisplayGrade`, 2-word `Descriptor`, `GroundRule`, provenance-tagged probabilities (`VegasImplied`, `FairNoVig`, `RawModel`, `HitReality`), `HoldText`, `CorrTaxText`, `RealismLabel`/`RealismPct`, capped `SignalPills` (max 3), `ContextRadar` (max 2), `CleanupNudge`, live-slip `WhyPopovers`, and the 4-bullet `ModelCard`, while reframing `PayoutLabel` and `pillarToneClasses` off `--positive`.
- Added human-first `Vegas Implied` `1 in X` formatting across `internal/web/slips.go`, `internal/web/pick.go`, and `internal/web/research.go`, and updated `gameGradeBadgeClass` in `internal/web/pick.go` to use `.grade-a`–`.grade-f`.
- Added `build-grade-l1` and updated `grade-breakdown` in `web/templates/partials.html` (with drawer open/focus/close handlers in `web/static/app.js`) to render Layer 1 (Build Grade + descriptor, human-first `Vegas Implied` `1 in X` copy, max 3 clickable Signal Pills), Layer 2 (`.leg-sheet` drawer with Ground-Rule Banner, separated Market Reality vs. Slip Diagnostics, Hit-Rate Realism bar, `Already in the line` pills, and optional cleanup nudge), Layer 3 (inline live-slip "Why does this matter?" popovers + 4-bullet Model Card), and reframed highlights (`Cleanest builds on the board` / `Cleanest builds in this game`).
- Updated `web/templates/research_bet.html` and `web/templates/analyzer_guide.html` with 3-layer disclosure, provenance tags, and the 4-bullet Model Card, regenerated `web/static/app.css`, and updated/expanded tests in `internal/web/analyzer_test.go`, `internal/web/slips_test.go`, `internal/web/pick_test.go`, and `internal/web/research_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 6

- `--color-grade-clean*` and `--color-grade-solid*` are registered in `@theme` in `web/tailwind/input.css` so Task 7 templates and view models can also use `bg-grade-clean*`, `text-grade-clean-ink`, `bg-grade-solid*`, and `text-grade-solid-ink` utilities when replacing any remaining `--positive` build-grade accents.

### Task 7

- Phase 4 can wire `ClassifyProcessOutcome` and `SuiteReport.SlipGrades` into `/slips/{id}`, `/record`, and `/record/members/{id}` using `.grade-*` for pre-kickoff Build Grades and reserving `--positive` strictly for settled wins and `Clean Hit` outcome stamps.

## Files touched

- `internal/web/analyzer.go`
- `internal/web/analyzer_test.go`
- `internal/web/pick.go`
- `internal/web/pick_test.go`
- `internal/web/research.go`
- `internal/web/research_test.go`
- `internal/web/slips.go`
- `internal/web/slips_test.go`
- `web/embed_test.go`
- `web/static/app.css`
- `web/static/app.js`
- `web/tailwind/input.css`
- `web/templates/analyzer_guide.html`
- `web/templates/partials.html`
- `web/templates/research_bet.html`

## Run log

- Mode: 
- Checkpoints: task-6, task-7
- Verify: verify passed: generate 600ms, lint 5.1s, test 9.1s; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
