package reminders

import (
	"context"
	"errors"
	"fmt"
	"sync"
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
	// Durable reports whether enqueued tasks survive a process restart. Sync
	// trusts the reminder_tasks row only for a durable scheduler, so a dev
	// server re-enqueues its pending timers after a restart.
	Durable() bool
}

// FireFunc delivers one reminder. *Service.Fire satisfies it.
type FireFunc func(ctx context.Context, slipID int64, start time.Time) (Report, error)

// LocalTimers schedules reminders with time.AfterFunc. It uses the wall clock,
// so it suits a dev server that stays up; sandbox mode wires no scheduler
// because its clock is frozen. The timers live only in this process, so a
// restart drops them and the next Sync arms them again.
type LocalTimers struct {
	fire   FireFunc
	logger zerolog.Logger
	after  func(time.Duration, func()) *time.Timer

	mu    sync.Mutex
	armed map[string]struct{}
}

// NewLocalTimers returns an in-process scheduler that calls fire when a timer
// elapses.
func NewLocalTimers(fire FireFunc, logger zerolog.Logger) *LocalTimers {
	return &LocalTimers{
		fire:   fire,
		logger: logger,
		after:  time.AfterFunc,
		armed:  make(map[string]struct{}),
	}
}

// Durable reports false: a restart loses every timer.
func (l *LocalTimers) Durable() bool { return false }

// Enqueue starts one timer per slip and start. A repeat call for the same pair
// is a no-op, so the Sync that runs after a restart can re-arm without making
// duplicate timers.
func (l *LocalTimers) Enqueue(_ context.Context, slipID int64, start, at time.Time) error {
	if l.fire == nil {
		return errors.New("reminders: local timers have no fire function")
	}
	key := fmt.Sprintf("%d-%d", slipID, start.Unix())
	l.mu.Lock()
	if _, ok := l.armed[key]; ok {
		l.mu.Unlock()
		return nil
	}
	l.armed[key] = struct{}{}
	l.mu.Unlock()

	delay := max(time.Until(at), 0)
	l.after(delay, func() {
		l.mu.Lock()
		delete(l.armed, key)
		l.mu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if _, err := l.fire(ctx, slipID, start); err != nil {
			l.logger.Warn().Err(err).Int64("slip", slipID).Msg("reminders: local fire")
		}
	})
	return nil
}
