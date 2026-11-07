# Product

## Slips

- A **week** is an ESPN NFL week: season, season type (regular or postseason), and week number.
- A **slip** is one shared parlay inside a week. A week usually has one slip but can have more.
- A slip has **members**: a subset of users, guests included.
- A slip has `legs_per_member`, default 2, range 1–5.
- The home page (`/`) opens the newest slip of the current week. With none, it shows the create-slip form.
- Create-slip form: week (defaults to current; past weeks allowed), name (default `Week 9`, then `Week 9 #2`), legs per
  member (default: the previous slip's value), members (default: the previous slip's members).
- A user can belong to several slips in a week. The Slip tab has a switcher when the week has more than one slip.
- Everyone places the bet at their own book with their own stake. The app never handles money.

## Legs

A leg has: owner, optional game, market, team, optional player, side, optional line, American odds, and a display
`selection` string generated from the fields.

| Market       | Fields                         | Selection example                   |
| ------------ | ------------------------------ | ----------------------------------- |
| `SPREAD`     | team, line                     | `BUF -3.5`                          |
| `ML`         | team                           | `BUF ML`                            |
| `TOTAL`      | side (OVER/UNDER), line        | `BUF@MIA Over 47.5`                 |
| `TEAM_TOTAL` | team, side, line               | `BUF Team Total Over 27.5`          |
| `PASS_YDS`   | player, team, side, line       | `Josh Allen Over 245.5 Pass Yds`    |
| `PASS_TDS`   | player, team, side, line       | `Josh Allen Over 1.5 Pass TDs`      |
| `RUSH_YDS`   | player, team, side, line       | `James Cook Over 64.5 Rush Yds`     |
| `RUSH_ATT`   | player, team, side, line       | `James Cook Over 14.5 Rush Att`     |
| `REC_YDS`    | player, team, side, line       | `Khalil Shakir Over 52.5 Rec Yds`   |
| `RECEPTIONS` | player, team, side, line       | `Khalil Shakir Over 4.5 Receptions` |
| `ANYTIME_TD` | player, team (side `YES`)      | `James Cook Anytime TD`             |
| `KICKER_PTS` | player, team, side, line       | `Tyler Bass Over 7.5 Kicking Pts`   |
| `OTHER`      | free-text selection, team opt. | whatever the user typed             |

- Team is the ESPN abbreviation of the team the leg is about. For `TOTAL`, team is empty.
- Odds default to -110 in the form; quick-add pills prefill the price (see [Odds math](../docs/reference/odds-math.md)).
- A leg needs a game, except on a slip for a past week, where game is optional and `OTHER` is the usual market.
- `OTHER` legs skip the analyzer and grade by hand.

## Editing and locking

These rules live in `internal/slips` and are enforced on the server. `now` comes from an injected clock.

1. Only slip members can add legs to it. Members can edit or delete only their own legs.
2. A member can't have more than `legs_per_member` legs.
3. A leg is **kickoff-locked** once `now ≥ game.kickoff_utc`. A kickoff-locked leg can't be edited or deleted, and no
   leg can be added for a game that has kicked off.
4. **Lock in** sets the member's `locked_at`. It requires exactly `legs_per_member` legs. A locked-in member can't add,
   edit, or delete legs.
5. The slip's status becomes `LOCKED` when every member is locked in.
6. **Unlock** is available to any member while the slip is `LOCKED` or any member is locked in. It clears every member's
   `locked_at`, sets status `OPEN`, and writes "unlocked by {name}" to the activity feed. Kickoff locks still apply.
7. Members and `legs_per_member` can change only while the slip is `OPEN`. A member with legs can't be removed until
   their legs are deleted.
8. Slips for past weeks skip rule 3, so history can be entered.
9. Every mutation writes one activity entry (e.g. "Adam added BUF -3.5", "Sam locked in") and publishes `slip-updated`
   for that slip.

## Grading

Leg results: `PENDING`, `NEEDS_GRADING`, `WON`, `LOST`, `PUSH`, `VOID`.

- Until `grading-record-book`, any member sets any leg's result by hand. After `grading-record-book`, the grader sets
  results and any member can override them. `graded_by` records `auto` or the user's name.
- `PUSH` and `VOID` legs drop out of the slip's odds and count as neither a hit nor a miss.
- **Slip result**, recomputed on every leg result change:
  - any leg `LOST` → `LOST` (a slip can bust before all games end);
  - else, any leg `PENDING` or `NEEDS_GRADING` → `PENDING`;
  - else, every leg `PUSH` or `VOID` → `PUSH`;
  - else → `WON`.
- The slip's status becomes `GRADED` once its result is not `PENDING`.

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

## History

- `seed/history.yaml` loads on every boot; a slip already present (same week and name) is skipped.
- Unknown user names in the file are created as active users.
- Historical legs are usually `OTHER` with free text; game and player links are optional.

```yaml
slips:
  - season: 2026
    season_type: 2 # 2 regular, 3 postseason
    week: 7
    name: Week 7
    legs_per_member: 2
    legs:
      - user: Adam
        selection: BUF -3.5
        market: OTHER # optional, default OTHER
        odds: -110
        result: WON
```

The slip result is computed from leg results, not stored in the file.

## Screens

Phone first: one column with a bottom tab bar. At `lg` width and up, Slip, Games, and Research sit side by side as 3
columns and Record Book is a top-nav link.

- **Top bar:** week label, slip switcher, current user's avatar (links to profile: home book, switch user, manage
  users).
- **Slip tab:**
  - header: slip name, "2/3 locked in", status;
  - one section per member: avatar, name, lock state, their legs (selection, odds, result chip, edit and delete when
    allowed), "Add leg" for the current user (opens the leg form as a bottom sheet), "Lock in" when they have the full
    count;
  - reference odds and a payout calculator (stake default $10, boost % default 0), recomputed by the server on input
    with a 300 ms debounce;
  - **analyzer banner:** green "Looks clean" when there are no warnings, else one card per warning, sorted Danger, Warn,
    Info, colored red, amber, blue, showing the message and the legs involved with their owners;
  - activity feed, collapsed by default, newest first.
- **Games tab:** one card per game sorted by kickoff: `BUF @ MIA`, local kickoff time, spread and total (home-book
  price, else median), weather badge, status (upcoming, live, final). Badges: 🏠 Indoor (gray); 💨 wind mph (amber at ≥
  15, red at ≥ 20); 🌧 when precipitation ≥ 0.1 in/hr. Tapping a card expands quick-add pills: both spreads, both
  moneylines, Over and Under, plus "Custom leg" (opens the leg form with the game set). From `props-research`, prop
  pills too. A pill adds the leg in one tap; the leg can be edited afterward.
- **Research tab (`props-research`):** opens from a game card or a player name. Player view: last-5 bars, green over the
  line and red under, with median and mean marked, plus season games played. Game view: key injuries filtered to QB, RB,
  WR, TE, LT, C, CB, S.
- **Record Book tab (`grading-record-book`):** as described above.
- Times render as `<time datetime="{UTC ISO}">` and `app.js` formats them in the browser's local zone.
- A "lines as of 10:42 AM" note sits under the game board, from `sync_state`.
