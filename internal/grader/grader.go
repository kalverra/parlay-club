// Package grader evaluates parlay legs against final game scores and ESPN box
// scores. It is pure and deterministic: no database, HTTP, or clock access.
package grader

import (
	"math"
	"strings"

	"github.com/kalverra/parlay-club/internal/ingest/espn"
	"github.com/kalverra/parlay-club/internal/parlay"
)

const floatEpsilon = 1e-9

// LegInput is the slice of a slip leg needed to grade it.
type LegInput struct {
	Market     string
	Team       string
	PlayerID   string
	PlayerName string
	Side       string
	Line       *float64
}

// GameScore is the final score state of one NFL game.
type GameScore struct {
	HomeTeam  string
	AwayTeam  string
	HomeScore *int
	AwayScore *int
	Final     bool
}

// IsPlayerMarket reports whether market targets an athlete's box score stats
// rather than a game or team score.
func IsPlayerMarket(market string) bool {
	switch strings.ToUpper(strings.TrimSpace(market)) {
	case "PASS_YDS", "PASS_TDS", "RUSH_YDS", "RUSH_ATT",
		"REC_YDS", "RECEPTIONS", "ANYTIME_TD", "KICKER_PTS":
		return true
	default:
		return false
	}
}

// GradeLeg evaluates leg against a final game score and optional ESPN box
// score. Non-final games return parlay.Pending. Missing scores, missing lines,
// unmatched teams, unpopulated box scores (HasStats == false), and OTHER
// markets return parlay.NeedsGrading. Absent players in a populated box score
// (HasStats == true) return parlay.Void (DNP).
func GradeLeg(leg LegInput, game GameScore, box *espn.BoxScore) parlay.Result {
	if !game.Final {
		return parlay.Pending
	}

	market := strings.ToUpper(strings.TrimSpace(leg.Market))
	switch market {
	case "SPREAD":
		return gradeSpread(leg, game)
	case "ML":
		return gradeMoneyline(leg, game)
	case "TOTAL":
		return gradeTotal(leg, game)
	case "TEAM_TOTAL":
		return gradeTeamTotal(leg, game)
	case "PASS_YDS", "PASS_TDS", "RUSH_YDS", "RUSH_ATT",
		"REC_YDS", "RECEPTIONS", "ANYTIME_TD", "KICKER_PTS":
		return gradePlayerProp(market, leg, box)
	case "OTHER":
		return parlay.NeedsGrading
	default:
		return parlay.NeedsGrading
	}
}

func gradeSpread(leg LegInput, game GameScore) parlay.Result {
	if leg.Line == nil {
		return parlay.NeedsGrading
	}
	teamScore, oppScore, ok := teamScores(leg.Team, game)
	if !ok {
		return parlay.NeedsGrading
	}
	margin := float64(teamScore) + *leg.Line - float64(oppScore)
	switch {
	case math.Abs(margin) < floatEpsilon:
		return parlay.Push
	case margin > 0:
		return parlay.Won
	default:
		return parlay.Lost
	}
}

func gradeMoneyline(leg LegInput, game GameScore) parlay.Result {
	teamScore, oppScore, ok := teamScores(leg.Team, game)
	if !ok {
		return parlay.NeedsGrading
	}
	switch {
	case teamScore > oppScore:
		return parlay.Won
	case teamScore < oppScore:
		return parlay.Lost
	default:
		return parlay.Push
	}
}

func gradeTotal(leg LegInput, game GameScore) parlay.Result {
	if leg.Line == nil || game.HomeScore == nil || game.AwayScore == nil {
		return parlay.NeedsGrading
	}
	total := float64(*game.HomeScore + *game.AwayScore)
	return gradeOverUnder(total, *leg.Line, leg.Side)
}

func gradeTeamTotal(leg LegInput, game GameScore) parlay.Result {
	if leg.Line == nil {
		return parlay.NeedsGrading
	}
	teamScore, _, ok := teamScores(leg.Team, game)
	if !ok {
		return parlay.NeedsGrading
	}
	return gradeOverUnder(float64(teamScore), *leg.Line, leg.Side)
}

func gradePlayerProp(market string, leg LegInput, box *espn.BoxScore) parlay.Result {
	side := strings.ToUpper(strings.TrimSpace(leg.Side))
	if market == "ANYTIME_TD" {
		if side == "" {
			side = "YES"
		}
		if side != "YES" && side != "NO" {
			return parlay.NeedsGrading
		}
	} else {
		if leg.Line == nil {
			return parlay.NeedsGrading
		}
		if side != "OVER" && side != "UNDER" {
			return parlay.NeedsGrading
		}
	}

	playerID := strings.TrimSpace(leg.PlayerID)
	normName := normalizePlayerName(leg.PlayerName)
	if playerID == "" && normName == "" {
		return parlay.NeedsGrading
	}
	if box == nil || !box.HasStats {
		return parlay.NeedsGrading
	}

	player, found := findPlayer(box.Players, playerID, normName, leg.Team)
	if !found {
		return parlay.Void
	}

	if market == "ANYTIME_TD" {
		// Passing TDs do not count toward anytime TD scorer props.
		tds := player.Rushing.RushingTDs + player.Receiving.ReceivingTDs + player.OtherTDs
		scored := tds >= 1
		if side == "NO" {
			if scored {
				return parlay.Lost
			}
			return parlay.Won
		}
		if scored {
			return parlay.Won
		}
		return parlay.Lost
	}

	stat, ok := propStat(market, player)
	if !ok {
		return parlay.NeedsGrading
	}
	return gradeOverUnder(stat, *leg.Line, side)
}

func propStat(market string, p espn.PlayerBoxStats) (float64, bool) {
	switch market {
	case "PASS_YDS":
		return float64(p.Passing.PassingYards), true
	case "PASS_TDS":
		return float64(p.Passing.PassingTDs), true
	case "RUSH_YDS":
		return float64(p.Rushing.RushingYards), true
	case "RUSH_ATT":
		return float64(p.Rushing.RushingAttempts), true
	case "REC_YDS":
		return float64(p.Receiving.ReceivingYards), true
	case "RECEPTIONS":
		return float64(p.Receiving.Receptions), true
	case "KICKER_PTS":
		pts := p.Kicking.TotalKickingPoints
		if pts == 0 {
			pts = p.Kicking.FieldGoalsMade*3 + p.Kicking.ExtraPointsMade
		}
		return float64(pts), true
	default:
		return 0, false
	}
}

func gradeOverUnder(stat, line float64, side string) parlay.Result {
	diff := stat - line
	if math.Abs(diff) < floatEpsilon {
		return parlay.Push
	}
	switch strings.ToUpper(strings.TrimSpace(side)) {
	case "OVER":
		if diff > 0 {
			return parlay.Won
		}
		return parlay.Lost
	case "UNDER":
		if diff < 0 {
			return parlay.Won
		}
		return parlay.Lost
	default:
		return parlay.NeedsGrading
	}
}

func teamScores(team string, game GameScore) (teamScore, oppScore int, ok bool) {
	if game.HomeScore == nil || game.AwayScore == nil {
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

func findPlayer(players []espn.PlayerBoxStats, playerID, normName, team string) (espn.PlayerBoxStats, bool) {
	if playerID != "" {
		for _, p := range players {
			if strings.TrimSpace(p.PlayerID) == playerID {
				return p, true
			}
		}
	}
	if normName == "" {
		return espn.PlayerBoxStats{}, false
	}
	teamNorm := strings.ToUpper(strings.TrimSpace(team))
	for _, p := range players {
		if teamNorm != "" && p.TeamAbbrev != "" && !strings.EqualFold(strings.TrimSpace(p.TeamAbbrev), teamNorm) {
			continue
		}
		if normalizePlayerName(p.DisplayName) == normName {
			return p, true
		}
	}
	return espn.PlayerBoxStats{}, false
}

// normalizePlayerName mirrors db.NormalizePlayerName: lowercases, strips '.',
// ”', '-', and ',', collapses whitespace, and drops a trailing generational
// suffix (jr, sr, ii, iii, iv, v) when preceded by a space.
func normalizePlayerName(name string) string {
	name = strings.ToLower(name)
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		switch r {
		case '.', '\'', '-', ',':
		default:
			b.WriteRune(r)
		}
	}
	fields := strings.Fields(b.String())
	if len(fields) == 0 {
		return ""
	}
	switch fields[len(fields)-1] {
	case "jr", "sr", "ii", "iii", "iv", "v":
		if len(fields) > 1 {
			fields = fields[:len(fields)-1]
		}
	}
	return strings.Join(fields, " ")
}
