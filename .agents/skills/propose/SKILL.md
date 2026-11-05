---
name: propose
description: Write a good-vibes proposal in changes/<slug>/proposal.md. Use when the user wants to propose a change, start a new feature, or asks for a proposal.
---

# Propose a change

A proposal explains why a change is worth making and sets the direction. It includes a brief high-level how and why this
way, saving detailed mechanics for the design.

## Steps

1. Choose a slug: lowercase, hyphenated, no number prefix. Run `good-vibes propose <slug> --title "<Title>"`. This
   writes `changes/<slug>/proposal.md`.
2. Fill the three sections in plain language:
   - **Context** — what happens today and why it hurts. Use numbers or links when you have them.
   - **Goals and non-goals** — why solve it now, what success looks like, and explicit non-goals to bound scope.
   - **Decision** — 1–3 paragraphs on the chosen direction, high-level shape, and why it beats obvious alternatives.
3. Delete the HTML comments once a section is done.
4. Leave `status: proposed`. A person accepts the change, not an agent.
5. Run `good-vibes check <slug>` and fix what it reports.

## Rules

- Save detailed mechanics for the design: no file paths, schemas, or API signatures. The proposal gives the high-level
  shape and rationale; `design.md` specifies how it works.
- Do not accept your own proposal. `good-vibes accept` needs a person's name; a review URL is optional.
- One change, one problem. If you need two Decisions, write two proposals.
