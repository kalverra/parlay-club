package analyzer

import (
	"fmt"
	"math"
	"strings"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
)

const (
	chalkMaxOdds = -500

	sharpPriceEdgeThreshold = 0.025
	sharpPriceFadeThreshold = -0.045
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

// SlipEvaluation holds both the analyzer warnings and deterministic grade from
// a single evaluation pass over a parlay slip.
type SlipEvaluation struct {
	Warnings []Warning
	Grade    SlipGrade
}

// ScoreLeg computes the deterministic A-F quality grade and factor breakdown
// for a single leg using ActiveRubric. Legs with market OTHER, no game, or a
// missing game are skipped (Letter == "", Score == 0).
func ScoreLeg(leg Leg, games map[string]Game, stats map[string]PlayerStats) LegGrade {
	return ScoreLegWithRubric(leg, games, stats, ActiveRubric)
}

// ScoreLegWithRubric computes the deterministic A-F quality grade and factor
// breakdown for a single leg using the supplied rubric.
func ScoreLegWithRubric(leg Leg, games map[string]Game, stats map[string]PlayerStats, rubric Rubric) LegGrade {
	if leg.Market == marketOther || leg.GameID == "" {
		return LegGrade{LegID: leg.ID}
	}
	game, ok := games[leg.GameID]
	if !ok {
		return LegGrade{LegID: leg.ID}
	}

	var factors []GradeFactor
	factors = append(factors, scoreFormAndUsage(leg, game, stats, rubric)...)
	factors = append(factors, scoreMatchupAndEnv(leg, game, rubric)...)
	factors = append(factors, scoreLineAndPrice(leg, game, rubric)...)

	score := rubric.LegBaseline
	for _, f := range factors {
		score += f.Delta
	}
	score = clampInt(score, 0, 100)

	return LegGrade{
		LegID:   leg.ID,
		Letter:  letterGrade(score, rubric),
		Score:   score,
		Factors: factors,
	}
}

// ScoreSlip computes per-leg grades and the overall parlay grade using
// ActiveRubric.
func ScoreSlip(legs []Leg, games map[string]Game, stats map[string]PlayerStats) SlipGrade {
	return ScoreSlipWithRubric(legs, games, stats, ActiveRubric)
}

// ScoreSlipWithRubric computes per-leg grades and the overall parlay grade from
// the graded legs' baseline, script correlations, parlay length drag, and chalk
// drag using the supplied rubric.
func ScoreSlipWithRubric(legs []Leg, games map[string]Game, stats map[string]PlayerStats, rubric Rubric) SlipGrade {
	return EvaluateSlipWithRubric(legs, games, stats, rubric).Grade
}

// EvaluateSlip runs slip analysis and rubric grading in a single pass using
// ActiveRubric.
func EvaluateSlip(legs []Leg, games map[string]Game, stats map[string]PlayerStats) SlipEvaluation {
	return EvaluateSlipWithRubric(legs, games, stats, ActiveRubric)
}

// EvaluateSlipWithRubric runs slip analysis once and computes per-leg and
// overall parlay grades using the supplied rubric.
func EvaluateSlipWithRubric(
	legs []Leg,
	games map[string]Game,
	stats map[string]PlayerStats,
	rubric Rubric,
) SlipEvaluation {
	warnings := AnalyzeSlip(legs, games, stats)

	legGrades := make([]LegGrade, len(legs))
	gradedSum := 0
	gradedCount := 0
	for i, leg := range legs {
		lg := ScoreLegWithRubric(leg, games, stats, rubric)
		legGrades[i] = lg
		if lg.Letter != "" {
			gradedSum += lg.Score
			gradedCount++
		}
	}
	if gradedCount == 0 {
		return SlipEvaluation{
			Warnings: warnings,
			Grade:    SlipGrade{LegGrades: legGrades},
		}
	}

	baseline := int(math.Round(float64(gradedSum) / float64(gradedCount)))

	var factors []GradeFactor
	chalkCount := 0
	for _, w := range warnings {
		switch w.Rule {
		case ruleStackShootout, ruleStackClockDrain:
			factors = append(factors, GradeFactor{
				Rule:   w.Rule,
				Label:  "Positive stack",
				Delta:  rubric.StackBonus,
				Detail: w.Message,
			})
		case ruleBlowoutPassing:
			factors = append(factors, GradeFactor{
				Rule:   w.Rule,
				Label:  "Script conflict",
				Delta:  rubric.WarnConflict,
				Detail: w.Message,
			})
		case ruleTrailingRB, ruleTotalContradiction:
			factors = append(factors, GradeFactor{
				Rule:   w.Rule,
				Label:  "Danger conflict",
				Delta:  rubric.DangerConflict,
				Detail: w.Message,
			})
		case ruleChalk:
			chalkCount++
		}
	}

	if len(legs) > rubric.ParlayDragFreeLegs {
		extra := len(legs) - rubric.ParlayDragFreeLegs
		factors = append(factors, GradeFactor{
			Rule:  ruleParlayDrag,
			Label: fmt.Sprintf("%d-leg parlay drag", len(legs)),
			Delta: rubric.ParlayDragPerLeg * extra,
			Detail: fmt.Sprintf(
				"-%d per leg beyond %d legs (%d extra).",
				-rubric.ParlayDragPerLeg,
				rubric.ParlayDragFreeLegs,
				extra,
			),
		})
	}
	if chalkCount > 1 {
		extra := chalkCount - 1
		factors = append(factors, GradeFactor{
			Rule:  ruleChalkDrag,
			Label: fmt.Sprintf("%d chalk legs", chalkCount),
			Delta: rubric.ChalkDragPerExtraLeg * extra,
			Detail: fmt.Sprintf(
				"-%d per heavy favorite beyond the first (%d extra).",
				-rubric.ChalkDragPerExtraLeg,
				extra,
			),
		})
	}

	score := baseline
	for _, f := range factors {
		score += f.Delta
	}
	score = clampInt(score, 0, 100)
	letter := letterGrade(score, rubric)

	return SlipEvaluation{
		Warnings: warnings,
		Grade: SlipGrade{
			Letter:    letter,
			Score:     score,
			Summary:   slipGradeSummary(gradedCount, factors, warnings),
			Factors:   factors,
			LegGrades: legGrades,
		},
	}
}

func scoreFormAndUsage(leg Leg, game Game, stats map[string]PlayerStats, rubric Rubric) []GradeFactor {
	if leg.PlayerID == "" || len(stats) == 0 {
		return nil
	}
	ps, ok := stats[leg.PlayerID]
	if !ok {
		return nil
	}

	outTeammate, hasOutTeammate := outSamePositionTeammate(leg, game, ps.Position)

	var candidates []GradeFactor
	if median, _, hit := evalMedianTrap(leg, stats); hit {
		delta := rubric.MedianTrap
		detail := fmt.Sprintf("Line %s sits above last-5 median of %s.", number(*leg.Line), number(median))
		if hasOutTeammate {
			delta = rubric.MedianTrapSoftened
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

	if hits, total, ok := l5HitRate(leg, ps); ok && hits >= 4 {
		candidates = append(candidates, GradeFactor{
			Rule:   ruleHitRate,
			Label:  fmt.Sprintf("Strong L5 hit rate (%d/%d)", hits, total),
			Delta:  rubric.HitRate,
			Detail: fmt.Sprintf("Cleared this mark in %d of the last %d games.", hits, total),
		})
	}

	if (leg.Side == sideOver || leg.Side == sideYes) && ps.Usage != nil && len(ps.Usage.Weeks) >= 3 {
		snapGain := ps.Usage.Last3SnapAvg - ps.Usage.SeasonSnapAvg
		targetGain := ps.Usage.Last3TargetShare - ps.Usage.SeasonTargetShare
		if snapGain >= risingSnapGain-floatEps || targetGain >= risingTargetShareGain-floatEps {
			candidates = append(candidates, GradeFactor{
				Rule:   ruleRisingUsage,
				Label:  "Rising usage",
				Delta:  rubric.RisingUsage,
				Detail: "Last-3-game snap or target share is trending above season average.",
			})
		}
	}

	if hasOutTeammate {
		candidates = append(candidates, GradeFactor{
			Rule:  ruleTeammateOutBump,
			Label: "Teammate out (role bump)",
			Delta: rubric.TeammateOutBump,
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
			Delta:  rubric.ShrinkingRole,
			Detail: fmt.Sprintf("%s's %s.", playerName(leg), reason),
		})
	}

	return clampCategoryFactors(candidates, rubric.FormUsageMin, rubric.FormUsageMax)
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

func scoreMatchupAndEnv(leg Leg, game Game, rubric Rubric) []GradeFactor {
	var candidates []GradeFactor

	if split, rank, hit := favorableMatchupHit(leg, game); hit {
		candidates = append(candidates, GradeFactor{
			Rule:   ruleFavorableMatchup,
			Label:  fmt.Sprintf("Weak %s defense (#%d)", split, rank),
			Delta:  rubric.FavorableMatchup,
			Detail: fmt.Sprintf("Opposing %s defense ranks #%d in EPA allowed.", split, rank),
		})
	} else if split, rank, hit := toughMatchupHit(leg, game); hit {
		candidates = append(candidates, GradeFactor{
			Rule:   ruleToughMatchup,
			Label:  fmt.Sprintf("Tough %s defense (#%d)", split, rank),
			Delta:  rubric.ToughMatchup,
			Detail: fmt.Sprintf("Opposing %s defense ranks #%d in EPA allowed.", split, rank),
		})
	}

	if sev, windy := evalWindHazard(leg, game); windy {
		delta := rubric.WindWarn
		if sev == Danger {
			delta = rubric.WindDanger
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
	if isFakeWeatherLeg(leg, game) {
		candidates = append(candidates, GradeFactor{
			Rule:   ruleFakeWeather,
			Label:  "Light rain myth",
			Delta:  rubric.FakeWeather,
			Detail: "Light rain without wind does not lower NFL scoring.",
		})
	}

	if team, _, adj, ok := qbDragTeam(game, leg.Team); ok && adj <= qbDragElo && qbDragHit(leg, team) {
		candidates = append(candidates, GradeFactor{
			Rule:   ruleQBDrag,
			Label:  "nfelo QB drag",
			Delta:  rubric.QBDrag,
			Detail: fmt.Sprintf("nfelo marks down %s's QB.", team),
		})
	}

	return clampCategoryFactors(candidates, rubric.MatchupEnvMin, rubric.MatchupEnvMax)
}

func scoreLineAndPrice(leg Leg, game Game, rubric Rubric) []GradeFactor {
	var candidates []GradeFactor

	if leg.Market == marketSpread && leg.Line != nil {
		model, hasModel := game.ModelSpread(leg.Team)
		cushion := 0.0
		if hasModel {
			cushion = *leg.Line - model
		}
		if _, _, _, onHook := evalSpreadHook(leg); onHook {
			delta := rubric.Hook
			detail := selection(leg) + " sits on a key NFL hook number."
			if hasModel && cushion >= 2 {
				delta = rubric.HookSoftened
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
				Rule:  ruleNfeloCushion,
				Label: "nfelo spread cushion",
				Delta: rubric.NfeloCushion,
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
				Delta:  rubric.TotalKeyNumber,
				Detail: fmt.Sprintf("Total %s sits near common NFL total %s.", number(*leg.Line), number(keys[0])),
			})
		}
	}

	if isChalkLeg(leg) {
		delta := chalkPenalty(leg.Odds, rubric)
		candidates = append(candidates, GradeFactor{
			Rule:   ruleChalk,
			Label:  fmt.Sprintf("Heavy chalk (%d)", leg.Odds),
			Delta:  delta,
			Detail: fmt.Sprintf("Long odds of %d add little payout for the bust risk.", leg.Odds),
		})
	}

	candidates = append(candidates, scoreSharpBenchmark(leg, game, rubric)...)

	return clampCategoryFactors(candidates, rubric.LinePriceMin, rubric.LinePriceMax)
}

func scoreSharpBenchmark(leg Leg, game Game, rubric Rubric) []GradeFactor {
	if len(game.Sharp) == 0 {
		return nil
	}
	sharp, ok := game.Sharp[sharpKeyForLeg(leg)]
	if !ok {
		return nil
	}

	source := sharpSourceLabel(sharp)
	market := normalizeSharpMarket(leg.Market)
	side := strings.ToUpper(strings.TrimSpace(leg.Side))

	if leg.Line != nil && sharp.Line != nil && math.Abs(*leg.Line-*sharp.Line) > lineMatchEpsilon {
		var lineEdge float64
		switch {
		case market == marketSpread:
			lineEdge = *leg.Line - *sharp.Line
		case side == sideOver:
			lineEdge = *sharp.Line - *leg.Line
		case side == sideUnder:
			lineEdge = *leg.Line - *sharp.Line
		default:
			return nil
		}

		threshold := sharpLineThreshold(market)
		sharpLine := sharpLineDisplay(market, sharp.Line)
		if lineEdge >= threshold-lineMatchEpsilon {
			return []GradeFactor{{
				Rule:   ruleSharpLineEdge,
				Label:  "Sharp line edge",
				Delta:  sharpTierDelta(sharp.Tier, rubric.SharpLineEdge, rubric.SharpLineEdgeConsensus),
				Detail: fmt.Sprintf("Getting %s while %s sits at %s.", selection(leg), source, sharpLine),
			}}
		}
		if lineEdge <= -threshold+lineMatchEpsilon {
			legLine := selection(leg)
			if market == marketSpread {
				legLine = signedLine(leg.Line)
			}
			return []GradeFactor{{
				Rule:  ruleSharpLineFade,
				Label: "Sharp line fade",
				Delta: sharpTierDelta(sharp.Tier, rubric.SharpLineFade, rubric.SharpLineFadeConsensus),
				Detail: fmt.Sprintf(
					"%s sits at %s, so %s gives up %s.",
					source,
					sharpLine,
					legLine,
					sharpDiffText(market, math.Abs(lineEdge)),
				),
			}}
		}
		return nil
	}

	linesMatch := (leg.Line == nil && sharp.Line == nil) ||
		(leg.Line != nil && sharp.Line != nil && math.Abs(*leg.Line-*sharp.Line) <= lineMatchEpsilon)
	if !linesMatch || sharp.FairProb <= 0 || sharp.FairProb >= 1 || leg.Odds == 0 {
		return nil
	}

	edge := parlay.PriceEdge(leg.Odds, sharp.FairProb)
	fairAmerican := sharp.FairAmerican
	if fairAmerican == 0 {
		fairAmerican = parlay.DecimalToAmerican(1 / sharp.FairProb)
	}
	if edge >= sharpPriceEdgeThreshold-lineMatchEpsilon {
		return []GradeFactor{{
			Rule:  ruleSharpPriceEdge,
			Label: "Sharp price edge",
			Delta: sharpTierDelta(sharp.Tier, rubric.SharpPriceEdge, rubric.SharpPriceEdgeConsensus),
			Detail: fmt.Sprintf(
				"Strong price at %s vs. %s fair price near %s.",
				americanOddsText(leg.Odds),
				source,
				americanOddsText(fairAmerican),
			),
		}}
	}
	if edge <= sharpPriceFadeThreshold+lineMatchEpsilon {
		return []GradeFactor{{
			Rule:  ruleSharpPriceFade,
			Label: "Steep price vs. sharp",
			Delta: sharpTierDelta(sharp.Tier, rubric.SharpPriceFade, rubric.SharpPriceFadeConsensus),
			Detail: fmt.Sprintf(
				"Steep price at %s vs. %s fair price near %s.",
				americanOddsText(leg.Odds),
				source,
				americanOddsText(fairAmerican),
			),
		}}
	}
	return nil
}

func sharpKeyForLeg(leg Leg) SharpKey {
	market := normalizeSharpMarket(leg.Market)
	team := strings.ToUpper(strings.TrimSpace(leg.Team))
	if market == marketTotal {
		team = ""
	}
	side := strings.ToUpper(strings.TrimSpace(leg.Side))
	if (market == marketML || market == marketSpread) && side == "" {
		side = "TEAM"
	}
	var player string
	if db.IsPlayerMarket(market) || strings.TrimSpace(leg.PlayerName) != "" {
		player = db.NormalizePlayerName(leg.PlayerName)
	}
	return SharpKey{
		MarketType: market,
		Team:       team,
		PlayerName: player,
		Side:       side,
	}
}

func normalizeSharpMarket(market string) string {
	m := strings.ToUpper(strings.TrimSpace(market))
	if m == "MONEYLINE" {
		return marketML
	}
	return m
}

func sharpLineThreshold(market string) float64 {
	switch market {
	case marketPassYds, marketRushYds, marketRecYds, marketRushAtt:
		return 2.0
	default:
		return 0.5
	}
}

func sharpTierDelta(tier, sharpDelta, consensusDelta int) int {
	if tier == sharpTier3 {
		return consensusDelta
	}
	return sharpDelta
}

func sharpSourceLabel(sharp SharpQuote) string {
	if s := strings.TrimSpace(sharp.Source); s != "" {
		return s
	}
	if sharp.Tier == sharpTier3 {
		return consensusBookLabel
	}
	return "sharp market"
}

func sharpLineDisplay(market string, line *float64) string {
	if market == marketSpread {
		return signedLine(line)
	}
	return lineText(line)
}

func sharpDiffText(market string, diff float64) string {
	switch market {
	case marketPassYds, marketRushYds, marketRecYds:
		return number(diff) + " yards"
	case marketRushAtt:
		return number(diff) + " attempts"
	case marketReceptions:
		return number(diff) + " receptions"
	default:
		if math.Abs(diff-1.0) <= lineMatchEpsilon {
			return "a full point"
		}
		return number(diff) + " points"
	}
}

func americanOddsText(odds int) string {
	return fmt.Sprintf("%+d", odds)
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

func chalkPenalty(odds int, rubric Rubric) int {
	if odds <= chalkMaxOdds {
		return rubric.ChalkMax
	}
	if odds >= chalkOdds {
		return rubric.ChalkMin
	}
	ratio := float64(-odds-(-chalkOdds)) / float64(-chalkMaxOdds-(-chalkOdds))
	penalty := rubric.ChalkMin - int(math.Round(ratio*float64(rubric.ChalkMin-rubric.ChalkMax)))
	return clampInt(penalty, rubric.ChalkMax, rubric.ChalkMin)
}

func letterGrade(score int, rubric Rubric) string {
	switch {
	case score >= rubric.BandA:
		return "A"
	case score >= rubric.BandB:
		return "B"
	case score >= rubric.BandC:
		return "C"
	case score >= rubric.BandD:
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
