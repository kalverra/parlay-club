# Phase 1 handoff: Land on groups and the hub

## Done

`GET /` is now the group list and every slip screen says what to do next.

- **Landing (`internal/web/home.go`, `web/templates/home.html`).** One card per membership, oldest first, showing the
  group name and the viewer's `NextStep` label on that group's newest slip of the latest week ("Pick 1 more", "Waiting
  on Ben", "Ready to place", "No slip yet"). A person in one group gets a 303 to `/groups/{id}/week` from a full page
  load; an HTMX request still gets the list. The `#group-list` element carries `data-refresh-on-visible` and
  `hx-get="/"`, and `app.js` triggers the refetch on `visibilitychange` and `focus`, so the page holds no SSE stream.
- **This week (`GET /groups/{id}/week`).** Checks membership (404 for a non-member, before any cookie is written), then
  makes the group active through a new `Dependencies.SetGroup` seam that `cmd/server/main.go` wires to
  `cookies.SetGroup`. It 303s to the group's newest slip of the latest week, or renders the create form. The shell
  marks the group the page acts in, not the one the cookie named a moment ago.
- **Group switching on slip links.** `loadSlip` no longer 404s a slip in another of the viewer's groups. It checks
  membership, reads the group, writes `pc_group` through the same seam, and renders the hub under that group. A slip
  outside every group the viewer belongs to still 404s, unchanged.
- **Joining a slip.** `slips.AddMember` gained `loadJoin`, so a group member who is not on the slip can add themselves
  while it is `OPEN`, with an "Ada joined" activity entry. Adding someone else still requires the caller to be on the
  slip, and everyone else still gets `ErrNotMember`.
- **Bottom bar (`internal/web/page.go`).** Groups (`/`), This week (`/groups/{active}/week`, falling back to `/` when
  the page has no active group), Record Book. `TabSlip`, `TabGames`, and `TabResearch` are gone. Games and Research
  keep full pages and leave the bar; the hub's pick action links to `/games`, and leg and game rows keep their
  Research links.
- **Hub (`internal/slips/next.go`, `internal/web/slips.go`, `web/templates/partials.html`).** The step banner sits at
  the top of `#slip-panel`: title, one line of detail, the deadline as
  `Picks close for MIA @ BUF at <time datetime="…">`, and one action. Pick links to `/games`; lock in posts
  `/slips/{id}/lock`; join posts `/slips/{id}/members` with the viewer's id. The old inline Lock in button is gone, so
  the banner is the only primary action. Members, legs, export, payout, and activity keep their order and partials, and
  the hub still opens one stream.
- **`slips.NextStep`.** A pure function over the slip, members, legs, the week's games, the viewer, and `now`. It
  returns the step (`join`, `pick`, `closed`, `lock_in`, `waiting`, `export`, `done`), legs still needed, names not
  locked in (member order), the first kickoff still ahead, and the result. `Label()` holds the shared copy. An empty
  week reads as `pick`, not `closed`; a graded slip wins over every other state; the last lock reads as `export` even
  before the status row catches up; unlocking drops back to `pick` or `lock_in`.
- **Store.** `Store.GroupByID` reads one group and wraps `ErrGroupNotFound`.
- **Tests.** `internal/slips/next_test.go` tables every step, the unlock path, the three-name list, the London 9:30
  a.m. ET first kickoff as the deadline, a past week with no deadline, an empty week, and each grade label.
  `internal/web/home_test.go` covers the list, the one-group redirect, the HTMX partial, the group switch from a card,
  the create form, the non-member 404, the slip-link switch, the outsider 404, and the three-tab bar.
  `internal/web/next_test.go` drives each banner through the injected clock, including the join round trip and the
  deadline disappearing once everyone locks. Layout, page, routes, and render tests moved to three tabs and the home
  page.
- **Docs.** `groups.md`, `access.md`, `routes.md`, and `slips.md` describe the group list, the week route, the cookie
  switch, the three-tab shell, the hub table, joining, and Research leaving the bar. Front matter sources and
  `last_verified` are updated, and `docs/llms.txt` carries the new `slips.md` summary.
- **CSS.** `web/static/app.css` was regenerated with the pinned Tailwind CLI; the checked-in build predated the
  banner's `bg-sky-950/50`, which would otherwise have rendered with no background.

Verification in this worktree: `go test -count=1 ./...` (every package passes), `golangci-lint run ./...` (0 issues),
`good-vibes docs lint` (0 errors; 15 pre-existing stale-source warnings in docs this change does not touch), and
`good-vibes check weekly-pick-flow` (passes).

## Deviations

- The design called join "the existing add-member route with the viewer's own ID". The rule required the caller to be
  on the slip, so joining would have failed. `AddMember` now falls through to `loadJoin` when the actor is the target
  and belongs to the group. The design sentence now says so.
- The hub's old Lock in button lived inside the legs section and read `CanLockIn`. It is removed so the banner holds
  the one primary action; `CanLockIn` stays on the view as data.
- Games and Research pages build their `Page` without a `Tab` and without `Page.Groups`, so they render the bar with
  no current tab, "This week" falls back to `/`, and the group switcher is hidden there. That was already true of their
  group data; making them non-tabs exposes it. Phase 2 deletes `/games` anyway, but the board page should carry the
  slip's group.
- `Store.GroupByID` was added. The design implied a group read for the week route and the switch but did not name it.

## Open issues

- The hub's pick action points at the old accordion Games board. Phase 2 must repoint it at `/slips/{id}/pick`. The hub
  test `TestHubBannerPicksAndShowsTheDeadline` asserts `href="/games"` and needs updating then.
- A group card says "No slip yet" whenever the group's newest slip is in an older week than the latest stored week.
  That reads correctly for "this week", but the card cannot open history; the week route shows the create form instead.
  Phase 4's empty states should decide whether that is the intended answer.
- The list has an empty state ("No groups yet. Start one from the groups page.") with no link, and with the one-group
  redirect it is unreachable: sign-up always creates "Just me". A solo person tapping Groups lands on This week. Phase 4
  owns what "only Just me" looks like.
- `loadLeg` still refuses a leg outside the active group, unlike `loadSlip`. A write from a stale tab after a group
  switch 404s instead of switching. Phase 2's replace route (`PUT /legs/{id}`) should resolve the leg's group the same
  way `loadSlip` does.
- Landing and every refetch-on-visible run four queries per group (slips, members, legs, games). Fine for a handful of
  groups on one SQLite instance; noted in the design's Risks.
- The hub shows no vote tallies yet; that is phase 3.
- `docs/guides/local-dev.md` and `docs/reference/sandbox-replay.md` still describe `/` as the slip. Phase 4 updates
  them, so the two-persona sandbox walk should start at the group list.
- The 15 stale docs are unrelated leftovers from the sandbox change (sources touched before this change). Left alone.

## Next-phase notes

- Phase 2 owns `internal/board`, `Store.OddsForWeek`, `GET /slips/{id}/pick` and its results partial, the tray,
  replace at the limit, `/games` becoming a 303, and `?slip=` on `/games/{id}/props`. The only hub change it needs is
  where `nextView` sends the `pick` action.
- Reuse `pageForGroup` for the board so the shell keeps the slip's group current, and keep the board's
  `data-live-url` the slip's stream so a page never opens two.
- `NextStep` is pure and settled; the tray's "Review and lock in" can read `StepLockIn` instead of re-deriving the
  limit. Phase 3 votes do not change the step function, because unlock already reopens `OPEN`.
- `Dependencies.SetGroup` is how any handler makes a group active in tests. `landingFixture` records the ids it was
  asked to set; a new fixture can do the same.
- This project configures no checks. The gate is `go test ./...` and `golangci-lint run ./...`, plus
  `good-vibes docs lint` and `good-vibes check weekly-pick-flow` when docs move.

## Files touched

- `cmd/server/main.go`
- `docs/concepts/access.md`
- `docs/concepts/groups.md`
- `docs/concepts/slips.md`
- `docs/llms.txt`
- `docs/reference/routes.md`
- `internal/db/groups.go`
- `internal/slips/next.go`
- `internal/slips/next_test.go`
- `internal/slips/rules.go`
- `internal/web/games.go`
- `internal/web/home.go`
- `internal/web/home_test.go`
- `internal/web/layout_test.go`
- `internal/web/next_test.go`
- `internal/web/page.go`
- `internal/web/page_test.go`
- `internal/web/render.go`
- `internal/web/render_test.go`
- `internal/web/research.go`
- `internal/web/routes.go`
- `internal/web/routes_test.go`
- `internal/web/slips.go`
- `web/static/app.css`
- `web/static/app.js`
- `web/templates/home.html`
- `web/templates/layout.html`
- `web/templates/partials.html`

## Run log

- Mode: 
- Checkpoints: task-1, task-2
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
