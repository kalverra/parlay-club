package analyzer

import (
	"fmt"
	"strconv"
	"strings"
)

// Market names, matching docs/concepts/slips.md.
const (
	marketSpread     = "SPREAD"
	marketML         = "ML"
	marketTotal      = "TOTAL"
	marketTeamTotal  = "TEAM_TOTAL"
	marketPassYds    = "PASS_YDS"
	marketPassTDs    = "PASS_TDS"
	marketRushYds    = "RUSH_YDS"
	marketRushAtt    = "RUSH_ATT"
	marketRecYds     = "REC_YDS"
	marketReceptions = "RECEPTIONS"
	marketAnytimeTD  = "ANYTIME_TD"
	marketKickerPts  = "KICKER_PTS"
	marketOther      = "OTHER"
)

// Sides a leg can carry.
const (
	sideOver  = "OVER"
	sideUnder = "UNDER"
	sideYes   = "YES"
)

// Rule IDs are stable across releases.
const (
	ruleBlowoutPassing     = "blowout_passing"
	ruleTrailingRB         = "trailing_rb"
	ruleTotalContradiction = "total_contradiction"
	ruleStackClockDrain    = "stack_clock_drain"
	ruleStackShootout      = "stack_shootout"
)

// Thresholds from plan/analyzer.md.
const (
	blowoutSpread  = -7.0
	trailingSpread = 4.5
)

// blowoutPassing warns when a big favorite's spread asks for a quiet fourth
// quarter while a passing or receiving over on that team needs volume.
func blowoutPassing(a *analysis) []Warning {
	var warnings []Warning
	for _, spread := range a.legs {
		if spread.Market != marketSpread || spread.Line == nil || *spread.Line > blowoutSpread {
			continue
		}
		props := a.matching(spread, func(leg Leg) bool {
			return sameTeam(spread, leg) && passOrRecYardsOver(leg)
		})
		if len(props) == 0 {
			continue
		}

		involved := append([]Leg{spread}, props...)
		owners := mixedOwners(involved)
		subject := selection(spread)
		if owners {
			subject = ownedSelection(spread)
		}
		message := fmt.Sprintf(
			"If %s covers, they'll likely run the clock in Q4. %s %s better with a closer spread, or try %s's WR for garbage time.",
			subject,
			describe(props, owners),
			pairVerb(len(props)),
			teamOrOther(a.opponent(spread)),
		)
		warnings = append(warnings, a.warning(ruleBlowoutPassing, Warn, message, involved))
	}
	return warnings
}

// trailingRB warns when an underdog leg needs a trailing script while a rush
// over on that team needs the lead.
func trailingRB(a *analysis) []Warning {
	var warnings []Warning
	for _, dog := range a.legs {
		if !trailingUnderdog(dog) {
			continue
		}
		props := a.matching(dog, func(leg Leg) bool {
			return sameTeam(dog, leg) && rushOver(leg)
		})
		if len(props) == 0 {
			continue
		}

		involved := append([]Leg{dog}, props...)
		owners := mixedOwners(involved)
		message := fmt.Sprintf(
			"%s means they're probably trailing, and trailing teams stop running. %s %s that script.",
			underdogSubject(dog, owners), describe(props, owners), fightVerb(len(props)))
		warnings = append(warnings, a.warning(ruleTrailingRB, Danger, message, involved))
	}
	return warnings
}

// totalContradiction warns when a total under shares a game with two or more
// legs that need points.
func totalContradiction(a *analysis) []Warning {
	var warnings []Warning
	for _, under := range a.legs {
		if under.Market != marketTotal || under.Side != sideUnder {
			continue
		}
		qualifiers := a.matching(under, func(leg Leg) bool {
			return leg.Market == marketAnytimeTD || passingOrReceivingOver(leg)
		})
		if len(qualifiers) < 2 {
			continue
		}

		involved := append([]Leg{under}, qualifiers...)
		owners := mixedOwners(involved)
		subject := selection(under)
		if owners {
			subject = ownedSelection(under)
		}
		message := fmt.Sprintf(
			"%s needs a quiet game, but %s need points. One of these sides is going to lose.",
			subject, describe(qualifiers, owners))
		warnings = append(warnings, a.warning(ruleTotalContradiction, Danger, message, involved))
	}
	return warnings
}

// stackClockDrain notes a favorite spread, a rush over on that team, and a team
// total under on the opponent: the same quiet, run-heavy script wins all three.
func stackClockDrain(a *analysis) []Warning {
	var warnings []Warning
	for _, spread := range a.legs {
		if spread.Market != marketSpread || !favorite(spread) {
			continue
		}
		opponent := a.opponent(spread)
		if opponent == "" {
			continue
		}

		rush := a.matching(spread, func(leg Leg) bool {
			return sameTeam(spread, leg) && rushOver(leg)
		})
		totals := a.matching(spread, func(leg Leg) bool {
			return leg.Market == marketTeamTotal && leg.Side == sideUnder && leg.Team == opponent
		})
		if len(rush) == 0 || len(totals) == 0 {
			continue
		}

		involved := append([]Leg{spread}, rush...)
		involved = append(involved, totals...)
		owners := mixedOwners(involved)
		spreadText := selection(spread)
		if owners {
			spreadText = ownedSelection(spread)
		}
		message := fmt.Sprintf(
			"Nice stack: %s covers, %s %s the clock, %s %s quiet. These all win together.",
			spreadText, describePlayers(rush, owners), grindVerb(len(rush)),
			describe(totals, owners), stayVerb(len(totals)))
		warnings = append(warnings, a.warning(ruleStackClockDrain, Info, message, involved))
	}
	return warnings
}

// stackShootout notes a pass over, a different receiver's over on the same
// team, and a game total over: one track meet wins all three.
func stackShootout(a *analysis) []Warning {
	var warnings []Warning
	for _, pass := range a.legs {
		if pass.Market != marketPassYds || pass.Side != sideOver {
			continue
		}
		receivers := a.matching(pass, func(leg Leg) bool {
			return sameTeam(pass, leg) && receivingOver(leg) && !samePlayer(pass, leg)
		})
		if len(receivers) == 0 {
			continue
		}
		totals := a.matching(pass, func(leg Leg) bool {
			return leg.Market == marketTotal && leg.Side == sideOver
		})
		if len(totals) == 0 {
			continue
		}

		receiver, total := receivers[0], totals[0]
		involved := []Leg{pass, receiver, total}
		owners := mixedOwners(involved)
		message := fmt.Sprintf(
			"Shootout stack: %s, %s, game %s. If it's a track meet, all three hit.",
			describe([]Leg{pass}, owners), describe([]Leg{receiver}, owners), selection(total))
		warnings = append(warnings, a.warning(ruleStackShootout, Info, message, involved))
	}
	return warnings
}

// matching returns the legs in anchor's game, other than anchor itself, that
// keep approves. The legs stay in slip order.
func (a *analysis) matching(anchor Leg, keep func(Leg) bool) []Leg {
	var matches []Leg
	for _, leg := range a.legs {
		if leg.ID == anchor.ID || !sameGame(anchor, leg) {
			continue
		}
		if keep(leg) {
			matches = append(matches, leg)
		}
	}
	return matches
}

// sameGame reports whether both legs are tied to the same known game.
func sameGame(left, right Leg) bool {
	return left.GameID != "" && left.GameID == right.GameID
}

// sameTeam reports whether both legs are about the same known team.
func sameTeam(left, right Leg) bool {
	return left.Team != "" && left.Team == right.Team
}

// samePlayer reports whether both legs name the same player. When either ID is
// missing it falls back to the name; two unknown players are different.
func samePlayer(left, right Leg) bool {
	if left.PlayerID != "" && right.PlayerID != "" {
		return left.PlayerID == right.PlayerID
	}
	if left.PlayerName == "" || right.PlayerName == "" {
		return false
	}
	return left.PlayerName == right.PlayerName
}

// opponent returns the other team in the leg's game, or "" when the leg's team
// is not one of the two.
func (a *analysis) opponent(leg Leg) string {
	game, ok := a.games[leg.GameID]
	if !ok {
		return ""
	}
	switch leg.Team {
	case game.Home:
		return game.Away
	case game.Away:
		return game.Home
	default:
		return ""
	}
}

// favorite reports a spread with a negative line or a moneyline with negative
// odds.
func favorite(leg Leg) bool {
	switch leg.Market {
	case marketSpread:
		return leg.Line != nil && *leg.Line < 0
	case marketML:
		return leg.Odds < 0
	default:
		return false
	}
}

// underdog reports a spread with a positive line or a moneyline with positive
// odds.
func underdog(leg Leg) bool {
	switch leg.Market {
	case marketSpread:
		return leg.Line != nil && *leg.Line > 0
	case marketML:
		return leg.Odds > 0
	default:
		return false
	}
}

// trailingUnderdog narrows underdog to the legs that ask for a trailing script:
// a spread of +4.5 or more, or any underdog moneyline.
func trailingUnderdog(leg Leg) bool {
	if leg.Market == marketSpread {
		return leg.Line != nil && *leg.Line >= trailingSpread
	}
	return underdog(leg)
}

// passOrRecYardsOver reports an over on passing or receiving yards.
func passOrRecYardsOver(leg Leg) bool {
	if leg.Side != sideOver {
		return false
	}
	return leg.Market == marketPassYds || leg.Market == marketRecYds
}

// passingOrReceivingOver reports an over on a passing or receiving market.
func passingOrReceivingOver(leg Leg) bool {
	if leg.Side != sideOver {
		return false
	}
	switch leg.Market {
	case marketPassYds, marketPassTDs, marketRecYds, marketReceptions:
		return true
	default:
		return false
	}
}

// receivingOver reports an over on a receiving market.
func receivingOver(leg Leg) bool {
	if leg.Side != sideOver {
		return false
	}
	return leg.Market == marketRecYds || leg.Market == marketReceptions
}

// rushOver reports an over on a rushing market.
func rushOver(leg Leg) bool {
	if leg.Side != sideOver {
		return false
	}
	return leg.Market == marketRushYds || leg.Market == marketRushAtt
}

// selection spells a leg the way the slip shows it.
func selection(leg Leg) string {
	switch leg.Market {
	case marketSpread:
		return words(leg.Team, signedLine(leg.Line))
	case marketML:
		return words(leg.Team, "ML")
	case marketTotal:
		return words(sideWord(leg.Side), lineText(leg.Line))
	case marketTeamTotal:
		return words(leg.Team, "Team Total", sideWord(leg.Side), lineText(leg.Line))
	case marketAnytimeTD:
		return words(playerName(leg), "Anytime TD")
	default:
		return words(playerName(leg), sideWord(leg.Side), lineText(leg.Line), marketLabel(leg.Market))
	}
}

// underdogSubject spells an underdog leg and shows moneyline odds when there
// are no points to show.
func underdogSubject(leg Leg, owners bool) string {
	subject := selection(leg)
	if leg.Market == marketML {
		subject = fmt.Sprintf("%s %+d", subject, leg.Odds)
	}
	if owners && leg.Owner != "" {
		return leg.Owner + "'s " + subject
	}
	return subject
}

// describe joins legs for a message, naming owners when owners is set.
func describe(legs []Leg, owners bool) string {
	labels := make([]string, len(legs))
	for i, leg := range legs {
		if owners {
			labels[i] = ownedSelection(leg)
		} else {
			labels[i] = selection(leg)
		}
	}
	return humanJoin(labels)
}

// describePlayers joins the players on legs for a message, naming owners when
// owners is set.
func describePlayers(legs []Leg, owners bool) string {
	names := make([]string, len(legs))
	for i, leg := range legs {
		if owners && leg.Owner != "" {
			names[i] = leg.Owner + "'s " + playerName(leg)
		} else {
			names[i] = playerName(leg)
		}
	}
	return humanJoin(names)
}

// ownedSelection precedes a selection with its owner when one is known.
func ownedSelection(leg Leg) string {
	if leg.Owner == "" {
		return selection(leg)
	}
	return leg.Owner + "'s " + selection(leg)
}

// mixedOwners reports whether the legs belong to more than one person.
func mixedOwners(legs []Leg) bool {
	if len(legs) < 2 {
		return false
	}
	for _, leg := range legs[1:] {
		if leg.Owner != legs[0].Owner {
			return true
		}
	}
	return false
}

// playerName falls back to the team when a leg has no player.
func playerName(leg Leg) string {
	if leg.PlayerName != "" {
		return leg.PlayerName
	}
	return leg.Team
}

// humanJoin joins labels in a short list: "a", "a and b", "a, b, and c".
func humanJoin(labels []string) string {
	switch len(labels) {
	case 0:
		return ""
	case 1:
		return labels[0]
	case 2:
		return labels[0] + " and " + labels[1]
	default:
		return strings.Join(labels[:len(labels)-1], ", ") + ", and " + labels[len(labels)-1]
	}
}

// words joins the non-empty parts with single spaces.
func words(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, " ")
}

// lineText prints an optional line without trailing zeros.
func lineText(line *float64) string {
	if line == nil {
		return ""
	}
	return strconv.FormatFloat(*line, 'f', -1, 64)
}

// signedLine keeps the plus on a positive spread.
func signedLine(line *float64) string {
	if line == nil {
		return ""
	}
	text := lineText(line)
	if *line > 0 {
		return "+" + text
	}
	return text
}

// sideWord spells a side for a message.
func sideWord(side string) string {
	switch side {
	case sideOver:
		return "Over"
	case sideUnder:
		return "Under"
	case sideYes:
		return "Yes"
	default:
		return side
	}
}

// marketLabel is the short market suffix a selection shows.
func marketLabel(market string) string {
	switch market {
	case marketPassYds:
		return "Pass Yds"
	case marketPassTDs:
		return "Pass TDs"
	case marketRushYds:
		return "Rush Yds"
	case marketRushAtt:
		return "Rush Att"
	case marketRecYds:
		return "Rec Yds"
	case marketReceptions:
		return "Receptions"
	case marketKickerPts:
		return "Kicking Pts"
	default:
		return market
	}
}

// teamOrOther keeps a message readable when a leg's team is not in its game.
func teamOrOther(team string) string {
	if team == "" {
		return "the other team"
	}
	return team
}

// pairVerb, grindVerb, fightVerb, and stayVerb keep the message grammar right
// for one leg or several.
func pairVerb(count int) string {
	if count == 1 {
		return "pairs"
	}
	return "pair"
}

func grindVerb(count int) string {
	if count == 1 {
		return "grinds"
	}
	return "grind"
}

func fightVerb(count int) string {
	if count == 1 {
		return "fights"
	}
	return "fight"
}

func stayVerb(count int) string {
	if count == 1 {
		return "stays"
	}
	return "stay"
}
