## Commit
Wire remaining callers to shared analyzer seams

Evaluating non-active rubrics in internal/eval previously leaked global ActiveRubric labels, and the wind warning rule duplicated outdoor wind checks already extracted into evalWindHazard. Routing both callers through GradeRubricFor and evalWindHazard closes the bypassed seams without changing scoring or warning behavior.
