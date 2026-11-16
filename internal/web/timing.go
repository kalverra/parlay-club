package web

import (
	"context"
	"sync/atomic"
	"time"
)

type phaseTimingsKey struct{}

// PhaseTimings accumulates per-phase durations (in nanoseconds) for a single
// HTTP request so middleware can log and expose Server-Timing headers.
type PhaseTimings struct {
	dbNS     atomic.Int64
	boardNS  atomic.Int64
	renderNS atomic.Int64
}

// WithPhaseTimings attaches a fresh PhaseTimings tracker to ctx.
func WithPhaseTimings(ctx context.Context) (context.Context, *PhaseTimings) {
	timings := &PhaseTimings{}
	return context.WithValue(ctx, phaseTimingsKey{}, timings), timings
}

// PhaseTimingsFrom returns the request's PhaseTimings tracker, or nil if none
// is attached.
func PhaseTimingsFrom(ctx context.Context) *PhaseTimings {
	if ctx == nil {
		return nil
	}
	timings, _ := ctx.Value(phaseTimingsKey{}).(*PhaseTimings)
	return timings
}

// AddDBTiming records database time on the request's PhaseTimings, if present.
func AddDBTiming(ctx context.Context, d time.Duration) {
	if t := PhaseTimingsFrom(ctx); t != nil && d > 0 {
		t.dbNS.Add(int64(d))
	}
}

// AddBoardTiming records board-build time on the request's PhaseTimings, if present.
func AddBoardTiming(ctx context.Context, d time.Duration) {
	if t := PhaseTimingsFrom(ctx); t != nil && d > 0 {
		t.boardNS.Add(int64(d))
	}
}

// AddRenderTiming records template-render time on the request's PhaseTimings, if present.
func AddRenderTiming(ctx context.Context, d time.Duration) {
	if t := PhaseTimingsFrom(ctx); t != nil && d > 0 {
		t.renderNS.Add(int64(d))
	}
}

// DB returns the accumulated database duration.
func (t *PhaseTimings) DB() time.Duration {
	return time.Duration(t.dbNS.Load())
}

// Board returns the accumulated board-build duration.
func (t *PhaseTimings) Board() time.Duration {
	return time.Duration(t.boardNS.Load())
}

// Render returns the accumulated template-render duration.
func (t *PhaseTimings) Render() time.Duration {
	return time.Duration(t.renderNS.Load())
}
