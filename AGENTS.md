# Sunday Parlay Club

A friendly way to manage collaboratively building parlays with friends.

This repo uses **good-vibes** for change planning. Every change moves through three steps: Propose, Design, Implement +
Document.

## The flow

1. **Propose.** A change starts in `changes/<slug>/proposal.md`. It explains *why* the change is worth making, sets
   goals and non-goals, and describes the high-level direction and rationale over alternatives. Detailed file paths,
   schemas, and signatures belong in the design.
2. **Accept.** A person accepts the proposal with `good-vibes accept <slug> --by <names> [--in <review-url>]`. Agents do
   not accept their own proposals.
3. **Design.** `changes/<slug>/design.md` says *how* the change works, and its `## TODO` section lists the phases and
   tasks in order.
4. **Implement.** Work through the TODO phases. Update `docs/` in the same change, then write the phase handoff in
   `changes/<slug>/handoffs/`.
5. **Close.** Write `changes/<slug>/closeout.md` with the close skill. After a person reviews it,
   `good-vibes done <slug>` folds it into an ADR in `docs/decisions/` and deletes the change folder.

## Skills

- `.agents/skills/propose` — write a proposal
- `.agents/skills/design` — write a design and its TODO phases
- `.agents/skills/implement` — implement a phase
- `.agents/skills/docs` — update living docs
- `.agents/skills/close` — write the close-out before a change closes

## Rules

- Read the change folder before writing code. The design is the contract.
- Never tick checkboxes in `design.md` unless you are implementing that change. The implement flow and the binary own
  the checkboxes.
- `docs/` describes the system as it is now. A change updates the docs it invalidates; it does not append history.
- Every doc has YAML front matter. Run `good-vibes docs lint` after writing and `good-vibes docs index` after adding or
  renaming a doc.
- Do not edit `design.md` while a `good-vibes implement` run is active. The worktree's copy is the truth.
- Keep commands thin: prefer `good-vibes` commands over hand-editing status or ADR files.

## Commands

```sh
good-vibes status                 # list active changes
good-vibes check                  # validate changes and docs
good-vibes propose <slug>         # scaffold a proposal
good-vibes accept <slug> --by a,b [--in <url>]
good-vibes design <slug>          # scaffold or confirm a design
good-vibes done <slug>            # write the ADR and delete the change folder
good-vibes docs list              # docs by id, path, and summary
good-vibes docs get <id>#<head>   # one doc or one section
good-vibes docs lint              # check docs
good-vibes docs index             # regenerate docs/llms.txt
good-vibes docs stale             # docs whose sources moved on
good-vibes config show --origin   # where each setting came from
```

## Product rules

- Friendly, never blocking. Analyzer warnings read like a sharp friend's heads-up ("Heads up: 22 mph winds in Buffalo,
passing overs are risky"). Every pick stays allowed, and the tone stays free of +EV lectures.
- Shared passcode + name picker is the whole auth model. Keep it that way; no accounts, OAuth, or passwords.
- The app tracks bets; people place them. No sportsbook integrations and no money handling. Deterministic logic only.
  The analyzer and every request path run plain Go rules, with no LLM calls.
- Phone first. Build and check every screen at phone width, then widen.

## Free-tier guardrails

- Odds API credits are scarce (500/month). Fetch game lines on lazy sync only, props only for a game someone opens,
  always through the cache in sync_state. Respect the credit floors.
- Single instance. SQLite and the in-memory SSE hub assume one Cloud Run instance; keep shared state in SQLite or the
  hub, never in a second process.
- Close idle SSE streams per the live-updates policy; an open stream keeps the instance billed.
- Sync is lazy (triggered by page loads); add scheduled jobs only with a plan change.
