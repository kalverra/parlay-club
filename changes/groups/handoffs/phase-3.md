# Phase 3 handoff: Invites and screens

## Done

- `internal/db/groups.go` completes the group store. `ByToken` resolves an invite to its group, `JoinByToken` inserts a
  membership with `ON CONFLICT (group_id, user_id) DO NOTHING` inside a transaction, `ResetInvite` mints a fresh 128-bit
  token in place, `Leave` refuses the last group and deletes only the membership row, and `Members` lists the active
  people in a group. The errors callers branch on are `ErrInviteNotFound`, `ErrGroupNotFound`, `ErrLastGroup`, and
  `ErrNotMember`.
- `internal/groups.Store` gained `JoinByToken`, `ResetInvite`, and `Leave`. A second `InviteStore` embeds `Store` and
  adds `ByToken` and `Members`, so the screens use one interface and `*db.Store` still satisfies both.
- `internal/groups/handlers.go` mounts the group screens inside `RequireUser` and `Scope`:
  - `GET /groups` lists the caller's groups with the active one marked, each invite link, copy and reset, leave when
    they have another group, and the create form.
  - `POST /groups` creates a group, makes the caller its first member, makes it active, and returns to `/groups`.
  - `POST /groups/switch` checks membership and sets `pc_group`; `POST /groups/{id}/reset-invite` and
    `POST /groups/{id}/leave` reject non-members with 404 so a stranger cannot learn which groups exist.
  - `GET /join/{token}` shows "Join {group}?" with member names and an already-a-member state. `POST /join/{token}` adds
    the membership idempotently, makes the group active, and lands on `/`.
  - `Leave` moves the active cookie to the caller's remaining first group. Reset, switch, and leave answer HTMX with
    `HX-Refresh`, because the group list and the top-bar switcher both change.
- The login return path now runs through `pc_oauth`. `RequireUser` records a `/join/` path when it sends a signed-out
  visitor to `/login`, `googleStart` carries that path through the OAuth round trip, and both the Google callback and
  the dev sign-in land on `destination(saved.ReturnTo)`. `safeReturn` accepts only a `/join/` path; everything else
  falls back to `/`. The OAuth cookie's path moved from `/auth` to `/` so a join link can set it before sign-in, and
  `SetReturnTo` writes a return-only cookie that `OAuthState` deliberately rejects until a real round trip starts.
- The top bar has the group switcher. With one group it shows the group name as a link to `/groups`; with more than one
  it shows the active name and opens a bottom sheet of groups, each a `POST /groups/switch` form. `groups.Options` reads
  the memberships `Scope` already loaded, so the switcher costs no extra query. `app.js` copies the invite link and
  closes the sheet on an outside tap; `app.css` was regenerated for the new classes.
- `web/templates/groups.html` and `web/templates/join.html` render the two screens, and both are phone-first: truncating
  names, `min-w-0` inputs, wrapping action rows, and a sheet with safe-area padding.
- Tests:
  - `internal/db/groups_test.go` covers invite lookup, join idempotence, reset invalidation, last-group refusal,
    non-member leave, and member ordering.
  - `internal/groups/handlers_test.go` wires the real auth, scope, store, cookies, and templates. It covers the join
    page, unknown and reset links, joining twice, leave, switch, create, blank names, non-member 404s, and the full
    signed-out invite flow through `/login` and back.
  - `internal/auth/handlers_test.go` and `middleware_test.go` cover the `pc_oauth` return path on both the Google and
    dev flows, including rejecting a non-join target.
  - `internal/web/layout_test.go` covers the switcher with one group and with several.
  - `cmd/server/main_test.go` keeps the scope wired around the shell and checks the fallback group sticks.
- Docs: added `docs/concepts/groups.md`; updated `docs/concepts/access.md`, `docs/reference/schema.md`,
  `docs/reference/config.md`, `docs/guides/local-dev.md`, `docs/runbooks/google-oauth-setup.md`, and `docs/llms.txt`;
  fixed the Routes and middleware rows in `plan/architecture.md`; and removed the Record Book group note from
  `changes/grading-record-book/proposal.md`, since `docs/concepts/groups.md` now owns it.
- `mise x -- go test ./...` passes, `mise x -- golangci-lint run ./...` reports 0 issues, and `good-vibes check`,
  `good-vibes docs lint`, and `good-vibes docs stale` are clean.

## Deviations

- The docs task named `docs/concepts/slips.md` and `docs/concepts/live-updates.md`, but neither file exists on this
  branch. The group rules that would go in them are covered in `docs/concepts/groups.md`, which says the slip routes and
  live streams ship with the changes that build them. Those two docs still need their group sections when they land.
- The signed-out invite redirect lives in `auth.RequireUser`, not in the join handler. The handler never runs for a
  signed-out visitor, and `RequireUser` is the one place that already owns the sign-in redirect and the `pc_oauth`
  cookie. The join handler still redirects to `/login` if it is mounted without `RequireUser`.
- The join page's "sees the slips" ending is `/` for now, because this branch has no slip list. The cookie and group
  switch are already right, so the existing home page will show the joined group's slips when the slip work lands.
- Reset, leave, and switch use a full `HX-Refresh` rather than swapping a partial. The group list, the active badge, and
  the top bar all change together, so a reload is simpler and avoids duplicating both screens in fragments.
- Creating a group makes it active. The design says the page has a create form but does not say which group becomes
  active; switching to the new group avoids creating one and then not seeing it.
- Invite links use the request's host and `X-Forwarded-Proto` rather than `BASE_URL`. They stay correct locally and on
  Cloud Run, and they do not depend on config being loaded into the handler.
- `Members` lists active users only. The design says "member names"; inactive users cannot sign in, so showing them on
  an invite page would be misleading. The membership rows themselves are untouched.
- Phone-width fit was checked by structure (truncation, wrapping, `min-w-0`, safe-area padding) and template tests, not
  in a browser.

## Open issues

- The slip, SSE, Record Book, home-list, and seed work from Phase 2 is still out of this branch, exactly as the Phase 2
  handoff says. `docs/concepts/groups.md` describes those rules, but no code implements them here. Do not assume slip
  routes, `group:{id}`, membership-checked streams, group filters, or the club seed exist.
- A group can become empty without being deleted. Two people who each have another group can both leave, and the group
  keeps its slips and a working invite link. That matches "leave removes the membership only", but nobody can see the
  slips until someone joins again. If that is unwanted, a later change needs an ownerless-group cleanup rule.
- `/groups` calls `ForUser` while `Scope` has already loaded the same memberships. It is one extra query per page view.
  Reusing the context memberships would need an exported accessor; not worth a second seam yet.
- Invite URLs are built from the request host. If a proxy ever fails to send `X-Forwarded-Proto`, the copied link would
  say `http` on an HTTPS page.
- `slips.group_id` is `NOT NULL` with no backfill. Delete an existing `data/parlay.db` before running the app.
- `db.UnlinkedUserByName` still has no caller. The history seed must set an email or create users before their first
  sign-in.

## Next-phase notes

- The waiting changes (`slips-live`, `slate-and-lines`, `history-and-deploy`) should build slips group-aware from the
  start: take the group from `Actor` or `groups.From(ctx)`, and put `slips.group_id` in every query filter. Do not
  filter rows in handlers.
- Every route that names a slip must resolve that slip's group and check membership, whatever `pc_group` says. A
  non-member gets 404. A member opening a slip from another of their groups calls `SetGroup` before rendering.
- Add the `group:{id}` topic when a slip is created or renamed, and check membership before subscribing an SSE client to
  `slip:{id}`. `week:{weekID}` stays global and must carry no group data.
- The home list and Record Book read the active group. `internal/seed` should create "Sunday Parlay Club" and its
  members when missing, then load `seed/history.yaml` into it.
- Add the route-table isolation test with the slip routes: two groups, a non-member 404 or no other group's rows on
  every slip route, the SSE stream, the home list, and the Record Book. A route that forgets `Scope` or the slip-group
  guard should fail it.
- When `docs/concepts/slips.md` and `docs/concepts/live-updates.md` exist, fold in the group sections from
  `docs/concepts/groups.md` and link back. Run `good-vibes docs index` after adding or renaming docs.

## Files touched

- `changes/grading-record-book/proposal.md`
- `cmd/server/main.go`
- `cmd/server/main_test.go`
- `docs/concepts/access.md`
- `docs/concepts/groups.md`
- `docs/guides/local-dev.md`
- `docs/llms.txt`
- `docs/reference/config.md`
- `docs/reference/schema.md`
- `docs/runbooks/google-oauth-setup.md`
- `internal/auth/cookies.go`
- `internal/auth/cookies_test.go`
- `internal/auth/handlers.go`
- `internal/auth/handlers_test.go`
- `internal/auth/middleware.go`
- `internal/auth/middleware_test.go`
- `internal/db/groups.go`
- `internal/db/groups_test.go`
- `internal/groups/groups.go`
- `internal/groups/handlers.go`
- `internal/groups/handlers_test.go`
- `internal/groups/scope_test.go`
- `internal/web/layout_test.go`
- `internal/web/page.go`
- `internal/web/render.go`
- `internal/web/render_test.go`
- `internal/web/routes.go`
- `internal/web/routes_test.go`
- `plan/architecture.md`
- `web/static/app.css`
- `web/static/app.js`
- `web/templates/groups.html`
- `web/templates/join.html`
- `web/templates/layout.html`

## Run log

- Mode: 
- Checkpoints: task-5, task-6, task-7
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
