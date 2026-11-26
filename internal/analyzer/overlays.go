package analyzer

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/modelv2"
)

const (
	// HeightenedUncertaintyBlendBoost is the market-anchor blend shift applied
	// when any active ContextOverlay flags heightened uncertainty.
	HeightenedUncertaintyBlendBoost = 0.15

	rulePlayerInjuryStatus     = "player_injury_status"
	ruleNarrativeStreakTrap    = "narrative_streak_trap"
	ruleModelMarketDivergence  = "model_market_divergence"
	ruleExternalContextSignals = "external_context_signals"

	injuryOutDelta          = -20
	injuryOutUnderDelta     = -10
	injuryDoubtfulDelta     = -15
	injuryDoubtfulUnderDel  = -5
	injuryQuestionableDelta = -6

	tdSpikeOverlayDelta       = -6
	narrativeStreakTrapDelta  = -8
	narrativeStreakProbMax    = 0.50
	narrativeStreakConfident  = 0.55
	modelMarketDivergenceGap  = 0.08
	modelMarketDivergenceDel  = -4
	anytimeTDDefaultThreshold = 0.5
)

// ContextSignal is an optional structured external context event (e.g. breaking
// injury update, inactive report, sharp steam, or future structured news item)
// attached to a Game or PlayerStats without retraining model artifacts.
type ContextSignal struct {
	Kind                  string // "injury", "news", "line_move", "inactive"
	Team                  string
	PlayerID              string
	Label                 string
	Detail                string
	Severity              Severity
	ScoreDelta            int
	HeightenedUncertainty bool
}

// OverlayFinding is one deterministic finding emitted by a ContextOverlay.
type OverlayFinding struct {
	Rule                  string
	Label                 string
	Detail                string
	Severity              Severity
	ScoreDelta            int
	HeightenedUncertainty bool
	LegIDs                []int64
}

// ContextOverlay inspects one leg alongside its game context, player stats,
// and optional model prediction without requiring model retraining.
type ContextOverlay func(leg Leg, game Game, stats map[string]PlayerStats, pred *modelv2.LegPrediction) []OverlayFinding

// DefaultContextOverlays returns the ordered list of built-in deterministic
// overlays for v2 context, injury, weather, key-number, and trap checks.
func DefaultContextOverlays() []ContextOverlay {
	return []ContextOverlay{
		playerInjuryOverlay,
		teammateOutOverlay,
		qbDragOverlay,
		windOverlay,
		fakeWeatherOverlay,
		hookOverlay,
		totalKeyNumberOverlay,
		shrinkingRoleOverlay,
		tdSpikeRiskOverlay,
		narrativeStreakTrapOverlay,
		modelMarketDivergenceOverlay,
		externalContextSignalsOverlay,
	}
}

// EvaluateOverlays runs the supplied overlays (or DefaultContextOverlays when
// none are provided) against leg and returns findings stably sorted by
// severity (Danger first).
func EvaluateOverlays(
	leg Leg,
	game Game,
	stats map[string]PlayerStats,
	pred *modelv2.LegPrediction,
	overlays ...ContextOverlay,
) []OverlayFinding {
	if leg.Market == marketOther || leg.GameID == "" {
		return nil
	}
	if len(overlays) == 0 {
		overlays = DefaultContextOverlays()
	}

	var findings []OverlayFinding
	for _, overlay := range overlays {
		if overlay == nil {
			continue
		}
		findings = append(findings, overlay(leg, game, stats, pred)...)
	}
	slices.SortStableFunc(findings, func(left, right OverlayFinding) int {
		return cmp.Compare(right.Severity, left.Severity)
	})
	return findings
}

// HasHeightenedUncertainty reports whether any finding indicates breaking
// injury, role, divergence, or external context uncertainty.
func HasHeightenedUncertainty(findings []OverlayFinding) bool {
	for _, f := range findings {
		if f.HeightenedUncertainty {
			return true
		}
	}
	return false
}

// UncertaintyBoost returns HeightenedUncertaintyBlendBoost when any finding
// flags heightened uncertainty, or 0 otherwise.
func UncertaintyBoost(findings []OverlayFinding) float64 {
	if HasHeightenedUncertainty(findings) {
		return HeightenedUncertaintyBlendBoost
	}
	return 0
}

// PredictLegWithOverlays runs model inference for leg while shifting the market
// blend weight toward the live market anchor whenever any ContextOverlay sets
// HeightenedUncertainty = true.
func PredictLegWithOverlays(
	model modelv2.DistModelArtifact,
	features []float64,
	leg Leg,
	game Game,
	stats map[string]PlayerStats,
	marketProb float64,
	overlays ...ContextOverlay,
) (modelv2.LegPrediction, []OverlayFinding, error) {
	line, err := propPredictionLine(leg)
	if err != nil {
		return modelv2.LegPrediction{}, nil, err
	}

	preFindings := EvaluateOverlays(leg, game, stats, nil, overlays...)
	boost := UncertaintyBoost(preFindings)

	pred, err := model.PredictLeg(features, leg.Side, line, marketProb, boost)
	if err != nil {
		return modelv2.LegPrediction{}, nil, err
	}

	findings := EvaluateOverlays(leg, game, stats, &pred, overlays...)
	if boost == 0 && HasHeightenedUncertainty(findings) {
		pred, err = model.PredictLeg(features, leg.Side, line, marketProb, HeightenedUncertaintyBlendBoost)
		if err != nil {
			return modelv2.LegPrediction{}, nil, err
		}
		findings = EvaluateOverlays(leg, game, stats, &pred, overlays...)
	}
	return pred, findings, nil
}

func propPredictionLine(leg Leg) (float64, error) {
	if leg.Line != nil {
		return *leg.Line, nil
	}
	if leg.Market == marketAnytimeTD {
		return anytimeTDDefaultThreshold, nil
	}
	return 0, errors.New("prop prediction requires non-nil leg line")
}

func playerInjuryOverlay(leg Leg, game Game, _ map[string]PlayerStats, _ *modelv2.LegPrediction) []OverlayFinding {
	inj, ok := matchingPlayerInjury(leg, game)
	if !ok {
		return nil
	}
	status := strings.TrimSpace(inj.Status)
	lower := strings.ToLower(status)
	name := playerName(leg)
	isUnder := isNegativeSide(leg.Side)

	var (
		sev   Severity
		delta int
		label string
		text  string
	)
	switch lower {
	case "out", "injured reserve", "ir":
		label = "Player listed " + status
		if isUnder {
			sev = Warn
			delta = injuryOutUnderDelta
			text = fmt.Sprintf("%s is listed %s — if inactive, this leg voids.", name, status)
		} else {
			sev = Danger
			delta = injuryOutDelta
			text = fmt.Sprintf("%s is listed %s — verify inactive status before locking this in.", name, status)
		}
	case "doubtful":
		label = "Player listed Doubtful"
		if isUnder {
			sev = Warn
			delta = injuryDoubtfulUnderDel
			text = name + " is Doubtful — could sit and void or play limited snaps."
		} else {
			sev = Danger
			delta = injuryDoubtfulDelta
			text = name + " is Doubtful — unlikely to see normal volume."
		}
	case "questionable":
		label = "Player listed Questionable"
		if isUnder {
			sev = Info
			delta = 0
			text = name + " is Questionable — snap count and availability carry extra uncertainty."
		} else {
			sev = Warn
			delta = injuryQuestionableDelta
			text = name + " is Questionable — snap count and role carry extra uncertainty."
		}
	default:
		return nil
	}

	return []OverlayFinding{{
		Rule:                  rulePlayerInjuryStatus,
		Label:                 label,
		Detail:                text,
		Severity:              sev,
		ScoreDelta:            delta,
		HeightenedUncertainty: true,
		LegIDs:                []int64{leg.ID},
	}}
}

func matchingPlayerInjury(leg Leg, game Game) (Injury, bool) {
	if (leg.PlayerID == "" && leg.PlayerName == "") || len(game.Injuries) == 0 {
		return Injury{}, false
	}
	normLegName := db.NormalizePlayerName(leg.PlayerName)
	matches := func(inj Injury) bool {
		if leg.PlayerID != "" && inj.AthleteID != "" {
			return inj.AthleteID == leg.PlayerID
		}
		if normLegName == "" || inj.FullName == "" {
			return false
		}
		return db.NormalizePlayerName(inj.FullName) == normLegName
	}

	if leg.Team != "" {
		for _, inj := range game.Injuries[leg.Team] {
			if matches(inj) {
				return inj, true
			}
		}
		return Injury{}, false
	}

	teams := make([]string, 0, len(game.Injuries))
	for team := range game.Injuries {
		teams = append(teams, team)
	}
	slices.Sort(teams)
	for _, team := range teams {
		for _, inj := range game.Injuries[team] {
			if matches(inj) {
				return inj, true
			}
		}
	}
	return Injury{}, false
}

func teammateOutOverlay(leg Leg, game Game, stats map[string]PlayerStats, _ *modelv2.LegPrediction) []OverlayFinding {
	pos := playerPositionForLeg(leg, stats)
	if pos == "" {
		return nil
	}
	outTeammate, ok := outSamePositionTeammate(leg, game, pos)
	if !ok {
		return nil
	}
	return []OverlayFinding{{
		Rule:     ruleTeammateOutBump,
		Label:    "Teammate out (role bump)",
		Severity: Info,
		Detail: fmt.Sprintf(
			"%s %s %s is Out, opening extra volume.",
			leg.Team,
			strings.ToUpper(strings.TrimSpace(pos)),
			outTeammate.FullName,
		),
		ScoreDelta:            RubricV1.TeammateOutBump,
		HeightenedUncertainty: true,
		LegIDs:                []int64{leg.ID},
	}}
}

func playerPositionForLeg(leg Leg, stats map[string]PlayerStats) string {
	if leg.PlayerID == "" || len(stats) == 0 {
		return ""
	}
	ps, ok := stats[leg.PlayerID]
	if !ok {
		return ""
	}
	if pos := strings.TrimSpace(ps.Position); pos != "" {
		return pos
	}
	switch ps.Role {
	case RoleQB:
		return "QB"
	case RoleWR1, RoleWR2:
		return "WR"
	case RoleTE1:
		return "TE"
	case RoleRB1, RoleRB2:
		return "RB"
	default:
		return ""
	}
}

func qbDragOverlay(leg Leg, game Game, _ map[string]PlayerStats, _ *modelv2.LegPrediction) []OverlayFinding {
	team, qb, adj, ok := qbDragTeam(game, leg.Team)
	if !ok || adj > qbDragElo || !qbDragHit(leg, team) {
		return nil
	}
	return []OverlayFinding{{
		Rule:       ruleQBDrag,
		Label:      "nfelo QB drag",
		Detail:     qbDragMessage(team, qb, adj, selection(leg)),
		Severity:   Info,
		ScoreDelta: RubricV1.QBDrag,
		LegIDs:     []int64{leg.ID},
	}}
}

func windOverlay(leg Leg, game Game, _ map[string]PlayerStats, _ *modelv2.LegPrediction) []OverlayFinding {
	sev, windy := evalWindHazard(leg, game)
	if !windy {
		return nil
	}
	delta := RubricV1.WindWarn
	if sev == Danger {
		delta = RubricV1.WindDanger
	}
	return []OverlayFinding{{
		Rule:       ruleWind,
		Label:      "Wind headwind",
		Detail:     fmt.Sprintf("%s in %s hurts passing and kicking overs.", windText(*game.Weather), game.Home),
		Severity:   sev,
		ScoreDelta: delta,
		LegIDs:     []int64{leg.ID},
	}}
}

func fakeWeatherOverlay(leg Leg, game Game, _ map[string]PlayerStats, _ *modelv2.LegPrediction) []OverlayFinding {
	if !isFakeWeatherLeg(leg, game) {
		return nil
	}
	return []OverlayFinding{{
		Rule:       ruleFakeWeather,
		Label:      "Light rain myth",
		Detail:     "Light rain without wind does not lower NFL scoring.",
		Severity:   Info,
		ScoreDelta: RubricV1.FakeWeather,
		LegIDs:     []int64{leg.ID},
	}}
}

func hookOverlay(leg Leg, game Game, _ map[string]PlayerStats, _ *modelv2.LegPrediction) []OverlayFinding {
	landing, buyTo, moneyline, ok := evalSpreadHook(leg)
	if !ok {
		return nil
	}
	delta := RubricV1.Hook
	if model, hasModel := game.ModelSpread(leg.Team); hasModel && *leg.Line-model >= 2 {
		delta = RubricV1.HookSoftened
	}
	detail := hookMessage(leg, *leg.Line, landing, buyTo, moneyline) + hookModelNote(game, leg)
	return []OverlayFinding{{
		Rule:       ruleHook,
		Label:      "Spread on hook",
		Detail:     detail,
		Severity:   Warn,
		ScoreDelta: delta,
		LegIDs:     []int64{leg.ID},
	}}
}

func totalKeyNumberOverlay(leg Leg, _ Game, _ map[string]PlayerStats, _ *modelv2.LegPrediction) []OverlayFinding {
	if leg.Market != marketTotal || leg.Line == nil {
		return nil
	}
	keys := keyNumbersNear(*leg.Line)
	if len(keys) == 0 {
		return nil
	}
	return []OverlayFinding{{
		Rule:       ruleTotalKeyNumber,
		Label:      "Total on key number",
		Detail:     keyNumberMessage(*leg.Line, keys),
		Severity:   Info,
		ScoreDelta: RubricV1.TotalKeyNumber,
		LegIDs:     []int64{leg.ID},
	}}
}

func shrinkingRoleOverlay(leg Leg, _ Game, stats map[string]PlayerStats, _ *modelv2.LegPrediction) []OverlayFinding {
	if leg.PlayerID == "" || len(stats) == 0 {
		return nil
	}
	ps, ok := stats[leg.PlayerID]
	if !ok {
		return nil
	}
	reason, hit := shrinkingRoleHit(leg, ps)
	if !hit {
		return nil
	}
	return []OverlayFinding{{
		Rule:  ruleShrinkingRole,
		Label: "Shrinking role",
		Detail: fmt.Sprintf(
			"%s's %s — recent volume is fading for %s.",
			playerName(leg),
			reason,
			selection(leg),
		),
		Severity:              Warn,
		ScoreDelta:            RubricV1.ShrinkingRole,
		HeightenedUncertainty: true,
		LegIDs:                []int64{leg.ID},
	}}
}

func tdSpikeRiskOverlay(leg Leg, _ Game, stats map[string]PlayerStats, _ *modelv2.LegPrediction) []OverlayFinding {
	if leg.PlayerID == "" || len(stats) == 0 {
		return nil
	}
	ps, ok := stats[leg.PlayerID]
	if !ok {
		return nil
	}
	detail, hit := evalTDSpikeRisk(leg, ps)
	if !hit {
		return nil
	}
	return []OverlayFinding{{
		Rule:       ruleTDSpikeRisk,
		Label:      "TD spike risk",
		Detail:     detail,
		Severity:   Info,
		ScoreDelta: tdSpikeOverlayDelta,
		LegIDs:     []int64{leg.ID},
	}}
}

func narrativeStreakTrapOverlay(
	leg Leg,
	_ Game,
	stats map[string]PlayerStats,
	pred *modelv2.LegPrediction,
) []OverlayFinding {
	if leg.Side != sideOver || leg.Line == nil || leg.PlayerID == "" || len(stats) == 0 {
		return nil
	}
	ps, ok := stats[leg.PlayerID]
	if !ok {
		return nil
	}
	games := ps.Games[leg.Market]
	if len(games) < 5 {
		return nil
	}

	l5 := games[:5]
	l5Mean := Mean(l5)
	l5Median := Median(l5)
	hits, _, hasRate := l5HitRate(leg, ps)
	fairProb := effectiveFairProb(pred)

	if hasRate && hits == 5 && fairProb > 0 && fairProb < narrativeStreakProbMax {
		return []OverlayFinding{{
			Rule:  ruleNarrativeStreakTrap,
			Label: "Narrative streak trap",
			Detail: fmt.Sprintf(
				"%s cleared %s in 5 straight, but matchup and context still put fair odds below 50/50.",
				playerName(leg),
				number(*leg.Line),
			),
			Severity:   Warn,
			ScoreDelta: narrativeStreakTrapDelta,
			LegIDs:     []int64{leg.ID},
		}}
	}

	if len(games) >= 6 {
		priorMedian := Median(games[5:])
		margin := streakMaterialMargin(leg.Market, l5Mean)
		if allAbove(l5, priorMedian) &&
			*leg.Line >= l5Mean+margin-floatEps &&
			*leg.Line >= l5Median+margin-floatEps &&
			(fairProb == 0 || fairProb < narrativeStreakConfident) {
			detail := fmt.Sprintf(
				"%s cleared prior marks in 5 straight, and the book pushed the line to %s above their %s recent average.",
				playerName(leg),
				number(*leg.Line),
				number(math.Round(l5Mean*10)/10),
			)
			return []OverlayFinding{{
				Rule:       ruleNarrativeStreakTrap,
				Label:      "Narrative streak trap",
				Detail:     detail,
				Severity:   Warn,
				ScoreDelta: narrativeStreakTrapDelta,
				LegIDs:     []int64{leg.ID},
			}}
		}
	}

	return nil
}

func effectiveFairProb(pred *modelv2.LegPrediction) float64 {
	if pred == nil {
		return 0
	}
	if pred.CalibratedProb > 0 && pred.CalibratedProb < 1 {
		return pred.CalibratedProb
	}
	if pred.ModelProb > 0 && pred.ModelProb < 1 {
		return pred.ModelProb
	}
	return 0
}

func allAbove(vals []float64, threshold float64) bool {
	for _, v := range vals {
		if v <= threshold {
			return false
		}
	}
	return true
}

func streakMaterialMargin(market string, l5Mean float64) float64 {
	if isYardageMarket(market) {
		return max(3.0, 0.05*l5Mean)
	}
	return max(0.5, 0.08*l5Mean)
}

func modelMarketDivergenceOverlay(
	leg Leg,
	_ Game,
	_ map[string]PlayerStats,
	pred *modelv2.LegPrediction,
) []OverlayFinding {
	if pred == nil {
		return nil
	}
	if pred.ModelProb <= 0 || pred.ModelProb >= 1 || pred.MarketProb <= 0 || pred.MarketProb >= 1 {
		return nil
	}
	if math.Abs(pred.ModelProb-pred.MarketProb) < modelMarketDivergenceGap-floatEps {
		return nil
	}
	return []OverlayFinding{
		{
			Rule:                  ruleModelMarketDivergence,
			Label:                 "Model-market gap",
			Detail:                "Model and market see this differently — double-check late injury, weather, or role news.",
			Severity:              Warn,
			ScoreDelta:            modelMarketDivergenceDel,
			HeightenedUncertainty: true,
			LegIDs:                []int64{leg.ID},
		},
	}
}

func externalContextSignalsOverlay(
	leg Leg,
	game Game,
	stats map[string]PlayerStats,
	_ *modelv2.LegPrediction,
) []OverlayFinding {
	var candidates []ContextSignal
	for _, sig := range game.ContextSignals {
		if contextSignalMatchesGameLeg(sig, leg) {
			candidates = append(candidates, sig)
		}
	}
	if leg.PlayerID != "" && len(stats) > 0 {
		if ps, ok := stats[leg.PlayerID]; ok {
			for _, sig := range ps.ContextSignals {
				if contextSignalMatchesPlayerLeg(sig, leg) {
					candidates = append(candidates, sig)
				}
			}
		}
	}
	if len(candidates) == 0 {
		return nil
	}

	type sigKey struct {
		kind     string
		team     string
		playerID string
		label    string
		detail   string
	}
	seen := make(map[sigKey]struct{}, len(candidates))
	findings := make([]OverlayFinding, 0, len(candidates))
	for _, sig := range candidates {
		label := strings.TrimSpace(sig.Label)
		if label == "" {
			label = defaultContextSignalLabel(sig.Kind)
		}
		detail := strings.TrimSpace(sig.Detail)
		if detail == "" {
			detail = label
		}
		k := sigKey{
			kind:     strings.ToLower(strings.TrimSpace(sig.Kind)),
			team:     strings.ToUpper(strings.TrimSpace(sig.Team)),
			playerID: strings.TrimSpace(sig.PlayerID),
			label:    label,
			detail:   detail,
		}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}

		findings = append(findings, OverlayFinding{
			Rule:                  ruleExternalContextSignals,
			Label:                 label,
			Detail:                detail,
			Severity:              sig.Severity,
			ScoreDelta:            sig.ScoreDelta,
			HeightenedUncertainty: sig.HeightenedUncertainty,
			LegIDs:                []int64{leg.ID},
		})
	}
	return findings
}

func contextSignalMatchesGameLeg(sig ContextSignal, leg Leg) bool {
	if sig.PlayerID != "" {
		return sig.PlayerID == leg.PlayerID
	}
	if sig.Team != "" {
		return leg.Team != "" && strings.EqualFold(strings.TrimSpace(sig.Team), strings.TrimSpace(leg.Team))
	}
	return true
}

func contextSignalMatchesPlayerLeg(sig ContextSignal, leg Leg) bool {
	if sig.PlayerID != "" && sig.PlayerID != leg.PlayerID {
		return false
	}
	if sig.Team != "" && leg.Team != "" &&
		!strings.EqualFold(strings.TrimSpace(sig.Team), strings.TrimSpace(leg.Team)) {
		return false
	}
	return true
}

func defaultContextSignalLabel(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "injury":
		return "Injury update"
	case "inactive":
		return "Inactive report"
	case "line_move":
		return "Line movement"
	case "news":
		return "Context news"
	default:
		return "Context signal"
	}
}

func isNegativeSide(side string) bool {
	s := strings.ToUpper(strings.TrimSpace(side))
	return s == sideUnder || s == "NO"
}
