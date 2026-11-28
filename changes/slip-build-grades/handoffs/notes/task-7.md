# Task 7

## Done

- Extended `GradeView` and view builders in `internal/web/analyzer.go` with `DisplayGrade`, 2-word `Descriptor`, `GroundRule`, provenance-tagged probabilities (`VegasImplied`, `FairNoVig`, `RawModel`, `HitReality`), `HoldText`, `CorrTaxText`, `RealismLabel`/`RealismPct`, capped `SignalPills` (max 3), `ContextRadar` (max 2), `CleanupNudge`, live-slip `WhyPopovers`, and the 4-bullet `ModelCard`, while reframing `PayoutLabel` and `pillarToneClasses` off `--positive`.
- Added human-first `Vegas Implied` `1 in X` formatting across `internal/web/slips.go`, `internal/web/pick.go`, and `internal/web/research.go`, and updated `gameGradeBadgeClass` in `internal/web/pick.go` to use `.grade-a`–`.grade-f`.
- Added `build-grade-l1` and updated `grade-breakdown` in `web/templates/partials.html` (with drawer open/focus/close handlers in `web/static/app.js`) to render Layer 1 (Build Grade + descriptor, human-first `Vegas Implied` `1 in X` copy, max 3 clickable Signal Pills), Layer 2 (`.leg-sheet` drawer with Ground-Rule Banner, separated Market Reality vs. Slip Diagnostics, Hit-Rate Realism bar, `Already in the line` pills, and optional cleanup nudge), Layer 3 (inline live-slip "Why does this matter?" popovers + 4-bullet Model Card), and reframed highlights (`Cleanest builds on the board` / `Cleanest builds in this game`).
- Updated `web/templates/research_bet.html` and `web/templates/analyzer_guide.html` with 3-layer disclosure, provenance tags, and the 4-bullet Model Card, regenerated `web/static/app.css`, and updated/expanded tests in `internal/web/analyzer_test.go`, `internal/web/slips_test.go`, `internal/web/pick_test.go`, and `internal/web/research_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Phase 4 can wire `ClassifyProcessOutcome` and `SuiteReport.SlipGrades` into `/slips/{id}`, `/record`, and `/record/members/{id}` using `.grade-*` for pre-kickoff Build Grades and reserving `--positive` strictly for settled wins and `Clean Hit` outcome stamps.

## Commit

Add 3-layer build-grade disclosure across web UI

Replace green +EV-style grade popovers with cool/neutral Build Grade chips, human-first Vegas Implied 1-in-X odds copy, clickable Signal Pills, a phone-first Market Reality vs. Slip Diagnostics breakdown drawer, and inline live-slip Why popovers with a 4-bullet Model Card. This aligns the slip hub, pick tray, highlights, bet research, and grading guide around ticket construction and market reality.
