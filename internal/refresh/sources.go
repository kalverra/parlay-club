package refresh

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/espn"
	"github.com/kalverra/parlay-club/internal/ingest/odds"
	"github.com/kalverra/parlay-club/internal/ingest/weather"
)

// scoreboardSource syncs the current week's slate.
func (s *Service) scoreboardSource(games []db.Game, now time.Time) source {
	live := anyLive(games)
	return source{
		key: ScoreboardSource,
		due: func(_ context.Context, state db.SyncState) (bool, error) {
			return scoreboardStale(state.FetchedAt, now, live), nil
		},
		run: func(ctx context.Context) (bool, error) {
			return s.syncScoreboard(ctx, now)
		},
	}
}

// weatherSource syncs one game's kickoff-window forecast.
func (s *Service) weatherSource(game db.Game, now time.Time) source {
	return source{
		key: WeatherSource(game.ID),
		due: func(_ context.Context, state db.SyncState) (bool, error) {
			return weatherStale(state.FetchedAt, now, game), nil
		},
		run: func(ctx context.Context) (bool, error) {
			return s.syncWeather(ctx, game, now)
		},
	}
}

// oddsSource syncs the week's game lines. The sync checks the credit floor
// after the free event list reports a fresh balance.
func (s *Service) oddsSource(games []db.Game, now time.Time) source {
	return source{
		key: OddsLinesSource,
		due: func(_ context.Context, state db.SyncState) (bool, error) {
			return oddsStale(state.FetchedAt, now, games), nil
		},
		run: func(ctx context.Context) (bool, error) {
			return s.syncOdds(ctx, games, now)
		},
	}
}

// syncScoreboard writes the week and each game, and records the fetch. A rerun
// of the same slate changes no rows.
func (s *Service) syncScoreboard(ctx context.Context, now time.Time) (bool, error) {
	board, err := s.scoreboard.Scoreboard(ctx, espn.ScoreboardParams{})
	if err != nil {
		return false, fmt.Errorf("scoreboard: %w", err)
	}

	changed := false
	var weekID int64
	err = s.store.Tx(ctx, func(tx *db.Store) error {
		week, err := tx.UpsertWeek(ctx, board.Season.Year, board.Season.Type, board.Week.Number)
		if err != nil {
			return err
		}
		weekID = week.ID
		existing, err := tx.GamesForWeek(ctx, week.ID)
		if err != nil {
			return err
		}
		before := make(map[string]db.Game, len(existing))
		for _, game := range existing {
			before[game.ID] = game
		}

		for _, game := range board.Games {
			stored, err := tx.UpsertGame(ctx, slateGame(game, week.ID, now))
			if err != nil {
				return err
			}
			old, found := before[game.ID]
			if !found || !reflect.DeepEqual(old, stored) {
				changed = true
			}
		}

		return tx.RecordSync(ctx, ScoreboardSource, now, nil)
	})
	if err != nil {
		return changed, err
	}
	if changed {
		s.publishWeek(ctx, weekID)
	}
	return changed, nil
}

// slateGame maps one ESPN game to a games row.
func slateGame(game espn.Game, weekID int64, now time.Time) db.Game {
	return db.Game{
		ID:         game.ID,
		WeekID:     weekID,
		HomeTeam:   game.Home.Abbreviation,
		AwayTeam:   game.Away.Abbreviation,
		HomeName:   game.Home.Name,
		AwayName:   game.Away.Name,
		HomeESPNID: game.Home.ID,
		AwayESPNID: game.Away.ID,
		Kickoff:    game.Kickoff,
		Status:     db.GameStatus(game.Status),
		VenueID:    db.OptionalString(game.Venue.ID),
		VenueName:  db.OptionalString(game.Venue.Name),
		Indoor:     game.Venue.Indoor,
		HomeScore:  game.Home.Score,
		AwayScore:  game.Away.Score,
		UpdatedAt:  now,
	}
}

// syncWeather fetches one game's forecast and writes it.
func (s *Service) syncWeather(ctx context.Context, game db.Game, now time.Time) (bool, error) {
	if game.Indoor || game.VenueID == nil {
		return false, nil
	}
	if now.Before(game.Kickoff.Add(-weatherLookahead)) || !now.Before(game.Kickoff) {
		return false, nil
	}
	venue, ok := weather.VenueByID(*game.VenueID)
	if !ok {
		return false, nil
	}

	forecast, err := s.forecast.Forecast(ctx, venue, game.Kickoff)
	if err != nil {
		return false, fmt.Errorf("game %s forecast: %w", game.ID, err)
	}

	changed := false
	err = s.store.Tx(ctx, func(tx *db.Store) error {
		weatherChanged, err := tx.SetGameWeather(
			ctx,
			game.ID,
			forecast.TempF,
			forecast.WindMPH,
			forecast.WindGustMPH,
			forecast.PrecipIN,
			now,
		)
		if err != nil {
			return err
		}
		changed = weatherChanged
		return tx.RecordSync(ctx, WeatherSource(game.ID), now, nil)
	})
	if err != nil {
		return changed, err
	}
	if changed {
		s.publishWeek(ctx, game.WeekID)
	}
	return changed, nil
}

// syncOdds matches the free event list to the week's games, then buys one
// lines response for the whole week. It writes rows only for matched games,
// because the response also carries next week's events.
func (s *Service) syncOdds(ctx context.Context, games []db.Game, now time.Time) (bool, error) {
	if len(games) == 0 || allKickedOff(games, now) {
		return false, nil
	}

	weekID := games[0].WeekID
	events, credits, err := s.lines.Events(ctx)
	if err != nil {
		return false, fmt.Errorf("odds events: %w", err)
	}
	if err := s.store.SetSyncCredits(ctx, OddsLinesSource, credits.Remaining); err != nil {
		return false, err
	}

	matches, _ := odds.MatchEvents(events, matchGames(games))
	byID := make(map[string]db.Game, len(games))
	for _, game := range games {
		byID[game.ID] = game
	}

	if credits.Remaining < OddsLinesFloor {
		s.logger.Warn().
			Int("remaining", credits.Remaining).
			Int("floor", OddsLinesFloor).
			Msg("odds lines paused below credit floor")
		return s.publishOdds(ctx, weekID, matches, nil, credits.Remaining, now)
	}

	lines, lineCredits, err := s.lines.Lines(ctx)
	if err != nil {
		return false, fmt.Errorf("odds lines: %w", err)
	}
	if err := s.store.SetSyncCredits(ctx, OddsLinesSource, lineCredits.Remaining); err != nil {
		return false, err
	}

	eventGames := make(map[string]db.Game, len(matches))
	for gameID, eventID := range matches {
		eventGames[eventID] = byID[gameID]
	}

	rows := make([]db.Odds, 0, len(lines))
	for _, line := range lines {
		game, ok := eventGames[line.EventID]
		if !ok {
			continue // another week's event
		}
		row, err := oddsRow(game, line, now)
		if err != nil {
			return false, err
		}
		rows = append(rows, row)
	}

	return s.publishOdds(ctx, weekID, matches, rows, lineCredits.Remaining, now)
}

// publishOdds writes the odds sync's rows and signals the week when anything
// changed. Below-floor calls pass no rows but still store the free matches.
func (s *Service) publishOdds(
	ctx context.Context,
	weekID int64,
	matches map[string]string,
	rows []db.Odds,
	remaining int,
	now time.Time,
) (bool, error) {
	changed, err := s.writeOdds(ctx, matches, rows, remaining, now)
	if err != nil {
		return changed, err
	}
	if changed {
		s.publishWeek(ctx, weekID)
	}
	return changed, nil
}

// writeOdds records the event matches, the priced rows, and the fetch in one
// transaction. A nil rows slice is the below-floor case: matches are free and
// still stored, but no lines were bought.
func (s *Service) writeOdds(
	ctx context.Context,
	matches map[string]string,
	rows []db.Odds,
	remaining int,
	now time.Time,
) (bool, error) {
	changed := false
	err := s.store.Tx(ctx, func(tx *db.Store) error {
		for gameID, eventID := range matches {
			matchChanged, err := tx.SetGameOddsEvent(ctx, gameID, eventID)
			if err != nil {
				return err
			}
			changed = changed || matchChanged
		}

		rowsChanged, err := tx.UpsertOdds(ctx, rows)
		if err != nil {
			return err
		}
		changed = changed || rowsChanged

		return tx.RecordSync(ctx, OddsLinesSource, now, &remaining)
	})
	if err != nil {
		return changed, err
	}
	return changed, nil
}

// oddsRow maps one line onto an odds row and converts the Odds API display
// name to the game's ESPN abbreviation.
func oddsRow(game db.Game, line odds.Line, now time.Time) (db.Odds, error) {
	team := ""
	switch line.MarketType {
	case odds.MarketML, odds.MarketSpread:
		switch line.Team {
		case game.HomeName:
			team = game.HomeTeam
		case game.AwayName:
			team = game.AwayTeam
		default:
			return db.Odds{}, fmt.Errorf(
				"event %s team %q matches neither %q nor %q",
				line.EventID, line.Team, game.HomeName, game.AwayName,
			)
		}
	case odds.MarketTotal:
		if line.Team != "" {
			return db.Odds{}, fmt.Errorf("total event %s names team %q", line.EventID, line.Team)
		}
	default:
		return db.Odds{}, fmt.Errorf("event %s unknown market %q", line.EventID, line.MarketType)
	}

	return db.Odds{
		GameID:     game.ID,
		Book:       line.Book,
		MarketType: line.MarketType,
		Team:       team,
		PlayerName: "",
		Side:       line.Side,
		Line:       line.Line,
		Price:      line.Price,
		Link:       db.OptionalString(line.Link),
		Sid:        db.OptionalString(line.Sid),
		FetchedAt:  now,
	}, nil
}

// matchGames adapts stored games to the matcher's shape.
func matchGames(games []db.Game) []odds.MatchGame {
	matches := make([]odds.MatchGame, 0, len(games))
	for _, game := range games {
		matches = append(matches, odds.MatchGame{
			ID:       game.ID,
			HomeName: game.HomeName,
			AwayName: game.AwayName,
			Kickoff:  game.Kickoff,
		})
	}
	return matches
}
