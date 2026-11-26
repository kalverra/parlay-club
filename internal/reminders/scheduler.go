package reminders

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"
)

// ErrAlreadyExists reports that the scheduler already holds the named task.
// Enqueuing the same slip and start twice is not an error, so Sync records the
// row and moves on.
var ErrAlreadyExists = errors.New("reminder task already exists")

// Scheduler places one timed wake-up for a slip. The production
// implementation is the Cloud Tasks client; LocalTimers covers a long-running
// dev server.
type Scheduler interface {
	// Enqueue asks for one reminder fire for slipID at start, delivered at at.
	Enqueue(ctx context.Context, slipID int64, start, at time.Time) error
}

// FireFunc delivers one reminder. *Service.Fire satisfies it.
type FireFunc func(ctx context.Context, slipID int64, start time.Time) (Report, error)

// LocalTimers schedules reminders with time.AfterFunc. It uses the wall clock,
// so it suits a dev server that stays up; sandbox mode wires no scheduler
// because its clock is frozen.
type LocalTimers struct {
	fire   FireFunc
	logger zerolog.Logger
	after  func(time.Duration, func()) *time.Timer
}

// NewLocalTimers returns an in-process scheduler that calls fire when a timer
// elapses.
func NewLocalTimers(fire FireFunc, logger zerolog.Logger) *LocalTimers {
	return &LocalTimers{
		fire:   fire,
		logger: logger,
		after:  time.AfterFunc,
	}
}

// Enqueue starts one timer. A past instant fires at once, which lets a dev
// server catch up after a restart.
func (l *LocalTimers) Enqueue(_ context.Context, slipID int64, start, at time.Time) error {
	if l.fire == nil {
		return errors.New("reminders: local timers have no fire function")
	}
	delay := max(time.Until(at), 0)
	l.after(delay, func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if _, err := l.fire(ctx, slipID, start); err != nil {
			l.logger.Warn().Err(err).Int64("slip", slipID).Msg("reminders: local fire")
		}
	})
	return nil
}
