---
slug: weekly-pick-flow
status: implementing
created: "2026-11-11"
accepted_by:
    - adam
accepted_at: "2026-11-11T19:24:27Z"
effort: hard
---

# Make the weekly pick flow the center of the app

## Context

The app has the right parts. It does not have a path through them.

Today the tabs are Slip, Games, Research, and Record. Each tab is a kind of data, not a step in a task. The one thing
nearly everyone does every week is: open the app, see what the group needs from me, pick my legs, lock in, and place the
parlay. That path crosses three tabs and has gaps:

- **No place to land.** Sign-in drops you on the newest slip of whichever group is active. Groups live in a top-bar
  dropdown. You never see "my groups, and what is happening in each." Every person starts in a one-person "Just me"
  group and can join any number of clubs, so most people have two or more groups and no way to see them side by side.
- **No "what do I do next."** The slip page shows members, legs, and lock state, but never says "you owe one more leg"
  or "waiting on Ben."
- **The picker is a list of accordions.** The Games tab shows spreads, moneylines, and totals as pills. Player props
  load when you open a game, grouped by athlete. There is no search, no sort, and no category filter. Finding "Allen
  over passing yards" means opening games one by one.
- **Picking and the slip are far apart.** You add a leg on one tab and see whether it counted on another. At the limit
  you get an error ("you've already added all your legs") instead of a way to swap.
- **No way to react to a friend's pick.** The product is "friends build a parlay together," but the only social surface
  is an activity feed. There are no votes.
- **The last step is buried.** Export appears below the panel once everyone locks, with nothing pulling you toward it.

Research is useful but sits as a top-level tab, even though people reach it from a player or a pick.

## Goals and non-goals

**Why now.** The sandbox and live controls make the full week testable end to end. The data layer is done: lines, props,
weather, rosters, export links. The weak part is how a person moves through it, and every later change (record book,
grading) will bolt onto whatever shape the app has.

**Goals:**

- One happy path, built as a sequence of screens:
  **groups, group hub, pick board, back to the hub, lock in, vote, export.** Someone doing it for the first time needs
  no explanation.
- Every screen has **one primary action** that matches the state: pick your legs, lock in, waiting on others, open in
  DraftKings or FanDuel.
- The group hub is the home of the current week. It shows who has picked, who has locked in, what is missing, and when
  the first kickoff cuts picks off.
- The pick board lets you **search** (player, team, game, prop category), **filter** by category and kickoff window,
  **sort** (by player, category, odds, kickoff), open a game for all its props, and choose 1 to n legs. A tray shows
  your picks, your progress ("1 of 2"), and a way back to the hub from every screen of the flow.
- At the leg limit, choosing another pick offers to **replace** one instead of failing.
- Slip members can **upvote or downvote** each other's legs on the hub, one vote per person per leg, never their own.
  The hub shows a plain tally on each leg, so an owner sees whether friends like the pick. Votes inform; they never
  block or change anything else. The app records them so later stats can use them (for example, how many legs you liked
  that hit versus missed).
- Your own **solo betslip stays first-class.** "Just me" sits in the groups list beside every club you join, with the
  same hub and the same flow.
- **Phone first.** Every screen is built at phone width with thumb-reach actions, then widened.
- Navigation follows the task, not the data. Games folds into the pick board; Research becomes a link from players,
  props, and picks rather than a tab.

**Non-goals:**

- Push notifications, email, or SMS. There is no scheduled job or second process. Nudges are a share-sheet message the
  user sends themselves.
- Changing lock, grading, or kickoff rules. Replace is a delete and an add in one step, so it obeys the same rules: no
  replacing a leg whose game has kicked off, and no replacing after you lock in.
- Changing the analyzer's rules or tone. The pick board shows its existing heads-ups earlier.
- Record Book, auto-grading, and any new data source. Those stay in their own changes.
- Vote stats screens. This change records votes and shows the tally; showing "liked legs that hit" belongs with the
  Record Book.
- Votes that carry weight: no thresholds, no gating the lock, no reordering by score.
- Building a way around the Odds API credit rules (see the search limit below).
- Hiding "Just me" or removing multiple slips per week.

## Decision

Rebuild the front of the app around the story, in this order. Look at the whole shape first:

```mermaid
flowchart LR
  subgraph Before
    B1[Sign in] --> B2[Slip tab<br/>newest slip of active group]
    B2 --> B3[Games tab<br/>accordion cards]
    B3 --> B2
    B2 -.-> B4[Research tab]
    B2 --> B5[Export card<br/>at page bottom]
  end
  subgraph After
    A1[Sign in] --> A2[My groups<br/>one card each, with state]
    A2 --> A3[Group hub<br/>this week, what I do next]
    A3 --> A4[Pick board<br/>search, filter, sort, tray]
    A4 --> A3
    A3 --> A5[Lock in + vote]
    A5 --> A6[Open in DraftKings / FanDuel]
  end
```

**The group hub is the center.** It is today's slip page, reshaped around state. A banner at the top says the one thing
to do now, and changes as the week moves: no slip yet, you need legs, you can lock in, waiting on named people, ready to
export, or finished. Below it, each member's legs with votes. A small group list is the new landing page, with one card
per group (clubs and "Just me" alike) showing the week's state. A person with exactly one group, such as a new sign-up
who has only "Just me," skips straight to its hub. The pick board is a focused screen you enter from the hub and leave
back to it, with no tab bar competing for attention. A persistent tray keeps your picks visible, so you never wonder
whether a tap worked. The Record tab keeps its place; Slip and Games give way to the groups list and the hub.

**Live updates stay one stream per page.** The hub keeps today's slip stream. The group list opens no stream: it reads
fresh state on load and when the tab becomes visible again. A list that watched every group would hold the instance open
for a page people glance at and leave.

**The pick board replaces the Games tab.** It reuses the existing props loading, the per-person home book prices, and
the quick-add path, so no new data source and no new credit use. The new work is the screen: search, category and day
chips, sorting, a game view with both sides of a line in one row, selected states that persist, and replace-at-limit.
Votes are one new idea; they attach to a leg and clear when its owner edits or replaces it, because a vote on a pick
that no longer exists is misleading.

**What the story left out, and why each piece matters** (the design will sort these into phases):

1. *A deadline.* Legs lock at each game's kickoff. The slate runs Sunday to Monday night and can start with a 9:30 a.m.
   ET London game. The hub must show the first kickoff that will cut picks off, or "wait for everyone" has no end.
2. *Replace at the limit, and a tray.* Sportsbooks all use a floating bet slip because people lose track of state across
   screens. It is the most proven pattern in this domain.
3. *What votes mean.* Votes are a tally, not a gate, in line with "friendly, never blocking." They open while the slip
   is open and are read-only once everyone locks, because no one can act on them after that. An unlock reopens them.
   They are kept after the week so stats can use them later.
4. *Search has an honest limit.* Props load per game on demand to protect the 500-credit month. Searching "Allen passing
   yards" across the whole week can only search what has been loaded. The board will search game lines, teams, games,
   rosters, and loaded props, and say plainly when a game's props have not been loaded, offering to load that one game.
   It will not fetch props for every game.
5. *Filters before sorting.* On a phone, category and day chips narrow faster than typing or column sorting. Sorting
   stays small: by player, category, odds, kickoff.
6. *Empty and first-run states.* A new person, a group with no slip, a week with no lines, and a slip with one person
   still missing each get a real screen.
7. *The analyzer at the moment of choosing.* The sharp-friend heads-up is most useful when you pick, not after. Show it
   in the tray.
8. *The hand-off.* "Prices may have moved" at export, and a share-sheet nudge for the person you are waiting on.

This beats the alternatives. Polishing each tab leaves the path broken, because the problem is between screens, not in
them. A new wizard that sits beside the current tabs leaves two apps to maintain. Reshaping the existing slip page and
Games tab keeps the rules, routes, live updates, and tests that already work, and moves them under a clearer shape.

The effort is `hard`. It touches navigation, three screens, a new vote concept, and live updates, while holding the
credit and single-instance limits. Each decision depends on the others, so it needs a careful design before any code.
