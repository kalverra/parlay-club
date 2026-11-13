---
id: adr-in-app-bug-reports
title: In-App Bug Reports
type: decision
summary: Add a bug report button to the shared app layout so it appears on every screen.
last_verified: "2026-11-12"
---

# In-App Bug Reports

## Context

When members hit a bug while building parlays, there is no easy way to report it. Someone has to take a manual
screenshot, switch to an external chat, and try to describe what went wrong. Often they say nothing at all, so bugs go
unnoticed.

When someone does report an issue in chat, key details are missing: the exact URL, screen size, user account, and
visible state of the page. Turning those messages into tracked bugs requires manual back-and-forth and extra work for
the maintainer.

## Goals and non-goals

**Goals:**

- Add a bug report trigger accessible from every screen.
- Capture a screenshot of the current page automatically when opening the report form.
- Let the user write a short description of what broke before sending.
- Post the bug report directly as an issue in the GitHub repository via the server.
- Allow any logged-in club member to submit reports without signing into GitHub.
- Include helpful context with the issue: screenshot, current route, logged-in user, and browser details.

**Non-goals:**

- Two-way comment threads or in-app issue management for members.
- Video recording or session replays.
- Supporting user-specific GitHub accounts or OAuth.
- Sending bug reports to third-party issue trackers other than GitHub.

## Decision

Add a bug report button to the shared app layout so it appears on every screen. Tapping the button takes a snapshot of
the current view, shows a quick preview, and provides a text field for the user's description.

Submitting the report sends the note, image, and page metadata to an authenticated server endpoint. The server
authenticates against the GitHub API with a repository token, uploads the image, and opens a new issue labeled as a bug
report.

```mermaid
flowchart TD
    subgraph Before["Before: Lost or Manual Feedback"]
        B1["User hits bug"] --> B2["Take manual screenshot"]
        B2 --> B3["Post in external chat or say nothing"]
        B3 --> B4["Maintainer manually recreates issue"]
    end
```

```mermaid
flowchart TD
    subgraph After["After: Direct GitHub Issues"]
        A1["User hits bug"] --> A2["Tap 'Report Bug' on current screen"]
        A2 --> A3["Client snaps screen + user enters note"]
        A3 --> A4["Server creates GitHub issue with screenshot"]
    end
```

This beats asking friends to file issues on GitHub directly, which fails because most members do not have GitHub
accounts. It also beats embedding an external feedback widget that adds tracking scripts and heavy dependencies.

The `effort: medium` rating fits because the change coordinates client-side screen capture across all views with
server-side GitHub issue creation and image handling, without changing existing betting logic.

## Consequences

- Signed-in members can report bugs from any page with one tap, attaching a viewport screenshot and page context without
  needing a GitHub account.
- Bug reports persist in SQLite before any GitHub API call, so GitHub downtime or an unset `GITHUB_TOKEN` never drops a
  report.
- Storing JPEG screenshot blobs in SQLite adds write volume to the single database file and Litestream replica; this is
  bounded by client-side 1280px downscaling, a 1.3 MB data-URL cap, and a 1 MiB decoded image limit on the server.
- `html2canvas-pro` (215 KB) is vendored in `web/static/` and lazy-loaded only when a signed-in user opens the bug
  report dialog, avoiding bundle cost on anonymous or normal page loads.

## Alternatives considered

- **User GitHub OAuth:** Rejected because most club members do not have GitHub accounts. A server-side `GITHUB_TOKEN`
  files issues on their behalf.
- **Third-party object storage (GCS/S3) for screenshots:** Rejected to avoid extra buckets, IAM configuration, and
  billing; SQLite is already replicated by Litestream, and capability URLs (`/screenshots/{token}.jpg`) serve images
  directly to GitHub's markdown proxy.
- **`navigator.mediaDevices.getDisplayMedia`:** Rejected because it triggers OS screen-sharing prompts and is
  unsupported on iOS Safari. In-page DOM capture runs without extra permissions.
- **`html2canvas` 1.4.1:** Replaced with `html2canvas-pro` 2.6.4 because Tailwind v4 emits `oklch()` and `color-mix()`
  colors that `html2canvas` 1.4.1 rejects.

## Implementation notes

- Go's `net/http.ServeMux` requires wildcards to span an entire path segment, so `GET /screenshots/{file}` is registered
  on the outer mux (outside `RequireUser`) and the `.jpg` suffix is validated inside `ScreenshotHandler`.
- `bug-report-dialog` starts with an empty `#bug-preview-slot` container rather than a blank `<img>` tag so pages
  without a user picture do not emit an `<img>` element; `app.js` inserts `<img id="bug-preview">` only when capture
  succeeds.
- Client-supplied context fields (`page_url`, `screen_size`, `user_agent`) are collapsed to one line and stripped of
  backticks before formatting GitHub issue markdown, and `description` is capped at 4000 runes to stay well below
  GitHub's 65,536-character issue body limit.

## Follow-ups

- Manual phone-width browser verification of `html2canvas-pro` capture on iOS Safari / Chrome against Tailwind v4
  `oklch()` styles.
- Configure `GITHUB_TOKEN` (and `GITHUB_REPO` if different from `kalverra/parlay-club`) in Cloud Run secrets for
  production issue filing.
