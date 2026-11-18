package board

import (
	"slices"
	"strings"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/espn"
)

// Suggestion badge labels attached to rows and player cards.
const (
	BadgeSmashMatchup = "Smash matchup"
	BadgeHighVolume   = "High volume"
	BadgeRoleBump     = "Role bump"
	BadgeFitsSlip     = "Fits slip"
)

const (
	smashTargetShareMin   = 0.20
	smashCarriesPerGame   = 12.0
	bottomDefenseRankMin  = 27
	roleBumpSnapDelta     = 0.15
	blowoutFavoriteSpread = -7.0
	maxPlayerTags         = 2
)

var badgePriority = []string{
	BadgeSmashMatchup,
	BadgeFitsSlip,
	BadgeRoleBump,
	BadgeHighVolume,
}

// ApplySignals evaluates nflverse, injury, and open-slip signals for one
// game's rows and returns a copy with Row.Badge populated.
func ApplySignals(game db.Game, rows []Row, signals BoardSignals) []Row {
	out := make([]Row, len(rows))
	copy(out, rows)
	if signals.empty() {
		return out
	}
	for i := range out {
		badges := rowBadges(game, out[i], signals)
		if len(badges) > 0 {
			out[i].Badge = badges[0]
		}
	}
	return out
}

func (s BoardSignals) empty() bool {
	return len(s.TeamEPA) == 0 && len(s.PlayerUsage) == 0 && len(s.Injuries) == 0 && len(s.SlipLegs) == 0
}

// rowBadges returns all matching badges for row in priority order.
func rowBadges(game db.Game, row Row, signals BoardSignals) []string {
	var badges []string
	if isSmashMatchup(game, row, signals) {
		badges = append(badges, BadgeSmashMatchup)
	}
	if fitsSlip(row, signals.SlipLegs) {
		badges = append(badges, BadgeFitsSlip)
	}
	if isRoleBump(row, signals) {
		badges = append(badges, BadgeRoleBump)
	}
	if isHighVolume(row, signals) {
		badges = append(badges, BadgeHighVolume)
	}
	return badges
}

// playerTags returns up to maxPlayerTags unique badges across a player's rows,
// ordered by priority.
func playerTags(game db.Game, rows []Row, signals BoardSignals) []string {
	if signals.empty() {
		return nil
	}
	seen := make(map[string]bool, len(badgePriority))
	for _, row := range rows {
		for _, badge := range rowBadges(game, row, signals) {
			seen[badge] = true
		}
	}
	if len(seen) == 0 {
		return nil
	}
	tags := make([]string, 0, min(len(seen), maxPlayerTags))
	for _, badge := range badgePriority {
		if seen[badge] {
			tags = append(tags, badge)
			if len(tags) == maxPlayerTags {
				break
			}
		}
	}
	return tags
}

// isSmashMatchup reports whether a player OVER prop has high share/carries
// against a bottom-6 opposing defense (rank >= 27).
func isSmashMatchup(game db.Game, row Row, signals BoardSignals) bool {
	if !isPlayerMarket(row.Market) || row.Side != "OVER" || row.PlayerID == "" {
		return false
	}
	usage := signals.PlayerUsage[row.PlayerID]
	if usage == nil || len(usage.Weeks) == 0 {
		return false
	}
	rank := opposingDefenseRank(game, row, signals.TeamEPA)
	if rank < bottomDefenseRankMin {
		return false
	}
	return usage.EffectiveTargetShare() >= smashTargetShareMin || usage.AverageCarries() >= smashCarriesPerGame
}

// isHighVolume reports whether a player prop OVER/YES has top-quartile usage
// for their position.
func isHighVolume(row Row, signals BoardSignals) bool {
	if !isPlayerMarket(row.Market) || (row.Side != "OVER" && row.Side != "YES") || row.PlayerID == "" {
		return false
	}
	usage := signals.PlayerUsage[row.PlayerID]
	if usage == nil || len(usage.Weeks) == 0 {
		return false
	}
	wopr := usage.RecentWOPR()
	targetShare := usage.EffectiveTargetShare()
	snapPct := usage.EffectiveSnapAvg()
	carries := usage.AverageCarries()

	switch strings.ToUpper(strings.TrimSpace(row.Position)) {
	case "WR":
		return wopr >= 0.55 || targetShare >= 0.24 || snapPct >= 0.85
	case "TE":
		return wopr >= 0.38 || targetShare >= 0.18 || snapPct >= 0.80
	case "RB":
		return snapPct >= 0.65 || carries >= 15.0 || (targetShare >= 0.12 && snapPct >= 0.50)
	case "QB":
		return carries >= 7.0
	case "K":
		return false
	default:
		return wopr >= 0.55 || targetShare >= 0.24 || snapPct >= 0.85
	}
}

// isRoleBump reports whether a teammate at the same position is Out and this
// player posted >= 15 percentage points higher OffensePct in a prior game.
func isRoleBump(row Row, signals BoardSignals) bool {
	if !isPlayerMarket(row.Market) || (row.Side != "OVER" && row.Side != "YES") || row.PlayerID == "" {
		return false
	}
	usage := signals.PlayerUsage[row.PlayerID]
	if usage == nil || len(usage.Weeks) < 2 {
		return false
	}
	if !hasOutTeammate(row, signals.Injuries[row.Team]) {
		return false
	}
	return maxOffensePct(usage.Weeks)-usage.EffectiveSeasonSnapAvg() >= roleBumpSnapDelta
}

func hasOutTeammate(row Row, injuries []espn.Injury) bool {
	for _, inj := range injuries {
		if !strings.EqualFold(strings.TrimSpace(inj.Status), "Out") {
			continue
		}
		if inj.AthleteID != "" && inj.AthleteID == row.PlayerID {
			continue
		}
		if inj.FullName != "" && strings.EqualFold(inj.FullName, row.PlayerName) {
			continue
		}
		if row.Position != "" && inj.Position != "" && !strings.EqualFold(row.Position, inj.Position) {
			continue
		}
		return true
	}
	return false
}

// fitsSlip reports whether row pairs positively with at least one leg already
// on the open slip for the same game.
func fitsSlip(row Row, slipLegs []analyzer.Leg) bool {
	if row.Picked != 0 || len(slipLegs) == 0 {
		return false
	}
	for _, leg := range slipLegs {
		if leg.GameID != row.GameID || rowMatchesAnalyzerLeg(row, leg) {
			continue
		}
		if legsPairPositively(row, leg) {
			return true
		}
	}
	return false
}

func legsPairPositively(row Row, leg analyzer.Leg) bool {
	switch {
	case leg.Market == "SPREAD" && leg.Line != nil && *leg.Line <= blowoutFavoriteSpread:
		return pairsWithFavoriteSpread(row, leg.Team)
	case leg.Market == "PASS_YDS" && leg.Side == "OVER":
		return pairsWithPassOver(row, leg)
	case isReceivingMarket(leg.Market) && leg.Side == "OVER":
		return pairsWithReceivingOver(row, leg.Team)
	case leg.Market == "TOTAL" && leg.Side == "OVER":
		return (row.Market == "PASS_YDS" || row.Market == "PASS_TDS" || isReceivingMarket(row.Market)) &&
			row.Side == "OVER"
	case isRushMarket(leg.Market) && leg.Side == "OVER":
		return row.Team == leg.Team && row.Market == "SPREAD" && row.Line != nil && *row.Line <= blowoutFavoriteSpread
	default:
		return false
	}
}

func pairsWithFavoriteSpread(row Row, favTeam string) bool {
	if row.Team == favTeam {
		return isRushMarket(row.Market) && row.Side == "OVER"
	}
	if row.Team == "" {
		return false
	}
	return (isReceivingMarket(row.Market) && row.Side == "OVER") ||
		(row.Market == "TEAM_TOTAL" && row.Side == "UNDER")
}

func pairsWithPassOver(row Row, leg analyzer.Leg) bool {
	if row.Market == "TOTAL" && row.Side == "OVER" {
		return true
	}
	if row.Team != leg.Team {
		return false
	}
	if row.Market == "TEAM_TOTAL" && row.Side == "OVER" {
		return true
	}
	return isReceivingMarket(row.Market) && row.Side == "OVER" && !sameBoardPlayer(row, leg)
}

func pairsWithReceivingOver(row Row, recTeam string) bool {
	if row.Market == "TOTAL" && row.Side == "OVER" {
		return true
	}
	return row.Team == recTeam && row.Market == "PASS_YDS" && row.Side == "OVER"
}

func isRushMarket(market string) bool {
	return market == "RUSH_YDS" || market == "RUSH_ATT"
}

func isReceivingMarket(market string) bool {
	return market == "REC_YDS" || market == "RECEPTIONS"
}

func sameBoardPlayer(row Row, leg analyzer.Leg) bool {
	if row.PlayerID != "" && leg.PlayerID != "" {
		return row.PlayerID == leg.PlayerID
	}
	return row.PlayerName != "" && strings.EqualFold(row.PlayerName, leg.PlayerName)
}

func rowMatchesAnalyzerLeg(row Row, leg analyzer.Leg) bool {
	return row.GameID == leg.GameID &&
		row.Market == leg.Market &&
		row.Team == leg.Team &&
		row.PlayerName == leg.PlayerName &&
		row.Side == leg.Side &&
		lineEqual(row.Line, leg.Line)
}

func groupSignalScore(game db.Game, group LineGroup, signals BoardSignals) int {
	if signals.empty() {
		return 0
	}
	best := 0
	for _, option := range group.Options {
		if score := rowSignalScore(game, option, signals); score > best {
			best = score
		}
	}
	return best
}

func playerSignalScore(game db.Game, player PlayerGroup, signals BoardSignals) int {
	if signals.empty() {
		return 0
	}
	best := 0
	for _, line := range player.Lines {
		if score := groupSignalScore(game, line, signals); score > best {
			best = score
		}
	}
	return best
}

func rowSignalScore(game db.Game, row Row, signals BoardSignals) int {
	if signals.empty() {
		return 0
	}
	score := 0
	badges := rowBadges(game, row, signals)
	if slices.Contains(badges, BadgeSmashMatchup) {
		score += 40
	}
	if slices.Contains(badges, BadgeFitsSlip) {
		score += 30
	}
	if slices.Contains(badges, BadgeRoleBump) {
		score += 25
	}
	if slices.Contains(badges, BadgeHighVolume) {
		score += 20
	}
	if rank := opposingDefenseRank(game, row, signals.TeamEPA); rank > 0 {
		switch row.Side {
		case "OVER":
			score += rank
		case "UNDER":
			score += 33 - rank
		}
	}
	return score
}

func opposingDefenseRank(game db.Game, row Row, teamEPA map[string]*analyzer.TeamDefenseProfile) int {
	if len(teamEPA) == 0 || row.Team == "" {
		return 0
	}
	var opponent string
	switch row.Team {
	case game.HomeTeam:
		opponent = game.AwayTeam
	case game.AwayTeam:
		opponent = game.HomeTeam
	default:
		return 0
	}
	_, rank, ok := analyzer.MatchupDefenseRank(row.Market, teamEPA[opponent])
	if !ok {
		return 0
	}
	return rank
}

func maxOffensePct(weeks []analyzer.PlayerWeekUsage) float64 {
	best := 0.0
	for _, w := range weeks {
		if w.OffensePct > best {
			best = w.OffensePct
		}
	}
	return best
}
