## Done

- Updated `docs/concepts/slips.md` (579 lines, within `max_lines: 585` and `max_spans_per_100_lines: 105`) to document the `All` + category chip bar, **Group by** switcher (`?group=` with Category default, Player, Grade), Grade sort, sibling Grade A–F popovers backed by `board.ScoreRows`, and `from=detail` OOB `#pick-detail` swaps in Grade mode.
- Updated `docs/reference/routes.md` (617 lines) to document `?group=` in the pick-board query contract, optional `?cat=` defaulting to `All`, `sort=grade`, pre-pick `Grade A–F` popovers (including at-limit `pickDetailData` and `pickPairRow`), `#pick-group`/`#pick-cat`/`#pick-sort` OOB sync, and `from=detail` on `POST /slips/{id}/legs`, `PUT /legs/{id}`, `DELETE /legs/{id}`, and `GET /slips/{id}/pick/swap`.
- Verified `good-vibes docs lint` passes with `0 error(s)`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

None.

## Commit

Document pick-board grouping, pre-pick grades, and detail swaps

Update the slips concept doc and routes reference to match the grouped pick-board UI, pre-pick grade popovers, and from=detail HTMX swap behavior.
