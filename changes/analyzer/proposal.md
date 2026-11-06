---
slug: analyzer
status: designed
created: "2026-11-01"
depends_on: [slate-and-lines]
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
