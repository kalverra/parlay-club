## Done

- Updated `docs/concepts/analyzer.md` (`### Versioned rubrics and offline evals`) to document full-slip replay (retaining unsettled legs on early-busted graded slips for `ScoreSlipWithRubric` while counting only settled legs in leg metrics), `(SlipID, LegID)` migration matching / `CompareRubrics`, and slip-level factor attribution (`SlipFactors`, `GradeRubricFor`) in `cmd/eval`.
- Updated `docs/concepts/analyzer.md` (`## In-app grading guide and results`) to document the secondary **Slip Factor Attribution** section on `GET /analyzer/results`.
- Ran `good-vibes docs lint` and verified `docs/concepts/analyzer.md` passes with 0 errors.

## Deviations

None.

## Open issues

None.

## Next-phase notes

None.

## Commit

Document full-slip replay and slip factor attribution

Update the analyzer concept doc so full pre-kickoff slip replay, identity-keyed rubric comparison, and slip-level factor attribution in cmd/eval and GET /analyzer/results match the implemented behavior.
