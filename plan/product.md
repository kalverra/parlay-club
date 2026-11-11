# Product

Slips, weeks, members, legs, manual grading, the pick board, votes, and the editing and locking rules live in
[Slips](../docs/concepts/slips.md). The group list, the active group, and the top bar live in
[Groups](../docs/concepts/groups.md).

## Grading

Leg results: `PENDING`, `NEEDS_GRADING`, `WON`, `LOST`, `PUSH`, `VOID`.

Auto-grading rules per market (`grading-record-book`), from the ESPN final box score:

| Market       | Stat compared with the line                                                     |
| ------------ | ------------------------------------------------------------------------------- |
| `SPREAD`     | team score + line − opponent score: `> 0` WON, `= 0` PUSH, `< 0` LOST           |
| `ML`         | team wins → WON; loses → LOST; tie → PUSH                                       |
| `TOTAL`      | combined score vs line                                                          |
| `TEAM_TOTAL` | team score vs line                                                              |
| `PASS_YDS`   | passing yards                                                                   |
| `PASS_TDS`   | passing TDs                                                                     |
| `RUSH_YDS`   | rushing yards                                                                   |
| `RUSH_ATT`   | rushing attempts                                                                |
| `REC_YDS`    | receiving yards                                                                 |
| `RECEPTIONS` | receptions                                                                      |
| `ANYTIME_TD` | rushing TDs + receiving TDs + return TDs ≥ 1 → WON (passing TDs don't count)    |
| `KICKER_PTS` | FG made × 3 + XP made                                                           |
| `OTHER`      | `NEEDS_GRADING`                                                                 |

- For OVER/UNDER: stat above the line wins OVER, below wins UNDER, equal is `PUSH`.
- A player prop whose player is absent from the box score is `VOID`.
- A prop with no `player_id` is `NEEDS_GRADING`.

The grader writes leg results the normal way, so the slip result and status recompute as described in
[Slips](../docs/concepts/slips.md).

## Record Book

Scoped to one NFL season (selector at the top). Only graded slips count.

- **Group:** record (W-L-P), units P/L, longest win streak, longest loss streak (by week, then slip creation order).
- **Leaderboard**, one row per user with at least one graded leg:
  - hit % = WON / (WON + LOST) legs; users with fewer than 6 decided legs show "–" and sort last;
  - average leg odds (mean decimal of their legs, shown as American);
  - **Parlay Terrorist** count: slips with result `LOST` where this user owns the only `LOST` leg. The leader gets the
    badge.
- **Dollar P/L**, per user, only from slips where they logged a wager: `WON → stake × (D − 1) × (1 + boost/100)` using
  their logged odds; `LOST → −stake`; `PUSH → 0`.

## Screens

Phone first: one column and a bottom bar with Groups, This week, and Record Book. The week runs from the group list
(`/`) to a group's week (`/groups/{id}/week`), then the hub (`/slips/{id}`), and the pick board
(`/slips/{id}/pick`). Those built screens, their steps, empty states, and layouts are documented in
[Groups](../docs/concepts/groups.md), [Slips](../docs/concepts/slips.md), and
[Routes](../docs/reference/routes.md). The hub stays one column at every width; the pick board puts its tray in a
second column at `lg`.

Still to build:

- **Record Book (`grading-record-book`):** the third bottom-bar destination, one column with the season selector at the
  top, then the group record, streaks, leaderboard, Parlay Terrorist badge, and dollar P/L described above.
