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

// Injury is one player's injury status on a game's team injury list.
type Injury struct {
	AthleteID string
	FullName  string
	Position  string
	Status    string
}

// Game is the slice of a game the analyzer needs: the two teams, whether it is
// indoors, the forecast when there is one, nfelo's model when it is fresh,
// optional nflverse team EPA/pace profiles, team injury lists, and optional
// sharp-book fair benchmarks.
type Game struct {
	ID       string
	Home     string
	Away     string
	Indoor   bool
	Weather  *Weather                // nil when unknown
	Nfelo    *Nfelo                  // nil when unknown or stale
	HomeEPA  *TeamDefenseProfile     // nil when unknown or stale (> 7 days)
	AwayEPA  *TeamDefenseProfile     // nil when unknown or stale (> 7 days)
	Injuries map[string][]Injury     // keyed by team abbreviation (e.g. "BUF")
	Sharp    map[SharpKey]SharpQuote `json:"-"` // nil when no sharp benchmarks are available
}

// TeamDefenseProfile holds one team's season-to-date and last-4-week
// offensive/defensive EPA, pace, and league ranks (1..32, where 1 is toughest
// defense, highest offense, fastest pace, or highest pass rate; 0 means unset).
type TeamDefenseProfile struct {
	OffPassEPA        float64
	OffRushEPA        float64
	DefPassEPAAllowed float64
	DefRushEPAAllowed float64
	PlaysPerGame      float64
	PassRate          float64

	L4OffPassEPA        float64
	L4OffRushEPA        float64
	L4DefPassEPAAllowed float64
	L4DefRushEPAAllowed float64
	L4PlaysPerGame      float64
	L4PassRate          float64

	OffPassEPARank        int
	OffRushEPARank        int
	DefPassEPAAllowedRank int
	DefRushEPAAllowedRank int
	PlaysPerGameRank      int
	PassRateRank          int
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

// ModelSpread returns the model's spread for team in the same sign a spread
// leg on that team uses. It reports false when there is no model or the team is
// not in the game.
func (g Game) ModelSpread(team string) (float64, bool) {
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

// ModelWinProb returns the model's win probability for team. It reports false
// when there is no model or the team is not in the game.
func (g Game) ModelWinProb(team string) (float64, bool) {
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

// TeamEPA returns the EPA and pace profile for team, or nil when missing or
// the team is not in the game.
func (g Game) TeamEPA(team string) *TeamDefenseProfile {
	switch team {
	case g.Home:
		return g.HomeEPA
	case g.Away:
		return g.AwayEPA
	default:
		return nil
	}
}

// OpponentEPA returns the opposing team's EPA and pace profile for team, or
// nil when missing or the team is not in the game.
func (g Game) OpponentEPA(team string) *TeamDefenseProfile {
	switch team {
	case g.Home:
		return g.AwayEPA
	case g.Away:
		return g.HomeEPA
	default:
		return nil
	}
}

// PlayerWeekUsage holds one week of nflverse usage and efficiency for a player.
// Share fields (OffensePct, TargetShare, AirYardsShare) are 0..1 fractions.
type PlayerWeekUsage struct {
	WeekNum       int
	OffensePct    float64
	TargetShare   float64
	AirYardsShare float64
	WOPR          float64
	Carries       int
	Targets       int
	TotalEPA      float64
	RZOpps        int
}

// PlayerUsage holds a matched player's nflverse usage weeks (most recent first)
// and rolling averages. Nil when missing or fewer than 3 games exist.
type PlayerUsage struct {
	Weeks             []PlayerWeekUsage
	Last3SnapAvg      float64
	SeasonSnapAvg     float64
	Last3TargetShare  float64
	SeasonTargetShare float64
	Last3WOPR         float64
	AvgCarries        float64
}

// EffectiveTargetShare returns the higher of Last3TargetShare and
// SeasonTargetShare, falling back to averaging Weeks when precomputed fields
// are zero.
func (u *PlayerUsage) EffectiveTargetShare() float64 {
	best := max(u.Last3TargetShare, u.SeasonTargetShare)
	if best > 0 || len(u.Weeks) == 0 {
		return best
	}
	var sum float64
	for _, w := range u.Weeks {
		sum += w.TargetShare
	}
	return sum / float64(len(u.Weeks))
}

// EffectiveSnapAvg returns the higher of Last3SnapAvg and SeasonSnapAvg,
// falling back to averaging Weeks when precomputed fields are zero.
func (u *PlayerUsage) EffectiveSnapAvg() float64 {
	return max(u.Last3SnapAvg, u.EffectiveSeasonSnapAvg())
}

// EffectiveSeasonSnapAvg returns SeasonSnapAvg, falling back to averaging
// Weeks when precomputed fields are zero.
func (u *PlayerUsage) EffectiveSeasonSnapAvg() float64 {
	if u.SeasonSnapAvg > 0 || len(u.Weeks) == 0 {
		return u.SeasonSnapAvg
	}
	var sum float64
	for _, w := range u.Weeks {
		sum += w.OffensePct
	}
	return sum / float64(len(u.Weeks))
}

// RecentWOPR returns Last3WOPR, falling back to averaging up to 3 most-recent
// Weeks when precomputed fields are zero.
func (u *PlayerUsage) RecentWOPR() float64 {
	if u.Last3WOPR > 0 || len(u.Weeks) == 0 {
		return u.Last3WOPR
	}
	n := min(len(u.Weeks), 3)
	var sum float64
	for _, w := range u.Weeks[:n] {
		sum += w.WOPR
	}
	return sum / float64(n)
}

// AverageCarries returns AvgCarries, falling back to averaging Weeks when
// precomputed fields are zero.
func (u *PlayerUsage) AverageCarries() float64 {
	if u.AvgCarries > 0 || len(u.Weeks) == 0 {
		return u.AvgCarries
	}
	var sum int
	for _, w := range u.Weeks {
		sum += w.Carries
	}
	return float64(sum) / float64(len(u.Weeks))
}

// PlayerStats holds a matched player's position, recent games per market,
// most recent game first, this season only, plus optional nflverse usage metrics.
type PlayerStats struct {
	Position string // ESPN position abbreviation (e.g. "TE", "WR", "RB")
	Games    map[string][]float64
	Usage    *PlayerUsage
}

// AnalyzeSlip runs every rule over the slip and returns the warnings sorted by
// severity (Danger first), then by the first leg's position in the slip. Legs
// with market OTHER, no game, or a game missing from games are skipped.
func AnalyzeSlip(legs []Leg, games map[string]Game, stats map[string]PlayerStats) []Warning {
	a := &analysis{
		games:     games,
		stats:     stats,
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
	for _, r := range rules {
		warnings = append(warnings, r.fn(a)...)
	}
	slices.SortStableFunc(warnings, func(left, right Warning) int {
		if bySeverity := cmp.Compare(right.Severity, left.Severity); bySeverity != 0 {
			return bySeverity
		}
		return cmp.Compare(a.position(left), a.position(right))
	})
	return warnings
}

// RuleIDs returns the stable IDs of every active analyzer rule in run order.
func RuleIDs() []string {
	ids := make([]string, len(rules))
	for i, r := range rules {
		ids[i] = r.id
	}
	return ids
}

// rule is one check. Each runs over the same filtered legs in a fixed order.
type rule func(*analysis) []Warning

type ruleSpec struct {
	id string
	fn rule
}

// rules is the fixed run order. Later phases append their rules here; the
// final sort handles severity and slip position.
var rules = []ruleSpec{
	{ruleBlowoutPassing, blowoutPassing},
	{ruleTrailingRB, trailingRB},
	{ruleTotalContradiction, totalContradiction},
	{ruleStackClockDrain, stackClockDrain},
	{ruleStackShootout, stackShootout},
	{ruleWind, wind},
	{ruleFakeWeather, fakeWeather},
	{ruleHook, hook},
	{ruleTotalKeyNumber, totalKeyNumber},
	{ruleChalk, chalk},
	{ruleQBDrag, qbDrag},
	{ruleMedianTrap, medianTrap},
	{ruleToughMatchup, toughMatchup},
	{ruleShrinkingRole, shrinkingRole},
	{rulePaceClash, paceClash},
	{ruleTDSpikeRisk, tdSpikeRisk},
}

// analysis is the filtered input shared by every rule.
type analysis struct {
	legs      []Leg
	games     map[string]Game
	stats     map[string]PlayerStats
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
