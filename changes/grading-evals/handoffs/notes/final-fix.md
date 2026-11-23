## Commit

Implement archive replay, eval CLI, and metric fixes

Complete the grading-evals change by adding the pure raw-JSON archive parsers, the leak-free historical dataset replayer, and the offline eval CLI so rubrics can be evaluated and compared against settled slips without reading overwritten live tables. Also fix missing-odds Lift calculation and use consistent rubric factor labels in aggregate eval reports.
