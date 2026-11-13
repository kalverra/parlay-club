package live

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Stream timing. Pings keep proxies from closing an idle connection. A stream
// for a week with a game close to kickoff lives longer, because that is when
// updates matter most.
const (
	pingEvery    = 25 * time.Second
	idleDeadline = 10 * time.Minute
	liveDeadline = 55 * time.Minute
)

// IsLive reports whether weekID has a game that is live or kicks off soon. The
// server supplies the game-aware rule; tests supply a fixed answer.
type IsLive func(weekID int64) bool

// Clock supplies the stream's timers. Tests substitute one they fire by hand.
type Clock interface {
	NewTimer(time.Duration) Timer
}

// Timer is the part of a timer a stream uses. It matches *time.Timer.
type Timer interface {
	C() <-chan time.Time
	Stop()
	Reset(time.Duration)
}

// Stream serves one SSE connection per slip page. Every event is only a signal:
// the browser refetches the partials it shows, so no per-user HTML travels
// through the hub.
type Stream struct {
	hub    *Hub
	isLive IsLive
	clock  Clock
	done   chan struct{}
	stop   sync.Once
}

// StreamOption configures a Stream.
type StreamOption func(*Stream)

// WithClock makes the stream take its timers from clock. Tests use it to reach
// the deadlines without waiting.
func WithClock(clock Clock) StreamOption {
	return func(s *Stream) {
		s.clock = clock
	}
}

// NewStream builds a Stream over hub. A nil isLive means no week is in its live
// window.
func NewStream(hub *Hub, isLive IsLive, opts ...StreamOption) *Stream {
	stream := &Stream{hub: hub, isLive: isLive, clock: systemClock{}, done: make(chan struct{})}
	for _, opt := range opts {
		opt(stream)
	}
	return stream
}

// Shutdown ends every open stream. ServeSlip returns without a bye event so the
// browser reconnects to the replacement instance instead of waiting for its tab
// to become visible. It is safe to call more than once.
func (s *Stream) Shutdown() {
	s.stop.Do(func() { close(s.done) })
}

// ServeSlip streams change signals for one slip and its week. It blocks until
// the client goes away, a deadline fires, or the stream shuts down. Both
// deadlines end with a bye event so the client knows to reopen the stream later
// instead of retrying at once. A shutdown ends the stream without a bye so
// EventSource reconnects on its own.
func (s *Stream) ServeSlip(w http.ResponseWriter, r *http.Request, slipID, weekID int64) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	slipUpdates, unsubscribeSlip := s.hub.Subscribe(fmt.Sprintf("slip:%d", slipID))
	defer unsubscribeSlip()
	weekUpdates, unsubscribeWeek := s.hub.Subscribe(fmt.Sprintf("week:%d", weekID))
	defer unsubscribeWeek()

	ping := s.clock.NewTimer(pingEvery)
	defer ping.Stop()

	live := s.isLive != nil && s.isLive(weekID)
	deadline := s.clock.NewTimer(deadlineFor(live))
	defer deadline.Stop()

	header := w.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache")
	header.Set("Connection", "keep-alive")
	header.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	for {
		select {
		case <-s.done:
			return
		case <-r.Context().Done():
			return
		case <-ping.C():
			if err := writeFrame(w, flusher, ": ping\n\n"); err != nil {
				return
			}
			ping.Reset(pingEvery)
		case <-slipUpdates:
			if err := writeFrame(w, flusher, "event: slip-updated\ndata: {}\n\n"); err != nil {
				return
			}
			if !live {
				deadline.Reset(idleDeadline)
			}
		case <-weekUpdates:
			if err := writeFrame(w, flusher, "event: board-updated\ndata: {}\n\n"); err != nil {
				return
			}
			if !live {
				deadline.Reset(idleDeadline)
			}
		case <-deadline.C():
			_ = writeFrame(w, flusher, "event: bye\ndata: {}\n\n")
			return
		}
	}
}

// deadlineFor picks how long a stream may live. A live window is fixed from
// kickoff; otherwise the deadline is an idle timeout that each event resets.
func deadlineFor(live bool) time.Duration {
	if live {
		return liveDeadline
	}
	return idleDeadline
}

// writeFrame sends one SSE frame and flushes it so Cloud Run and proxies do not
// hold it back.
func writeFrame(w http.ResponseWriter, flusher http.Flusher, frame string) error {
	if _, err := io.WriteString(w, frame); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

// systemClock is the wall-clock Clock used by the server.
type systemClock struct{}

// NewTimer starts a wall-clock timer.
func (systemClock) NewTimer(d time.Duration) Timer {
	return systemTimer{timer: time.NewTimer(d)}
}

// systemTimer adapts *time.Timer to Timer.
type systemTimer struct {
	timer *time.Timer
}

func (t systemTimer) C() <-chan time.Time {
	return t.timer.C
}

func (t systemTimer) Stop() {
	t.timer.Stop()
}

func (t systemTimer) Reset(d time.Duration) {
	t.timer.Reset(d)
}
