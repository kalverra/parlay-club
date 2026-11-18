---
name: gv-implement
description: Implement one phase of a good-vibes change, following changes/<slug>/design.md. Use when the user asks to implement a change or continue a phase.
---

# Implement a change

## Steps

1. Read `changes/<slug>/proposal.md` and `design.md`. Read the newest handoff in `changes/<slug>/handoffs/` when one
   exists. Also read `ARCHITECTURE.md` and `GLOSSARY.md`: the code must match the code map, and the code and docs use
   the glossary's words.
2. Work through the current phase in order, one task at a time.
3. After each task, build and run the project's tests. Tick the task in `design.md` (`- [ ]` to `- [x]`) only after it
   works.
4. Do any `Docs:` task in the phase. If the doc the change needs does not exist, write it. When the work coins a domain
   term, add it to `GLOSSARY.md` under its own `##` heading with a plain definition and, when useful, an `_Avoid:_` line
   naming the words not to use.
5. Before writing the handoff, run `good-vibes docs impact` with `--base <the commit the phase started from>`. It lists
   the docs whose `covers` entries the phase diff touched and that the diff did not edit. Open each one and update the
   ones the phase made wrong. For docs with no `covers`, fall back to `docs/llms.txt`: read each summary, compare it
   with the phase diff, and update every doc whose summary covers behavior the phase made wrong. A doc the phase
   invalidates and leaves stale is an unfinished task. Note each doc you checked in the handoff's **Done** section as
   `updated` or `still true: <why>`.
6. At the end of the phase, write `changes/<slug>/handoffs/phase-N.md` with:
   - **Done** — what landed, including each doc you checked as `updated` or `still true: <why>`
   - **Deviations** — where the design was wrong, and what you did instead
   - **Open issues** — anything the next phase must know
   - **Files touched**
   - **Next-phase notes**
7. Run `good-vibes check <slug>`.
8. After the last phase, close the change with the `gv-close` skill.

## Rules

- When you need handoff content, read `changes/<slug>/handoffs/phase-<n>.md`. Do not read `.log` files under
  `.good-vibes/runs/<id>/` as handoffs; those are transcripts for diagnosing a turn. Reading one is fine when you are
  debugging a turn.
- In an orchestrated run (`good-vibes implement --style orchestrated`), the binary ticks boxes, commits, and runs
  verify. Follow the prompt you are given; do not tick, commit, or run `good-vibes done`.
- Do not reword an open task. If the design is wrong, update `design.md` first, then implement.
- Never reword or remove an existing glossary term, and do not edit `ARCHITECTURE.md` beyond what the change's accepted
  proposal amends. Both guards fail the change otherwise.
- After a change moves a module boundary, an invariant, or a cross-cutting concern, update the matching
  `ARCHITECTURE.md` section in the same task, keeping it a diagram with a short caption.
- Do not commit or push unless the user asks.
- Keep the tree green after every task. A half-done task is not ticked.
- If the phase needs a doc the design did not name, add the task to `design.md` in the same change.
