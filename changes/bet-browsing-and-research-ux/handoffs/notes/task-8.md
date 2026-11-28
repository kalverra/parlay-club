## Done

- Updated `docs/concepts/slips.md` to document collapsible pick-board `<details>` sections (collapsed by default, auto-expanded on search, preserved via `app.js`), `SortPopular` (`popular`) as the default sort, compact paired O/U controls, Over/Under slip and tray badges, visual-first `/research/bet` layout, universal `Research ->` and `+ Add` actions, and `GET /bets/add-sheet`.
- Updated `docs/reference/routes.md` to document `sort=popular` as the default detail sort, collapsible sections and compact O/U controls on `GET /slips/{id}/pick/detail`, Over/Under leg badges and `Research ->` / `+ Add` links on slip legs, the visual-first `/research/bet` hierarchy, and the `GET /bets/add-sheet` route contract.
- Verified both docs stay within their line and code-span budgets and ran `good-vibes docs lint` (`0 error(s), 14 warning(s)`, none in the updated files).

## Deviations

None.

## Open issues

None.

## Next-phase notes

All Phase 4 tasks are complete and living documentation matches the implemented bet-browsing, O/U clarity, visual-first research, and eligible-slip chooser behavior.

## Commit

Update slip and route docs for bet UX changes

Keep the living concept and reference documentation aligned with the
collapsed pick-board groups, popularity-first sorting, compact O/U controls
and slip badges, visual-first bet research, and GET /bets/add-sheet.
