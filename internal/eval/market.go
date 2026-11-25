package eval

import (
	"strings"
	"time"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/board"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
)

// BuildMarketCandidates deduplicates pre-kickoff archived odds for one FINAL
// game into one synthetic analyzer.Leg per (game, market, player/team, side)
// using board.Build median line and median price consensus.
func BuildMarketCandidates(
	game db.Game,
	gameOdds []db.Odds,
	preKickoffRoster []db.RosterPlayer,
) []analyzer.Leg {
	if game.Status != db.GameFinal || len(gameOdds) == 0 {
		return nil
	}

	preKickoffGame := game
	preKickoffGame.Status = db.GameScheduled
	rows := board.Build([]db.Game{preKickoffGame}, gameOdds, preKickoffRoster, "", nil, time.Time{})
	if len(rows) == 0 {
		return nil
	}

	candidates := make([]analyzer.Leg, 0, len(rows))
	for _, row := range rows {
		market := strings.ToUpper(strings.TrimSpace(row.Market))
		if !isSupportedMarket(market) {
			continue
		}
		if row.Odds == 0 || parlay.AmericanToDecimal(row.Odds) <= 1 {
			continue
		}
		if marketRequiresLine(market) && row.Line == nil {
			continue
		}
		candidates = append(candidates, analyzer.Leg{
			GameID:     row.GameID,
			Market:     market,
			Team:       row.Team,
			PlayerID:   row.PlayerID,
			PlayerName: row.PlayerName,
			Side:       row.Side,
			Line:       row.Line,
			Odds:       row.Odds,
		})
	}
	return candidates
}

func isSupportedMarket(market string) bool {
	switch market {
	case "SPREAD", "ML", "TOTAL", "TEAM_TOTAL":
		return true
	default:
		return db.IsPlayerMarket(market)
	}
}

func marketRequiresLine(market string) bool {
	return market != "ML" && market != "ANYTIME_TD"
}
