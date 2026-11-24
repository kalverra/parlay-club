## Commit

Move sharp benchmark deltas into Rubric

Sharp line and price scoring previously bypassed the versioned Rubric seam by hardcoding deltas in the grading engine and guide builder. Moving those eight weights onto Rubric keeps scoring and guide documentation aligned with every other leg factor and lets alternate rubrics override sharp weights cleanly.
