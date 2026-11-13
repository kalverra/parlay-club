package refresh

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/ingest/espn"
)

// blockingScoreboard holds its fetch until its context ends, or until the test
// releases it. It stands in for a slow source during shutdown.
type blockingScoreboard struct {
	entered chan struct{}
	left    chan struct{}
	release chan struct{}
}

// Scoreboard signals that it started, then waits for its context or the
// release. A release makes it ignore its context, which is how a misbehaving
// source holds up the drain.
func (b *blockingScoreboard) Scoreboard(
	ctx context.Context,
	_ espn.ScoreboardParams,
) (espn.Scoreboard, error) {
	close(b.entered)
	if b.release != nil {
		<-b.release
		close(b.left)
		return espn.Scoreboard{}, errors.New("scoreboard released")
	}
	<-ctx.Done()
	close(b.left)
	return espn.Scoreboard{}, ctx.Err()
}

// TestShutdownCancelsAnInFlightSync proves Shutdown ends a fetch that is
// already running, instead of waiting out its 30 second source timeout.
func TestShutdownCancelsAnInFlightSync(t *testing.T) {
	t.Parallel()

	scoreboard := &blockingScoreboard{entered: make(chan struct{}), left: make(chan struct{})}
	service := New(Deps{
		Store:      openTestStore(t),
		Scoreboard: scoreboard,
		Logger:     zerolog.Nop(),
	})
	service.Kick()
	<-scoreboard.entered

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, service.Shutdown(ctx))

	select {
	case <-scoreboard.left:
	default:
		t.Fatal("Shutdown returned while the sync was still running")
	}
	require.NoError(t, service.Shutdown(t.Context()), "a second shutdown is safe")
}

// TestShutdownReturnsTheDeadlineWhenASourceIgnoresIt holds the fetch open past
// the drain's deadline, so the caller still gets control back and can close the
// database on its own schedule.
func TestShutdownReturnsTheDeadlineWhenASourceIgnoresIt(t *testing.T) {
	t.Parallel()

	scoreboard := &blockingScoreboard{
		entered: make(chan struct{}),
		left:    make(chan struct{}),
		release: make(chan struct{}),
	}
	service := New(Deps{
		Store:      openTestStore(t),
		Scoreboard: scoreboard,
		Logger:     zerolog.Nop(),
	})
	service.Kick()
	<-scoreboard.entered

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, service.Shutdown(ctx), context.DeadlineExceeded)

	close(scoreboard.release)
	drained, cancelDrain := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancelDrain()
	require.NoError(t, service.Shutdown(drained), "a second wait sees the source finish")
}

// TestKickAfterShutdownStartsNoWork keeps a late page load from starting a pass
// while the server is draining.
func TestKickAfterShutdownStartsNoWork(t *testing.T) {
	t.Parallel()

	scoreboard := &countingScoreboard{board: espnWeek()}
	service := New(Deps{
		Store:      openTestStore(t),
		Scoreboard: scoreboard,
		Now:        func() time.Time { return novemberTime(7, 11, 0) },
		Logger:     zerolog.Nop(),
	})

	require.NoError(t, service.Shutdown(context.Background()))
	service.Kick()
	service.wait()

	assert.Zero(t, scoreboard.calls.Load(), "a stopped service must not start another pass")
}
