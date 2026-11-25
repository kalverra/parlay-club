package eval

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/grader"
	"github.com/kalverra/parlay-club/internal/ingest/espn"
	"github.com/kalverra/parlay-club/internal/parlay"
)

// MarketGrader deterministically grades market-wide candidate legs offline
// against final game scores, completed archived ESPN box scores, and
// same-week nflverse_player_weeks rows.
type MarketGrader struct {
	boxByGame      map[string]*espn.BoxScore
	nflverseTeams  map[string]bool
	nflverseByID   map[string][]db.NFLversePlayerWeek
	nflverseByName map[string][]db.NFLversePlayerWeek
}

// NewMarketGrader loads same-week nflverse_player_weeks once for week so
// player prop grading can fall back to nflverse when no completed ESPN box
// score is available.
func NewMarketGrader(ctx context.Context, store *db.Store, week db.Week) *MarketGrader {
	g := &MarketGrader{
		boxByGame:      make(map[string]*espn.BoxScore),
		nflverseTeams:  make(map[string]bool),
		nflverseByID:   make(map[string][]db.NFLversePlayerWeek),
		nflverseByName: make(map[string][]db.NFLversePlayerWeek),
	}
	if store == nil {
		return g
	}
	rows, err := store.NFLversePlayerWeeksForSeason(ctx, week.Season, week.SeasonType)
	if err != nil {
		return g
	}
	for _, r := range rows {
		if r.WeekNum != week.WeekNum {
			continue
		}
		team := strings.ToUpper(strings.TrimSpace(r.Team))
		if team != "" {
			g.nflverseTeams[team] = true
		}
		if id := strings.TrimSpace(r.ESPNID); id != "" {
			g.nflverseByID[id] = append(g.nflverseByID[id], r)
		}
		if norm := db.NormalizePlayerName(r.PlayerName); norm != "" {
			g.nflverseByName[norm] = append(g.nflverseByName[norm], r)
		}
	}
	return g
}

// GradeCandidates grades each candidate leg and drops any that return Pending
// or NeedsGrading.
func (g *MarketGrader) GradeCandidates(
	ctx context.Context,
	store *db.Store,
	games map[string]db.Game,
	candidates []analyzer.Leg,
) []ReplayedLeg {
	out := make([]ReplayedLeg, 0, len(candidates))
	for _, leg := range candidates {
		game, ok := games[leg.GameID]
		if !ok {
			continue
		}
		dec := parlay.AmericanToDecimal(leg.Odds)
		if dec <= 1 {
			continue
		}
		res := g.GradeLeg(ctx, store, game, leg)
		if res == parlay.Pending || res == parlay.NeedsGrading {
			continue
		}
		out = append(out, ReplayedLeg{
			Leg:         leg,
			Result:      res,
			DecimalOdds: dec,
		})
	}
	return out
}

// GradeLeg deterministically grades a single candidate leg against game's
// final score, archived ESPN box score, or same-week nflverse_player_weeks.
func (g *MarketGrader) GradeLeg(
	ctx context.Context,
	store *db.Store,
	game db.Game,
	leg analyzer.Leg,
) parlay.Result {
	gameScore := grader.GameScore{
		HomeTeam:  game.HomeTeam,
		AwayTeam:  game.AwayTeam,
		HomeScore: game.HomeScore,
		AwayScore: game.AwayScore,
		Final:     game.Status == db.GameFinal,
	}
	if !gameScore.Final {
		return parlay.Pending
	}

	legInput := grader.LegInput{
		Market:     leg.Market,
		Team:       leg.Team,
		PlayerID:   leg.PlayerID,
		PlayerName: leg.PlayerName,
		Side:       leg.Side,
		Line:       leg.Line,
	}

	market := strings.ToUpper(strings.TrimSpace(leg.Market))
	if !db.IsPlayerMarket(market) {
		return grader.GradeLeg(legInput, gameScore, nil)
	}

	if box := g.completedBoxScore(ctx, store, game); box != nil {
		return grader.GradeLeg(legInput, gameScore, box)
	}

	return g.gradeFromNFLverse(game, legInput, gameScore)
}

func (g *MarketGrader) completedBoxScore(ctx context.Context, store *db.Store, game db.Game) *espn.BoxScore {
	if cached, ok := g.boxByGame[game.ID]; ok {
		return cached
	}

	var resolved *espn.BoxScore
	if game.BoxScoreJSON != nil && *game.BoxScoreJSON != "" {
		var box espn.BoxScore
		if err := json.Unmarshal([]byte(*game.BoxScoreJSON), &box); err == nil && box.Completed && box.HasStats {
			resolved = &box
		}
	}

	if resolved == nil && store != nil {
		if resp, ok, err := store.LatestAPIResponse(ctx, db.ArchiveSummaryKey(game.ID)); err == nil && ok {
			if raw, err := store.APIResponseBody(ctx, resp.BodySHA256); err == nil {
				if box, err := espn.ParseBoxScoreJSON(raw); err == nil && box.Completed && box.HasStats {
					resolved = &box
				}
			}
		}
	}

	g.boxByGame[game.ID] = resolved
	return resolved
}

func (g *MarketGrader) gradeFromNFLverse(
	game db.Game,
	leg grader.LegInput,
	gameScore grader.GameScore,
) parlay.Result {
	home := strings.ToUpper(strings.TrimSpace(game.HomeTeam))
	away := strings.ToUpper(strings.TrimSpace(game.AwayTeam))
	if !g.nflverseTeams[home] && !g.nflverseTeams[away] {
		return parlay.NeedsGrading
	}

	row, ok := g.findNFLversePlayer(home, away, leg)
	if !ok {
		return parlay.NeedsGrading
	}

	adapted := espn.BoxScore{
		Completed: true,
		HasStats:  true,
		Players: []espn.PlayerBoxStats{
			nflverseRowToBoxStats(row),
		},
	}
	return grader.GradeLeg(leg, gameScore, &adapted)
}

func (g *MarketGrader) findNFLversePlayer(home, away string, leg grader.LegInput) (db.NFLversePlayerWeek, bool) {
	inGame := func(team string) bool {
		t := strings.ToUpper(strings.TrimSpace(team))
		return (home != "" && t == home) || (away != "" && t == away)
	}

	if playerID := strings.TrimSpace(leg.PlayerID); playerID != "" {
		for _, r := range g.nflverseByID[playerID] {
			if inGame(r.Team) {
				return r, true
			}
		}
	}

	normName := db.NormalizePlayerName(leg.PlayerName)
	if normName == "" {
		return db.NFLversePlayerWeek{}, false
	}

	team := strings.TrimSpace(leg.Team)
	if team == "" {
		_, team = db.StripPlayerTag(leg.PlayerName)
	}
	teamUpper := strings.ToUpper(strings.TrimSpace(team))

	for _, r := range g.nflverseByName[normName] {
		if !inGame(r.Team) {
			continue
		}
		if teamUpper != "" && !strings.EqualFold(strings.TrimSpace(r.Team), teamUpper) {
			continue
		}
		return r, true
	}
	return db.NFLversePlayerWeek{}, false
}

func nflverseRowToBoxStats(r db.NFLversePlayerWeek) espn.PlayerBoxStats {
	return espn.PlayerBoxStats{
		PlayerID:    r.ESPNID,
		DisplayName: r.PlayerName,
		TeamAbbrev:  r.Team,
		Passing: espn.PassingStats{
			PassingYards: r.PassYds,
			PassingTDs:   r.PassTDs,
		},
		Rushing: espn.RushingStats{
			RushingAttempts: r.Carries,
			RushingYards:    r.RushYds,
			RushingTDs:      r.RushTDs,
		},
		Receiving: espn.ReceivingStats{
			Receptions:     r.Receptions,
			ReceivingYards: r.RecYds,
			ReceivingTDs:   r.RecTDs,
		},
		Kicking: espn.KickingStats{
			TotalKickingPoints: r.KickerPts,
		},
	}
}
