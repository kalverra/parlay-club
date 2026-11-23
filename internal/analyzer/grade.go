package analyzer

import (
	"fmt"
	"math"
	"strings"
)

const (
	legBaselineScore = 75

	formUsageMin = -20
	formUsageMax = 15

	matchupEnvMin = -20
	matchupEnvMax = 15

	linePriceMin = -20
	linePriceMax = 10

	chalkMaxOdds    = -500
	chalkMinPenalty = -10
	chalkMaxPenalty = -18

	stackBonus            = 8
	warnConflictPenalty   = -12
	dangerConflictPenalty = -20
	parlayDragPerLeg      = -3
	parlayDragFreeLegs    = 3
	chalkDragPerExtraLeg  = -5
)

// GradeFactor is one scoring adjustment applied to a leg or slip grade.
type GradeFactor struct {
	Rule   string
	Label  string
	Delta  int
	Detail string
}

// LegGrade is the deterministic quality grade for one slip leg.
type LegGrade struct {
	LegID   int64
	Letter  string        // "A", "B", "C", "D", "F", or "" when skipped
	Score   int           // 0..100 (0 when skipped)
	Factors []GradeFactor // applied adjustments in deterministic order
}

// SlipGrade is the deterministic overall quality grade for a parlay slip.
type SlipGrade struct {
	Letter    string        // "A", "B", "C", "D", "F", or "" when no legs are gradable
	Score     int           // 0..100
	Summary   string        // one-line sharp-friend summary
	Factors   []GradeFactor // slip-level adjustments (correlation, parlay drag, chalk drag)
	LegGrades []LegGrade    // per-leg grades in input leg order
}

// ScoreLeg computes the deterministic A-F quality grade and factor breakdown
// for a single leg. Legs with market OTHER, no game, or a missing game are
// skipped (Letter == "", Score == 0).
func ScoreLeg(leg Leg, games map[string]Game, stats map[string]PlayerStats) LegGrade {
	if leg.Market == marketOther || leg.GameID == "" {
		return LegGrade{LegID: leg.ID}
	}
	game, ok := games[leg.GameID]
	if !ok {
		return LegGrade{LegID: leg.ID}
	}

	var factors []GradeFactor
	factors = append(factors, scoreFormAndUsage(leg, game, stats)...)
	factors = append(factors, scoreMatchupAndEnv(leg, game)...)
	factors = append(factors, scoreLineAndPrice(leg, game)...)

	score := legBaselineScore
	for _, f := range factors {
		score += f.Delta
	}
	score = clampInt(score, 0, 100)

	return LegGrade{
		LegID:   leg.ID,
		Letter:  letterGrade(score),
		Score:   score,
		Factors: factors,
	}
}

// ScoreSlip computes per-leg grades and the overall parlay grade from the
// graded legs' baseline, script correlations, parlay length drag, and chalk
// drag.
func ScoreSlip(legs []Leg, games map[string]Game, stats map[string]PlayerStats) SlipGrade {
	legGrades := make([]LegGrade, len(legs))
	gradedSum := 0
	gradedCount := 0
	for i, leg := range legs {
		lg := ScoreLeg(leg, games, stats)
		legGrades[i] = lg
		if lg.Letter != "" {
			gradedSum += lg.Score
			gradedCount++
		}
	}
	if gradedCount == 0 {
		return SlipGrade{LegGrades: legGrades}
	}

	baseline := int(math.Round(float64(gradedSum) / float64(gradedCount)))
	warnings := AnalyzeSlip(legs, games, stats)

	var factors []GradeFactor
	chalkCount := 0
	for _, w := range warnings {
		switch w.Rule {
		case ruleStackShootout, ruleStackClockDrain:
			factors = append(factors, GradeFactor{
				Rule:   w.Rule,
				Label:  "Positive stack",
				Delta:  stackBonus,
				Detail: w.Message,
			})
		case ruleBlowoutPassing:
			factors = append(factors, GradeFactor{
				Rule:   w.Rule,
				Label:  "Script conflict",
				Delta:  warnConflictPenalty,
				Detail: w.Message,
			})
		case ruleTrailingRB, ruleTotalContradiction:
			factors = append(factors, GradeFactor{
				Rule:   w.Rule,
				Label:  "Danger conflict",
				Delta:  dangerConflictPenalty,
				Detail: w.Message,
			})
		case ruleChalk:
			chalkCount++
		}
	}

	if len(legs) > parlayDragFreeLegs {
		extra := len(legs) - parlayDragFreeLegs
		factors = append(factors, GradeFactor{
			Rule:   "parlay_drag",
			Label:  fmt.Sprintf("%d-leg parlay drag", len(legs)),
			Delta:  parlayDragPerLeg * extra,
			Detail: fmt.Sprintf("-%d per leg beyond %d legs (%d extra).", -parlayDragPerLeg, parlayDragFreeLegs, extra),
		})
	}
	if chalkCount > 1 {
		extra := chalkCount - 1
		factors = append(factors, GradeFactor{
			Rule:   "chalk_drag",
			Label:  fmt.Sprintf("%d chalk legs", chalkCount),
			Delta:  chalkDragPerExtraLeg * extra,
			Detail: fmt.Sprintf("-%d per heavy favorite beyond the first (%d extra).", -chalkDragPerExtraLeg, extra),
		})
	}

	score := baseline
	for _, f := range factors {
		score += f.Delta
	}
	score = clampInt(score, 0, 100)
	letter := letterGrade(score)

	return SlipGrade{
		Letter:    letter,
		Score:     score,
		Summary:   slipGradeSummary(gradedCount, factors, warnings),
		Factors:   factors,
		LegGrades: legGrades,
	}
}

func scoreFormAndUsage(leg Leg, game Game, stats map[string]PlayerStats) []GradeFactor {
	if leg.PlayerID == "" || len(stats) == 0 {
		return nil
	}
	ps, ok := stats[leg.PlayerID]
	if !ok {
		return nil
	}

	outTeammate, hasOutTeammate := outSamePositionTeammate(leg, game, ps.Position)

	var candidates []GradeFactor
	if isYardageMarket(leg.Market) && leg.Side == sideOver && leg.Line != nil {
		if games := ps.Games[leg.Market]; len(games) >= 3 {
			median := Median(games[:min(5, len(games))])
			if *leg.Line > median {
				delta := -15
				detail := fmt.Sprintf("Line %s sits above last-5 median of %s.", number(*leg.Line), number(median))
				if hasOutTeammate {
					delta = -5
					detail = fmt.Sprintf(
						"Line %s sits above last-5 median of %s, softened with %s teammate %s Out.",
						number(*leg.Line),
						number(median),
						strings.ToUpper(strings.TrimSpace(ps.Position)),
						outTeammate.FullName,
					)
				}
				candidates = append(candidates, GradeFactor{
					Rule:   ruleMedianTrap,
					Label:  "L5 median trap",
					Delta:  delta,
					Detail: detail,
				})
			}
		}
	}

	if hits, total, ok := l5HitRate(leg, ps); ok && hits >= 4 {
		candidates = append(candidates, GradeFactor{
			Rule:   "hit_rate",
			Label:  fmt.Sprintf("Strong L5 hit rate (%d/%d)", hits, total),
			Delta:  10,
			Detail: fmt.Sprintf("Cleared this mark in %d of the last %d games.", hits, total),
		})
	}

	if (leg.Side == sideOver || leg.Side == sideYes) && ps.Usage != nil && len(ps.Usage.Weeks) >= 3 {
		snapGain := ps.Usage.Last3SnapAvg - ps.Usage.SeasonSnapAvg
		targetGain := ps.Usage.Last3TargetShare - ps.Usage.SeasonTargetShare
		if snapGain >= risingSnapGain-floatEps || targetGain >= risingTargetShareGain-floatEps {
			candidates = append(candidates, GradeFactor{
				Rule:   "rising_usage",
				Label:  "Rising usage",
				Delta:  5,
				Detail: "Last-3-game snap or target share is trending above season average.",
			})
		}
	}

	if hasOutTeammate {
		candidates = append(candidates, GradeFactor{
			Rule:  "teammate_out_bump",
			Label: "Teammate out (role bump)",
			Delta: 5,
			Detail: fmt.Sprintf(
				"%s %s %s is Out, opening extra volume.",
				leg.Team,
				strings.ToUpper(strings.TrimSpace(ps.Position)),
				outTeammate.FullName,
			),
		})
	}

	if reason, hit := shrinkingRoleHit(leg, ps); hit {
		candidates = append(candidates, GradeFactor{
			Rule:   ruleShrinkingRole,
			Label:  "Shrinking role",
			Delta:  -15,
			Detail: fmt.Sprintf("%s's %s.", playerName(leg), reason),
		})
	}

	return clampCategoryFactors(candidates, formUsageMin, formUsageMax)
}

func outSamePositionTeammate(leg Leg, game Game, position string) (Injury, bool) {
	if leg.Team == "" || !isEligibleTeammateOutLeg(leg) {
		return Injury{}, false
	}
	pos := strings.ToUpper(strings.TrimSpace(position))
	if !isSkillVolumePosition(pos) {
		return Injury{}, false
	}
	for _, inj := range game.Injuries[leg.Team] {
		if !strings.EqualFold(strings.TrimSpace(inj.Status), "Out") {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(inj.Position), pos) {
			continue
		}
		if leg.PlayerID != "" && inj.AthleteID != "" && inj.AthleteID == leg.PlayerID {
			continue
		}
		if leg.PlayerName != "" && inj.FullName != "" &&
			strings.EqualFold(strings.TrimSpace(inj.FullName), strings.TrimSpace(leg.PlayerName)) {
			continue
		}
		return inj, true
	}
	return Injury{}, false
}

func isSkillVolumePosition(pos string) bool {
	switch pos {
	case "WR", "TE", "RB":
		return true
	default:
		return false
	}
}

func isEligibleTeammateOutLeg(leg Leg) bool {
	switch leg.Market {
	case marketRecYds, marketReceptions, marketRushYds, marketRushAtt:
		return leg.Side == sideOver
	case marketAnytimeTD:
		return leg.Side == sideYes
	default:
		return false
	}
}

func scoreMatchupAndEnv(leg Leg, game Game) []GradeFactor {
	var candidates []GradeFactor

	if split, rank, hit := favorableMatchupHit(leg, game); hit {
		candidates = append(candidates, GradeFactor{
			Rule:   "favorable_matchup",
			Label:  fmt.Sprintf("Weak %s defense (#%d)", split, rank),
			Delta:  10,
			Detail: fmt.Sprintf("Opposing %s defense ranks #%d in EPA allowed.", split, rank),
		})
	} else if split, rank, hit := toughMatchupHit(leg, game); hit {
		candidates = append(candidates, GradeFactor{
			Rule:   ruleToughMatchup,
			Label:  fmt.Sprintf("Tough %s defense (#%d)", split, rank),
			Delta:  -12,
			Detail: fmt.Sprintf("Opposing %s defense ranks #%d in EPA allowed.", split, rank),
		})
	}

	if !game.Indoor && game.Weather != nil {
		if windHit(leg) {
			if sev, windy := windLevel(*game.Weather); windy {
				delta := -8
				if sev == Danger {
					delta = -18
				}
				candidates = append(candidates, GradeFactor{
					Rule:  ruleWind,
					Label: "Wind headwind",
					Delta: delta,
					Detail: fmt.Sprintf(
						"%s in %s hurts passing and kicking overs.",
						windText(*game.Weather),
						game.Home,
					),
				})
			}
		}
		if leg.Market == marketTotal && leg.Side == sideUnder &&
			game.Weather.PrecipIn > 0 &&
			game.Weather.PrecipIn < fakeWeatherPrecipMax &&
			game.Weather.WindMPH < fakeWeatherWindMax {
			candidates = append(candidates, GradeFactor{
				Rule:   ruleFakeWeather,
				Label:  "Light rain myth",
				Delta:  -3,
				Detail: "Light rain without wind does not lower NFL scoring.",
			})
		}
	}

	if team, _, adj, ok := qbDragTeam(game, leg.Team); ok && adj <= qbDragElo && qbDragHit(leg, team) {
		candidates = append(candidates, GradeFactor{
			Rule:   ruleQBDrag,
			Label:  "nfelo QB drag",
			Delta:  -8,
			Detail: fmt.Sprintf("nfelo marks down %s's QB.", team),
		})
	}

	return clampCategoryFactors(candidates, matchupEnvMin, matchupEnvMax)
}

func scoreLineAndPrice(leg Leg, game Game) []GradeFactor {
	var candidates []GradeFactor

	if leg.Market == marketSpread && leg.Line != nil {
		model, hasModel := game.ModelSpread(leg.Team)
		cushion := 0.0
		if hasModel {
			cushion = *leg.Line - model
		}
		if _, _, _, onHook := hookPoint(*leg.Line); onHook {
			delta := -10
			detail := selection(leg) + " sits on a key NFL hook number."
			if hasModel && cushion >= 2 {
				delta = -4
				detail = fmt.Sprintf(
					"%s sits on a hook, softened by nfelo's %s projection.",
					selection(leg),
					modelSpreadText(model),
				)
			}
			candidates = append(candidates, GradeFactor{
				Rule:   ruleHook,
				Label:  "Spread on hook",
				Delta:  delta,
				Detail: detail,
			})
		}
		if hasModel && cushion >= 2 {
			candidates = append(candidates, GradeFactor{
				Rule:  "nfelo_cushion",
				Label: "nfelo spread cushion",
				Delta: 8,
				Detail: fmt.Sprintf(
					"nfelo has %s at %s, giving at least 2 points of cushion.",
					leg.Team,
					modelSpreadText(model),
				),
			})
		}
	}

	if leg.Market == marketTotal && leg.Line != nil {
		if keys := keyNumbersNear(*leg.Line); len(keys) > 0 {
			candidates = append(candidates, GradeFactor{
				Rule:   ruleTotalKeyNumber,
				Label:  "Total on key number",
				Delta:  -5,
				Detail: fmt.Sprintf("Total %s sits near common NFL total %s.", number(*leg.Line), number(keys[0])),
			})
		}
	}

	if leg.Odds <= chalkOdds {
		delta := chalkPenalty(leg.Odds)
		candidates = append(candidates, GradeFactor{
			Rule:   ruleChalk,
			Label:  fmt.Sprintf("Heavy chalk (%d)", leg.Odds),
			Delta:  delta,
			Detail: fmt.Sprintf("Long odds of %d add little payout for the bust risk.", leg.Odds),
		})
	}

	return clampCategoryFactors(candidates, linePriceMin, linePriceMax)
}

// clampCategoryFactors bounds the cumulative delta of a category to [minTotal, maxTotal]
// while keeping sum(factor.Delta) equal to the bounded category total.
func clampCategoryFactors(factors []GradeFactor, minTotal, maxTotal int) []GradeFactor {
	if len(factors) == 0 {
		return nil
	}
	running := 0
	var out []GradeFactor
	for _, f := range factors {
		next := clampInt(running+f.Delta, minTotal, maxTotal)
		applied := next - running
		if applied == 0 {
			continue
		}
		f.Delta = applied
		running = next
		out = append(out, f)
	}
	return out
}

func l5HitRate(leg Leg, ps PlayerStats) (hits, total int, ok bool) {
	games := ps.Games[leg.Market]
	if len(games) < 5 {
		return 0, 0, false
	}
	l5 := games[:5]
	switch {
	case leg.Market == marketAnytimeTD && leg.Side == sideYes:
		for _, v := range l5 {
			if v >= 1 {
				hits++
			}
		}
		return hits, 5, true
	case leg.Line != nil && leg.Side == sideOver:
		for _, v := range l5 {
			if v > *leg.Line {
				hits++
			}
		}
		return hits, 5, true
	case leg.Line != nil && leg.Side == sideUnder:
		for _, v := range l5 {
			if v < *leg.Line {
				hits++
			}
		}
		return hits, 5, true
	default:
		return 0, 0, false
	}
}

func chalkPenalty(odds int) int {
	if odds <= chalkMaxOdds {
		return chalkMaxPenalty
	}
	if odds >= chalkOdds {
		return chalkMinPenalty
	}
	ratio := float64(-odds-(-chalkOdds)) / float64(-chalkMaxOdds-(-chalkOdds))
	penalty := chalkMinPenalty - int(math.Round(ratio*float64(chalkMinPenalty-chalkMaxPenalty)))
	return clampInt(penalty, chalkMaxPenalty, chalkMinPenalty)
}

func letterGrade(score int) string {
	switch {
	case score >= 85:
		return "A"
	case score >= 73:
		return "B"
	case score >= 60:
		return "C"
	case score >= 48:
		return "D"
	default:
		return "F"
	}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func slipGradeSummary(gradedCount int, slipFactors []GradeFactor, warnings []Warning) string {
	var pos []string
	var neg []string
	for _, f := range slipFactors {
		if f.Delta > 0 {
			pos = append(pos, strings.ToLower(f.Label))
		} else if f.Delta < 0 {
			neg = append(neg, strings.ToLower(f.Label))
		}
	}
	if len(pos) == 0 && len(neg) == 0 {
		if len(warnings) == 0 {
			return fmt.Sprintf("Clean %d-leg slip with no major red flags", gradedCount)
		}
		return fmt.Sprintf("%d-leg slip with %d heads-up note(s)", gradedCount, len(warnings))
	}
	var parts []string
	if len(pos) > 0 {
		parts = append(parts, "boosted by "+humanJoin(pos))
	}
	if len(neg) > 0 {
		parts = append(parts, "dragged down by "+humanJoin(neg))
	}
	joined := strings.Join(parts, ", ")
	return strings.ToUpper(joined[:1]) + joined[1:]
}
