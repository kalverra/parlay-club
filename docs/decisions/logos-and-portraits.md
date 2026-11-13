---
id: adr-logos-and-portraits
title: Team Logos and Player Portraits
type: decision
summary: Capture official team logo and player headshot URLs from the existing ESPN scoreboard and roster syncs, store them alongside our game and roster records, and surface them in view models for screens where visual identity aids scanning.
last_verified: "2026-11-12"
---

# Team Logos and Player Portraits

## Context

Every NFL team and player across the app renders as plain text today. Pick-board slate cards show bare abbreviations
like `BUF @ MIA`, prop tabs and research headers show unstyled player names, and slip legs list text only—even though
member rows already show user profile photos with a clean fallback.

On a phone screen, walls of three-letter team codes and player names are slower to scan when browsing a full Sunday
slate, comparing player props, or reviewing a friend's slip. Official team logos and player headshots are already
included in the ESPN scoreboard and roster responses the app fetches, but we currently discard them during ingestion.

## Goals and non-goals

**Goals:**

- Show official team logos where games, matchups, and team picks appear, including pick-board slate cards, game headers,
  slip legs, and game research.
- Show official player portraits where a player is the subject, including player prop groups, player research headers,
  and matched player legs.
- Source logos and portraits from the existing ESPN scoreboard and roster syncs without adding new API calls or spending
  extra credits.
- Fall back cleanly to text or initials when an image is missing, fails to load, or belongs to a custom or unlinked
  historical pick, keeping phone-width layouts uncluttered.

**Non-goals:**

- Bundling, mirroring, or proxying copyrighted NFL logos or player photos from the app server.
- Crowding dense inline elements—such as quick-add prop pills, sort chips, or dropdown options—with thumbnails that
  break phone-width alignment.
- Scraping extra sites or adding a new data provider to backfill missing headshots.
- Changing odds math, player matching rules, or slip behavior.

## Decision

Capture official team logo and player headshot URLs from the existing ESPN scoreboard and roster syncs, store them
alongside our game and roster records, and surface them in view models for screens where visual identity aids scanning.
Render logos and portraits through reusable UI helpers that degrade cleanly to initials or plain text when a URL is
absent or fails to load, reserving images for cards, headers, and legs rather than compact buttons.

```mermaid
flowchart LR
    subgraph Before["Before: Text-Only Entities"]
        B1["ESPN scoreboard & roster"] --> B2["Keep IDs & names only"]
        B2 --> B3["Render plain text & codes"]
    end
```

```mermaid
flowchart LR
    subgraph After["After: Official Visuals with Fallbacks"]
        A1["ESPN scoreboard & roster"] --> A2["Keep logo & headshot URLs"]
        A2 --> A3["Render logos & portraits on cards, headers, legs"]
        A2 -. "missing or broken" .-> A4["Text or initials fallback"]
    end
```

Reading the URLs directly from the ESPN responses we already fetch beats hardcoding static team maps or guessing CDN
paths by ID, because ESPN stays the source of truth when teams rebrand or players change photos without adding any extra
network calls. It also beats self-hosting or proxying images, avoiding asset licensing headaches, repository bloat, and
Cloud Run bandwidth use.

The `effort: easy` rating fits because the image URLs already live in existing ESPN payloads and the work follows
established ingestion, storage, and template patterns with minimal architectural uncertainty.

## Consequences

- Visual scanning on phone-width screens is faster because team logos and player portraits appear across slate cards,
  detail headers, player prop groups, slip legs, and research views without adding network requests or spending Odds API
  credits.
- Direct `https://` ESPN CDN URLs avoid asset licensing, repo bloat, and Cloud Run egress, at the cost of depending on
  ESPN CDN availability (mitigated by lazy loading and client-side fallback badges).
- Adding a shared template partial (`web/templates/media.html`) requires keeping both `Renderer.parse` and
  `Renderer.RenderPartial` in sync when new partials are introduced, and regenerating `web/static/app.css` via
  `go generate ./web` whenever template utility classes change.

## Alternatives considered

- **Separate `teams` table for logos:** Rejected in favor of storing `home_logo`/`away_logo` on `games` and
  `headshot_url` on `roster_players` so scoreboard and roster upserts stay single-table without extra joins or table
  synchronization.
- **Hardcoded team maps or guessed CDN paths:** Rejected because ESPN scoreboard and roster payloads already supply
  authoritative URLs that stay current across rebrands and roster photo updates.
- **Bundling or proxying images from the app server:** Rejected to avoid copyrighted asset storage, binary growth, and
  Cloud Run bandwidth overhead.
- **Inline `onerror` attributes on `<img>` tags:** Rejected in favor of a single capturing `error` listener on
  `document` in `web/static/app.js` so templates stay CSP-clean and HTMX partial swaps work automatically.

## Implementation notes

- The design planned migration `0010_logos_portraits.sql`, but `0010_bug_reports.sql` already occupied version 10 on
  `main`; the migration was created as `0011_logos_portraits.sql` instead.
- In `web/templates/media.html`, `[data-fallback-badge]` uses `hidden` when an image URL is present and `flex` only when
  the URL is empty, because Tailwind v4 emits display utilities after `.hidden` in `web/static/app.css`.
- Cross-game team logo lookups in `Store.TeamLogos` derive logos from synced `games` rows, so teams without a synced
  game yet fall back to text or initials.

## Follow-ups

None.
