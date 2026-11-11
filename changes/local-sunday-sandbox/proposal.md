---
slug: local-sunday-sandbox
status: implementing
created: "2026-11-10"
effort: medium
---

# Local Sunday Sandbox

## Context

`mise run dev` starts the real app with hot reload, but it does not create a repeatable Sunday to test. A fresh checkout starts with an
empty, persistent database and live sports-source clients. Picking realistic legs therefore depends on network access, current dates, and
odds-provider availability. The raw-response archive saves successful calls, but no request path can replay them.

The ingredients are scattered across lower-level tests. Committed ESPN, odds, weather, and projection fixtures cover scheduled and
partially live slates. Development sign-in accepts any email. Manual grading already settles a slip as soon as one leg loses, handles an
all-won slip, and permits corrections. None of this is assembled into a running local scenario. There is no committed sample history, no
complete all-final Sunday fixture, and no way to reset and advance a Sunday as one exercise.

Testing with friends' perspectives is also slower than it should be. Every new development email creates a separate group, so testing a
shared slip requires private browser windows, manual invites, and switching cookies. Existing history seeding cannot fix this cleanly: it
targets the first group, creates name-only users rather than sign-in identities, skips existing slips, and does not seed games or lines.

This makes routine UI and rule checks expensive, encourages consumption of scarce odds credits, and leaves important sequences—early
failure, every leg clearing, and the end of Sunday—without a fast browser-level rehearsal.

## Goals and non-goals

**Goals:**

- Add an opt-in, offline Sunday scenario to the normal development workflow, using committed past responses so no sports API keys,
  network access, or odds credits are needed.
- Start from a realistic Sunday morning with a shared group, several named personas, a full selectable slate, game lines and props, and
  enough context to exercise analyzer warnings and pick building.
- Make perspective changes quick and obvious while retaining real sessions, memberships, activity attribution, and group boundaries;
  include an outsider for invite and access checks.
- Provide deterministic reset and stage controls for at least Sunday morning, an early losing leg, every leg winning, and all games final.
  Each stage must update the visible games, picks, grading, activity, and live clients through the app's real rules.
- Keep sandbox state separate and disposable so a clean retry takes one command or action and does not require hand-deleting the normal
  development database.
- Prove the complete flows in automated tests, including replay without external calls, persona isolation, every requested outcome, and
  the absence of sandbox controls in production.

**Non-goals:**

- Changing production authentication, adding accounts or roles, or allowing persona switching outside local development.
- Building auto-grading or the Record Book; the sandbox may exercise those features when their separate change lands.
- Capturing new live data, simulating play-by-play, supporting every historical week, or replacing unit tests with a large browser suite.
- Adding sportsbook integration, money handling, scheduled jobs, another running service, or any LLM calls.
- Using this sandbox as production seed data or changing normal grading and refresh rules to accommodate it.

## Decision

Add a development-only scenario mode to the existing dev task rather than a separate mock app. A named Sunday scenario will combine
committed source responses with a fixed scenario clock, a shared group, email-backed personas, and realistic slips. Replaying responses
through the existing ingest and refresh paths keeps parsing, caching, and page behavior honest; staging results through the existing slip
rules keeps settlement, activity, and live updates honest. Sandbox mode must fail closed against external sports requests and be impossible
to enable in production.

An explicit reset will recreate the morning in isolated local state, and a small set of development controls will reset or advance to the
requested situations. Fixed timestamps and saved morning, live, and final snapshots will make old data behave like the intended point in a
Sunday instead of letting the real clock or a lazy refresh change the week. Tests will cover the same path a developer uses in the browser.

This beats live API setup because it is deterministic and free, history seeding because it includes the whole user and game context, and a
checked-in SQLite snapshot because source fixtures remain inspectable and exercise the real ingestion path. The effort is medium: the work
spans replay, clocks, identity, scenario controls, and production safeguards, but it reuses established clients, domain services, dev auth,
and test fakes rather than introducing a new product subsystem.
