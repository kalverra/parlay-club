---
id: adr-local-sunday-sandbox
title: Local Sunday Sandbox
type: decision
summary: Add a development-only scenario mode to the existing dev task rather than a separate mock app.
area: infra-and-dev
---

# Local Sunday Sandbox

## Context

`mise run dev` starts the real app with hot reload, but it does not create a repeatable Sunday to test. A fresh checkout
starts with an empty, persistent database and live sports-source clients. Picking realistic legs therefore depends on
network access, current dates, and odds-provider availability. The raw-response archive saves successful calls, but no
request path can replay them.

The ingredients are scattered across lower-level tests. Committed ESPN, odds, weather, and projection fixtures cover
scheduled and partially live slates. Development sign-in accepts any email. Manual grading already settles a slip as
soon as one leg loses, handles an all-won slip, and permits corrections. None of this is assembled into a running local
scenario. There is no committed sample history, no complete all-final Sunday fixture, and no way to reset and advance a
Sunday as one exercise.

Testing with friends' perspectives is also slower than it should be. Every new development email creates a separate
group, so testing a shared slip requires private browser windows, manual invites, and switching cookies. Existing
history seeding cannot fix this cleanly: it targets the first group, creates name-only users rather than sign-in
identities, skips existing slips, and does not seed games or lines.

This makes routine UI and rule checks expensive, encourages consumption of scarce odds credits, and leaves important
sequences—early failure, every leg clearing, and the end of Sunday—without a fast browser-level rehearsal.

## Goals and non-goals

**Goals:**

- Add an opt-in, offline Sunday scenario to the normal development workflow, using committed past responses so no sports
  API keys, network access, or odds credits are needed.
- Start from a realistic Sunday morning with a shared group, several named personas, a full selectable slate, game lines
  and props, and enough context to exercise analyzer warnings and pick building.
- Make perspective changes quick and obvious while retaining real sessions, memberships, activity attribution, and group
  boundaries; include an outsider for invite and access checks.
- Provide deterministic reset and stage controls for at least Sunday morning, an early losing leg, every leg winning,
  and all games final. Each stage must update the visible games, picks, grading, activity, and live clients through the
  app's real rules.
- Keep sandbox state separate and disposable so a clean retry takes one command or action and does not require
  hand-deleting the normal development database.
- Prove the complete flows in automated tests, including replay without external calls, persona isolation, every
  requested outcome, and the absence of sandbox controls in production.

**Non-goals:**

- Changing production authentication, adding accounts or roles, or allowing persona switching outside local development.
- Building auto-grading or the Record Book; the sandbox may exercise those features when their separate change lands.
- Capturing new live data, simulating play-by-play, supporting every historical week, or replacing unit tests with a
  large browser suite.
- Adding sportsbook integration, money handling, scheduled jobs, another running service, or any LLM calls.
- Using this sandbox as production seed data or changing normal grading and refresh rules to accommodate it.

## Decision

Add a development-only scenario mode to the existing dev task rather than a separate mock app. A named Sunday scenario
will combine committed source responses with a fixed scenario clock, a shared group, email-backed personas, and
realistic slips. Replaying responses through the existing ingest and refresh paths keeps parsing, caching, and page
behavior honest; staging results through the existing slip rules keeps settlement, activity, and live updates honest.
Sandbox mode must fail closed against external sports requests and be impossible to enable in production.

An explicit reset will recreate the morning in isolated local state, and a small set of development controls will reset
or advance to the requested situations. Fixed timestamps and saved morning, live, and final snapshots will make old data
behave like the intended point in a Sunday instead of letting the real clock or a lazy refresh change the week. Tests
will cover the same path a developer uses in the browser.

This beats live API setup because it is deterministic and free, history seeding because it includes the whole user and
game context, and a checked-in SQLite snapshot because source fixtures remain inspectable and exercise the real
ingestion path. The effort is medium: the work spans replay, clocks, identity, scenario controls, and production
safeguards, but it reuses established clients, domain services, dev auth, and test fakes rather than introducing a new
product subsystem.

## Consequences

Testing a Sunday is now free and repeatable. `mise run sandbox` (or `SUNDAY_SANDBOX=1`) boots the ordinary server
against `./data/sunday-sandbox.db` with the full week 9 slate, lines, projections, forecasts, and the featured prop
board replayed from saved responses, so no keys, network, or odds credits are involved. The morning has a shared group
with three named people, six pending picks, and an outsider, and the controls page moves that same group through
`morning`, `early-loss`, `all-clear`, and `sunday-complete` through the real refresh, lock, and manual-grade paths. Any
button works from any prior state, resets cost nothing, and repeated resets produce the same people, groups, slip,
grades, activity, and empty cache.

Perspective checks are cheap: switching persona signs in through the normal dev flow and sets real session and group
cookies, so activity attribution, outsider 404s, and the invite join behave as they do for a person. The normal dev
database and history import are untouched, and sandbox mode is impossible in production.

The cost is a new fixture surface to maintain. The registry hard-codes request shapes that the weather and odds clients
own, the four stage scoreboards are hand-derived, and the sandbox only answers requests someone registered, so a client
change or a new screen can fail closed with a visible error rather than degrade. Startup reads all 60 response bodies
before listening, and the app now takes one injected clock everywhere it used to call `time.Now`, which every future
feature has to respect.

## Alternatives considered

- **Point development at live providers.** Rejected: scarce credits, current dates, and provider availability make it
  neither repeatable nor free, and a failed capture leaves no test to rerun.
- **Extend `internal/seed`.** Rejected: it imports history for the first group, creates name-only users, and has no
  games, lines, membership joins, or sign-in identities to build a shared Sunday from.
- **Commit a SQLite snapshot of a good Sunday.** Rejected: resets get awkward, the contents are opaque, and nothing
  exercises the real parsers, cache rules, or refresh path.
- **Replay through the raw-response archive.** Rejected as too broad: it would need a general export/import feature that
  this change does not need, and archive replay would also risk writing replayed calls back into the archive.
- **Run a second mock HTTP server.** Rejected: another process to learn and start, with no gain in fidelity over an
  `http.RoundTripper` inside the real clients.
- **Fixture the domain rows directly.** Rejected: it would bypass parsing, cache freshness, credit floors, and grading
  rules, and would hide fixture drift instead of failing on it.
- **Inject the persona into request context.** Rejected: it would fake identity and permissions. Real `auth.SignIn` and
  real group cookies keep activity, grading, and access checks honest.
- **Infer stage results from scores.** Rejected: staged legs are scripted manual grades through `SetLegResult`. Score
  inference and auto-grading stay with `grading-record-book`.

## Implementation notes

- The checkout did not contain the same-week live and final ESPN captures the design assumed, so all three stage
  scoreboards are derived by hand from the captured week 9 slate. The registry and the ESPN fixture README say so.
- Nine morning forecasts had to be derived as well; only Lumen Field had a real capture. The weather fixture README
  marks them.
- The design passed an `fs.FS` rooted at the repository. The sandbox instead finds the checkout by walking up to
  `go.mod`, so a binary started outside the checkout fails to preload fixtures rather than silently serving stale data.
- Reset and stage selection are the same operation: `Reset` delegates to `Stage(morning)`. One mutex, one rebuild, one
  publish path, one failure shape. Every non-morning stage locks all three shared members, since the seed has exactly
  three and the slip is shared.
- The stage row is saved before the destructive clear and before a stage's refresh. If a rebuild or grade fails, the
  saved stage and clock can be ahead of the data and the route answers 500. Reset is the only recovery; there is no
  per-stage rollback.
- A page-load `Kick` holds the transition mutex for its whole ingest pass, so a reset or stage chosen during a cold
  refresh waits for it. That is the price of never letting an old snapshot land after a fresh morning.
- The in-memory seed is empty after an Air restart. Stages rebuild morning first, so grading always resolves legs from a
  fresh seed, and the controls page reads personas and groups from the database.
- Replayed credit headers are static per response, so a repeated call sees the same balance. Both balances stay above
  the floors, which is what the guards need.
- Only the featured game's card and research surface is captured: its prop board, both rosters, its summary, and the
  game logs of its props players. Every other game answers props, summary, roster, and game log requests with a labeled
  empty fallback, so opening it renders the normal empty state instead of a replay miss. A request outside those four
  surfaces still fails visibly; the scenario is not a tour of every props surface.
- With the outsider, the shared invite join works but leaves Dex in their own group, so switching back to Dex after
  joining hides the shared slip again until a reset. That is deliberate for the control and is not the ordinary member
  experience.

## Follow-ups

- Replace the three hand-derived stage scoreboards with a generator over the captured slate, so a fixture reshuffle is
  one command instead of three edited JSON files.
- Register prop boards for every slate game so props research outside the featured game has real lines instead of the
  labeled empty fallback.
- Give a failed stage a recovery path better than a full reset: verify the rebuild before saving the stage row, or add a
  per-stage rollback.
- Embed the fixtures with `go:embed` so a packaged binary can serve the sandbox outside the checkout.
- Let a reset or stage queued behind a cold page-load pass wait on the refresh instead of blocking on the mutex, if that
  latency turns out to annoy.
- When `grading-record-book` lands, decide whether sandbox stages should grade through it or keep calling
  `SetLegResult`, and keep stages labeled as scripted manual grades either way.
