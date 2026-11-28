## Done

- Updated `docs/concepts/analyzer.md` under "What the banner shows" to document the 3-column **Overall / Hit chance / Payout value** summary strip inside the `grade-breakdown` popover, including `CalibratedProb` formatting (`~N%`, `<1%`, `>99%`), `Price & Vig` payout labels (`Strong value`, `Fair value`, `Slight juice`, `Steep juice`) without `+EV %` copy, and `EngineV1` fallback behavior.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Phase 2 and Phase 3 will add grouping modes and pre-pick grade pills on the pick board; `docs/concepts/analyzer.md` and `docs/concepts/slips.md` may need follow-up updates when those surfaces land.

## Commit

Document grade breakdown summary strip

Record the Overall, Hit chance, and Payout value summary strip in the analyzer concept doc so the living documentation matches the updated grade breakdown popover and its EngineV1 fallback.
