package live_test

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/live"
)

// sseClient is one open stream under test.
type sseClient struct {
	reader *bufio.Reader
}

// frame is one parsed SSE frame: an event, its data, or a comment.
type frame struct {
	event   string
	data    string
	comment string
}

func TestStreamDeliversSlipAndBoardUpdatesToEverySubscriber(t *testing.T) {
	t.Parallel()

	var hub live.Hub
	server := newStreamServer(t, &hub, false)
	t.Cleanup(server.Close)

	first := dial(t, server.URL)
	second := dial(t, server.URL)

	hub.Publish("slip:7")
	for _, client := range []*sseClient{first, second} {
		update := readFrame(t, client.reader)
		assert.Equal(t, "slip-updated", update.event)
		assert.Equal(t, "{}", update.data)
	}

	hub.Publish("week:2026209")
	for _, client := range []*sseClient{first, second} {
		update := readFrame(t, client.reader)
		assert.Equal(t, "board-updated", update.event)
		assert.Equal(t, "{}", update.data)
	}
}

func TestStreamSaysByeAtTheLiveDeadline(t *testing.T) {
	t.Parallel()

	var hub live.Hub
	clock := newFakeClock()
	stream := live.NewStream(&hub, func(int64) bool { return true }, live.WithClock(clock))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stream.ServeSlip(w, r, 7, 2026209)
	}))
	t.Cleanup(server.Close)

	client := dial(t, server.URL)
	clock.Fire(15 * time.Minute)

	bye := readFrame(t, client.reader)
	assert.Equal(t, "bye", bye.event)
	assert.Equal(t, "{}", bye.data)

	_, err := client.reader.ReadByte()
	require.ErrorIs(t, err, io.EOF, "the server should close the stream after bye")
}

func TestStreamSaysByeAtTheIdleDeadline(t *testing.T) {
	t.Parallel()

	var hub live.Hub
	clock := newFakeClock()
	stream := live.NewStream(&hub, func(int64) bool { return false }, live.WithClock(clock))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stream.ServeSlip(w, r, 7, 2026209)
	}))
	t.Cleanup(server.Close)

	client := dial(t, server.URL)
	clock.Fire(3 * time.Minute)

	bye := readFrame(t, client.reader)
	assert.Equal(t, "bye", bye.event)
	assert.Equal(t, "{}", bye.data)

	_, err := client.reader.ReadByte()
	require.ErrorIs(t, err, io.EOF, "the server should close the stream after bye")
}

func TestStreamShutdownEndsOpenStreamsWithoutBye(t *testing.T) {
	t.Parallel()

	var hub live.Hub
	clock := newFakeClock()
	stream := live.NewStream(&hub, func(int64) bool { return true }, live.WithClock(clock))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stream.ServeSlip(w, r, 7, 2026209)
	}))
	t.Cleanup(server.Close)

	first := dial(t, server.URL)
	second := dial(t, server.URL)

	stream.Shutdown()
	stream.Shutdown() // Shutdown is idempotent.

	for _, client := range []*sseClient{first, second} {
		body, err := io.ReadAll(client.reader)
		require.NoError(t, err)
		assert.Empty(t, string(body), "a shutdown must not send bye or any other frame")
	}
}

func TestStreamServesAfterShutdownWithoutHanging(t *testing.T) {
	t.Parallel()

	var hub live.Hub
	stream := live.NewStream(&hub, nil)
	stream.Shutdown()

	recorder := httptest.NewRecorder()
	start := time.Now()
	stream.ServeSlip(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/events", nil), 7, 2026209)

	assert.Equal(t, http.StatusOK, recorder.Code, "the headers are already sent before the loop")
	assert.Less(t, time.Since(start), 100*time.Millisecond, "shutdown must end the stream promptly")
}

func TestStreamPingsWhileItWaits(t *testing.T) {
	t.Parallel()

	var hub live.Hub
	clock := newFakeClock()
	stream := live.NewStream(&hub, func(int64) bool { return false }, live.WithClock(clock))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stream.ServeSlip(w, r, 7, 2026209)
	}))
	t.Cleanup(server.Close)

	client := dial(t, server.URL)
	clock.Fire(25 * time.Second)

	ping := readFrame(t, client.reader)
	assert.Equal(t, "ping", ping.comment)
	assert.Empty(t, ping.event)
}

type liveTickCall struct {
	slipID int64
	weekID int64
}

func TestStreamLiveTickFiresOnConnectAndIntervalUntilDeadline(t *testing.T) {
	t.Parallel()

	var hub live.Hub
	clock := newFakeClock()
	ticks := make(chan liveTickCall, 8)
	stream := live.NewStream(
		&hub,
		func(int64) bool { return true },
		live.WithClock(clock),
		live.WithLiveTick(25*time.Second, func(slipID, weekID int64) {
			ticks <- liveTickCall{slipID: slipID, weekID: weekID}
		}),
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stream.ServeSlip(w, r, 7, 2026209)
	}))
	t.Cleanup(server.Close)

	client := dial(t, server.URL)
	assert.Equal(t, liveTickCall{slipID: 7, weekID: 2026209}, awaitTick(t, ticks))

	clock.Fire(25 * time.Second)
	assert.Equal(t, liveTickCall{slipID: 7, weekID: 2026209}, awaitTick(t, ticks))
	assert.Equal(t, "ping", readFrame(t, client.reader).comment)

	clock.Fire(25 * time.Second)
	assert.Equal(t, liveTickCall{slipID: 7, weekID: 2026209}, awaitTick(t, ticks))
	assert.Equal(t, "ping", readFrame(t, client.reader).comment)

	clock.Fire(15 * time.Minute)
	bye := readFrame(t, client.reader)
	assert.Equal(t, "bye", bye.event)
	_, err := client.reader.ReadByte()
	require.ErrorIs(t, err, io.EOF)

	clock.Fire(25 * time.Second)
	assertNoTick(t, ticks)
}

func TestStreamLiveTickSkippedWhenNotLive(t *testing.T) {
	t.Parallel()

	var hub live.Hub
	clock := newFakeClock()
	ticks := make(chan liveTickCall, 4)
	stream := live.NewStream(
		&hub,
		func(int64) bool { return false },
		live.WithClock(clock),
		live.WithLiveTick(25*time.Second, func(slipID, weekID int64) {
			ticks <- liveTickCall{slipID: slipID, weekID: weekID}
		}),
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stream.ServeSlip(w, r, 7, 2026209)
	}))
	t.Cleanup(server.Close)

	client := dial(t, server.URL)
	clock.Fire(25 * time.Second)
	assert.Equal(t, "ping", readFrame(t, client.reader).comment)
	assertNoTick(t, ticks)
}

func TestStreamLiveTickStopsOnDisconnectAndShutdown(t *testing.T) {
	t.Parallel()

	t.Run("client disconnect", func(t *testing.T) {
		t.Parallel()

		var hub live.Hub
		clock := newFakeClock()
		ticks := make(chan liveTickCall, 4)
		done := make(chan struct{})
		stream := live.NewStream(
			&hub,
			func(int64) bool { return true },
			live.WithClock(clock),
			live.WithLiveTick(25*time.Second, func(slipID, weekID int64) {
				ticks <- liveTickCall{slipID: slipID, weekID: weekID}
			}),
		)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer close(done)
			stream.ServeSlip(w, r, 7, 2026209)
		}))
		t.Cleanup(server.Close)

		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, nil)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, liveTickCall{slipID: 7, weekID: 2026209}, awaitTick(t, ticks))

		require.NoError(t, resp.Body.Close())
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("ServeSlip did not return after client disconnect")
		}

		clock.Fire(25 * time.Second)
		assertNoTick(t, ticks)
	})

	t.Run("shutdown", func(t *testing.T) {
		t.Parallel()

		var hub live.Hub
		clock := newFakeClock()
		ticks := make(chan liveTickCall, 4)
		done := make(chan struct{})
		stream := live.NewStream(
			&hub,
			func(int64) bool { return true },
			live.WithClock(clock),
			live.WithLiveTick(25*time.Second, func(slipID, weekID int64) {
				ticks <- liveTickCall{slipID: slipID, weekID: weekID}
			}),
		)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer close(done)
			stream.ServeSlip(w, r, 7, 2026209)
		}))
		t.Cleanup(server.Close)

		client := dial(t, server.URL)
		assert.Equal(t, liveTickCall{slipID: 7, weekID: 2026209}, awaitTick(t, ticks))

		stream.Shutdown()
		body, err := io.ReadAll(client.reader)
		require.NoError(t, err)
		assert.Empty(t, string(body))

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("ServeSlip did not return after Shutdown")
		}

		clock.Fire(25 * time.Second)
		assertNoTick(t, ticks)
	})

	t.Run("already shut down", func(t *testing.T) {
		t.Parallel()

		var hub live.Hub
		ticks := make(chan liveTickCall, 2)
		stream := live.NewStream(
			&hub,
			func(int64) bool { return true },
			live.WithLiveTick(25*time.Second, func(slipID, weekID int64) {
				ticks <- liveTickCall{slipID: slipID, weekID: weekID}
			}),
		)
		stream.Shutdown()

		recorder := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/events", nil)
		stream.ServeSlip(recorder, req, 7, 2026209)
		assert.Equal(t, http.StatusOK, recorder.Code)
		assertNoTick(t, ticks)
	})
}

func awaitTick(t *testing.T, ticks <-chan liveTickCall) liveTickCall {
	t.Helper()
	select {
	case call := <-ticks:
		return call
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for liveTick")
		return liveTickCall{}
	}
}

func assertNoTick(t *testing.T, ticks <-chan liveTickCall) {
	t.Helper()
	select {
	case call := <-ticks:
		t.Fatalf("unexpected liveTick call: %+v", call)
	case <-time.After(30 * time.Millisecond):
	}
}

// newStreamServer serves one stream for slip 7 in week 2026209.
func newStreamServer(t *testing.T, hub *live.Hub, liveWindow bool) *httptest.Server {
	t.Helper()

	stream := live.NewStream(hub, func(int64) bool { return liveWindow }, live.WithClock(newFakeClock()))
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stream.ServeSlip(w, r, 7, 2026209)
	}))
}

// dial opens a stream and checks the headers that keep proxies from buffering
// it. The request context ends with the test, so a missing event fails instead
// of hanging forever.
func dial(t *testing.T, url string) *sseClient {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	require.NoError(t, err)

	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := response.Body.Close(); err != nil {
			t.Logf("close stream body: %v", err)
		}
	})

	require.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, "text/event-stream", response.Header.Get("Content-Type"))
	assert.Equal(t, "no-cache", response.Header.Get("Cache-Control"))
	assert.Equal(t, "no", response.Header.Get("X-Accel-Buffering"))

	return &sseClient{reader: bufio.NewReader(response.Body)}
}

// readFrame reads one frame, skipping the blank lines between frames.
func readFrame(t *testing.T, reader *bufio.Reader) frame {
	t.Helper()

	var parsed frame
	for {
		line, err := reader.ReadString('\n')
		require.NoError(t, err)
		line = strings.TrimRight(line, "\r\n")

		switch {
		case line == "":
			if parsed.event != "" || parsed.data != "" || parsed.comment != "" {
				return parsed
			}
		case strings.HasPrefix(line, ":"):
			parsed.comment = strings.TrimSpace(strings.TrimPrefix(line, ":"))
		case strings.HasPrefix(line, "event:"):
			parsed.event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			parsed.data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
	}
}

// fakeClock hands out timers the test fires by duration, so deadline tests do
// not sleep.
type fakeClock struct {
	mu     sync.Mutex
	timers []*fakeTimer
}

func newFakeClock() *fakeClock {
	return &fakeClock{}
}

// NewTimer starts a stopped fake timer that waits for Fire.
func (c *fakeClock) NewTimer(d time.Duration) live.Timer {
	timer := &fakeTimer{duration: d, ch: make(chan time.Time, 1)}

	c.mu.Lock()
	c.timers = append(c.timers, timer)
	c.mu.Unlock()

	return timer
}

// Fire fires every running timer with that duration.
func (c *fakeClock) Fire(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, timer := range c.timers {
		timer.fire(d)
	}
}

// fakeTimer is one timer the fake clock controls.
type fakeTimer struct {
	mu       sync.Mutex
	ch       chan time.Time
	duration time.Duration
	stopped  bool
}

var _ live.Timer = (*fakeTimer)(nil)

func (t *fakeTimer) C() <-chan time.Time {
	return t.ch
}

func (t *fakeTimer) Stop() {
	t.mu.Lock()
	t.stopped = true
	t.mu.Unlock()
}

func (t *fakeTimer) Reset(d time.Duration) {
	t.mu.Lock()
	t.duration = d
	t.stopped = false
	select {
	case <-t.ch:
	default:
	}
	t.mu.Unlock()
}

func (t *fakeTimer) fire(d time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.stopped || t.duration != d {
		return
	}
	select {
	case t.ch <- time.Time{}:
	default:
	}
	t.stopped = true
}
