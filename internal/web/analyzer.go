package web

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/db"
)

// nfeloSource is the sync_state row that records the last nfelo fetch.
const nfeloSource = "nfelo:projections"

// nfeloFreshFor is how long a successful nfelo fetch may shape a warning. The
// file changes about every two days, and a stale model is worse than none.
const nfeloFreshFor = 72 * time.Hour

// AnalyzerView is the sharp-friend banner under the slip's legs.
type AnalyzerView struct {
	Clean    bool
	Warnings []AnalyzerWarningView
}

// AnalyzerWarningView is one banner card.
type AnalyzerWarningView struct {
	Rule     string
	Severity string
	Class    string
	Message  string
	Legs     []AnalyzerLegView
}

// AnalyzerLegView is one leg a warning names, with the person who owns it.
type AnalyzerLegView struct {
	Owner     string
	Selection string
}

// analyzerGameMap maps the week's games into analyzer inputs. The map is keyed
// by the same ID the legs carry so no leg is dropped. nfelo rows are only
// present when the caller has already checked that they are fresh.
func analyzerGameMap(games []db.Game, nfelo map[string]db.NfeloGame) map[string]analyzer.Game {
	gamesByID := make(map[string]analyzer.Game, len(games))
	for _, game := range games {
		projection, ok := nfelo[game.ID]
		if !ok {
			gamesByID[game.ID] = analyzerGame(game, nil)
			continue
		}
		gamesByID[game.ID] = analyzerGame(game, &projection)
	}
	return gamesByID
}

// analyzerView shapes the analyzer's warnings for the banner.
func analyzerView(
	legs []db.Leg,
	gamesByID map[string]analyzer.Game,
	names map[int64]string,
) AnalyzerView {
	input := make([]analyzer.Leg, 0, len(legs))
	legsByID := make(map[int64]db.Leg, len(legs))
	for _, leg := range legs {
		legsByID[leg.ID] = leg
		input = append(input, analyzerLeg(leg, names))
	}

	warnings := analyzer.AnalyzeSlip(input, gamesByID, nil)
	view := AnalyzerView{Clean: len(warnings) == 0}
	for _, warning := range warnings {
		label, class := analyzerSeverity(warning.Severity)
		card := AnalyzerWarningView{
			Rule:     warning.Rule,
			Severity: label,
			Class:    class,
			Message:  warning.Message,
		}
		for _, id := range warning.LegIDs {
			leg, ok := legsByID[id]
			if !ok {
				continue
			}
			card.Legs = append(card.Legs, AnalyzerLegView{
				Owner:     names[leg.UserID],
				Selection: leg.Selection,
			})
		}
		view.Warnings = append(view.Warnings, card)
	}
	return view
}

// legModel is the nfelo chip beside a leg's odds. A spread shows the model's
// number from the leg team's side; a moneyline shows the model's win
// probability. Anything else, or a game without model data, has no chip.
func legModel(leg db.Leg, games map[string]analyzer.Game) string {
	if leg.Team == nil || leg.GameID == nil {
		return ""
	}
	game, ok := games[*leg.GameID]
	if !ok || game.Nfelo == nil {
		return ""
	}
	switch leg.MarketType {
	case "SPREAD":
		spread, ok := game.ModelSpread(*leg.Team)
		if !ok {
			return ""
		}
		return "nfelo " + formatModelSpread(spread)
	case "ML":
		probability, ok := game.ModelWinProb(*leg.Team)
		if !ok {
			return ""
		}
		return fmt.Sprintf("nfelo %d%%", int(math.Round(probability*100)))
	default:
		return ""
	}
}

// formatModelSpread prints a model spread with one decimal and a plus on a
// positive number.
func formatModelSpread(spread float64) string {
	text := strconv.FormatFloat(spread, 'f', 1, 64)
	if spread > 0 {
		return "+" + text
	}
	return text
}

// analyzerGame carries the slice of a stored game a rule reads. nfelo data is
// included only for a scheduled game, because a live or final game's model no
// longer describes the game in front of the slip.
func analyzerGame(game db.Game, projection *db.NfeloGame) analyzer.Game {
	converted := analyzer.Game{
		ID:     game.ID,
		Home:   game.HomeTeam,
		Away:   game.AwayTeam,
		Indoor: game.Indoor,
	}
	if projection != nil && game.Status == db.GameScheduled {
		converted.Nfelo = &analyzer.Nfelo{
			HomeSpread:  projection.HomeSpread,
			HomeWinProb: projection.HomeWinProb,
			HomeQB:      projection.HomeQB,
			AwayQB:      projection.AwayQB,
			HomeQBAdj:   projection.HomeQBAdj,
			AwayQBAdj:   projection.AwayQBAdj,
		}
	}
	if game.WindMPH != nil {
		weather := analyzer.Weather{WindMPH: *game.WindMPH}
		if game.WindGustMPH != nil {
			weather.GustMPH = *game.WindGustMPH
		}
		if game.PrecipIN != nil {
			weather.PrecipIn = *game.PrecipIN
		}
		if game.TempF != nil {
			weather.TempF = *game.TempF
		}
		converted.Weather = &weather
	}
	return converted
}

// analyzerLeg carries the pick fields a rule reads.
func analyzerLeg(leg db.Leg, names map[int64]string) analyzer.Leg {
	converted := analyzer.Leg{
		ID:     leg.ID,
		Owner:  names[leg.UserID],
		Market: leg.MarketType,
		Line:   leg.Line,
		Odds:   leg.Odds,
	}
	if leg.GameID != nil {
		converted.GameID = *leg.GameID
	}
	if leg.Team != nil {
		converted.Team = *leg.Team
	}
	if leg.PlayerID != nil {
		converted.PlayerID = *leg.PlayerID
	}
	if leg.PlayerName != nil {
		converted.PlayerName = *leg.PlayerName
	}
	if leg.Side != nil {
		converted.Side = *leg.Side
	}
	return converted
}

// nfeloForGames reads the model rows the analyzer may use. It returns none
// when nfelo has never synced or its last good fetch is older than 72 hours, so
// stale data cannot shape a sentence. A read error on one game just leaves that
// game without a model.
func (h *slipHandlers) nfeloForGames(
	ctx context.Context,
	games []db.Game,
) map[string]db.NfeloGame {
	state, err := h.deps.Store.SyncState(ctx, nfeloSource)
	if err != nil || time.Since(state.FetchedAt) > nfeloFreshFor {
		return nil
	}
	rows := make(map[string]db.NfeloGame, len(games))
	for _, game := range games {
		projection, ok, err := h.deps.Store.NfeloForGame(ctx, game.ID)
		if err != nil || !ok {
			continue
		}
		rows[game.ID] = projection
	}
	return rows
}

// analyzerSeverity names the card's heading and the classes that color it.
func analyzerSeverity(severity analyzer.Severity) (string, string) {
	switch severity {
	case analyzer.Danger:
		return "Danger", "border-rose-800 bg-rose-950 text-rose-200"
	case analyzer.Warn:
		return "Warn", "border-amber-700 bg-amber-950 text-amber-300"
	default:
		return "Info", "border-sky-700 bg-sky-950 text-sky-300"
	}
}
