## Done

- Normalized `prop.PlayerName` via `normalizePropSubject` in `propKeyOf` (`internal/ingest/odds/props.go`) so `odds.MergeProps` matches player names across punctuation, casing, and generational-suffix differences.
- Updated the `normalizePropSubject` doc comment to reflect its use in both quote grouping and cross-provider prop merging.
- Added `TestMergePropsMatchesNormalizedPlayerNames` in `internal/ingest/odds/props_test.go` covering generational suffixes (`III`, `Jr.`), initials with periods (`D.J.` vs `DJ`), and apostrophe/case variants.

## Deviations

None.

## Open issues

None.

## Next-phase notes

`odds.MergeProps` now merges variant player names into the primary row's `PlayerName` without duplicating selections, ready for the enrichment-only refresh path.

## Commit

Normalize player names in odds.MergeProps

Parlay API and The Odds API format player names differently with punctuation and generational suffixes. Normalizing player names in propKeyOf prevents duplicate prop rows and ensures cross-provider deep-link enrichment attaches to the existing player row.
