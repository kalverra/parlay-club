package sandbox

import (
	"slices"
	"strings"
	"sync"
	"time"
)

// MorningTime is the scenario clock on the week 9 Sunday morning. Every
// outdoor kickoff is after it and inside the 48-hour weather lookahead, so the
// first refresh can load the whole slate.
func MorningTime() time.Time {
	return time.Date(2026, time.November, 8, 13, 0, 0, 0, time.UTC)
}

// EarlyLossTime is the third quarter of the Madrid game, before the rest of the
// Sunday slate kicks off.
func EarlyLossTime() time.Time {
	return time.Date(2026, time.November, 8, 15, 30, 0, 0, time.UTC)
}

// AllClearTime is Sunday evening after the last Sunday game, while the Monday
// night game is still upcoming.
func AllClearTime() time.Time {
	return time.Date(2026, time.November, 9, 0, 30, 0, 0, time.UTC)
}

// SundayCompleteTime is Monday night after the final whistle of the Monday
// night game.
func SundayCompleteTime() time.Time {
	return time.Date(2026, time.November, 10, 4, 30, 0, 0, time.UTC)
}

// Stages returns every stage in baseline-first order.
func Stages() []Stage {
	return []Stage{Morning, EarlyLoss, AllClear, SundayComplete}
}

// Valid reports whether the stage is one the scenario registry knows.
func (s Stage) Valid() bool {
	return slices.Contains(Stages(), s)
}

// StageByName turns a control value into a known stage. An empty or unknown
// name is not a stage.
func StageByName(name string) (Stage, bool) {
	stage := Stage(strings.TrimSpace(name))
	return stage, stage.Valid()
}

// StageTime returns the fixed scenario clock for a stage.
func StageTime(stage Stage) (time.Time, bool) {
	switch stage {
	case Morning:
		return MorningTime(), true
	case EarlyLoss:
		return EarlyLossTime(), true
	case AllClear:
		return AllClearTime(), true
	case SundayComplete:
		return SundayCompleteTime(), true
	default:
		return time.Time{}, false
	}
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
