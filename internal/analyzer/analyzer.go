// Package analyzer runs the sharp-friend checks under a slip. It is pure and
// deterministic: no I/O, no clock, and no LLM calls. The rule spec lives in
// plan/analyzer.md.
package analyzer

import (
	"cmp"
	"slices"
)

// Severity orders a warning from quietest to loudest. The zero value is Info,
// so a Warning built without a severity is harmless.
type Severity int

// The three warning levels. Danger sorts first in output.
const (
	// Info is a friendly note.
	Info Severity = iota
	// Warn is a heads-up about a pick that fights the script.
	Warn
	// Danger is a real conflict the slip should probably fix.
	Danger
)

// Warning is one analyzer finding.
type Warning struct {
	Rule     string // stable ID from the rule list
	Severity Severity
	Message  string
	LegIDs   []int64 // legs involved, in slip order
}

// Leg is one slip leg as the analyzer sees it.
type Leg struct {
	ID         int64
	Owner      string // user name, for messages
	GameID     string // "" when unknown
	Market     string // product.md market names
	Team       string // ESPN abbreviation; "" for TOTAL
	PlayerID   string // ESPN athlete ID; "" when unmatched
	PlayerName string
	Side       string // TEAM, OVER, UNDER, YES
	Line       *float64
	Odds       int // American
}

// Game is the slice of a game the analyzer needs: the two teams, whether it is
// indoors, the forecast when there is one, and nfelo's model when it is fresh.
type Game struct {
	ID      string
	Home    string
	Away    string
	Indoor  bool
	Weather *Weather // nil when unknown
	Nfelo   *Nfelo   // nil when unknown or stale
}

// Nfelo is nfelo's pre-game model for one game. HomeSpread is from the home
// side, so a negative number favors the home team. HomeWinProb is the home
// team's win probability. The QB adjustments are Elo points.
type Nfelo struct {
	HomeSpread, HomeWinProb float64
	HomeQB, AwayQB          string
	HomeQBAdj, AwayQBAdj    float64
}

// Weather is the forecast for an outdoor game.
type Weather struct {
	WindMPH, GustMPH, PrecipIn, TempF float64
}

// modelSpread returns the model's spread for team in the same sign a spread
// leg on that team uses. It reports false when there is no model or the team is
// not in the game.
func (g Game) modelSpread(team string) (float64, bool) {
	if g.Nfelo == nil {
		return 0, false
	}
	switch team {
	case g.Home:
		return g.Nfelo.HomeSpread, true
	case g.Away:
		return -g.Nfelo.HomeSpread, true
	default:
		return 0, false
	}
}

// modelWinProb returns the model's win probability for team. It reports false
// when there is no model or the team is not in the game.
func (g Game) modelWinProb(team string) (float64, bool) {
	if g.Nfelo == nil {
		return 0, false
	}
	switch team {
	case g.Home:
		return g.Nfelo.HomeWinProb, true
	case g.Away:
		return 1 - g.Nfelo.HomeWinProb, true
	default:
		return 0, false
	}
}

// PlayerStats holds a matched player's recent games per market, most recent
// game first, this season only.
type PlayerStats struct {
	Games map[string][]float64
}

// AnalyzeSlip runs every rule over the slip and returns the warnings sorted by
// severity (Danger first), then by the first leg's position in the slip. Legs
// with market OTHER, no game, or a game missing from games are skipped.
func AnalyzeSlip(legs []Leg, games map[string]Game, stats map[string]PlayerStats) []Warning {
	// median_trap reads stats once props-research brings player numbers.
	// No Phase 1 rule needs them yet.
	_ = stats

	a := &analysis{
		games:     games,
		positions: make(map[int64]int, len(legs)),
		legs:      make([]Leg, 0, len(legs)),
	}
	for i, leg := range legs {
		a.positions[leg.ID] = i
		if leg.Market == marketOther || leg.GameID == "" {
			continue
		}
		if _, ok := games[leg.GameID]; !ok {
			continue
		}
		a.legs = append(a.legs, leg)
	}

	var warnings []Warning
	for _, rule := range rules {
		warnings = append(warnings, rule(a)...)
	}
	slices.SortStableFunc(warnings, func(left, right Warning) int {
		if bySeverity := cmp.Compare(right.Severity, left.Severity); bySeverity != 0 {
			return bySeverity
		}
		return cmp.Compare(a.position(left), a.position(right))
	})
	return warnings
}

// rule is one check. Each runs over the same filtered legs in a fixed order.
type rule func(*analysis) []Warning

// rules is the fixed run order. Later phases append their rules here; the
// final sort handles severity and slip position.
var rules = []rule{
	blowoutPassing,
	trailingRB,
	totalContradiction,
	stackClockDrain,
	stackShootout,
	wind,
	fakeWeather,
	hook,
	totalKeyNumber,
	chalk,
	qbDrag,
}

// analysis is the filtered input shared by every rule.
type analysis struct {
	legs      []Leg
	games     map[string]Game
	positions map[int64]int
}

// warning builds one finding with its legs in slip order.
func (a *analysis) warning(rule string, severity Severity, message string, legs []Leg) Warning {
	ordered := slices.Clone(legs)
	slices.SortStableFunc(ordered, func(left, right Leg) int {
		return cmp.Compare(a.positions[left.ID], a.positions[right.ID])
	})
	ids := make([]int64, len(ordered))
	for i, leg := range ordered {
		ids[i] = leg.ID
	}
	return Warning{Rule: rule, Severity: severity, Message: message, LegIDs: ids}
}

// position is the slip position of a warning's first leg. A warning with no
// legs sorts last within its severity.
func (a *analysis) position(w Warning) int {
	if len(w.LegIDs) == 0 {
		return len(a.legs)
	}
	if pos, ok := a.positions[w.LegIDs[0]]; ok {
		return pos
	}
	return len(a.legs)
}
