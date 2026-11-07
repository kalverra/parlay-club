# Design: Live shared slips

## Context

`foundation` gives a server, the full schema, auth, and an empty tab shell. There is no NFL data yet, so legs here have
an optional game; `slate-and-lines` adds the game picker. Read before starting:

- `plan/product.md`: Slips, Legs, Editing and locking, Odds and payout, Screens (top bar, Slip tab).
- `plan/architecture.md`: Routes (change 2 rows), Live updates.
- `docs/reference/schema.md`.

## Approach

- `internal/parlay` is pure math: American/decimal conversion, combined decimal, payout with boost, units. Reference
  price takes the candidate prices as input, so it stays pure.
- `internal/slips` is the one place slip rules live. It takes a `Store`, a `Publisher` (the hub), and a `now` func. Each
  method checks the rules from product.md "Editing and locking", writes in one transaction with an activity entry,
  updates slip status, then publishes `slip:{id}`. It returns typed errors (e.g. `ErrLegLimit`) that handlers map to 422
  with a short message.
- `internal/live` holds the hub (topics, buffer-1 channels, non-blocking publish) and the SSE handler. The handler takes
  a `LiveWindow(weekID) bool` func so the deadline rule is testable before games exist; until `slate-and-lines`, it
  returns false.
- `web/static/app.js` gains the `EventSource` client: refresh triggers, `bye`, and close/reopen on visibility.
- Handlers in `internal/web` stay thin. Partials refresh on `slip-updated from:body`.
- Kickoff lock (rule 3) is coded and tested now with games inserted directly in tests.
- Groups later (`groups`): build for one club but keep the seams open. `internal/live` topics take a string key and the
  hub knows nothing about slip or week names, so `groups` can prefix a group id. Slip store methods take the acting user
  rather than reading a global club. Do not add a group table or column in this change.

## Verification

- `go test ./...` and `golangci-lint run ./...` pass.
- Parlay table tests cover both conversion directions, a 3-leg combo, profit boost, push legs dropped, and an even-count
  median reference price.
- Every rule in `plan/product.md` "Editing and locking" has a table test with an injected clock, including kickoff locks
  and a past-week slip.
- A handler test drives two SSE subscribers through an add-leg request and both receive the update; the stream sends
  `bye` at both deadlines, and the client reconnects when the tab becomes visible.
- At phone width the Slip tab shows members, legs, lock state, payout, and activity; a rule violation returns 422 and
  renders its message in the form.

## Risks

- Cloud Run and proxies may buffer SSE: set `Content-Type: text/event-stream`, `Cache-Control: no-cache`,
  `X-Accel-Buffering: no`, and flush after each write.
- iOS Safari drops `EventSource` in the background; visibility handling plus refresh-on-focus covers it.

## TODO

### Phase 1: Parlay math

- [x] Add internal/parlay conversions, combined odds, payout with boost, units, and median reference price, with table
  tests for both conversion directions, a 3-leg combo, boost, push legs dropped, and an even-count median
- [x] Docs: add docs/reference/odds-math.md; delete the Odds and payout section from plan/product.md

### Phase 2: Slip rules

- [x] Add Store methods for weeks, slips, members, legs, and activity, and create-slip defaults copied from the previous
  slip, with tests
- [ ] Add member and leg mutations, lock in, unlock, and slip status updates with one activity entry each, and table
  tests for every Editing and locking rule with an injected clock, including past-week slips
- [ ] Docs: add docs/concepts/slips.md; delete Slips, Legs, and Editing and locking from plan/product.md

### Phase 3: Live updates

- [ ] Add internal/live hub with topic subscribe, unsubscribe, and coalescing publish; test bursts and unsubscribe
- [ ] Add the SSE handler with pings and deadlines, plus the EventSource client with refresh triggers, bye handling, and
  visibility close/reopen; test slip-updated delivery and bye at both deadlines
- [ ] Docs: add docs/concepts/live-updates.md; delete Live updates from plan/architecture.md

### Phase 4: Slip tab

- [ ] Add routes and handlers for change 2 rows in the routes table, mapping rule errors to 422, and a handler test that
  drives two SSE subscribers through an add-leg request
- [ ] Build the Slip tab: header, member sections, leg form bottom sheet, lock and unlock, activity feed, the payout
  calculator with 300 ms debounce, and the profile page with home book and active toggles
- [ ] Docs: add docs/reference/routes.md for built routes; delete those rows from plan/architecture.md
