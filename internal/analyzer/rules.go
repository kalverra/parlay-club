package analyzer

import (
	"fmt"
	"math"
	"slices"
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
	ruleWind               = "wind"
	ruleFakeWeather        = "fake_weather"
	ruleHook               = "hook"
	ruleTotalKeyNumber     = "total_key_number"
	ruleChalk              = "chalk"
	ruleQBDrag             = "qb_drag"
	ruleMedianTrap         = "median_trap"
	ruleToughMatchup       = "tough_matchup"
	ruleShrinkingRole      = "shrinking_role"
	rulePaceClash          = "pace_clash"
	ruleTDSpikeRisk        = "td_spike_risk"
)

// Thresholds from plan/analyzer.md and nflverse signal rules.
const (
	blowoutSpread  = -7.0
	trailingSpread = 4.5

	windDanger = 20.0
	windWarn   = 15.0
	gustWarn   = 25.0

	fakeWeatherPrecipMax = 0.1
	fakeWeatherWindMax   = 10.0

	chalkOdds = -230

	// qbDragElo is the nfelo QB adjustment at which a team's QB is dragging the
	// rating down. 25 Elo is about a point of spread.
	qbDragElo   = -30.0
	eloPerPoint = 25.0

	toughDefenseMaxRank     = 5
	favorableDefenseMinRank = 25

	shrinkingSnapDrop        = 0.15
	shrinkingTargetShareDrop = 0.08
	risingSnapGain           = 0.08
	risingTargetShareGain    = 0.04

	paceSlowMinRank      = 25
	passRateLowMinRank   = 23
	paceFastMaxRank      = 8
	offPassEPATopMaxRank = 8

	tdSpikeMinTDs            = 3.0
	tdSpikePassMinTDs        = 6.0
	tdSpikeMaxTargetShare    = 0.15
	tdSpikeMaxCarriesPerGame = 8.0

	floatEps = 1e-9
)

// totalKeyNumbers are the NFL final totals that decide the most bets.
var totalKeyNumbers = []float64{37, 41, 43, 44, 47, 51}

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
		if !favorite(spread) || spread.Market != marketSpread {
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

// wind warns when sustained wind or gusts make passing and kicking overs a
// worse bet. It flags every such leg in the game and gives one warning per
// outdoor game with known weather.
func wind(a *analysis) []Warning {
	var warnings []Warning
	seen := make(map[string]bool)
	for _, leg := range a.legs {
		if seen[leg.GameID] {
			continue
		}
		seen[leg.GameID] = true

		game, ok := a.games[leg.GameID]
		if !ok || game.Indoor || game.Weather == nil {
			continue
		}
		severity, windy := windLevel(*game.Weather)
		if !windy {
			continue
		}
		flagged := a.inGame(leg.GameID, windHit)
		if len(flagged) == 0 {
			continue
		}

		owners := mixedOwners(flagged)
		message := fmt.Sprintf(
			"Heads up: %s in %s today. Passing and kicking overs take a real hit in that, so %s %s tougher. Field goals drop to about 77%%.",
			windText(*game.Weather),
			game.Home,
			describe(flagged, owners),
			getVerb(len(flagged)),
		)
		warnings = append(warnings, a.warning(ruleWind, severity, message, flagged))
	}
	return warnings
}

// fakeWeather notes that light rain without wind is not the scoring brake the
// slip might expect it to be.
func fakeWeather(a *analysis) []Warning {
	var warnings []Warning
	seen := make(map[string]bool)
	for _, leg := range a.legs {
		if seen[leg.GameID] {
			continue
		}
		seen[leg.GameID] = true

		game, ok := a.games[leg.GameID]
		if !ok || game.Indoor || game.Weather == nil {
			continue
		}
		weather := *game.Weather
		if weather.PrecipIn <= 0 || weather.PrecipIn >= fakeWeatherPrecipMax || weather.WindMPH >= fakeWeatherWindMax {
			continue
		}
		unders := a.inGame(leg.GameID, func(leg Leg) bool {
			return leg.Market == marketTotal && leg.Side == sideUnder
		})
		if len(unders) == 0 {
			continue
		}

		owners := mixedOwners(unders)
		message := fmt.Sprintf(
			"FYI: light rain without wind doesn't actually lower NFL scoring. %s %s no extra help here.",
			describe(unders, owners), getVerb(len(unders)))
		warnings = append(warnings, a.warning(ruleFakeWeather, Info, message, unders))
	}
	return warnings
}

// hook warns when a spread sits on a half point that key numbers decide.
func hook(a *analysis) []Warning {
	var warnings []Warning
	for _, leg := range a.legs {
		if leg.Market != marketSpread || leg.Line == nil {
			continue
		}
		landing, buyTo, moneyline, ok := hookPoint(*leg.Line)
		if !ok {
			continue
		}

		message := hookMessage(leg, *leg.Line, landing, buyTo, moneyline)
		if game, ok := a.games[leg.GameID]; ok {
			message += hookModelNote(game, leg)
		}
		warnings = append(warnings, a.warning(ruleHook, Warn, message, []Leg{leg}))
	}
	return warnings
}

// totalKeyNumber notes a game total sitting on or next to a common NFL final
// total.
func totalKeyNumber(a *analysis) []Warning {
	var warnings []Warning
	for _, leg := range a.legs {
		if leg.Market != marketTotal || leg.Line == nil {
			continue
		}
		keys := keyNumbersNear(*leg.Line)
		if len(keys) == 0 {
			continue
		}

		message := keyNumberMessage(*leg.Line, keys)
		warnings = append(warnings, a.warning(ruleTotalKeyNumber, Info, message, []Leg{leg}))
	}
	return warnings
}

// chalk warns that a heavy favorite's leg adds little payout and still loses
// the slip about a third of the time.
func chalk(a *analysis) []Warning {
	var warnings []Warning
	for _, leg := range a.legs {
		if leg.Odds > chalkOdds {
			continue
		}
		odds := float64(-leg.Odds)
		multiplier := 1 + 100/odds
		bust := 1 - odds/(odds+100)
		message := fmt.Sprintf(
			"This %d leg only bumps the payout %s× but still busts the slip about %d%% of the time.",
			leg.Odds, multiplierText(multiplier), int(math.Round(bust*100)))
		if leg.Market == marketML {
			if game, ok := a.games[leg.GameID]; ok {
				message += chalkModelNote(game, leg)
			}
		}
		warnings = append(warnings, a.warning(ruleChalk, Warn, message, []Leg{leg}))
	}
	return warnings
}

// qbDrag notes a team whose nfelo QB adjustment drags the rating down while the
// slip asks that team to do the things a good QB does. It gives one warning per
// game and team.
func qbDrag(a *analysis) []Warning {
	var warnings []Warning
	seen := make(map[string]bool)
	for _, leg := range a.legs {
		game, ok := a.games[leg.GameID]
		if !ok || game.Nfelo == nil {
			continue
		}
		team, qb, adjustment, ok := qbDragTeam(game, leg.Team)
		if !ok || adjustment > qbDragElo {
			continue
		}
		key := leg.GameID + "\x00" + team
		if seen[key] {
			continue
		}
		flagged := a.inGame(leg.GameID, func(other Leg) bool {
			return qbDragHit(other, team)
		})
		if len(flagged) == 0 {
			continue
		}
		seen[key] = true

		owners := mixedOwners(flagged)
		message := qbDragMessage(team, qb, adjustment, describe(flagged, owners))
		warnings = append(warnings, a.warning(ruleQBDrag, Info, message, flagged))
	}
	return warnings
}

// hookModelNote appends nfelo's view of a hook. The cushion is the leg's line
// minus the model's spread for the same team, so a positive cushion means the
// model is farther from the number than the leg.
func hookModelNote(game Game, leg Leg) string {
	model, ok := game.ModelSpread(leg.Team)
	if !ok {
		return ""
	}
	cushion := *leg.Line - model
	switch {
	case cushion >= 2:
		return fmt.Sprintf(" nfelo has it nearer %s, so the hook matters less.", modelSpreadText(model))
	case cushion <= -2:
		return fmt.Sprintf(
			" nfelo has it closer to %s, so this asks for more than the model does.",
			modelSpreadText(model))
	default:
		return fmt.Sprintf(
			" nfelo sees it near the number too (%s), so the hook is live.",
			modelSpreadText(model))
	}
}

// chalkModelNote appends nfelo's win probability to a moneyline chalk warning.
func chalkModelNote(game Game, leg Leg) string {
	probability, ok := game.ModelWinProb(leg.Team)
	if !ok {
		return ""
	}
	return fmt.Sprintf(
		" nfelo has %s at about %d%% to win.", leg.Team, int(math.Round(probability*100)))
}

// qbDragTeam returns the team's nfelo QB and adjustment when the team is in the
// game and has a model.
func qbDragTeam(game Game, team string) (string, string, float64, bool) {
	if game.Nfelo == nil {
		return "", "", 0, false
	}
	switch team {
	case game.Home:
		return team, game.Nfelo.HomeQB, game.Nfelo.HomeQBAdj, true
	case game.Away:
		return team, game.Nfelo.AwayQB, game.Nfelo.AwayQBAdj, true
	default:
		return "", "", 0, false
	}
}

// qbDragHit reports a leg the QB downgrade cuts against: a favorite spread or
// moneyline, or an over on that team's passing, receiving, or team total.
func qbDragHit(leg Leg, team string) bool {
	if leg.Team != team {
		return false
	}
	switch leg.Market {
	case marketSpread, marketML:
		return favorite(leg)
	case marketPassYds, marketPassTDs, marketRecYds, marketReceptions, marketTeamTotal:
		return leg.Side == sideOver
	default:
		return false
	}
}

// qbDragMessage names the QB, the points the model takes off, and the legs it
// drags on.
func qbDragMessage(team, qb string, adjustment float64, legs string) string {
	points := strings.TrimSuffix(
		strconv.FormatFloat(math.Abs(adjustment)/eloPerPoint, 'f', 1, 64), ".0")
	if qb == "" {
		return fmt.Sprintf(
			"nfelo marks down %s's QB by about %s points. That drags on %s.",
			team, points, legs)
	}
	return fmt.Sprintf(
		"nfelo marks down %s's QB, %s, by about %s points. That drags on %s.",
		team, qb, points, legs)
}

// modelSpreadText prints a model spread with one decimal and a plus on a
// positive number.
func modelSpreadText(spread float64) string {
	text := strings.TrimSuffix(strconv.FormatFloat(spread, 'f', 1, 64), ".0")
	if spread > 0 {
		return "+" + text
	}
	return text
}

// medianTrap warns when a yardage over line is higher than the player's last-5
// game median. When the line is also below the season average, it notes that
// the average is propped up by one big game.
func medianTrap(a *analysis) []Warning {
	if len(a.stats) == 0 {
		return nil
	}
	var warnings []Warning
	for _, leg := range a.legs {
		if !isYardageMarket(leg.Market) || leg.Side != sideOver || leg.Line == nil || leg.PlayerID == "" {
			continue
		}
		playerStats, ok := a.stats[leg.PlayerID]
		if !ok || playerStats.Games == nil {
			continue
		}
		games := playerStats.Games[leg.Market]
		if len(games) < 3 {
			continue
		}

		l5Len := min(5, len(games))
		l5 := games[:l5Len]
		median := Median(l5)
		if *leg.Line <= median {
			continue
		}

		name := playerName(leg)
		message := fmt.Sprintf("%s's last-5 median is %s yards, so %s is a stretch.",
			name, number(median), number(*leg.Line))
		mean := Mean(games)
		if *leg.Line < mean {
			message += fmt.Sprintf(" His %s-yard average is propped up by one big game.",
				number(math.Round(mean)))
		}

		warnings = append(warnings, a.warning(ruleMedianTrap, Warn, message, []Leg{leg}))
	}
	return warnings
}

// toughMatchup warns when a yardage or TD over faces a top-5 opposing defense
// in the matching pass or rush EPA-allowed split.
func toughMatchup(a *analysis) []Warning {
	var warnings []Warning
	for _, leg := range a.legs {
		game, ok := a.games[leg.GameID]
		if !ok {
			continue
		}
		split, rank, hit := toughMatchupHit(leg, game)
		if !hit {
			continue
		}
		opp := teamOrOther(a.opponent(leg))
		message := fmt.Sprintf(
			"%s ranks #%d against the %s by EPA allowed, so %s is a tough matchup.",
			opp, rank, split, selection(leg))
		warnings = append(warnings, a.warning(ruleToughMatchup, Warn, message, []Leg{leg}))
	}
	return warnings
}

// shrinkingRole warns when a player prop over's recent snap share or target
// share has dropped meaningfully below their season average.
func shrinkingRole(a *analysis) []Warning {
	if len(a.stats) == 0 {
		return nil
	}
	var warnings []Warning
	for _, leg := range a.legs {
		ps, ok := a.stats[leg.PlayerID]
		if !ok {
			continue
		}
		reason, hit := shrinkingRoleHit(leg, ps)
		if !hit {
			continue
		}
		message := fmt.Sprintf(
			"%s's %s — recent volume is fading for %s.",
			playerName(leg), reason, selection(leg))
		warnings = append(warnings, a.warning(ruleShrinkingRole, Warn, message, []Leg{leg}))
	}
	return warnings
}

// paceClash warns when a game total fights both teams' pace and efficiency
// profile (slow & run-heavy vs. an Over, or fast & pass-efficient vs. an Under).
func paceClash(a *analysis) []Warning {
	var warnings []Warning
	for _, leg := range a.legs {
		if leg.Market != marketTotal {
			continue
		}
		game, ok := a.games[leg.GameID]
		if !ok || game.HomeEPA == nil || game.AwayEPA == nil {
			continue
		}
		switch leg.Side {
		case sideOver:
			if game.HomeEPA.PlaysPerGameRank >= paceSlowMinRank &&
				game.AwayEPA.PlaysPerGameRank >= paceSlowMinRank &&
				game.HomeEPA.PassRateRank >= passRateLowMinRank &&
				game.AwayEPA.PassRateRank >= passRateLowMinRank {
				message := fmt.Sprintf(
					"Both %s and %s sit bottom-8 in pace and bottom-10 in pass rate, so %s needs a lot of efficiency.",
					game.Away, game.Home, selection(leg))
				warnings = append(warnings, a.warning(rulePaceClash, Warn, message, []Leg{leg}))
			}
		case sideUnder:
			if inTopRank(game.HomeEPA.PlaysPerGameRank, paceFastMaxRank) &&
				inTopRank(game.AwayEPA.PlaysPerGameRank, paceFastMaxRank) &&
				inTopRank(game.HomeEPA.OffPassEPARank, offPassEPATopMaxRank) &&
				inTopRank(game.AwayEPA.OffPassEPARank, offPassEPATopMaxRank) {
				message := fmt.Sprintf(
					"Both %s and %s sit top-8 in pace and passing EPA, so %s is fighting two fast, efficient offenses.",
					game.Away, game.Home, selection(leg))
				warnings = append(warnings, a.warning(rulePaceClash, Warn, message, []Leg{leg}))
			}
		}
	}
	return warnings
}

// tdSpikeRisk notes when recent TD scoring outpaces a player's underlying
// opportunity (low target share and carry volume for Anytime TD, or negative EPA
// on high recent passing TDs).
func tdSpikeRisk(a *analysis) []Warning {
	if len(a.stats) == 0 {
		return nil
	}
	var warnings []Warning
	for _, leg := range a.legs {
		if leg.PlayerID == "" {
			continue
		}
		ps, ok := a.stats[leg.PlayerID]
		if !ok || ps.Usage == nil || len(ps.Usage.Weeks) < 3 {
			continue
		}
		games := ps.Games[leg.Market]
		if len(games) < 3 {
			continue
		}
		last3TDs := games[0] + games[1] + games[2]

		switch {
		case leg.Market == marketAnytimeTD && leg.Side == sideYes:
			avgCarries := last3CarriesAvg(ps.Usage.Weeks)
			if last3TDs >= tdSpikeMinTDs &&
				ps.Usage.Last3TargetShare < tdSpikeMaxTargetShare &&
				avgCarries < tdSpikeMaxCarriesPerGame {
				message := fmt.Sprintf(
					"%s has %s TDs in the last 3 games on a %d%% target share and %s carries/game — TD luck can cool off.",
					playerName(leg),
					number(last3TDs),
					int(math.Round(ps.Usage.Last3TargetShare*100)),
					number(math.Round(avgCarries*10)/10),
				)
				warnings = append(warnings, a.warning(ruleTDSpikeRisk, Info, message, []Leg{leg}))
			}
		case leg.Market == marketPassTDs && leg.Side == sideOver:
			avgEPA := last3TotalEPAAvg(ps.Usage.Weeks)
			if last3TDs >= tdSpikePassMinTDs && avgEPA <= 0 {
				message := fmt.Sprintf(
					"%s has %s passing TDs in the last 3 games despite negative EPA — TD efficiency can cool off.",
					playerName(leg), number(last3TDs))
				warnings = append(warnings, a.warning(ruleTDSpikeRisk, Info, message, []Leg{leg}))
			}
		}
	}
	return warnings
}

func inTopRank(rank, maxRank int) bool {
	return rank >= 1 && rank <= maxRank
}

// MatchupDefenseRank returns the opposing defense's EPA-allowed split ("pass"
// or "rush") and league rank (1..32) for a yardage, TD, or reception market.
func MatchupDefenseRank(market string, oppEPA *TeamDefenseProfile) (split string, rank int, ok bool) {
	if oppEPA == nil {
		return "", 0, false
	}
	switch market {
	case marketPassYds, marketPassTDs, marketRecYds, marketReceptions:
		if oppEPA.DefPassEPAAllowedRank <= 0 {
			return "", 0, false
		}
		return "pass", oppEPA.DefPassEPAAllowedRank, true
	case marketRushYds, marketRushAtt:
		if oppEPA.DefRushEPAAllowedRank <= 0 {
			return "", 0, false
		}
		return "rush", oppEPA.DefRushEPAAllowedRank, true
	default:
		return "", 0, false
	}
}

// matchupRank returns the opposing defense's EPA-allowed rank and split name
// ("pass" or "rush") for a yardage/TD over leg.
func matchupRank(leg Leg, game Game) (split string, rank int, ok bool) {
	if leg.Side != sideOver {
		return "", 0, false
	}
	return MatchupDefenseRank(leg.Market, game.OpponentEPA(leg.Team))
}

func toughMatchupHit(leg Leg, game Game) (split string, rank int, hit bool) {
	split, rank, ok := matchupRank(leg, game)
	if !ok || !inTopRank(rank, toughDefenseMaxRank) {
		return "", 0, false
	}
	return split, rank, true
}

func favorableMatchupHit(leg Leg, game Game) (split string, rank int, hit bool) {
	split, rank, ok := matchupRank(leg, game)
	if !ok || rank < favorableDefenseMinRank {
		return "", 0, false
	}
	return split, rank, true
}

func isPlayerPropMarket(market string) bool {
	switch market {
	case marketPassYds, marketPassTDs, marketRushYds, marketRushAtt, marketRecYds, marketReceptions:
		return true
	default:
		return false
	}
}

func shrinkingRoleHit(leg Leg, ps PlayerStats) (string, bool) {
	if leg.Side != sideOver || leg.PlayerID == "" || !isPlayerPropMarket(leg.Market) {
		return "", false
	}
	if ps.Usage == nil || len(ps.Usage.Weeks) < 3 {
		return "", false
	}
	last2Snap := (ps.Usage.Weeks[0].OffensePct + ps.Usage.Weeks[1].OffensePct) / 2
	if ps.Usage.SeasonSnapAvg-last2Snap >= shrinkingSnapDrop-floatEps {
		return fmt.Sprintf(
			"snap share dropped from %d%% on the season to %d%% over the last 2 games",
			int(math.Round(ps.Usage.SeasonSnapAvg*100)),
			int(math.Round(last2Snap*100)),
		), true
	}
	last2Target := (ps.Usage.Weeks[0].TargetShare + ps.Usage.Weeks[1].TargetShare) / 2
	if ps.Usage.SeasonTargetShare-last2Target >= shrinkingTargetShareDrop-floatEps {
		return fmt.Sprintf(
			"target share dropped from %d%% on the season to %d%% over the last 2 games",
			int(math.Round(ps.Usage.SeasonTargetShare*100)),
			int(math.Round(last2Target*100)),
		), true
	}
	return "", false
}

func last3CarriesAvg(weeks []PlayerWeekUsage) float64 {
	return float64(weeks[0].Carries+weeks[1].Carries+weeks[2].Carries) / 3.0
}

func last3TotalEPAAvg(weeks []PlayerWeekUsage) float64 {
	return (weeks[0].TotalEPA + weeks[1].TotalEPA + weeks[2].TotalEPA) / 3.0
}

// isYardageMarket reports whether the market is measured in yards.
func isYardageMarket(market string) bool {
	switch market {
	case marketPassYds, marketRushYds, marketRecYds:
		return true
	default:
		return false
	}
}

// Median computes the median of values.
func Median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	middle := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[middle-1] + sorted[middle]) / 2
	}
	return sorted[middle]
}

// Mean computes the arithmetic mean of values.
func Mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
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

// inGame returns every leg in the game that keep approves, in slip order.
func (a *analysis) inGame(gameID string, keep func(Leg) bool) []Leg {
	var matches []Leg
	for _, leg := range a.legs {
		if leg.GameID != gameID || !keep(leg) {
			continue
		}
		matches = append(matches, leg)
	}
	return matches
}

// windLevel grades a forecast. It reports false when the weather is mild
// enough that nothing needs a note.
func windLevel(weather Weather) (Severity, bool) {
	switch {
	case weather.WindMPH >= windDanger:
		return Danger, true
	case weather.WindMPH >= windWarn, weather.GustMPH >= gustWarn:
		return Warn, true
	default:
		return Info, false
	}
}

// windHit reports a leg that wind makes harder: an over on a passing,
// receiving, or kicking market, or a game total over.
func windHit(leg Leg) bool {
	if leg.Side != sideOver {
		return false
	}
	switch leg.Market {
	case marketPassYds, marketPassTDs, marketRecYds, marketReceptions, marketKickerPts, marketTotal:
		return true
	default:
		return false
	}
}

// windText names the weather that matters for the warning.
func windText(weather Weather) string {
	if weather.WindMPH >= windWarn {
		return number(weather.WindMPH) + " mph winds"
	}
	return "gusts to " + number(weather.GustMPH) + " mph"
}

// hookPoint maps a spread on the hook to the number it would land on, the
// number to buy to, and whether the moneyline is a sensible switch.
func hookPoint(line float64) (landing, buyTo string, moneyline, ok bool) {
	switch line {
	case -3.5:
		return "3", "-3", true, true
	case -7.5:
		return "7", "-7", true, true
	case 2.5:
		return "3", "+3", false, true
	case 6.5:
		return "7", "+7", false, true
	default:
		return "", "", false, false
	}
}

// hookMessage spells the hook warning for one spread.
func hookMessage(leg Leg, line float64, landing, buyTo string, moneyline bool) string {
	subject := selection(leg)
	if line == -3.5 {
		return subject +
			" is on the hook. About 15% of games land on exactly 3, and that's a loss here. " +
			"Buying to -3 or switching to the ML is worth a look."
	}
	suggestion := "Buying to " + buyTo + " is worth a look."
	if moneyline {
		suggestion = "Buying to " + buyTo + " or switching to the ML is worth a look."
	}
	return fmt.Sprintf(
		"%s is on the hook. Landing on exactly %s loses by the hook. %s",
		subject, landing, suggestion)
}

// keyNumbersNear returns the common final totals within half a point.
func keyNumbersNear(line float64) []float64 {
	var near []float64
	for _, key := range totalKeyNumbers {
		if math.Abs(line-key) <= 0.5 {
			near = append(near, key)
		}
	}
	return near
}

// keyNumberMessage explains where a total sits against the key numbers.
func keyNumberMessage(line float64, keys []float64) string {
	text := number(line)
	switch {
	case len(keys) == 1 && keys[0] == line:
		return text + " is one of the most common NFL final totals. A half point either way decides it."
	case len(keys) == 1:
		return fmt.Sprintf(
			"%s is half a point off %s, one of the most common NFL final totals. The half point matters here.",
			text, number(keys[0]))
	default:
		return fmt.Sprintf(
			"%s sits right between %s and %s, two of the most common NFL final totals. The half point matters here.",
			text, number(keys[0]), number(keys[1]))
	}
}

// multiplierText prints a payout multiplier with no trailing zeros.
func multiplierText(multiplier float64) string {
	return strings.TrimSuffix(fmt.Sprintf("%.2f", multiplier), "0")
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
	if !underdog(leg) {
		return false
	}
	if leg.Market == marketSpread {
		return *leg.Line >= trailingSpread
	}
	return true
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
	return number(*line)
}

// number prints a float without trailing zeros.
func number(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
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

func getVerb(count int) string {
	if count == 1 {
		return "gets"
	}
	return "get"
}
