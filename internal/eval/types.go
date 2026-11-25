// Package eval provides leak-free historical replay and calibration metrics for
// grading rubrics.
package eval

import (
	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/parlay"
)

// ReplayMode selects whether LoadDataset replays settled user slips or
// market-wide candidate legs.
type ReplayMode string

const (
	// ModeSlips replays settled user slips and their legs.
	ModeSlips ReplayMode = "slips"
	// ModeMarket replays deduplicated pre-kickoff market-wide candidate legs.
	ModeMarket ReplayMode = "market"
)

// DataCoverage records how many replayed legs had each historical signal
// available before kickoff.
type DataCoverage struct {
	TotalLegs      int `json:"total_legs"`
	WithGameLogs   int `json:"with_game_logs"`
	WithL5Fallback int `json:"with_l5_fallback,omitempty"`
	WithInjuries   int `json:"with_injuries"`
	WithNFLVerse   int `json:"with_nflverse"`
	WithNfelo      int `json:"with_nfelo"`
	WithOdds       int `json:"with_odds"`
	WithSharp      int `json:"with_sharp,omitempty"`
}

// ReplayedLeg pairs a settled leg with its decimal odds and outcome.
type ReplayedLeg struct {
	Leg         analyzer.Leg  `json:"leg"`
	Result      parlay.Result `json:"result"`
	DecimalOdds float64       `json:"decimal_odds,omitempty"`
}

// ReplayedSlip holds one settled slip and its legs for a replayed week.
type ReplayedSlip struct {
	SlipID       int64         `json:"slip_id"`
	Result       parlay.Result `json:"result"`
	CombinedOdds float64       `json:"combined_odds,omitempty"`
	Legs         []ReplayedLeg `json:"legs"`
}

// ReplayedWeek holds all pre-kickoff analyzer inputs and settled slips or
// market-wide candidate legs for one week.
type ReplayedWeek struct {
	Season     int                             `json:"season"`
	WeekNum    int                             `json:"week_num"`
	WeekType   string                          `json:"week_type,omitempty"`
	Games      map[string]analyzer.Game        `json:"games"`
	Stats      map[string]analyzer.PlayerStats `json:"stats"`
	Slips      []ReplayedSlip                  `json:"slips"`
	MarketLegs []ReplayedLeg                   `json:"market_legs,omitempty"`
}

// ReplayDataset is the leak-free historical input consumed by Evaluate.
type ReplayDataset struct {
	Weeks    []ReplayedWeek `json:"weeks"`
	Coverage DataCoverage   `json:"coverage"`
}
