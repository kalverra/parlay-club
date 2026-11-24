## Commit

Use eval.CompareRubrics in cmd/eval compare mode

Phase 2 introduced eval.CompareRubrics as the dataset-driven seam for comparing two rubrics so callers cannot accidentally pair reports from different datasets. Wiring cmd/eval through this entry point removes the redundant Evaluate calls and ensures the new seam has a non-test caller matching the design.
