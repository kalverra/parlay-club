package progress

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/espn"
	"github.com/kalverra/parlay-club/internal/parlay"
)

// State represents the live or settled evaluation state of a leg or slip.
type State string

// The evaluation states a leg or slip can carry.
const (
	StateScheduled   State = "SCHEDULED"
	StateLive        State = "LIVE"
	StateCashed      State = "CASHED"
	StateBusted      State = "BUSTED"
	StatePush        State = "PUSH"
	StateVoid        State = "VOID"
	StateUnavailable State = "UNAVAILABLE"
)

const (
	toneWon      = "won"
	toneOnTrack  = "on_track"
	toneTrailing = "trailing"
	toneLost     = "lost"
	toneNeutral  = "neutral"

	sigmaMargin    = 13.5
	sigmaGameTotal = 13.5
	sigmaTeamTotal = 9.5
)

// LegProgress describes the live or settled progress and hit probability of
// one parlay leg.
type LegProgress struct {
	State       State
	ProgressPct float64 // 0..100 fill width
	Tone        string  // "won", "on_track", "trailing", "lost", "neutral"
	StatLabel   string  // e.g. "182 / 245.5 yds", "BUF 21-17 (+7.5)"
	ClockLabel  string  // e.g. "Q3 8:42", "Halftime", "Final"
	PreGameProb float64 // 0..1 from parlay.ImpliedProb(leg.Odds)
	LiveProb    float64 // 0..1 updated hit probability
	Counts      bool    // false for PUSH/VOID
	HasLive     bool    // true when live/settled progress UI should show
}

// SlipProgress describes the aggregate live or settled progress and hit
// probability of a parlay slip.
type SlipProgress struct {
	Show        bool
	State       State   // LIVE, CASHED, BUSTED, PUSH, SCHEDULED
	ProgressPct float64 // 0..100 aggregate fill width
	Tone        string  // "won", "on_track", "trailing", "lost", "neutral"
	Summary     string  // e.g. "2/4 hit · 1 live"
	PreGameProb float64 // product of counting pre-game implied probs
	LiveProb    float64 // product of counting live probs
	HitCount    int
	LiveCount   int
	LostCount   int
	TotalCount  int
}

// EvaluateLeg computes the live or settled progress and hit probability for a
// single leg given its game state and optional ESPN box score.
func EvaluateLeg(leg db.Leg, game *db.Game, _ *espn.BoxScore) LegProgress {
	preProb := parlay.ImpliedProb(leg.Odds)
	clockLabel := ""
	if game != nil {
		clockLabel = ClockLabel(game.Status, game.Period, game.Clock)
	}

	if settled, ok := evaluateSettled(leg, preProb, clockLabel); ok {
		return settled
	}

	if game == nil || game.Status == db.GameScheduled {
		return LegProgress{
			State:       StateScheduled,
			Tone:        toneNeutral,
			ClockLabel:  clockLabel,
			PreGameProb: preProb,
			LiveProb:    preProb,
			Counts:      true,
		}
	}

	if game.Status != db.GameLive {
		return unavailableLeg(preProb, clockLabel)
	}

	market := strings.ToUpper(strings.TrimSpace(leg.MarketType))
	remFrac := RemainingFraction(game.Status, game.Period, game.Clock)
	p0 := anchorProb(preProb)

	switch market {
	case "SPREAD":
		return evaluateSpread(leg, game, preProb, p0, remFrac, clockLabel)
	case "ML":
		return evaluateMoneyline(leg, game, preProb, p0, remFrac, clockLabel)
	case "TOTAL":
		return evaluateTotal(leg, game, preProb, p0, remFrac, clockLabel)
	case "TEAM_TOTAL":
		return evaluateTeamTotal(leg, game, preProb, p0, remFrac, clockLabel)
	default:
		return unavailableLeg(preProb, clockLabel)
	}
}

func evaluateSettled(leg db.Leg, preProb float64, clockLabel string) (LegProgress, bool) {
	switch leg.Result {
	case parlay.Won:
		return LegProgress{
			State:       StateCashed,
			ProgressPct: 100,
			Tone:        toneWon,
			ClockLabel:  clockLabel,
			PreGameProb: preProb,
			LiveProb:    1.0,
			Counts:      true,
			HasLive:     true,
		}, true
	case parlay.Lost:
		return LegProgress{
			State:       StateBusted,
			ProgressPct: 0,
			Tone:        toneLost,
			ClockLabel:  clockLabel,
			PreGameProb: preProb,
			LiveProb:    0.0,
			Counts:      true,
			HasLive:     true,
		}, true
	case parlay.Push:
		return LegProgress{
			State:       StatePush,
			ProgressPct: 0,
			Tone:        toneNeutral,
			ClockLabel:  clockLabel,
			PreGameProb: preProb,
			LiveProb:    0.0,
			Counts:      false,
			HasLive:     true,
		}, true
	case parlay.Void:
		return LegProgress{
			State:       StateVoid,
			ProgressPct: 0,
			Tone:        toneNeutral,
			ClockLabel:  clockLabel,
			PreGameProb: preProb,
			LiveProb:    0.0,
			Counts:      false,
			HasLive:     true,
		}, true
	default:
		return LegProgress{}, false
	}
}

func unavailableLeg(preProb float64, clockLabel string) LegProgress {
	return LegProgress{
		State:       StateUnavailable,
		Tone:        toneNeutral,
		ClockLabel:  clockLabel,
		PreGameProb: preProb,
		LiveProb:    preProb,
		Counts:      true,
	}
}

func evaluateSpread(leg db.Leg, game *db.Game, preProb, p0, remFrac float64, clockLabel string) LegProgress {
	if leg.Line == nil {
		return unavailableLeg(preProb, clockLabel)
	}
	team := derefString(leg.Team)
	teamScore, oppScore, ok := teamScores(team, game)
	if !ok {
		return unavailableLeg(preProb, clockLabel)
	}

	line := *leg.Line
	liveProb := marginLiveProb(float64(teamScore-oppScore), line, p0, remFrac)
	statLabel := fmt.Sprintf(
		"%s %d-%d (%s)",
		strings.ToUpper(strings.TrimSpace(team)),
		teamScore,
		oppScore,
		formatSignedFloat(line),
	)
	return LegProgress{
		State:       StateLive,
		ProgressPct: liveProb * 100,
		Tone:        liveTone(liveProb, p0),
		StatLabel:   statLabel,
		ClockLabel:  clockLabel,
		PreGameProb: preProb,
		LiveProb:    liveProb,
		Counts:      true,
		HasLive:     true,
	}
}

func evaluateMoneyline(leg db.Leg, game *db.Game, preProb, p0, remFrac float64, clockLabel string) LegProgress {
	team := derefString(leg.Team)
	teamScore, oppScore, ok := teamScores(team, game)
	if !ok {
		return unavailableLeg(preProb, clockLabel)
	}

	liveProb := marginLiveProb(float64(teamScore-oppScore), 0, p0, remFrac)
	return LegProgress{
		State:       StateLive,
		ProgressPct: liveProb * 100,
		Tone:        liveTone(liveProb, p0),
		StatLabel:   fmt.Sprintf("%s %d-%d", strings.ToUpper(strings.TrimSpace(team)), teamScore, oppScore),
		ClockLabel:  clockLabel,
		PreGameProb: preProb,
		LiveProb:    liveProb,
		Counts:      true,
		HasLive:     true,
	}
}

func marginLiveProb(rawMargin, line, p0, remFrac float64) float64 {
	muFull := -line + sigmaMargin*normInv(p0)
	muRem := muFull * remFrac
	sigmaRem := sigmaMargin * math.Sqrt(remFrac)
	cur := rawMargin + line
	return clamp(normCDF((cur+muRem)/sigmaRem), minLiveProb, maxLiveProb)
}

func evaluateTotal(leg db.Leg, game *db.Game, preProb, p0, remFrac float64, clockLabel string) LegProgress {
	if leg.Line == nil || game.HomeScore == nil || game.AwayScore == nil {
		return unavailableLeg(preProb, clockLabel)
	}
	side := strings.ToUpper(strings.TrimSpace(derefString(leg.Side)))
	if side != "OVER" && side != "UNDER" {
		return unavailableLeg(preProb, clockLabel)
	}

	cur := float64(*game.HomeScore + *game.AwayScore)
	line := *leg.Line
	statLabel := fmt.Sprintf("%s / %s pts", formatFloat(cur), formatFloat(line))
	return evaluateMonotonicNormalTotal(cur, line, side, sigmaGameTotal, preProb, p0, remFrac, statLabel, clockLabel)
}

func evaluateTeamTotal(leg db.Leg, game *db.Game, preProb, p0, remFrac float64, clockLabel string) LegProgress {
	if leg.Line == nil {
		return unavailableLeg(preProb, clockLabel)
	}
	side := strings.ToUpper(strings.TrimSpace(derefString(leg.Side)))
	if side != "OVER" && side != "UNDER" {
		return unavailableLeg(preProb, clockLabel)
	}
	team := derefString(leg.Team)
	teamScore, _, ok := teamScores(team, game)
	if !ok {
		return unavailableLeg(preProb, clockLabel)
	}

	cur := float64(teamScore)
	line := *leg.Line
	statLabel := fmt.Sprintf("%s %d / %s pts", strings.ToUpper(strings.TrimSpace(team)), teamScore, formatFloat(line))
	return evaluateMonotonicNormalTotal(cur, line, side, sigmaTeamTotal, preProb, p0, remFrac, statLabel, clockLabel)
}

func evaluateMonotonicNormalTotal(
	cur, line float64,
	side string,
	sigmaFull, preProb, p0, remFrac float64,
	statLabel, clockLabel string,
) LegProgress {
	if cur > line {
		if side == "OVER" {
			return LegProgress{
				State:       StateCashed,
				ProgressPct: 100,
				Tone:        toneWon,
				StatLabel:   statLabel,
				ClockLabel:  clockLabel,
				PreGameProb: preProb,
				LiveProb:    1.0,
				Counts:      true,
				HasLive:     true,
			}
		}
		return LegProgress{
			State:       StateBusted,
			ProgressPct: 0,
			Tone:        toneLost,
			StatLabel:   statLabel,
			ClockLabel:  clockLabel,
			PreGameProb: preProb,
			LiveProb:    0.0,
			Counts:      true,
			HasLive:     true,
		}
	}

	sigmaRem := sigmaFull * math.Sqrt(remFrac)
	var liveProb float64
	var progressPct float64

	if side == "OVER" {
		muFull := line + sigmaFull*normInv(p0)
		muRem := muFull * remFrac
		liveProb = clamp(normCDF((cur+muRem-line)/sigmaRem), minLiveProb, maxLiveProb)
		if line > 0 {
			progressPct = clamp(cur/line*100, 0, 100)
		}
	} else {
		muFull := line - sigmaFull*normInv(p0)
		muRem := muFull * remFrac
		liveProb = clamp(normCDF((line-cur-muRem)/sigmaRem), minLiveProb, maxLiveProb)
		if line > 0 {
			progressPct = clamp((line-cur)/line*100, 0, 100)
		}
	}

	return LegProgress{
		State:       StateLive,
		ProgressPct: progressPct,
		Tone:        liveTone(liveProb, p0),
		StatLabel:   statLabel,
		ClockLabel:  clockLabel,
		PreGameProb: preProb,
		LiveProb:    liveProb,
		Counts:      true,
		HasLive:     true,
	}
}

func liveTone(liveProb, p0 float64) string {
	if liveProb >= p0 {
		return toneOnTrack
	}
	return toneTrailing
}

func teamScores(team string, game *db.Game) (teamScore, oppScore int, ok bool) {
	if game == nil || game.HomeScore == nil || game.AwayScore == nil {
		return 0, 0, false
	}
	t := strings.ToUpper(strings.TrimSpace(team))
	if t == "" {
		return 0, 0, false
	}
	switch t {
	case strings.ToUpper(strings.TrimSpace(game.HomeTeam)):
		return *game.HomeScore, *game.AwayScore, true
	case strings.ToUpper(strings.TrimSpace(game.AwayTeam)):
		return *game.AwayScore, *game.HomeScore, true
	default:
		return 0, 0, false
	}
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func formatSignedFloat(v float64) string {
	if v > 0 {
		return "+" + formatFloat(v)
	}
	return formatFloat(v)
}
