# Phase 4: Slip tab

## Done

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

## Deviations

- There is no current-week concept yet. `GET /{$}` and `GET /slips/new` use `LatestWeek` as a stand-in so the flow
  works before the slate lands. The design and architecture say "current week"; slate-and-lines must define that and
  switch these handlers.
- The leg form carries an optional `game_id` hidden field and accepts `?game_id=`, but there is no game picker. The
  slate change adds it; the field already flows through the form and store.
- The payout calculator is a separate partial outside `#slip-panel`. That keeps an SSE panel swap from resetting the
  stake someone typed, which the design implied but did not spell out.
- The profile needs inactive members, so the handlers call the new `GroupMembers` rather than `Members`. Pickers and
  create-slip defaults still use active members only.
- `cmd/server/main.go` passes `nil` for `IsLive` instead of a func that always returns false. `live.NewStream` treats
  nil the same way, so every stream uses the 10-minute idle deadline.
- The docs task said to delete the change 2 rows from `plan/architecture.md`. Because `routes.md` documents every built
  route, the change 1 and groups rows went too, along with the now-duplicated 422 and live-updates prose.
- `data-live-url` sits on the slip section, not the body. The client uses `document.querySelector("[data-live-url]")`,
  and pages without a slip have no attribute.
- There is still no automated browser test for the JS. The repository has no JS test runner; the SSE behavior is
  covered by handler tests, and the sheet, toast, and reconnect paths are manual checks.

## Open issues

- `slate-and-lines` must introduce the current week and update `home` and `newSlipForm`. Until then, a week with no
  games can still be opened and used.
- `IsLive` is nil in production and there is no `week:{weekID}` publisher. `internal/refresh` must publish
  `week:{weekID}` when a sync changes games or odds, publish affected `slip:{id}` topics, and the server must pass a
  game-aware `IsLive`.
- `board-updated` fires with no listener because no Games board exists yet. The slate change must add
  `hx-trigger="board-updated from:body"` to the board partial.
- Payout uses the combined odds of every leg on the slip, including legs still `PENDING`. Grading and push handling
  belong to later changes; pushes are already dropped by `internal/parlay` when results exist.
- No game-aware locks are visible in the UI yet beyond the rule's 422 message. Kickoff locks are enforced by
  `internal/slips` and tested there.
- `web/static/app.css` is a checked-in minified build. Run `go generate ./...` after any template or CSS class change,
  or new utilities will be missing at runtime.
- `go generate` needs the pinned Tailwind CLI from `mise.toml`; the plain `go test` flow does not rebuild CSS.

## Next-phase notes

- Add new authenticated routes in `slipHandlers.Routes` (or a sibling handler) and mount it from `web.NewWithDeps`. The
  `Dependencies` struct already carries `Store`, `Slips`, `Stream`, `UserFrom`, `GroupFrom`, and `GroupsFrom`.
- Keep handlers thin: parse, call `h.deps.Slips`/`h.deps.Store`, then `h.renderPage` or `h.renderPartial`. Map service
  errors with `h.respondError`, which already sends 422 for rule errors and 404 for missing rows.
- New page templates go in `pageFiles` (`internal/web/render.go`); new HTMX fragments go in `partialFiles`.
- Live-refreshed fragments follow the existing pattern:
  `hx-get="/..." hx-trigger="slip-updated from:body" hx-swap="outerHTML"`. The Games board should use `board-updated`
  in the same shape.
- `internal/web/slips_test.go` has a `newSlipFixture` helper that wires the real store, hub, slips service, and stream
  with stubbed user and group. Reuse it for slate handler tests.
- For the two-subscriber check, open `httptest` clients against `/slips/{id}/events`, then make the mutation through the
  normal handler and read one `event:` line from each body.
- The hub stays in-process. Do not add a second Cloud Run instance or move it out of memory without replacing the
  fan-out.

## Files touched

- `cmd/server/main.go`
- `docs/llms.txt`
- `docs/reference/routes.md`
- `internal/db/groups.go`
- `internal/db/groups_test.go`
- `internal/db/users.go`
- `internal/db/users_test.go`
- `internal/db/weeks.go`
- `internal/db/weeks_test.go`
- `internal/web/layout_test.go`
- `internal/web/render.go`
- `internal/web/render_test.go`
- `internal/web/routes.go`
- `internal/web/slips.go`
- `internal/web/slips_test.go`
- `plan/architecture.md`
- `web/static/app.css`
- `web/static/app.js`
- `web/tailwind/input.css`
- `web/templates/partials.html`
- `web/templates/profile.html`
- `web/templates/slip.html`

## Run log

- Mode: 
- Checkpoints: task-9, task-10, task-11
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
