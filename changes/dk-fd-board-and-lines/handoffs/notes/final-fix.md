## Commit

Deduplicate slip-hub board loads and update docs

Compute `slips.NextStep` before loading week odds and rosters in `slipPageData` so `board.BuildDualBook` runs once per open slip page render instead of separately for leg split-line badges and highlights. Update `docs/reference/routes.md` and `docs/concepts/slips.md` to match `board.BuildDualBook` and its dual-book pricing rule.
