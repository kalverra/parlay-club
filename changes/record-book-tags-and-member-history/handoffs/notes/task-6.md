## Done

- Updated `docs/concepts/record-book.md`: removed stale `sources` and `last_verified` front-matter fields, added `## Overview` with a Mermaid flow diagram, and documented `ImpliedLift`, `VotesForGroupSeason`, fail-soft `eval.Cache` leg grades, the 14-tag catalog with thresholds and deterministic ordering, `#tag-explain-dialog` clickable tag explanations, and `GET /record/members/{id}?season=`.
- Updated `docs/reference/routes.md`: added `GET /record/members/{id}?season=` to the Record Book route table, documented its `400`/`404` scoping and rendered sections, updated `GET /record` notes for votes, cached grades, tag chips, and member links, and bumped `budget.max_lines` from 620 to 650.
- Ran `good-vibes docs lint` (0 errors; 0 warnings on both updated docs) and `good-vibes docs index` (updating `docs/llms.txt`).

## Deviations

None.

## Open issues

None.

## Next-phase notes

None.

## Commit

Document Record Book tags and member history

Update the Record Book concept doc and routes reference so the living docs match the 14-tag catalog, clickable tag explanation dialog, and per-member bet history drill-down.
