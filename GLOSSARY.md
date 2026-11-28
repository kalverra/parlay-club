---
id: glossary
title: Glossary
type: glossary
summary: The words this project uses, and the words to avoid.
tags: [glossary]
---

# Glossary

<!--
  One ## heading per term. The body is the definition, then an optional _Avoid:_ line. Add a term when you coin it;
  never reword or remove an existing term without a change that amends this doc. Terms must be unique ignoring case,
  and every term needs a definition.
-->

```markdown
## Run

One orchestrated execution of a change's TODO in its own worktree, from start to the final gate.

_Avoid:_ job, session
```

## Push subscription

One browser's Web Push endpoint and encryption keys, stored per device rather than per account. A person may have
several, and turning notifications off, signing out, or a `404` or `410` from the push service removes one.

_Avoid:_ push token

## Slip start

The earliest kickoff among a slip's eligible games, fixed even after it passes. A kickoff reminder is keyed to it, so a
moved kickoff or a later kickoff wave cannot send a second reminder. A slip's next deadline is different: it rolls
forward to the next kickoff still ahead.

_Avoid:_ first game, deadline

## Build grade

A pre-kickoff letter grade (`A` through `F` with `+`/`-` subdivisions and a two-word descriptor) that scores how cleanly
a leg or slip is constructed across price vig, leg synergy, hit-rate realism, and priced-in context. It measures
structural quality, not predictive edge over Vegas.

_Avoid:_ edge score, lock

## Correlation tax

The payout shave a sportsbook takes on a same-game parlay relative to multiplying the legs' independent odds, expressed
as a percentage.

## Process vs. Outcome

A post-game 2x2 classification pairing a pre-kickoff build grade (`A`–`B` vs. `C`–`F`) with a settled `Won` or `Lost`
result: `Clean Hit`, `Tough Beat`, `Total Bailout`, or `Book Donation`. Pushed, voided, pending, and ungraded entries
are left unclassified.
