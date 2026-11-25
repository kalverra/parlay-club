// Package progress evaluates live parlay leg and slip progress using
// deterministic game-clock and statistical models. It is pure and has no
// database, HTTP, or wall-clock dependencies.
package progress

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/kalverra/parlay-club/internal/db"
)

const (
	regulationSeconds = 3600.0
	quarterSeconds    = 900.0
	minLiveRemFrac    = 0.005
	maxOTRemFrac      = 0.167
	fallbackLiveFrac  = 0.50
)

// RemainingFraction returns the fraction of regulation remaining in [0, 1]
// (or up to 0.167 in OT) for a game's status, period, and clock string.
func RemainingFraction(status db.GameStatus, period *int, clock *string) float64 {
	switch status {
	case db.GameScheduled:
		return 1.0
	case db.GameFinal:
		return 0.0
	case db.GameLive:
		return liveRemainingFraction(period, clock)
	default:
		return 1.0
	}
}

func liveRemainingFraction(period *int, clock *string) float64 {
	if clock != nil {
		c := strings.TrimSpace(*clock)
		lower := strings.ToLower(c)
		if lower == "halftime" || lower == "half" {
			return 0.50
		}
		if strings.HasPrefix(lower, "end of ") {
			if period != nil && *period >= 1 && *period <= 4 {
				return clamp(float64(4-*period)*quarterSeconds/regulationSeconds, minLiveRemFrac, 1.0)
			}
			switch lower {
			case "end of 1st":
				return 0.75
			case "end of 2nd":
				return 0.50
			case "end of 3rd":
				return 0.25
			case "end of 4th", "end of ot":
				return minLiveRemFrac
			default:
				if strings.HasSuffix(lower, "ot") {
					return minLiveRemFrac
				}
				return fallbackLiveFrac
			}
		}
	}

	if period == nil || *period <= 0 || clock == nil {
		return fallbackLiveFrac
	}

	clockSec, ok := parseClockSeconds(*clock)
	if !ok {
		return fallbackLiveFrac
	}

	if *period >= 5 {
		return clamp(clockSec/regulationSeconds, minLiveRemFrac, maxOTRemFrac)
	}

	remSec := float64(4-*period)*quarterSeconds + clockSec
	return clamp(remSec/regulationSeconds, minLiveRemFrac, 1.0)
}

func parseClockSeconds(clock string) (float64, bool) {
	s := strings.TrimSpace(clock)
	if s == "" {
		return 0, false
	}
	if minStr, secStr, ok := strings.Cut(s, ":"); ok {
		mins, err1 := strconv.Atoi(strings.TrimSpace(minStr))
		secs, err2 := strconv.ParseFloat(strings.TrimSpace(secStr), 64)
		if err1 != nil || err2 != nil || mins < 0 || secs < 0 || secs >= 60 || math.IsNaN(secs) || math.IsInf(secs, 0) {
			return 0, false
		}
		total := float64(mins)*60 + secs
		if total > quarterSeconds {
			total = quarterSeconds
		}
		return total, true
	}

	secs, err := strconv.ParseFloat(s, 64)
	if err != nil || secs < 0 || secs > 60 || math.IsNaN(secs) || math.IsInf(secs, 0) {
		return 0, false
	}
	return secs, true
}

// ClockLabel formats a concise display label for a game's current clock state.
func ClockLabel(status db.GameStatus, period *int, clock *string) string {
	switch status {
	case db.GameFinal:
		return "Final"
	case db.GameScheduled:
		return ""
	case db.GameLive:
		c := ""
		if clock != nil {
			c = strings.TrimSpace(*clock)
		}
		if c != "" {
			lower := strings.ToLower(c)
			if lower == "halftime" || lower == "half" {
				return "Halftime"
			}
			if strings.HasPrefix(lower, "end of ") {
				return c
			}
			if period != nil && *period > 0 {
				return formatPeriodPrefix(*period) + " " + c
			}
			return c
		}
		if period != nil && *period > 0 {
			return formatPeriodPrefix(*period)
		}
		return "Live"
	default:
		return ""
	}
}

func formatPeriodPrefix(period int) string {
	switch {
	case period >= 1 && period <= 4:
		return fmt.Sprintf("Q%d", period)
	case period == 5:
		return "OT"
	default:
		return fmt.Sprintf("%dOT", period-4)
	}
}
