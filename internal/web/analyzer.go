package web

import (
	"context"
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

// analyzerView maps the slip's legs and the week's games into analyzer inputs,
// then shapes the warnings for the banner. The games map is keyed by the same
// ID the legs carry so no leg is dropped. nfelo rows are only present when the
// caller has already checked that they are fresh.
func analyzerView(
	legs []db.Leg,
	games []db.Game,
	names map[int64]string,
	nfelo map[string]db.NfeloGame,
) AnalyzerView {
	gamesByID := make(map[string]analyzer.Game, len(games))
	for _, game := range games {
		projection, ok := nfelo[game.ID]
		if !ok {
			gamesByID[game.ID] = analyzerGame(game, nil)
			continue
		}
		gamesByID[game.ID] = analyzerGame(game, &projection)
	}

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
