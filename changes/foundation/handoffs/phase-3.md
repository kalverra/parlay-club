# Phase 3 handoff: Passcode and user picker

## Done

- `internal/db` now has a `Store` (`store.go`) and the user queries (`users.go`). `User` carries ID, name,
  `AvatarColor`, `HomeBook`, and `Active`. `ListActiveUsers` returns active users sorted by name, case-insensitive.
  `CreateUser` runs in one transaction, rejects a duplicate name ignoring case with `ErrNameTaken`, and assigns the next
  palette color round robin over every stored user, inactive ones included. `UserByID` returns `ErrUserNotFound` for a
  missing row and hands back the `Active` flag so callers can refuse an inactive user.
- The avatar palette is eight names (`sky`, `rose`, `amber`, `emerald`, `violet`, `orange`, `teal`, `pink`) in
  `internal/db`. `AvatarColor.Class()` maps a name to a literal Tailwind class (`bg-sky-500`, …), falling back to
  `bg-slate-600` for a name outside the palette. `go generate` compiles the classes because they are literal strings in
  `internal/`, which `web/tailwind/input.css` scans.
- `internal/auth/cookies.go` signs and verifies both cookies. `pc_auth` is `HMAC-SHA256(SESSION_KEY, PASSCODE)` as
  lowercase hex; `pc_user` is `{id}.{HMAC-SHA256(SESSION_KEY, "user:"+id)}`. Both are `HttpOnly`, `Path=/`,
  `SameSite=Lax`, one year, and `Secure` in prod. MACs compare with `hmac.Equal`, and `PasscodeMatches` compares the
  submitted passcode with `subtle.ConstantTimeCompare`. `UserID` also rejects non-canonical IDs such as `042`.
- `internal/auth/handlers.go` serves the screens. `GET /login` shows the form; `POST /login` waits one second and
  re-renders with 401 on a wrong passcode, or sets `pc_auth` and redirects 303 to `/who`. `GET /who` lists active users
  with initials and avatar color plus the add-name field. `POST /who` signs in only an existing active user (422
  otherwise) and redirects to `/`. `POST /users` trims the name, enforces 1–24 characters, maps `ErrNameTaken` to 422,
  keeps the typed name on an error, signs in the new user, and redirects to `/`. All three pages send
  `Cache-Control: no-store`, and forms are capped at 4 KiB. The wrong-passcode pause is injected, so tests record the
  delay instead of sleeping.
- `internal/auth/middleware.go` adds `RequirePasscode`, `RequireUser`, and `CSRF`. `RequirePasscode` sends a missing or
  tampered `pc_auth` to `/login`. `RequireUser` loads `pc_user`, sends a missing, tampered, unknown, or inactive user to
  `/who`, and puts the `db.User` on the request context for `auth.UserFrom`. `CSRF` rejects any non-GET request without
  `HX-Request: true`, except `POST /login`, `POST /who`, and `POST /users`, with a 403 and a warning log. Both redirects
  send a 302 normally, or 200 with `HX-Redirect` for an HTMX request.
- `web/templates/login.html` and `web/templates/who.html` follow the shared `content` convention. The layout hides the
  top bar when it has nothing to show and the tab bar when the page has no tab, so the auth screens stand alone.
  `internal/web` registers both pages in `pageFiles`, and `Renderer.Render` now takes `any` data. `LoginPage` and
  `WhoPage` embed `web.Page` and add their own fields.
- `cmd/server` builds the store, cookie signer, and auth handlers, then `newMux(shell, static, handlers)` registers
  `/healthz`, the static handler, and the auth screens, and wraps the shell in `RequirePasscode → RequireUser`, all
  inside `CSRF`. `web.New` still serves the shell's own `/static` route; the outer mount handles it first.
- Tests: store color rotation and wrap, duplicate and inactive handling, ordering; cookie wire format and every
  attribute; passcode matching; handler flows for login, who, and add-name including the one-second pause and 422 paths;
  and the middleware matrix for missing, tampered, unknown, and inactive cookies, `HX-Redirect`, and CSRF rejects and
  exemptions. `cmd/server` checks health, the shell redirect, and static bypassing auth.
- Docs: added `docs/concepts/access.md`; updated `docs/guides/local-dev.md` with the sign-in step and cookie lifetime,
  and added `internal/auth/*.go` to its sources; regenerated `docs/llms.txt`. Deleted `## Users and access` from
  `plan/product.md` and `## Auth` from `plan/architecture.md`, and updated the `plan/README.md` doc table.

Checks run in this phase: `go generate ./...` leaves no diff, `golangci-lint run ./...` is clean,
`go test -short -race ./...` passes, `rumdl check docs plan` is clean, and `good-vibes docs lint` reports ok.

## Deviations

- The schema comment stores a Tailwind color name such as `sky`, not a full class string. Phase 2's note suggested full
  classes in `internal/db`; storing names keeps rows small and lets `AvatarColor.Class()` own the mapping. The class
  strings still live as literals in `internal/db`, so Tailwind compiles them.
- `POST /users` needs the passcode but not a user cookie. A literal reading of the plan would require both, but a
  first-time visitor has no user yet, and adding a name is how they get one. `/who` is likewise passcode-only.
- `/static` is registered on the outer mux as well as inside `web.New`, so the login page can load its CSS and JS before
  auth. The outer pattern wins; the inner route is now redundant but keeps `web.New` self-contained and its tests
  intact.
- Auth page data embeds `web.Page`, which required `Renderer.Render` to accept `any` instead of `Page`. The layout still
  reads the same fields through promotion.
- The layout hides the top bar when there is no week, no user, and no slip switcher, and hides the tab bar when
  `Page.Tab` is empty. That gives auth pages a clean frame without a second layout file.
- Auth pages set `no-store`; shell pages still do not. Deciding the shell's cache policy belongs with the pages that
  first carry a user's data (change 2).
- The Go source spells the headers `Hx-Request` and `Hx-Redirect` to satisfy `canonicalheader`. Header names are
  case-insensitive, and a live run confirmed HTMX receives `HX-Redirect` correctly.
- `CSRF` treats only `GET` as safe, following the plan's "every non-GET" wording. `HEAD` therefore gets a 403. Nothing
  in the app issues `HEAD` today.
- There is still no profile page, home-book editing, or active-toggle UI; those belong to change 2. The `User` struct
  and the middleware already expose `HomeBook` and `Active` for it.

## Open issues

- The shell handlers do not pass the signed-in user to the layout yet. `Page.User` stays nil, so no avatar shows and the
  `/me` link is unreachable. Change 2 must read `auth.UserFrom` in the shell handlers and fill
  `User{Name, Initials, AvatarClass}`; `db.User` maps to it directly except for initials.
- Shell HTML still sends no `Cache-Control`. Add `no-store` to pages that carry a user's data when change 2 starts
  rendering it.
- `HEAD` requests to protected routes return 403 through `CSRF`. Revisit if a browser, crawler, or CDN starts sending
  `HEAD`.
- There is no logout or switch-user control. Visiting `/who` with a valid passcode cookie works as a switch, and
  `POST /who` overwrites `pc_user`, but nothing links there yet. The profile in change 2 should add it.
- `GET /login` renders even when the browser is already signed in. Harmless today; a redirect there would need a rule
  for a valid passcode with no user.
- The one-second pause is the only brake on passcode guessing. There is no lockout or rate limit, which is acceptable
  for a private group behind a long passcode but worth remembering if the URL leaks.
- The CSRF exemption list is hardcoded to three method-plus-path pairs. Any new plain form in change 2 (`POST /me`,
  active toggle) needs either `HX-Request: true` or a deliberate addition to that list.
- `internal/web.pageFiles` is the registry for every page, including the auth screens. A new template that is not added
  there fails at startup with "unknown page".
- Phase 2 nits still open: dev static files set the one-year immutable cache even though `go generate` can rewrite
  `app.css`, and a bare `/static/` lists the embedded file names. Both are harmless for now.
- `auth.Handlers` takes the concrete `*db.Store`; tests use a real temp database. If later handlers need a fake store,
  introduce a small interface then.
- Deactivating a user has no UI yet, but the behavior is live: `RequireUser` re-reads the row and sends an inactive user
  to `/who` on their next request.

## Files touched

- `internal/db/store.go`, `internal/db/users.go`, `internal/db/users_test.go`
- `internal/auth/cookies.go`, `internal/auth/cookies_test.go`
- `internal/auth/handlers.go`, `internal/auth/handlers_test.go`
- `internal/auth/middleware.go`, `internal/auth/middleware_test.go`
- `web/templates/login.html`, `web/templates/who.html`, `web/templates/layout.html`
- `internal/web/render.go`, `internal/web/render_test.go`
- `cmd/server/main.go`, `cmd/server/main_test.go`
- `web/static/app.css`
- `docs/concepts/access.md`, `docs/guides/local-dev.md`, `docs/llms.txt`
- `plan/README.md`, `plan/architecture.md`, `plan/product.md`

## Next-phase notes

- Change 2 should build the shell's page data from `auth.UserFrom`: name, `web.Initials(name)`, and
  `user.AvatarColor.Class()`. That turns on the avatar the layout already renders and makes `/me` worth adding.
- Register new shell routes inside `internal/web`; they are already behind passcode and user checks because `newMux`
  wraps the whole shell. Auth-owned routes stay in `auth.Handlers.Routes`.
- Any new non-GET route from HTMX must send `HX-Request: true`. Plain forms (`/me` and the active toggle) are not exempt
  yet, so either make them HTMX forms or extend `csrfExempt` in `internal/auth/middleware.go`.
- `db.User` carries `HomeBook` and `Active`; there are no update methods for either. Change 2 adds the profile queries
  next to `UserByID`, and `ErrUserNotFound` / `ErrNameTaken` are the existing sentinel style to follow.
- Add new page templates to `internal/web.pageFiles`; auth page data types embed `web.Page`, so their content templates
  can use both layout and page fields. The layout hides chrome when the data is empty, not only for auth.
- `/static` is currently reachable through both the outer mux and the shell mux. New shell routes do not need to repeat
  it; if the duplication is tidied later, keep the outer mount so the login page can load assets.
- The wrong-passcode wait is injected through `auth.NewHandlers`; keep any new slow-path tests recording the requested
  delay rather than sleeping.
- Docs staleness: `docs/concepts/access.md` sources the auth files, `internal/db/users.go`, `cmd/server/main.go`, and
  the auth templates. `docs/guides/local-dev.md` also sources `internal/auth/*.go`. Changing those marks the docs stale,
  which is correct.

## Run log

- Mode: balanced
- Checkpoints: task-11, task-12, task-13, task-14, task-15, task-16
- Verify: passed
- Rounds used: 1 of 3
- Nits:
  - phase 2, task 6: `web/tailwind/input.css` has `@source "../templates"`, but `web/templates/` does not exist yet. Tailwind runs fine without it, and later phases add templates, so no action needed now.
  - phase 2, task 9: in dev, `StaticHandler` serves from the embedded FS and sets the one-year immutable cache on every response. After `go generate` rewrites `app.css`, a dev browser keeps the stale file until the server is rebuilt (which changes the `?v=` hash) or the cache is cleared. Consider a shorter/no-cache header when `APP_ENV=dev`, or serving static from disk in dev like templates.
  - phase 2 handoff: The bare-`/static/` directory-listing open issue is asserted but not covered by a test; a one-line test would pin the behavior either way.
  - phase 3, task 11: `plan/product.md` requires names to be 1–24 characters, but `CreateUser` (internal/db/users.go:94) does no length
  - phase 3, task 11: `TestAvatarColorClass` (internal/db/users_test.go) iterates a hardcoded color list instead of the palette, so a
  - phase 3, task 14: `CSRF` (internal/auth/middleware.go) exempts only GET, so HEAD and OPTIONS get 403. That matches the plan's
  - phase 3, task 16: Deleting "Users and access" from plan/product.md also dropped two not-yet-built details: "any user can toggle

## Reviewer check

- Reviewer 1 (opencode:top): approved
