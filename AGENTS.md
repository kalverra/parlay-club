# Sunday Parlay Club

A friendly way to manage collaboratively building parlays with friends.

## Product rules

- Google Cloud in production, easy local testing
- Friendly, never blocking. Analyzer warnings read like a sharp friend's heads-up ("Heads up: 22 mph winds in Buffalo,
passing overs are risky"). Every pick stays allowed, and the tone stays free of +EV lectures.
- Google OAuth is the only auth model. No custom accounts or permissions.
- The app tracks bets; people place them. No sportsbook integrations and no money handling. Deterministic logic only.
  The analyzer and every request path run plain Go rules, with no LLM calls.
- Phone first. Build and check every screen at phone width, then widen.

## Free-tier guardrails

- Odds API (500/month) and The Parlay API (1,000/month) credits are scarce. Fetch game lines on the per-game-day morning
  pre-fill (first authenticated full-page visit at or after 6:00 AM ET on a scheduled game day while upcoming games
  remain on that date, tracked via `prefill:gameday` in `sync_state`, or the optional `POST /internal/prefill` fallback)
  or on pick-time views, and props on pre-fill (Parlay-only, scoped to that game date's upcoming games, or all upcoming
  week games on a non-game-day manual fallback) or for a game someone opens, always through the cache in `sync_state`.
  Respect the credit floors.
- Single instance. SQLite and the in-memory SSE hub assume one Cloud Run instance; keep shared state in SQLite or the
  hub, never in a second process.
- Close idle SSE streams per the live-updates policy; an open stream keeps the instance billed.
- Background `Kick()` runs on authenticated full-page loads only, syncing free/unmetered sources outside the one-shot
  per-game-day morning pre-fill (`prefill:gameday`); scheduled jobs are limited to the optional `POST /internal/prefill`
  fallback and per-slip reminder tasks on Cloud Tasks (one task per open slip at its start minus 20 minutes), and any
  additional scheduled jobs require a plan change.

<!-- good-vibes:begin -->
<!-- Managed by `good-vibes init`. Edits inside this block are overwritten. -->

## good-vibes

This repo uses `good-vibes` for change management. Every significant change moves through five steps.

Each has a skill in `.agents/skills/gv-<step>/`.

The `gv-analyze` skill sits outside the five steps and reads finished run history for recurring review findings.

### The Flow

1. **Propose** (`gv-propose`). A change starts in `changes/<slug>/proposal.md`. It explains *why* the change is worth
   making, sets goals and non-goals, and gives the high-level direction and why it beat the alternatives. File paths,
   schemas, and signatures belong in the design. When another change must finish first, record it in the `depends_on`
   front matter with `good-vibes propose <slug> --depends-on a,b` or by editing the list, not in prose. `check` reports
   bad entries, `status` shows the wait, and `implement` refuses to start until they are done.
2. **Accept.** A person runs `good-vibes accept <slug> --by <names> [--in <review-url>]`. Agents leave their own
   proposals for a person to accept.
3. **Design** (`gv-design`). `changes/<slug>/design.md` says *how* the change works. Its `## TODO` section lists the
   phases and tasks in order. Keep order between changes in `depends_on`, not in phases or prose.
4. **Implement** (`gv-implement`, `gv-docs`). Work through the TODO phases. Update `docs/` in the same change, then
   write the phase handoff in `changes/<slug>/handoffs/`.
5. **Close** (`gv-close`). Write `changes/<slug>/closeout.md`, or read and fix the file a run left there. Then
   `good-vibes done <slug>` folds it into an ADR in `docs/decisions/` and deletes the change folder.

### Rules

- Read the change folder before writing code. The design is the contract.
- Run `good-vibes docs for <path>` before editing unfamiliar code to see the living docs and ADRs that apply.
- Only the implement flow and the binary tick checkboxes in `design.md`.
- Leave `design.md` alone while a `good-vibes implement` run is active. The worktree's copy is the truth.
- `docs/` describes the system as it is now. A change rewrites the docs it invalidates and leaves history to the ADR.
- `README.md`, `ARCHITECTURE.md`, and `GLOSSARY.md` are high-level docs: read them before a change. A change that alters
  a module's responsibility, an invariant, or a cross-cutting concern amends `architecture`; one that redefines or
  removes a term amends `glossary`; one that alters the project's top-level overview, positioning, or quickstart amends
  `readme`. Add a glossary term when you coin it. Keep `README.md` a Diátaxis front door (orient and link into `docs/`)
  structured as an inverted pyramid (what it is, why it matters, and a bare-bones quickstart first).
- Every doc has YAML front matter. Run `good-vibes docs lint` after writing, and `good-vibes docs index` after adding or
  renaming a doc.
- Use `good-vibes` commands for status and ADR files. Run `good-vibes --help` for the command list.
<!-- good-vibes:end -->
