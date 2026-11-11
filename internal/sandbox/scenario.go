package sandbox

import (
	"sync"
	"time"
)

// MorningTime is the scenario clock on the week 9 Sunday morning. Every
// outdoor kickoff is after it and inside the 48-hour weather lookahead, so the
// first refresh can load the whole slate.
func MorningTime() time.Time {
	return time.Date(2026, time.November, 8, 13, 0, 0, 0, time.UTC)
}

// Scenario is the sandbox's one clock and stage. Refresh, slips, and web all
// read Now, so pickers, kickoff locks, projections, and staleness agree.
// Auth cookies and SSE timers deliberately keep the wall clock.
type Scenario struct {
	mu    sync.RWMutex
	stage Stage
	now   time.Time
}

// NewScenario returns a scenario at stage and now.
func NewScenario(stage Stage, now time.Time) *Scenario {
	return &Scenario{stage: stage, now: now}
}

// Now returns the scenario time.
func (s *Scenario) Now() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.now
}

// Stage returns the active stage.
func (s *Scenario) Stage() Stage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.stage
}

// State returns the scenario's stage and time as one value.
func (s *Scenario) State() State {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return State{Stage: s.stage, Now: s.now}
}

// Set replaces the active stage and time. A reset or stage change calls it
// before rebuilding, so every service reads the same new instant.
func (s *Scenario) Set(state State) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stage = state.Stage
	s.now = state.Now
}
