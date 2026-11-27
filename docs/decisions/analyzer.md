---
id: adr-analyzer
title: Sharp-friend analyzer
type: decision
summary: Add a deterministic rules engine that reviews the whole slip and shows friendly, non-blocking heads-ups to everyone, including pairings across two people's legs.
area: analyzer
---

# Sharp-friend analyzer

## Context

Friends often stack legs that fight each other, such as a big favorite's spread with that team's passing over, or lay a
-3.5 hook, without noticing.

Nobody on the slip spots bad correlations, weather traps, key-number hooks, or heavy chalk before the bet is placed.

## Goals and non-goals

**Goals:**

- Review the whole slip with plain Go rules, including pairings across two people's legs.
- Warn about correlation traps and smart stacks, wind and rain, key numbers, and heavy chalk.
- Show friendly, non-blocking heads-ups under the slip, sorted by severity and naming the legs and owners involved.
- Keep every pick allowed and the tone free of +EV lectures.

**Non-goals:**

- `median_trap` or other player-stat rules. `props-research` adds those with player data.
- LLM calls or any model-generated advice.
- Blocking a pick, changing odds, or grading results.
- Telling people what to bet.

## Decision

Add a deterministic rules engine that reviews the whole slip and shows friendly, non-blocking heads-ups to everyone,
including pairings across two people's legs. It covers correlation traps and smart stacks, wind and rain, key numbers,
and chalk. No LLM calls.

## Consequences

The slip panel now reviews every pick with plain Go rules as bets are built. It spots correlation traps (such as a big
favorite's spread paired with its passing over, or a heavy underdog paired with its rush over), contradictory totals,
weather hazards like high winds or light rain, key-number hooks on spreads and totals, and heavy chalk below −230. When
a warning involves picks from multiple people, both owners are named.

The engine is deterministic, pure, and runs in memory with no external API calls and no LLM latency or cost. The slip
panel stays friendly and non-blocking: clean slips show a reassuring green banner, flagged combinations explain the
friction simply, and every pick remains allowed.

The slip panel now listens for `board-updated` in addition to `slip-updated`, which means weather updates re-render the
slip panel without separate SSE channels, but also adds a render pass whenever the board refreshes. Hand-entered legs
without a matching game ID are silently skipped by design.

What we would revisit is adding input range validation on leg forms so bizarre lines never hit the analyzer, and tuning
warning thresholds or phrasing once friends use the banner across Sunday slates.

## Alternatives considered

- **LLM-generated slip advice.** Rejected: adds latency, unpredictable tone, token costs, and potential hallucinations.
  The app uses deterministic Go rules with pre-approved, friendly messaging.
- **Blocking conflicting picks.** Rejected: contrary to product principles. The analyzer acts like a sharp friend
  offering a heads-up, never a gatekeeper.
- **A separate SSE stream or background worker for analysis.** Rejected: the analyzer evaluates in microseconds during
  view mapping. Hooking `board-updated` into the slip panel's `hx-trigger` updates weather warnings without extra
  infrastructure.
- **Computing slip EV or win probabilities.** Rejected: out of scope, patronizing, and contrary to friendly social play.

## Implementation notes

- **Concrete analysis struct instead of interface.** The design sketched `func(ctx) []Warning`. The implementation uses
  `func(*analysis) []Warning` over a shared, pre-filtered view of valid legs and games, avoiding unnecessary
  abstraction.
- **`stats` map passed as nil.** Player statistics are deferred until `props-research`. The analyzer accepts the
  argument so the signature is forward-compatible, but skips stat-dependent checks like `median_trap`.
- **`board-updated` trigger wiring.** The slip panel previously only triggered on `slip-updated`. Adding `board-updated`
  to `hx-trigger` was necessary so forecast changes refresh weather warnings.
- **Chalk message formatting.** The initial plan suggested "about 1 in 4 times". The implementation uses formula-driven
  percentages (such as "about 25% of the time") matching the calculated bust chance.
- **Known limits:** legs without a `GameID` or unknown to the week's schedule are skipped; leg odds and lines are not
  range-checked at form entry; `hook` only evaluates spreads; and `favorite` supports moneyline legs even though
  `stack_clock_drain` currently only checks spreads.

## Follow-ups

- Add line and odds range validation to leg creation and edit forms to catch bogus values before they reach the
  analyzer.
- Implement the `median_trap` rule once player stats land in `props-research`.
- Add nfelo rating signals to analyzer rules when `nfelo-signals` is scheduled.
- Revisit expanding `stack_clock_drain` to cover moneyline favorites if user slips frequently combine heavy moneyline
  picks with rush overs.
