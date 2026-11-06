---
id: adr-google-oauth
title: Google sign-in
type: decision
summary: Replace the passcode and the name picker with Google's OpenID Connect sign-in.
last_verified: "2026-11-06"
---

# Google sign-in

## Context

Foundation shipped a shared passcode and a name picker. Anyone who knows the passcode gets in and can pick any name.
That was a stopgap, and `AGENTS.md` already says Google OAuth is the only auth model. The foundation close-out lists
this mismatch as a follow-up to settle before the profile screen is built.

The stopgap hurts in three ways. A user is just a typed name, so nothing ties a person to their history. Anyone can sign
in as anyone. And revoking one person means rotating the passcode for everybody.

## Goals and non-goals

**Goals:**

- Sign in with a Google account. No passwords, no reset flow, no account forms.
- Name and avatar come from Google. Nobody types a name or picks a color.
- Each person is one stable user, so slips, picks, and history follow them across devices.
- Only invited friends can get in for now. A stranger with a Google account is turned away with a friendly message.
- Existing users from the foundation and the history import keep their data.
- Local testing stays easy. A developer can sign in without a real Google round trip, and prod uses real Google sign-in.
- A user is a person, not a member of one fixed club. Nothing here should have to be undone when groups arrive.

**Non-goals:**

- Other sign-in providers, email and password, or magic links.
- Roles or permissions. Every signed-in friend can do the same things.
- Groups, solo play, or open sign-up. We expect to let more friends form their own groups or play alone. That is a
  separate proposal, and this one only makes sure it fits.
- Calling any Google API beyond sign-in. We read the profile and nothing else.
- Per-device session management or a "sign out everywhere" screen.

## Decision

Replace the passcode and the name picker with Google's OpenID Connect sign-in. The app sends people to Google, gets back
a verified email, name, and picture, and starts a signed session cookie. We keep no password and store only what Google
tells us: a stable Google ID, the email, the display name, and the picture. The rest of the app keeps reading "the
current user" from the request, so handlers do not change.

Access stays friendly and small. The group's emails are listed in config, and a sign-in from any other email is refused
politely. This keeps the app private without building invites or an admin screen. It also guards the scarce Odds API
credits, which more users would burn faster. The list is the only gate, kept in one place, so the groups change can
replace it with open sign-up or invites without touching the sign-in flow. Sign-in answers "who are you" and nothing
more; "which club are you in" stays a later question. On first sign-in, a Google account links to an existing user when
the emails match, so old slips and history survive. Users from the history import have names but no emails, so the
design must say how they get matched once, by hand.

Using the standard library flow beats rolling our own or adding a hosted auth service. Google is the only provider, the
app is one Go instance, and a signed cookie already works. The effort is medium: the login flow is small, but the user
table change, the one-time account linking, and the Google Cloud client setup for local and prod each need care.

## Consequences

Sign-in now identifies a person instead of a typed name. Every user has a stable Google subject, email, and picture, so
slips and history follow them across devices, and removing one friend from `ALLOWED_EMAILS` stops their next page load
without rotating anything for everyone else. The old `pc_auth` and `pc_user` cookies and the name picker are gone.

The gate moved into config. Adding or removing a friend means editing one environment variable and redeploying, not
building an admin screen. That is deliberate for a private club, but it puts account management on whoever runs the
deploy.

Google is now part of startup and sign-in. The server discovers Google's OIDC document at boot, so a Google outage can
stop an instance from starting. Prod requires four Google values plus `SESSION_KEY`; a missing one fails startup with
every problem listed.

The users table was rebuilt in migration `0002` to drop `UNIQUE(name)` and add the Google columns. That runs on real
data on the first prod deploy, so the deploy needs a Litestream snapshot first.

## Alternatives considered

- **Hosted auth service.** Rejected. One provider and one instance leave nothing to manage, and it adds a dependency and
  a bill.
- **Server-side session table.** Rejected. The signed `pc_session` cookie plus the per-request user read already gives
  revocation: an inactive user or a removed email is refused on the next request.
- **Rolling our own OAuth flow.** Rejected. `golang.org/x/oauth2` and `go-oidc` handle PKCE, state, nonce, signature,
  audience, and issuer checks with less code to get wrong.
- **Editing `0001_init.sql` in place.** The design first proposed this on the assumption there was no deployed data. The
  implementation used a `0002` rebuild instead, because the foundation and the history import can have real rows and the
  table already has foreign keys pointing at it.
- **A boot-time link step or CLI for history users.** Rejected. An `ALLOWED_EMAILS` entry of the form `email=Name` links
  one unlinked user on first sign-in, so there is no extra tool to run.
- **Open sign-up or invites.** Deferred. The gate is one small type, so the groups change can replace the config list
  without touching the sign-in flow.

## Implementation notes

- `auth.SignIn` takes the store and gate explicitly instead of reading package-level state. That is the seam the tests
  use, and `RequireUser` uses the same gate on every request.
- The five-step resolution order is: known subject refreshes the picture; the gate refuses a stranger before any write;
  a matching email links; an `=Name` entry links one unlinked history user and keeps its name; otherwise create from
  Google's profile. A linked user never gets renamed by Google.
- The known-subject path refreshes only the picture, not the email. If a Google account changes address, the stored
  email stays stale and updating `ALLOWED_EMAILS` to the new address will not let that person back in.
- Only the lowest-id unlinked user is linked for a given `=Name` entry. Two history-import rows with the same display
  name leave one unlinked.
- The provider discovers OIDC at startup and fails startup if Google is unreachable. That keeps the first callback fast
  but couples boot to Google.
- The dev login is registered only when `APP_ENV` is not prod and `GOOGLE_CLIENT_ID` is empty. It posts with the same
  CSRF exemption as logout, and it requires an email because `SignIn` cannot own a user without one.
- `pc_oauth` carries its own issue time, so the ten-minute limit is enforced server-side, not only by the cookie's
  `Max-Age`. `ErrNotInvited` leaves that cookie in the browser until it expires; it holds no session.
- A real Google round trip has not been run by a person. The fake provider and an `httptest` OIDC server cover the code
  paths, but the runbook's local and prod steps still need a manual pass.

## Follow-ups

- **Run the manual Google check.** Follow `docs/runbooks/google-oauth-setup.md` in dev and then prod: sign in, confirm a
  second account off the list sees the friendly 403, and check both pages at phone width.
- **Add a sign-out control.** `POST /logout` exists, but the shell has no button, so a person has to post the form by
  hand.
- **Refresh the email on the known-subject path, or document the limit.** Decide whether a Google address change should
  update `users.email`, and cover the gate interaction with a test.
- **Wire the avatar to the profile page.** The top bar links to `/me`, which lands later. Feed `web.User.PictureURL`
  from `db.User.PictureURL` wherever a page builds its view model.
- **Decide the fate of `ListActiveUsers`.** It has no production caller and its comment still mentions the deleted
  picker. Delete it or keep it explicitly for the member screen.
- **Deduplicate the email-shape check.** `auth.validEmail` and `config.looksLikeEmail` are the same rule in two
  packages; export one and reuse it.
- **Fix the `BASE_URL` advice in `docs/guides/local-dev.md`.** It tells readers to change `BASE_URL` when changing
  `PORT`, but config already derives the dev default from the port.
- **Decouple `internal/web` from `internal/db`.** `routes.go` takes a `func(context.Context) (db.User, bool)`; a view
  model built in `cmd/server` or `auth` would keep the web layer off the storage package.
- **Carry the new secrets into `history-and-deploy`.** Its Cloud Run flags must use `GOOGLE_CLIENT_ID`,
  `GOOGLE_CLIENT_SECRET`, `BASE_URL`, and `ALLOWED_EMAILS`, and must not reintroduce `PASSCODE`.
- **Take a Litestream snapshot before the first prod deploy.** Migration `0002` rebuilds `users` on real rows.
- **Handle duplicate names from the history import.** If two unlinked users share a name, only one can be linked through
  `=Name`; the import needs a merge path or a second email entry.
- **Consider lazy OIDC discovery.** Startup currently fails if Google's discovery endpoint is unreachable. A retry or
  first-use discovery would make an instance less brittle.
