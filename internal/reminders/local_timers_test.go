package reminders

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalTimersCallTheFireFunction(t *testing.T) {
	t.Parallel()

	var (
		delay    time.Duration
		callback func()
		slipID   int64
		start    time.Time
	)
	timers := NewLocalTimers(func(_ context.Context, gotSlip int64, gotStart time.Time) (Report, error) {
		slipID = gotSlip
		start = gotStart
		return Report{Sent: 1}, nil
	}, zerolog.Nop())
	timers.after = func(d time.Duration, fn func()) *time.Timer {
		delay = d
		callback = fn
		return nil
	}

	kickoff := time.Now().Add(24 * time.Hour)
	at := kickoff.Add(-LeadTime)
	require.NoError(t, timers.Enqueue(t.Context(), 42, kickoff, at))

	assert.Positive(t, delay)
	require.NotNil(t, callback)
	callback()
	assert.Equal(t, int64(42), slipID)
	assert.True(t, kickoff.Equal(start))
}
