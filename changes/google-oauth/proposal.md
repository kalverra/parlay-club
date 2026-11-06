---
slug: google-oauth
status: implementing
created: "2026-11-05"
effort: medium
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
