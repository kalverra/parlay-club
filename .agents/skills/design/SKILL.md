---
name: design
description: Write a good-vibes design in changes/<slug>/design.md, including the TODO phases. Use when a proposal is accepted and the user wants to plan the implementation.
---

# Design a change

The design says how the change works. The TODO section says what to do, in order.

## Steps

1. Run `good-vibes design <slug>` to scaffold `changes/<slug>/design.md`.
2. Fill the sections:
   - **Context** — what exists today, and what constrains the change.
   - **Approach** — modules touched, data flow, trade-offs. Name the seams.
   - **Risks** — what could break, and how we would notice.
3. Write the TODO:
   - One `### Phase N: Name` per step that leaves the repo green.
   - Tasks are concrete: one file, one migration, one test.
   - Every phase ends with a `Docs:` task naming the doc it updates, or `Docs: none (reason)`.
4. Run `good-vibes design <slug>` again. It validates the TODO grammar and moves the status to `designed`.
5. Run `good-vibes check <slug>`.

## Rules

- Phases run in sequence. Do not plan parallel work.
- Do not tick checkboxes. The implement flow owns them.
- If the design grows past a page, the change is too big. Split it.
- Keep every task on one line under 120 columns. The linter reflows longer lines, and the TODO parser reads only the
  first line.
- Code in the design is a sketch. Keep it short enough to stay true.
