# Task 5: Docs update

## Done

- Updated `docs/concepts/export.md` to document link-preserving `Store.UpsertOdds` behavior, FanDuel `42.*` jurisdiction prefix preference and mixed-prefix combiner fallback, and the 5-case phone verification matrix.
- Updated `docs/concepts/data-refresh.md` to document lock-time game-line and prop export enrichment (`KickSlipExportEnrich` and `odds:lines:enrich`), the `40`-credit `TheOddsAPIEnrichFloor`, and FanDuel `42.*` link preference.
- Updated `docs/reference/data-sources.md` to document `40`-credit `TheOddsAPIEnrichFloor`, FanDuel `42.*` jurisdiction prefix handling, and link-preserving upserts.
- Verified with `good-vibes docs lint` (0 errors) and refreshed `docs/llms.txt` and `docs/decisions/llms.txt` via `good-vibes docs index`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

Run the 5-case phone verification matrix on iOS and Android for both DraftKings and FanDuel after deploying.

## Commit

Document export link reliability and enrich floor

Living docs now reflect link-preserving upserts, FanDuel jurisdiction prefix handling, the lowered 40-credit enrichment floor, and lock-time game-line enrichment so operators and future changes have an accurate contract. It also records the 5-case phone verification matrix for post-deploy checks.
