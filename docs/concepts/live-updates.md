---
id: live-updates
title: Live updates
type: concept
summary: How a slip page hears about changes through one SSE stream, and how the hub, deadlines, and client reconnect keep that stream cheap.
tags: [live, sse, hub, eventsource, deadlines, hub-topics]
sources: [internal/live/hub.go, internal/live/stream.go, internal/refresh/live.go, internal/slips/rules.go, web/static/app.js]
last_verified: 2026-11-07
---

# Live updates

## Summary

A slip page keeps one `EventSource` open. Events carry no HTML; they only say that something changed. The page then
refetches the partials it already shows, so every person gets fresh data without per-person HTML crossing the hub.

## Signals, not payloads

A change signal never includes slip data. It names a topic, and the browser reacts by asking HTMX to refresh.

- `slip-updated` refreshes the slip panel and its activity.
- `board-updated` refreshes the game board.

This keeps the hub small, avoids leaking one person's view to another, and means a missed event does no harm: the next
refetch catches up.

## Hub (`internal/live`)

The hub is an in-memory fan-out keyed by string topics. It knows nothing about slips, weeks, or groups.

- `Subscribe(topic)` returns a receive-only channel and an unsubscribe func. The channel holds one signal.
- `Publish(topic)` never blocks. If a listener's buffer is already full, the signal is dropped because one wake-up is
  enough for a burst.
- Unsubscribe is safe to call more than once and removes the topic when its last listener leaves.
- The zero value `Hub` is ready to use.

Topics are plain strings:

| Topic           | Published by               | Meaning                             |
| --------------- | -------------------------- | ----------------------------------- |
| `slip:{id}`     | `internal/slips`           | A mutation committed on that slip   |
| `week:{weekID}` | `internal/refresh`         | Games or odds changed for that week |

`internal/slips` publishes `slip:{id}` once, after the transaction commits. A `Publisher` is one method,
`Publish(topic string)`, so `*live.Hub` satisfies it and tests can record topics. Group scoping later means prefixing a
group id; the hub does not need to change.

The hub lives in the process. It only works because the app runs as a single Cloud Run instance.

## Stream (`GET /slips/{id}/events`)

`live.Stream.ServeSlip` serves one connection for one slip and its week.

- It subscribes to `slip:{id}` and `week:{weekID}` and unsubscribes when the request ends.
- It sends `event: slip-updated` or `event: board-updated` with `data: {}`.
- It sends a `: ping` comment every 25 seconds so proxies keep the connection open.
- It sets `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`, and
  `X-Accel-Buffering: no`, and flushes every frame.

### Deadlines

An open stream keeps the instance billed, so every stream ends itself.

- **Live window:** when a game in the week is live or kicks off soon, the stream may live 55 minutes. The window is
  fixed from the start; events do not extend it.
- **Otherwise:** the stream closes after 10 minutes without an event. Each `slip-updated` or `board-updated` resets that
  idle timer.

At either deadline the server sends `event: bye` and returns. The `bye` tells the client this was planned, not a
failure, so it waits for the tab to become visible instead of reconnecting at once.

The `IsLive` func passed to `live.NewStream` is `refresh.Service.LiveWindow`: a week is live when a game is live or a
scheduled game kicks off within 30 minutes. A read error falls back to the idle deadline. The stream takes its timers
from a `Clock` interface, so tests can fire deadlines by hand. See [Data refresh](data-refresh.md).

## Client (`web/static/app.js`)

The page marks its body with `data-live-url`; the client opens one `EventSource` for it.

- `slip-updated` and `board-updated` call `htmx.trigger(document.body, ...)`. Partials listen with
  `hx-trigger="slip-updated from:body"`.
- `bye` closes the stream without retrying.
- When the tab becomes hidden, the stream closes. When it becomes visible again, or the window gets focus, the client
  reopens the stream and triggers both refresh events to cover anything missed.
- After every `htmx:afterSwap`, `<time>` elements are formatted in local time with `Intl.DateTimeFormat`.

Closing on hide matters on iOS Safari, which drops background connections and would otherwise leave a dead stream.

## Guardrails

- An open stream keeps the instance billed, so keep the deadlines and the visibility close.
- Set the anti-buffering headers and flush every write, or Cloud Run and proxies hold events back.
- Publish once per committed mutation. Publishing inside a transaction can tell clients about a write that rolls back.
- Keep per-person data out of events. Send a signal and let the page fetch its own view.
