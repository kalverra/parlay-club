# Sunday Parlay Club

A Go + HTMX web app where a few friends build a shared NFL parlay each week.

- `plan/` is the **target design**: behavior not built yet. [plan/README.md](plan/README.md) lists the changes in order;
  read the plan sections a change cites before designing or building it.
- `docs/` is the **living docs**: the system as it is now. Start from `docs/llms.txt`.
- Each change moves what it built from `plan/` into `docs/` and deletes it from `plan/`.

## Setup and checks

- `mise install` installs the toolchain. `lefthook install` wires the git hooks.
- Work is green when `go generate ./...` leaves no diff, `golangci-lint run ./...` is clean, and
  `go test -short -race ./...` passes. `good-vibes.toml` runs the same commands as `implement.verify`.

## The good-vibes flow

Every change moves through Propose, Design, Implement + Document.

1. **Propose.** A change starts in `changes/<slug>/proposal.md`. It says *why* the change is worth making and *what*
   problem it solves. No implementation detail.
2. **Accept.** A person accepts the proposal with `good-vibes accept <slug> --by <names> --in <review-url>`. Agents do
   not accept their own proposals.
3. **Design.** `changes/<slug>/design.md` says *how* the change works, and its `## TODO` section lists the phases and
   tasks in order.
4. **Implement.** Work through the TODO phases. Update `docs/` in the same change, then write the phase handoff in
   `changes/<slug>/handoffs/`.
5. **Close.** Write `changes/<slug>/closeout.md` with the close skill. After a person reviews it,
   `good-vibes done <slug>` folds it into an ADR in `docs/decisions/` and deletes the change folder.

Skills live in `.agents/skills/`: `propose`, `design`, `implement`, `docs`, `close`.

### Flow rules

- Read the change folder before writing code. The design is the contract.
- Never tick checkboxes in `design.md` unless you are implementing that change. The implement flow and the binary own
  the checkboxes.
- `docs/` describes the system as it is now. A change updates the docs it invalidates; it does not append history.
- Every doc has YAML front matter. Run `good-vibes docs lint` after writing and `good-vibes docs index` after adding or
  renaming a doc.
- Do not edit `design.md` while a `good-vibes implement` run is active. The worktree's copy is the truth.
- Keep commands thin: prefer `good-vibes` commands over hand-editing status or ADR files.

```sh
good-vibes status                 # list active changes
good-vibes check                  # validate changes and docs
good-vibes propose <slug>         # scaffold a proposal
good-vibes accept <slug> --by a,b --in <url>
good-vibes design <slug>          # scaffold or confirm a design
good-vibes implement <slug>       # run the next phase (mode from good-vibes.toml)
good-vibes done <slug>            # write the ADR and delete the change folder
good-vibes docs get <id>#<head>   # one doc or one section
good-vibes docs index             # regenerate docs/llms.txt
```

## Product rules

- **Friendly, never blocking.** Analyzer warnings read like a sharp friend's heads-up ("Heads up: 22 mph winds in
  Buffalo, passing overs are risky"). Every pick stays allowed, and the tone stays free of +EV lectures.
- **Shared passcode + name picker is the whole auth model.** Keep it that way; no accounts, OAuth, or passwords.
- **The app tracks bets; people place them.** No sportsbook integrations and no money handling.
- **Deterministic logic only.** The analyzer and every request path run plain Go rules, with no LLM calls.
- **Phone first.** Build and check every screen at phone width, then widen.

## Free-tier guardrails

- **Odds API credits are scarce** (500/month). Fetch game lines on lazy sync only, props only for a game someone opens,
  always through the cache in `sync_state`. Respect the credit floors.
- **Single instance.** SQLite and the in-memory SSE hub assume one Cloud Run instance; keep shared state in SQLite or
  the hub, never in a second process.
- **Close idle SSE streams** per the live-updates policy; an open stream keeps the instance billed.
- Sync is lazy (triggered by page loads); add scheduled jobs only with a plan change.

## Code conventions

- Standard library first: `net/http` routing, `html/template`, `go:embed` for assets. The frontend is HTMX + Tailwind
  built by the standalone CLI; keep Node and JS build tools out.
- SQLite through `modernc.org/sqlite` (pure Go, no CGO).
- Logging with zerolog. Tests with `github.com/stretchr/testify`.
- Each external source (ESPN, Open-Meteo, The Odds API) sits behind a small interface. Unit tests replay saved responses
  from `testdata/` with `httptest`; live-endpoint tests skip under `-short`.
- Store times in UTC; render in the browser's zone.
