---
name: gv-close
description: Write a good-vibes close-out in changes/<slug>/closeout.md before the change closes. Use when every phase of a change is done and the user wants to close it.
---

# Close a change

`good-vibes done` turns the proposal and the close-out into the ADR, then deletes the change folder. The ADR is the only
record the change leaves, so the close-out keeps what a future reader needs and drops the rest.

When the change ran, `done` also carries the run summary's `## Docs checked` section into the ADR, so the close-out
mentions a doc only when a judgment needs explaining.

## Steps

1. Read `changes/<slug>/proposal.md`, `design.md`, `summary.md` when the change ran, and every handoff in
   `changes/<slug>/handoffs/`.
2. Write `changes/<slug>/closeout.md` with a `# Close-out` title and these sections, in order:
   - **Consequences** — what got easier, what got harder, and what we would revisit.
   - **Alternatives considered** — rejected options and trade-offs distilled from `proposal.md`, `design.md`, and
     `handoffs/`, explaining why each was not chosen.
   - **Implementation notes** — where the plan was wrong, and limits a user or maintainer will hit.
   - **Follow-ups** — work left open, each item specific enough to propose.
3. Write "None." in a section with nothing to say. `done` refuses an empty or missing section.
4. A run may have drafted the file already, with `draft: true` in its front matter. Read every section against the
   proposal, design, and handoffs, and fix what is wrong.
5. Ask a person to review the close-out. Once they approve, delete the `draft: true` line if the file has one, then run
   `good-vibes done <slug>`. `done` refuses a draft, so deleting the line is the record that a person read it.

## Rules

- When you need handoff content, read `changes/<slug>/handoffs/phase-<n>.md`. Do not read `.log` files under
  `.good-vibes/runs/<id>/` as handoffs; those are transcripts for diagnosing a turn. Reading one is fine when you are
  debugging a turn.
- Distill; do not copy. Phase-by-phase logs, file lists, and task names stay out.
- Keep a fact only if it is still true and not already in `docs/`. If it describes how the system works now, update the
  doc instead.
- Do not run `good-vibes done` before a person has reviewed the close-out. It deletes the working papers.
