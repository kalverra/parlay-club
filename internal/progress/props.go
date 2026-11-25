package progress

import (
	"fmt"
	"math"
	"strings"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/grader"
	"github.com/kalverra/parlay-club/internal/ingest/espn"
)

const (
	poissonSearchIters = 20
	maxPoissonLambda   = 20.0
)

type propSpec struct {
	cv        float64
	monotonic bool
	unit      string
}

var propSpecs = map[string]propSpec{
	"PASS_YDS":   {cv: 0.30, monotonic: false, unit: "yds"},
	"RUSH_YDS":   {cv: 0.45, monotonic: false, unit: "yds"},
	"REC_YDS":    {cv: 0.50, monotonic: false, unit: "yds"},
	"RUSH_ATT":   {cv: 0.30, monotonic: true, unit: "att"},
	"RECEPTIONS": {cv: 0.40, monotonic: true, unit: "rec"},
	"KICKER_PTS": {cv: 0.45, monotonic: true, unit: "pts"},
}

func evaluatePlayerProp(
	market string,
	leg db.Leg,
	box *espn.BoxScore,
	preProb, p0, remFrac float64,
	clockLabel string,
) LegProgress {
	side := strings.ToUpper(strings.TrimSpace(derefString(leg.Side)))
	if market == "ANYTIME_TD" {
		if side == "" {
			side = "YES"
		}
		if side != "YES" && side != "NO" {
			return unavailableLeg(preProb, clockLabel)
		}
	} else {
		if leg.Line == nil {
			return unavailableLeg(preProb, clockLabel)
		}
		if side != "OVER" && side != "UNDER" {
			return unavailableLeg(preProb, clockLabel)
		}
	}

	playerID := strings.TrimSpace(derefString(leg.PlayerID))
	normName := db.NormalizePlayerName(derefString(leg.PlayerName))
	if playerID == "" && normName == "" {
		return unavailableLeg(preProb, clockLabel)
	}

	if box == nil || !box.HasStats {
		return LegProgress{
			State:       StateLive,
			ProgressPct: 0,
			Tone:        toneNeutral,
			StatLabel:   "Live · stats pending",
			ClockLabel:  clockLabel,
			PreGameProb: preProb,
			LiveProb:    preProb,
			Counts:      true,
			HasLive:     true,
		}
	}

	team := derefString(leg.Team)
	if strings.TrimSpace(team) == "" {
		_, team = db.StripPlayerTag(derefString(leg.PlayerName))
	}
	// Absent players in a populated live box score evaluate with 0 stats rather
	// than voiding before the game is final.
	player, _ := grader.FindPlayer(box.Players, playerID, normName, team)

	if market == "ANYTIME_TD" {
		return evaluateAnytimeTD(player, side, preProb, p0, remFrac, clockLabel)
	}
	if market == "PASS_TDS" {
		return evaluatePassTDs(player, *leg.Line, side, preProb, p0, remFrac, clockLabel)
	}

	spec, ok := propSpecs[market]
	if !ok {
		return unavailableLeg(preProb, clockLabel)
	}
	cur, ok := grader.PlayerMarketStat(market, player)
	if !ok {
		return unavailableLeg(preProb, clockLabel)
	}

	line := *leg.Line
	statLabel := fmt.Sprintf("%s / %s %s", formatFloat(cur), formatFloat(line), spec.unit)
	sigmaFull := spec.cv * math.Max(line, 1.0)
	if spec.monotonic {
		return evaluateMonotonicNormalTotal(cur, line, side, sigmaFull, preProb, p0, remFrac, statLabel, clockLabel)
	}
	return evaluateNormalOverUnder(cur, line, side, sigmaFull, preProb, p0, remFrac, statLabel, clockLabel)
}

func evaluateAnytimeTD(
	player espn.PlayerBoxStats,
	side string,
	preProb, p0, remFrac float64,
	clockLabel string,
) LegProgress {
	curTDs := grader.AnytimeTDCount(player)
	statLabel := fmt.Sprintf("%d TD", curTDs)
	if curTDs >= 1 {
		return earlyMonotonicResult(side == "YES", preProb, statLabel, clockLabel)
	}

	p0Yes := p0
	if side == "NO" {
		p0Yes = 1.0 - p0
	}
	lambdaFull := -math.Log(1.0 - p0Yes)
	pYes := 1.0 - math.Exp(-lambdaFull*remFrac)

	var liveProb float64
	var progressPct float64
	if side == "YES" {
		liveProb = clamp(pYes, minLiveProb, maxLiveProb)
		progressPct = 0
	} else {
		liveProb = clamp(1.0-pYes, minLiveProb, maxLiveProb)
		progressPct = 100
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

func evaluatePassTDs(
	player espn.PlayerBoxStats,
	line float64,
	side string,
	preProb, p0, remFrac float64,
	clockLabel string,
) LegProgress {
	cur := float64(player.Passing.PassingTDs)
	statLabel := fmt.Sprintf("%s / %s TD", formatFloat(cur), formatFloat(line))
	if cur > line {
		return earlyMonotonicResult(side == "OVER", preProb, statLabel, clockLabel)
	}

	var liveProb float64
	var progressPct float64

	if side == "OVER" {
		kFull := max(int(math.Floor(line))+1, 1)
		lambdaFull := solvePoissonLambda(kFull, p0)
		needed := int(math.Floor(line-cur)) + 1
		liveProb = clamp(poissonTail(needed, lambdaFull*remFrac), minLiveProb, maxLiveProb)
		if line > 0 {
			progressPct = clamp(cur/line*100, 0, 100)
		}
	} else {
		kFull := max(int(math.Ceil(line)), 1)
		lambdaFull := solvePoissonLambda(kFull, 1.0-p0)
		needed := int(math.Ceil(line - cur))
		liveProb = clamp(1.0-poissonTail(needed, lambdaFull*remFrac), minLiveProb, maxLiveProb)
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

// solvePoissonLambda solves P(Poisson(lambda) >= k) = targetProb via bounded
// binary search.
func solvePoissonLambda(k int, targetProb float64) float64 {
	lo, hi := 0.0, maxPoissonLambda
	for range poissonSearchIters {
		mid := 0.5 * (lo + hi)
		if poissonTail(k, mid) < targetProb {
			lo = mid
		} else {
			hi = mid
		}
	}
	return 0.5 * (lo + hi)
}

// poissonTail returns P(X >= k) for X ~ Poisson(lambda).
func poissonTail(k int, lambda float64) float64 {
	if k <= 0 {
		return 1.0
	}
	if lambda <= 0 {
		return 0.0
	}
	term := math.Exp(-lambda)
	cdf := term
	for i := 1; i < k; i++ {
		term *= lambda / float64(i)
		cdf += term
	}
	return clamp(1.0-cdf, 0.0, 1.0)
}
