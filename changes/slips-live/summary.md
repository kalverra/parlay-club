---
slug: slips-live
run: 20261107-185806-slips-live
outcome: done
generated: 2026-11-07T19:42:36Z
---

# Run summary: slips-live

## Outcome

- Status: done
- Branch: slips-live
- Final verdict: agy@gemini-4-argon-medium approved (issues: 0)

## Totals

- Tasks: 11 of 11
- Phases: 4
- Checkpoints: 11
- Duration: 40m5s
- Cost: $1.86
- Input tokens: 42793331
- Output tokens: 366230
- Cache read tokens: 38851802

## Phases

### Phase 1: Parlay math

Tasks:

- [x] Add internal/parlay conversions, combined odds, payout with boost, units, and median reference price, with table tests for both conversion directions, a 3-leg combo, boost, push legs dropped, and an even-count median
- [x] Docs: add docs/reference/odds-math.md; delete the Odds and payout section from plan/product.md

Phase totals: 2 checkpoints, 4m13s, $0.16, 1693808 input / 28161 output / 1306610 cache tokens

Checkpoints: task-1, task-2 Review fixes: 0 Integration checks: 0

#### Done

- Added `internal/parlay`, a pure package with no database or clock:
  - `AmericanToDecimal` and `DecimalToAmerican`, both treating `0` / `d ≤ 1` as "not a price" rather than erroring.
  - `CombinedOdds`, which multiplies counting leg decimals and returns `1` for no counting legs.
  - `ReferencePrice`, which returns the median of caller-supplied candidates, the mean of the middle two for an even
    count, and the leg's own price when there are no candidates. It sorts a clone, so callers keep their input order.
  - `Payout` with a profit-boost percentage, and `Units` for a graded result.
  - `Result` constants and `Result.Counts()`, used by `CombinedOdds` to drop `PUSH` and `VOID` legs.
- Table tests cover both conversion directions, a round trip, a 3-leg combo, push and void legs dropped, payout with and
  without boost, odd and even-count medians, unsorted candidates, units for every result, and `Counts()`.
- Added `docs/reference/odds-math.md` with the formulas, worked examples, and a function table.
- Deleted "Odds and payout" from `plan/product.md` and pointed the quick-add sentence at the new reference doc.
- Regenerated `docs/llms.txt`.

### Phase 2: Slip rules

Tasks:

- [x] Add Store methods for weeks, slips, members, legs, and activity, and create-slip defaults copied from the previous slip, with tests
- [x] Add member and leg mutations, lock in, unlock, and slip status updates with one activity entry each, and table tests for every Editing and locking rule with an injected clock, including past-week slips
- [x] Docs: add docs/concepts/slips.md; delete Slips, Legs, and Editing and locking from plan/product.md

Phase totals: 3 checkpoints, 12m46s, $0.62, 11090401 input / 140146 output / 9789094 cache tokens

Checkpoints: task-3, task-4, task-5 Review fixes: 0 Integration checks: 0

#### Done

- Added the `internal/db` store layer:
  - Weeks: `WeekID`, `UpsertWeek` (idempotent by season, type, and week), and `WeekByID`.
  - Slips: `CreateSlip` (trims names, enforces 1–5 legs and at least one member, ignores repeat
    members), `SlipByID`, `SlipsForWeek`, `NewestSlipForWeek`, `PreviousSlip` (same week first, then
    earlier weeks), `SlipMembers`, `SlipMemberIDs`, `AddSlipMember`, `RemoveSlipMember`,
    `SetSlipMemberLock`, `UpdateSlip`, and `SetSlipStatus`.
  - Legs: `CreateLeg` (0 odds becomes -110, new legs start `PENDING`), `LegByID`, `LegsForSlip`,
    `UpdateLeg` (pick fields only), and `DeleteLeg`.
  - Activity: `AddActivity` (blank messages refused) and `ActivityForSlip` (newest first).
  - Games: `GameKickoff` and `WeekPast` as the clock inputs for the kickoff rule.
- Added `internal/slips`:
  - `CreateDefaults` plus `DefaultLegsPerMember` (2), copying the previous slip's limit and member IDs.
  - `Service` with an injected publisher and clock. It enforces every rule in
    `docs/concepts/slips.md` "Editing and locking", writes each change and one activity entry in a
    transaction, updates the slip status on lock in and unlock, and publishes `slip:{id}` once after
    the commit.
  - Typed errors for every refusal, wrapped with context so `errors.Is` still works in handlers.
- Tests:
  - Store tests cover week upserts, slip validation, previous-slip selection, member and lock
    mutations, status changes, leg CRUD and defaults, activity ordering, and missing rows.
  - Rule table tests 1–9 run on a fixed injected clock: membership and ownership, the leg limit,
    kickoff locks (upcoming, started, exactly now, no game, past week), lock in with exactly and too
    many legs, no-op lock in, locked-in member editing, the slip locking when everyone is in, unlock,
    settings only while `OPEN`, the limit floor, removing a member with legs, past-week edits, one
    activity entry and one publish per mutation, and refused mutations leaving no trace.
- Docs: added `docs/concepts/slips.md`, deleted Slips, Legs, and Editing and locking from
  `plan/product.md`, and updated the `plan/README.md`, `plan/architecture.md`, and `docs/llms.txt`
  pointers.

### Phase 3: Live updates

Tasks:

- [x] Add internal/live hub with topic subscribe, unsubscribe, and coalescing publish; test bursts and unsubscribe
- [x] Add the SSE handler with pings and deadlines, plus the EventSource client with refresh triggers, bye handling, and visibility close/reopen; test slip-updated delivery and bye at both deadlines
- [x] Docs: add docs/concepts/live-updates.md; delete Live updates from plan/architecture.md

Phase totals: 3 checkpoints, 7m16s, $0.28, 3808413 input / 50617 output / 3138844 cache tokens

Checkpoints: task-6, task-7, task-8 Review fixes: 0 Integration checks: 0

#### Done

- Added `internal/live` with two pieces:
  - `Hub` fans out string topics. `Subscribe(topic)` returns a receive-only channel with room for one signal and an
    idempotent unsubscribe func. `Publish(topic)` never blocks: a full buffer keeps the one queued wake-up and drops
    the rest, so bursts coalesce. The zero value is ready to use.
  - `Stream.ServeSlip(w, r, slipID, weekID)` serves one SSE connection. It subscribes to `slip:{id}` and
    `week:{weekID}`, sends `slip-updated` and `board-updated` signals with `data: {}`, pings every 25 seconds, and sets
    the streaming headers (`text/event-stream`, `no-cache`, `keep-alive`, `X-Accel-Buffering: no`) and flushes every
    frame.
  - Deadlines: a live window lives 55 minutes from stream start; otherwise the stream closes after 10 minutes without a
    slip or board event (pings do not reset it). Both paths send `event: bye` and return. Timers come from a `Clock` /
    `Timer` seam, so tests fire them by hand.
  - `Stream` takes an `IsLive(weekID) bool` func. Nothing wires a game-aware answer yet, so callers pass false until
    `slate-and-lines`.
- Added the `EventSource` client in `web/static/app.js`:
  - Opens one stream from the page's `data-live-url`.
  - `slip-updated` and `board-updated` trigger the matching events on `document.body` for HTMX partials.
  - `bye` closes the stream without retrying.
  - Hiding the tab closes the stream; becoming visible again or gaining focus reopens it and triggers both refreshes to
    cover missed events.
- Tests:
  - Hub: topic isolation, multiple subscribers, burst coalescing, unsubscribe (including idempotence and leaving other
    subscribers alone), re-subscribing after a topic empties, and concurrent publish/unsubscribe.
  - Stream: two subscribers both receive slip and board signals, both deadlines send `bye` and close the connection,
    pings arrive as comments, and the anti-buffering headers are present.
- Docs: added `docs/concepts/live-updates.md`, removed the Live updates section from `plan/architecture.md`, and updated
  the architecture, plan README, and `docs/llms.txt` pointers.

### Phase 4: Slip tab

Tasks:

- [x] Add routes and handlers for change 2 rows in the routes table, mapping rule errors to 422, and a handler test that drives two SSE subscribers through an add-leg request
- [x] Build the Slip tab: header, member sections, leg form bottom sheet, lock and unlock, activity feed, the payout calculator with 300 ms debounce, and the profile page with home book and active toggles
- [x] Docs: add docs/reference/routes.md for built routes; delete those rows from plan/architecture.md

Phase totals: 3 checkpoints, 15m50s, $0.80, 26200709 input / 147306 output / 24617254 cache tokens

Checkpoints: task-9, task-10, task-11 Review fixes: 0 Integration checks: 0

#### Done

- Added the change 2 routes and handlers in `internal/web/slips.go`:
  - Slip tab and create flow: `GET /{$}`, `GET /slips/new`, `POST /slips`, `GET /slips/{id}`.
  - Partials: `GET /slips/{id}/panel`, `GET /slips/{id}/payout`, `GET /slips/{id}/activity`,
    `GET /slips/{id}/legs/new`, `GET /legs/{id}/edit`.
  - Mutations: add and remove member, `PATCH /slips/{id}`, add, update, and delete leg, lock in, and unlock.
  - Profile: `GET /me`, `POST /me`, `POST /users/{id}/active`.
  - Live stream: `GET /slips/{id}/events`, which calls `live.Stream.ServeSlip` with the slip and its week.
- Rule refusals from `internal/slips` and fixable store errors answer 422 with the short message. Missing slips, legs,
  members, and users answer 404. Anything outside the active group is also 404.
- Every slip and leg route resolves through `loadSlip`/`loadLeg`, so the active group guards the row before the service
  runs.
- Built the Slip tab at phone width:
  - Header with slip name, locked-in count, legs per member, and status chip.
  - One member section each, with avatar, lock state, legs (selection, odds, result chip), edit and delete, per-member
    add-leg gated on the remaining count, and lock-in once the member reaches the limit.
  - Leg form in a `<dialog>` bottom sheet for add and edit, with inline error rendering, preserved fields, and a close
    button. It closes on the `leg-saved` trigger after a successful save.
  - Lock in and unlock controls.
  - Payout calculator as its own partial. Inputs refresh it after a 300 ms debounce
    (`hx-trigger="input changed delay:300ms, slip-updated from:body"`), and it refreshes on live updates too.
  - Activity feed in a collapsed `<details>` block, newest first, with `<time>` elements formatted in the browser's
    zone.
  - Top-bar slip switcher lists the week's slips and navigates on change.
- Built the profile page with the home book form and switch-style active toggles. Inactive people stay visible so they
  can be turned back on.
- Added store helpers: `GroupMembers` (active and inactive, for the profile), `SetUserHomeBook`, `SetUserActive`,
  `Weeks`, `LatestWeek`, and `WeekByID`, with tests.
- Wired the hub, `slips.Service`, `live.Stream`, store, and group lookup into `cmd/server/main.go` through
  `web.NewWithDeps`.
- Added the EventSource client refinements: toast fallback for 422s outside a form error slot, `leg-saved` closes the
  sheet, and form errors clear before a retry.
- Added the bottom-sheet CSS in `web/tailwind/input.css` and regenerated `web/static/app.css`.
- Tests:
  - Two SSE subscribers on `/slips/{id}/events` both receive `slip-updated` after one add-leg request through the
    regular handler.
  - A leg past the limit answers 422 with the rule message.
  - The slip page advertises `data-live-url`, renders the slip, and carries the payout debounce trigger, sheet, and
    activity block.
  - The leg form has the close control, error slot, selection field, and game id.
  - The profile renders the home-book form and keeps inactive members visible with an "on" toggle.
  - Store tests cover `GroupMembers`, home book, active, and the week helpers.
- Docs: added `docs/reference/routes.md` covering middleware, every built route, status codes, HTMX headers, and SSE
  frames. Deleted the built route rows and the old 422/live-updates prose from `plan/architecture.md`, leaving only
  routes still to come. Regenerated `docs/llms.txt`.
- Verification: `go test ./...`, `go test -short -race ./...`, and `golangci-lint run ./...` pass. `go generate ./...`
  is idempotent for the checked-in CSS.

## Final gate

- Rounds used: 0; verify failures: 0
- Final verify: no commands configured
- Open issues: none
