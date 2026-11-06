---
name: propose
description: Write a good-vibes proposal in changes/<slug>/proposal.md. Use when the user wants to propose a change, start a new feature, or asks for a proposal.
---

# Propose a change

A proposal explains why a change is worth making and sets the direction. It includes a brief high-level how and why this
way, saving detailed mechanics for the design.

## Steps

1. Run `good-vibes status` to see the active changes and their order. Choose a slug: lowercase, hyphenated, no number
   prefix. Run `good-vibes propose <slug> --title "<Title>"` to write `changes/<slug>/proposal.md`. When another change
   must finish first, pass `--depends-on a,b` or edit the `depends_on` front matter by hand. Record the order there, not
   in prose; `check`, `status`, and `implement` read it.
2. Set `effort` in the front matter to `easy`, `medium`, or `hard`. Rate how hard the change is, not how long it takes.
   Leave the key out to skip the rating.
3. Fill the three sections in plain language:
   - **Context** — what happens today and why it hurts. Use numbers or links when you have them. When this change builds
     a section of `plan/`, name that section instead of restating it.
   - **Goals and non-goals** — why solve it now, what success looks like, and explicit non-goals to bound scope. When
     the plan lists acceptance checks for this change, they are the success test.
   - **Decision** — 1–3 paragraphs on the chosen direction, high-level shape, and why it beats obvious alternatives. Say
     why the `effort` rating fits in one sentence.
4. Delete the HTML comments once a section is done.
5. Leave `status: proposed`. A person accepts the change, not an agent.
6. Run `good-vibes check <slug>` and fix what it reports.

## Rules

- Save detailed mechanics for the design: no file paths, schemas, or API signatures. The proposal gives the high-level
  shape and rationale; `design.md` specifies how it works.
- Record the order between changes in `depends_on`, not prose. `good-vibes check` validates the list and
  `good-vibes implement` waits on every unfinished entry.
- Do not accept your own proposal. `good-vibes accept` needs a person's name; a review URL is optional.
- One change, one problem. If you need two Decisions, write two proposals.
