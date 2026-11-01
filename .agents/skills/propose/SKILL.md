---
name: propose
description: Write a good-vibes proposal in changes/<slug>/proposal.md. Use when the user wants to propose a change, start a new feature, or asks for a proposal.
---

# Propose a change

A proposal says why a change is worth making. It does not say how.

## Steps

1. Choose a slug: lowercase, hyphenated, no number prefix. Run `good-vibes propose <slug> --title "<Title>"`. This
   writes `changes/<slug>/proposal.md`.
2. Fill the three sections in plain language:
   - **Why** — what happens today and why it hurts. Use numbers or links when you have them.
   - **Problem** — the specific problem this change solves.
   - **Decision** — what we will do, in one or two paragraphs.
3. Delete the HTML comments once a section is done.
4. Leave `status: proposed`. A person accepts the change, not an agent.
5. Run `good-vibes check <slug>` and fix what it reports.

## Rules

- No implementation detail: no file names, schemas, or API signatures. Those belong in the design.
- Do not accept your own proposal. `good-vibes accept` needs a person's name and a review URL.
- One change, one problem. If you need two Decisions, write two proposals.
