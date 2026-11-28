package analyzer

import (
	"fmt"
	"math"
	"slices"
)

// Rule IDs for sportsbook combination-eligibility checks.
const (
	ruleBookComboIneligible = "book_combo_ineligible"
	ruleBookComboRestricted = "book_combo_restricted"
)

// CheckComboConflict evaluates whether candidate conflicts with any leg in
// existing under the sportsbook combination-eligibility rules. It returns the
// highest-severity warning found (Danger before Warn) and true, or zero and
// false when no conflict exists.
func CheckComboConflict(candidate Leg, existing []Leg) (Warning, bool) {
	if candidate.Market == marketOther || candidate.GameID == "" {
		return Warning{}, false
	}

	var sameGameExisting []Leg
	for _, leg := range existing {
		if leg.Market == marketOther || !sameGame(candidate, leg) {
			continue
		}
		if candidate.ID != 0 && candidate.ID == leg.ID {
			continue
		}
		sameGameExisting = append(sameGameExisting, leg)
	}
	if len(sameGameExisting) == 0 {
		return Warning{}, false
	}

	var firstWarn *Warning
	for _, other := range sameGameExisting {
		if msg, ok := ineligiblePairMessage(candidate, other); ok {
			return Warning{
				Rule:     ruleBookComboIneligible,
				Severity: Danger,
				Message:  msg,
				LegIDs:   comboLegIDs(candidate, other),
			}, true
		}
		if firstWarn == nil {
			if msg, ok := restrictedPairMessage(candidate, other); ok {
				firstWarn = &Warning{
					Rule:     ruleBookComboRestricted,
					Severity: Warn,
					Message:  msg,
					LegIDs:   comboLegIDs(candidate, other),
				}
			}
		}
	}

	if w, ok := checkCandidateTeamTotalVsGameTotal(candidate, sameGameExisting); ok {
		return w, true
	}

	if firstWarn != nil {
		return *firstWarn, true
	}
	return Warning{}, false
}

// bookComboIneligible flags combinations that DraftKings and FanDuel
// consistently reject on the same parlay or SGP.
func bookComboIneligible(a *analysis) []Warning {
	var warnings []Warning
	for i := range a.legs {
		for j := i + 1; j < len(a.legs); j++ {
			left, right := a.legs[i], a.legs[j]
			if !sameGame(left, right) {
				continue
			}
			if msg, ok := ineligiblePairMessage(left, right); ok {
				warnings = append(warnings, a.warning(ruleBookComboIneligible, Danger, msg, []Leg{left, right}))
			}
		}
	}
	warnings = append(warnings, teamTotalVsGameTotalWarnings(a)...)
	return warnings
}

// bookComboRestricted flags same-game combinations that DraftKings or FanDuel
// frequently restrict or drop from SGP eligibility.
func bookComboRestricted(a *analysis) []Warning {
	var warnings []Warning
	for i := range a.legs {
		for j := i + 1; j < len(a.legs); j++ {
			left, right := a.legs[i], a.legs[j]
			if !sameGame(left, right) {
				continue
			}
			if msg, ok := restrictedPairMessage(left, right); ok {
				warnings = append(warnings, a.warning(ruleBookComboRestricted, Warn, msg, []Leg{left, right}))
			}
		}
	}
	return warnings
}

func comboLegIDs(candidate, other Leg) []int64 {
	if candidate.ID == 0 {
		return []int64{other.ID}
	}
	return []int64{candidate.ID, other.ID}
}

func ineligiblePairMessage(left, right Leg) (string, bool) {
	if isDuplicateSelection(left, right) {
		return fmt.Sprintf(
			"Heads up: %s and %s target the same selection, so sportsbooks won't combine them on one slip.",
			pairLabel(left, right, true),
			pairLabel(left, right, false),
		), true
	}
	if isOppositeSameMarket(left, right) {
		return fmt.Sprintf(
			"Heads up: %s and %s are opposite sides of the same market — sportsbooks won't parlay both sides.",
			pairLabel(left, right, true),
			pairLabel(left, right, false),
		), true
	}
	if isSameTeamMLAndSpread(left, right) {
		return fmt.Sprintf(
			"Heads up: DraftKings and FanDuel usually won't combine %s with %s on the same slip.",
			pairLabel(left, right, true),
			pairLabel(left, right, false),
		), true
	}
	if isOppositeMLAndFavSpread(left, right) {
		return fmt.Sprintf(
			"Heads up: %s and %s are mutually exclusive, so sportsbooks won't combine them.",
			pairLabel(left, right, true),
			pairLabel(left, right, false),
		), true
	}
	return "", false
}

func restrictedPairMessage(left, right Leg) (string, bool) {
	if isDuplicatePlayerMilestone(left, right) {
		return fmt.Sprintf(
			"Heads up: DraftKings and FanDuel often restrict combining two %s lines (%s and %s) on the same SGP.",
			marketLabel(left.Market),
			pairLabel(left, right, true),
			pairLabel(left, right, false),
		), true
	}
	if isInversePlayerPropPair(left, right) {
		return fmt.Sprintf(
			"Heads up: books often restrict inverse same-player props like %s with %s on an SGP.",
			pairLabel(left, right, true),
			pairLabel(left, right, false),
		), true
	}
	if isKickerWithTotal(left, right) {
		return fmt.Sprintf(
			"Heads up: DraftKings and FanDuel frequently block combining %s with %s on an SGP.",
			pairLabel(left, right, true),
			pairLabel(left, right, false),
		), true
	}
	return "", false
}

func pairLabel(left, right Leg, first bool) string {
	target := right
	if first {
		target = left
	}
	if mixedOwners([]Leg{left, right}) {
		return ownedSelection(target)
	}
	return selection(target)
}

func isDuplicateSelection(left, right Leg) bool {
	if left.Market != right.Market || left.Side == "" || left.Side != right.Side {
		return false
	}
	switch left.Market {
	case marketTotal:
		return true
	case marketSpread, marketML, marketTeamTotal:
		return sameTeam(left, right)
	default:
		if !isComboPlayerPropMarket(left.Market) {
			return false
		}
		return samePlayer(left, right) && linesEqual(left.Line, right.Line)
	}
}

func isDuplicatePlayerMilestone(left, right Leg) bool {
	if left.Market != right.Market || !isComboPlayerPropMarket(left.Market) {
		return false
	}
	if left.Side == "" || left.Side != right.Side {
		return false
	}
	return samePlayer(left, right) && !linesEqual(left.Line, right.Line)
}

func isOppositeSameMarket(left, right Leg) bool {
	if left.Market != right.Market {
		return false
	}
	switch left.Market {
	case marketSpread, marketML:
		return left.Team != "" && right.Team != "" && left.Team != right.Team
	case marketTotal:
		return oppositeOverUnder(left.Side, right.Side)
	case marketTeamTotal:
		return sameTeam(left, right) && oppositeOverUnder(left.Side, right.Side)
	default:
		return isComboPlayerPropMarket(left.Market) &&
			samePlayer(left, right) &&
			oppositeOverUnder(left.Side, right.Side)
	}
}

func isSameTeamMLAndSpread(left, right Leg) bool {
	ml, spread, ok := orderMLAndSpread(left, right)
	if !ok {
		return false
	}
	return sameTeam(ml, spread)
}

func isOppositeMLAndFavSpread(left, right Leg) bool {
	ml, spread, ok := orderMLAndSpread(left, right)
	if !ok {
		return false
	}
	if ml.Team == "" || spread.Team == "" || ml.Team == spread.Team {
		return false
	}
	return spread.Line != nil && *spread.Line < 0
}

func orderMLAndSpread(left, right Leg) (ml, spread Leg, ok bool) {
	switch {
	case left.Market == marketML && right.Market == marketSpread:
		return left, right, true
	case left.Market == marketSpread && right.Market == marketML:
		return right, left, true
	default:
		return Leg{}, Leg{}, false
	}
}

func isInversePlayerPropPair(left, right Leg) bool {
	if !samePlayer(left, right) || !oppositeOverUnder(left.Side, right.Side) {
		return false
	}
	m1, m2 := left.Market, right.Market
	if m1 > m2 {
		m1, m2 = m2, m1
	}
	switch {
	case m1 == marketPassTDs && m2 == marketPassYds:
		return true
	case m1 == marketRecYds && m2 == marketReceptions:
		return true
	case m1 == marketRushAtt && m2 == marketRushYds:
		return true
	default:
		return false
	}
}

func isKickerWithTotal(left, right Leg) bool {
	return (left.Market == marketKickerPts && isGameOrTeamTotal(right.Market)) ||
		(right.Market == marketKickerPts && isGameOrTeamTotal(left.Market))
}

func isGameOrTeamTotal(market string) bool {
	return market == marketTotal || market == marketTeamTotal
}

func isComboPlayerPropMarket(market string) bool {
	return isPlayerPropMarket(market) || market == marketAnytimeTD || market == marketKickerPts
}

func oppositeOverUnder(leftSide, rightSide string) bool {
	return (leftSide == sideOver && rightSide == sideUnder) ||
		(leftSide == sideUnder && rightSide == sideOver)
}

func linesEqual(left, right *float64) bool {
	if left == nil && right == nil {
		return true
	}
	if left == nil || right == nil {
		return false
	}
	return math.Abs(*left-*right) < floatEps
}

func teamTotalVsGameTotalWarnings(a *analysis) []Warning {
	var warnings []Warning
	byGame := make(map[string][]Leg)
	for _, leg := range a.legs {
		if leg.Market == marketTotal || leg.Market == marketTeamTotal {
			byGame[leg.GameID] = append(byGame[leg.GameID], leg)
		}
	}
	for _, gameLegs := range byGame {
		var overs, unders []Leg
		var totalOvers, totalUnders []Leg
		for _, leg := range gameLegs {
			if leg.Line == nil {
				continue
			}
			switch leg.Market {
			case marketTeamTotal:
				switch leg.Side {
				case sideOver:
					overs = append(overs, leg)
				case sideUnder:
					unders = append(unders, leg)
				}
			case marketTotal:
				switch leg.Side {
				case sideOver:
					totalOvers = append(totalOvers, leg)
				case sideUnder:
					totalUnders = append(totalUnders, leg)
				}
			}
		}

		for _, gt := range totalUnders {
			if tt1, tt2, ok := findContradictingTeamTotals(overs, gt, true); ok {
				involved := []Leg{tt1, tt2, gt}
				warnings = append(warnings, a.warning(
					ruleBookComboIneligible,
					Danger,
					teamTotalContradictionMessage(tt1, tt2, gt),
					involved,
				))
			}
		}
		for _, gt := range totalOvers {
			if tt1, tt2, ok := findContradictingTeamTotals(unders, gt, false); ok {
				involved := []Leg{tt1, tt2, gt}
				warnings = append(warnings, a.warning(
					ruleBookComboIneligible,
					Danger,
					teamTotalContradictionMessage(tt1, tt2, gt),
					involved,
				))
			}
		}
	}
	return warnings
}

func checkCandidateTeamTotalVsGameTotal(candidate Leg, sameGameExisting []Leg) (Warning, bool) {
	if candidate.Line == nil || (candidate.Market != marketTotal && candidate.Market != marketTeamTotal) {
		return Warning{}, false
	}
	all := append(slices.Clone(sameGameExisting), candidate)
	var overs, unders []Leg
	var totalOvers, totalUnders []Leg
	for _, leg := range all {
		if leg.Line == nil {
			continue
		}
		switch leg.Market {
		case marketTeamTotal:
			switch leg.Side {
			case sideOver:
				overs = append(overs, leg)
			case sideUnder:
				unders = append(unders, leg)
			}
		case marketTotal:
			switch leg.Side {
			case sideOver:
				totalOvers = append(totalOvers, leg)
			case sideUnder:
				totalUnders = append(totalUnders, leg)
			}
		}
	}

	for _, gt := range totalUnders {
		if tt1, tt2, ok := findContradictingTeamTotals(overs, gt, true); ok {
			return Warning{
				Rule:     ruleBookComboIneligible,
				Severity: Danger,
				Message:  teamTotalContradictionMessage(tt1, tt2, gt),
				LegIDs:   nonZeroLegIDs(tt1, tt2, gt),
			}, true
		}
	}
	for _, gt := range totalOvers {
		if tt1, tt2, ok := findContradictingTeamTotals(unders, gt, false); ok {
			return Warning{
				Rule:     ruleBookComboIneligible,
				Severity: Danger,
				Message:  teamTotalContradictionMessage(tt1, tt2, gt),
				LegIDs:   nonZeroLegIDs(tt1, tt2, gt),
			}, true
		}
	}
	return Warning{}, false
}

func findContradictingTeamTotals(teamTotals []Leg, gameTotal Leg, teamOvers bool) (Leg, Leg, bool) {
	for i := range teamTotals {
		for j := i + 1; j < len(teamTotals); j++ {
			a, b := teamTotals[i], teamTotals[j]
			if a.Team == "" || b.Team == "" || a.Team == b.Team {
				continue
			}
			combined := *a.Line + *b.Line
			if teamOvers && combined+floatEps >= *gameTotal.Line {
				return a, b, true
			}
			if !teamOvers && combined-floatEps <= *gameTotal.Line {
				return a, b, true
			}
		}
	}
	return Leg{}, Leg{}, false
}

func teamTotalContradictionMessage(tt1, tt2, gt Leg) string {
	involved := []Leg{tt1, tt2, gt}
	owners := mixedOwners(involved)
	return fmt.Sprintf(
		"Heads up: %s and %s can't both clear alongside %s — sportsbooks won't combine mutually exclusive totals.",
		describe([]Leg{tt1}, owners),
		describe([]Leg{tt2}, owners),
		describe([]Leg{gt}, owners),
	)
}

func nonZeroLegIDs(legs ...Leg) []int64 {
	ids := make([]int64, 0, len(legs))
	for _, leg := range legs {
		if leg.ID != 0 {
			ids = append(ids, leg.ID)
		}
	}
	return ids
}
