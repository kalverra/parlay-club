package refresh

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"time"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/archive"
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
			return scoreboardStale(state.FetchedAt, now, games, live), nil
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

// syncScoreboard writes the week and each game, and records the fetch. When all
// games on ESPN's default scoreboard are final (e.g., Tuesday before ESPN's
// weekly rollover), it also fetches the next week's slate so upcoming weeks are
// available for slip creation. A rerun of the same slate changes no rows.
func (s *Service) syncScoreboard(ctx context.Context, now time.Time) (bool, error) {
	if s.scoreboard == nil {
		return false, nil
	}
	board, err := s.scoreboard.Scoreboard(ctx, espn.ScoreboardParams{})
	if err != nil {
		return false, fmt.Errorf("scoreboard: %w", err)
	}

	boards := []espn.Scoreboard{board}
	if allBoardFinal(board.Games) {
		next, err := s.nextScoreboard(ctx, board)
		if err != nil {
			s.logger.Warn().Err(err).Int("week", board.Week.Number+1).Msg("fetch next scoreboard")
		} else if len(next.Games) > 0 && !sameScoreboardWeek(next, board) {
			boards = append(boards, next)
		}
	}

	changed := false
	newlyFinal := false
	var changedWeeks []int64
	err = s.store.Tx(ctx, func(tx *db.Store) error {
		for _, b := range boards {
			weekChanged, weekFinal, weekID, err := upsertScoreboard(ctx, tx, b, now)
			if err != nil {
				return err
			}
			if weekChanged {
				changed = true
				changedWeeks = append(changedWeeks, weekID)
			}
			if weekFinal {
				newlyFinal = true
			}
		}

		return tx.RecordSync(ctx, ScoreboardSource, now, nil)
	})
	if err != nil {
		return changed, err
	}
	for _, weekID := range changedWeeks {
		s.publishWeek(ctx, weekID)
	}
	if newlyFinal {
		s.notifyMarketDataChange()
	}
	return changed, nil
}

// nextScoreboard fetches the week following board, falling back from the end of
// the regular season (season type 2) to Wild Card weekend (season type 3, week 1).
func (s *Service) nextScoreboard(ctx context.Context, board espn.Scoreboard) (espn.Scoreboard, error) {
	next, err := s.scoreboard.Scoreboard(ctx, espn.ScoreboardParams{
		Season:     board.Season.Year,
		SeasonType: board.Season.Type,
		Week:       board.Week.Number + 1,
	})
	if err != nil {
		return espn.Scoreboard{}, fmt.Errorf("next scoreboard: %w", err)
	}
	if len(next.Games) == 0 && board.Season.Type == 2 {
		post, err := s.scoreboard.Scoreboard(ctx, espn.ScoreboardParams{
			Season:     board.Season.Year,
			SeasonType: 3,
			Week:       1,
		})
		if err != nil {
			return espn.Scoreboard{}, fmt.Errorf("postseason scoreboard: %w", err)
		}
		return post, nil
	}
	return next, nil
}

func upsertScoreboard(
	ctx context.Context,
	tx *db.Store,
	board espn.Scoreboard,
	now time.Time,
) (changed, newlyFinal bool, weekID int64, err error) {
	week, err := tx.UpsertWeek(ctx, board.Season.Year, board.Season.Type, board.Week.Number)
	if err != nil {
		return false, false, 0, err
	}
	existing, err := tx.GamesForWeek(ctx, week.ID)
	if err != nil {
		return false, false, 0, err
	}
	before := make(map[string]db.Game, len(existing))
	for _, game := range existing {
		before[game.ID] = game
	}

	for _, game := range board.Games {
		stored, err := tx.UpsertGame(ctx, slateGame(game, week.ID, now))
		if err != nil {
			return false, false, 0, err
		}
		old, found := before[game.ID]
		if !found || !reflect.DeepEqual(old, stored) {
			changed = true
		}
		if stored.Status == db.GameFinal && (!found || old.Status != db.GameFinal) {
			newlyFinal = true
		}
	}
	return changed, newlyFinal, week.ID, nil
}

func allBoardFinal(games []espn.Game) bool {
	if len(games) == 0 {
		return false
	}
	for _, game := range games {
		if game.Status != espn.Final {
			return false
		}
	}
	return true
}

func sameScoreboardWeek(a, b espn.Scoreboard) bool {
	return a.Season.Year == b.Season.Year &&
		a.Season.Type == b.Season.Type &&
		a.Week.Number == b.Week.Number
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
		HomeLogo:   db.OptionalString(game.Home.Logo),
		AwayLogo:   db.OptionalString(game.Away.Logo),
		Kickoff:    game.Kickoff,
		Status:     db.GameStatus(game.Status),
		Period:     game.Period,
		Clock:      game.Clock,
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
	if s.forecast == nil || game.Indoor || game.VenueID == nil {
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

// oddsFetch is one provider's contribution to a game-line sync. matches and
// rows are already normalized to ESPN game IDs and team abbreviations, so the
// two providers never need to agree on event IDs.
type oddsFetch struct {
	matches     map[string]string
	rows        []db.Odds
	remaining   *int
	linesBought bool
	disabled    bool
	err         error
}

// fetchOdds runs one provider's free event list and, when the balance permits,
// its paid lines call. record stores each response's reported balance. A
// missing client and an empty API key are both "provider disabled", not a
// failure.
func (s *Service) fetchOdds(
	ctx context.Context,
	client LinesClient,
	name string,
	floor int,
	games []db.Game,
	byID map[string]db.Game,
	record func(odds.Credits) error,
	now time.Time,
) oddsFetch {
	if client == nil {
		return oddsFetch{disabled: true}
	}

	events, credits, err := client.Events(ctx)
	if errors.Is(err, odds.ErrNoAPIKey) {
		return oddsFetch{disabled: true}
	}
	if err != nil {
		return oddsFetch{err: fmt.Errorf("%s events: %w", name, err)}
	}

	remaining := credits.Remaining
	fetch := oddsFetch{remaining: &remaining}
	fetch.matches, _ = odds.MatchEvents(events, matchGames(games))
	if err := record(credits); err != nil {
		fetch.err = err
		return fetch
	}

	if credits.Remaining < floor {
		s.logger.Warn().
			Str("provider", name).
			Int("remaining", credits.Remaining).
			Int("floor", floor).
			Msg("odds lines paused below credit floor")
		return fetch
	}

	lines, lineCredits, err := client.Lines(ctx)
	if err != nil {
		fetch.err = fmt.Errorf("%s lines: %w", name, err)
		return fetch
	}
	if err := record(lineCredits); err != nil {
		fetch.err = err
		return fetch
	}

	lineRemaining := lineCredits.Remaining
	fetch.remaining = &lineRemaining
	fetch.linesBought = true
	fetch.rows, err = rowsForLines(lines, fetch.matches, byID, now)
	if err != nil {
		fetch.err = fmt.Errorf("%s: %w", name, err)
	}
	return fetch
}

// rowsForLines maps one provider's lines onto the week's games. Lines for
// other weeks are dropped.
func rowsForLines(
	lines []odds.Line,
	matches map[string]string,
	byID map[string]db.Game,
	now time.Time,
) ([]db.Odds, error) {
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
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// syncOdds matches Parlay API's free event list to the week's games and buys
// lines when the balance permits. The Odds API is only called as a fallback
// when Parlay API is unconfigured, fails, sits below its floor, or returns
// zero rows.
func (s *Service) syncOdds(ctx context.Context, games []db.Game, now time.Time) (bool, error) {
	if len(games) == 0 || allKickedOff(games, now) {
		return false, nil
	}
	start := time.Now()

	weekID := games[0].WeekID
	byID := make(map[string]db.Game, len(games))
	for _, game := range games {
		byID[game.ID] = game
	}

	primary := s.fetchOdds(
		ctx,
		s.lines,
		"odds",
		OddsLinesFloor,
		games,
		byID,
		func(credits odds.Credits) error {
			return s.store.SetSyncCredits(ctx, OddsLinesSource, credits.Remaining)
		},
		now,
	)

	fallback := oddsFetch{disabled: true}
	needsFallback := primary.disabled || primary.err != nil || !primary.linesBought || len(primary.rows) == 0
	if s.fallbackLines != nil && needsFallback {
		fallbackCtx := archive.WithKey(ctx, TheOddsAPISource)
		fallback = s.fetchOdds(
			fallbackCtx,
			s.fallbackLines,
			"fallback odds",
			TheOddsAPIFallbackFloor,
			games,
			byID,
			func(credits odds.Credits) error {
				remaining := credits.Remaining
				return s.store.RecordSync(fallbackCtx, TheOddsAPISource, now, &remaining)
			},
			now,
		)
	}

	configured, failures := 0, make([]error, 0, 2)
	for _, fetch := range []oddsFetch{primary, fallback} {
		if fetch.disabled {
			continue
		}
		configured++
		if fetch.err != nil {
			failures = append(failures, fetch.err)
		}
	}
	if configured == 0 {
		return false, odds.ErrNoAPIKey
	}
	if len(failures) == configured {
		return false, errors.Join(failures...)
	}
	for _, failure := range failures {
		s.logger.Warn().
			Err(failure).
			Msg("odds provider unavailable; committing the other provider's lines")
	}

	matches := make(map[string]string, len(primary.matches)+len(fallback.matches))
	maps.Copy(matches, primary.matches)
	for gameID, eventID := range fallback.matches {
		if _, ok := primary.matches[gameID]; ok {
			continue
		}
		game, ok := byID[gameID]
		if !ok || game.OddsEventID != nil {
			continue // keep Parlay API's event ID through a transient outage
		}
		matches[gameID] = eventID
	}

	rows := mergeOdds(primary.rows, fallback.rows)
	changed, err := s.publishOdds(ctx, weekID, matches, rows, primary.remaining, now, fallback.linesBought)
	s.logger.Info().
		Float64("duration_ms", durationMS(time.Since(start))).
		Int64("week_id", weekID).
		Int("rows", len(rows)).
		Bool("changed", changed).
		Err(err).
		Msg("refresh sync odds")
	return changed, err
}

// oddsLinesEnrichSource enriches the week's existing game lines with The Odds
// API deep links when a locked slip's game-line leg lacks a DraftKings link or
// carries a non-42 FanDuel marketId prefix.
func (s *Service) oddsLinesEnrichSource(weekID int64, games []db.Game, now time.Time) source {
	return source{
		key: OddsLinesEnrichSource,
		due: func(ctx context.Context, state db.SyncState) (bool, error) {
			if s.fallbackLines == nil || len(games) == 0 || allKickedOff(games, now) {
				return false, nil
			}
			if state.FetchedAt.IsZero() {
				return true, nil
			}
			linesState, err := s.store.SyncState(ctx, OddsLinesSource)
			if err != nil && !errors.Is(err, db.ErrSyncStateNotFound) {
				return false, err
			}
			if err == nil && state.FetchedAt.Before(linesState.FetchedAt) {
				return true, nil
			}
			return oddsPickLinesStale(state.FetchedAt, now, games), nil
		},
		run: func(ctx context.Context) (bool, error) {
			return s.enrichWeekLines(ctx, weekID, games, now)
		},
	}
}

// enrichWeekLines fetches game lines from The Odds API only (never Parlay API),
// merges them into the week's existing current game-line rows at a unified
// FetchedAt, and records OddsLinesEnrichSource.
func (s *Service) enrichWeekLines(ctx context.Context, weekID int64, games []db.Game, now time.Time) (bool, error) {
	if s.fallbackLines == nil || len(games) == 0 || allKickedOff(games, now) {
		return false, nil
	}
	start := time.Now()

	byID := make(map[string]db.Game, len(games))
	for _, game := range games {
		byID[game.ID] = game
	}

	fallbackCtx := archive.WithKey(ctx, TheOddsAPISource)
	fallback := s.fetchOdds(
		fallbackCtx,
		s.fallbackLines,
		"fallback odds",
		TheOddsAPIEnrichFloor,
		games,
		byID,
		func(credits odds.Credits) error {
			remaining := credits.Remaining
			return s.store.RecordSync(fallbackCtx, TheOddsAPISource, now, &remaining)
		},
		now,
	)
	if fallback.disabled {
		return false, nil
	}
	if fallback.err != nil {
		return false, fallback.err
	}
	if !fallback.linesBought {
		return false, nil
	}

	changed, mergedCount, err := s.commitWeekLinesEnrich(ctx, weekID, byID, fallback.matches, fallback.rows, now)
	if err != nil {
		return changed, err
	}
	s.logger.Info().
		Float64("duration_ms", durationMS(time.Since(start))).
		Int64("week_id", weekID).
		Int("rows", mergedCount).
		Bool("changed", changed).
		Msg("refresh enrich week lines")
	return changed, nil
}

// enrichWeekLinesForMarkets fetches only the requested game-line markets from
// The Odds API for draftkings and fanduel using pre-matched event IDs, then
// merges them into the week's existing current game-line rows.
func (s *Service) enrichWeekLinesForMarkets(
	ctx context.Context,
	weekID int64,
	games []db.Game,
	matches map[string]string,
	markets []string,
	now time.Time,
) (bool, error) {
	if s.fallbackLines == nil || len(games) == 0 || len(markets) == 0 || allKickedOff(games, now) {
		return false, nil
	}
	start := time.Now()

	byID := make(map[string]db.Game, len(games))
	for _, game := range games {
		byID[game.ID] = game
	}

	fallbackCtx := archive.WithKey(ctx, TheOddsAPISource)
	var (
		lines       []odds.Line
		lineCredits odds.Credits
		err         error
	)
	if scoped, ok := s.fallbackLines.(marketLinesClient); ok {
		lines, lineCredits, err = scoped.LinesForMarkets(fallbackCtx, markets, enrichBookmakers)
	} else {
		lines, lineCredits, err = s.fallbackLines.Lines(fallbackCtx)
	}
	if err != nil {
		return false, fmt.Errorf("fallback odds lines: %w", err)
	}
	remaining := lineCredits.Remaining
	if err := s.store.RecordSync(fallbackCtx, TheOddsAPISource, now, &remaining); err != nil {
		return false, err
	}

	rows, err := rowsForLines(lines, matches, byID, now)
	if err != nil {
		return false, fmt.Errorf("fallback odds: %w", err)
	}

	changed, mergedCount, err := s.commitWeekLinesEnrich(ctx, weekID, byID, matches, rows, now)
	if err != nil {
		return changed, err
	}
	s.logger.Info().
		Float64("duration_ms", durationMS(time.Since(start))).
		Int64("week_id", weekID).
		Strs("markets", markets).
		Int("rows", mergedCount).
		Bool("changed", changed).
		Msg("refresh enrich week lines")
	return changed, nil
}

func (s *Service) commitWeekLinesEnrich(
	ctx context.Context,
	weekID int64,
	byID map[string]db.Game,
	matches map[string]string,
	rows []db.Odds,
	now time.Time,
) (bool, int, error) {
	existingOdds, err := s.store.OddsForWeek(ctx, weekID)
	if err != nil {
		return false, 0, fmt.Errorf("load existing lines for week %d: %w", weekID, err)
	}
	var existingLines []db.Odds
	for _, row := range existingOdds {
		if !db.IsPropRow(row) {
			existingLines = append(existingLines, row)
		}
	}

	merged := mergeOdds(existingLines, rows)
	for i := range merged {
		merged[i].FetchedAt = now
	}

	changed := false
	err = s.store.Tx(ctx, func(tx *db.Store) error {
		for gameID, eventID := range matches {
			game, ok := byID[gameID]
			if !ok || game.OddsEventID != nil {
				continue
			}
			matchChanged, err := tx.SetGameOddsEvent(ctx, gameID, eventID)
			if err != nil {
				return err
			}
			changed = changed || matchChanged
		}
		if len(merged) > 0 {
			rowsChanged, err := tx.UpsertOdds(ctx, merged)
			if err != nil {
				return err
			}
			changed = changed || rowsChanged
		}
		return tx.RecordSync(ctx, OddsLinesEnrichSource, now, nil)
	})
	if err != nil {
		return changed, len(merged), err
	}
	if changed {
		s.publishWeek(ctx, weekID)
	}
	return changed, len(merged), nil
}

// oddsKey identifies one book's price for one selection across providers.
type oddsKey struct {
	gameID     string
	book       string
	marketType string
	team       string
	playerName string
	side       string
}

// oddsRowKey reads the selection key of a stored odds row.
func oddsRowKey(row db.Odds) oddsKey {
	return oddsKey{
		gameID:     row.GameID,
		book:       row.Book,
		marketType: row.MarketType,
		team:       row.Team,
		playerName: db.NormalizePlayerName(row.PlayerName),
		side:       row.Side,
	}
}

// gameBook identifies one book's board for one game.
type gameBook struct {
	gameID string
	book   string
}

// mergeOdds merges a secondary provider's rows into the primary provider's
// rows. A primary row with no link (or a FanDuel row with a non-42 marketId
// prefix when secondary carries a preferred 42.* link) adopts the secondary
// row's link and IDs; if the line differs, it also adopts the secondary line and
// price so the stored line matches the deep link. A linked primary row keeps
// its line, price, and link. Secondary-only selections are appended.
func mergeOdds(primary, secondary []db.Odds) []db.Odds {
	if len(primary) == 0 {
		return append([]db.Odds(nil), secondary...)
	}
	if len(secondary) == 0 {
		return append([]db.Odds(nil), primary...)
	}

	rows := append([]db.Odds(nil), primary...)
	index := make(map[oddsKey]int, len(rows))
	for i, row := range rows {
		index[oddsRowKey(row)] = i
	}

	// A book's event ID belongs to the game, not one selection, so backfill it
	// on every primary row for that game and book.
	eventSids := make(map[gameBook]string, len(secondary))
	for _, row := range secondary {
		if row.EventSid != nil && *row.EventSid != "" {
			eventSids[gameBook{gameID: row.GameID, book: row.Book}] = *row.EventSid
		}
	}
	for i := range rows {
		if rows[i].EventSid != nil {
			continue
		}
		if eventSid, ok := eventSids[gameBook{gameID: rows[i].GameID, book: rows[i].Book}]; ok {
			rows[i].EventSid = &eventSid
		}
	}

	for _, sec := range secondary {
		i, ok := index[oddsRowKey(sec)]
		if !ok {
			rows = append(rows, sec)
			index[oddsRowKey(sec)] = len(rows) - 1
			continue
		}

		prim := &rows[i]
		primLink, secLink := "", ""
		if prim.Link != nil {
			primLink = *prim.Link
		}
		if sec.Link != nil {
			secLink = *sec.Link
		}
		switch {
		case (prim.Link == nil && sec.Link != nil) || odds.PreferFanDuelLink(prim.Book, primLink, secLink):
			prim.Link = sec.Link
			prim.Sid = sec.Sid
			if sec.EventSid != nil {
				prim.EventSid = sec.EventSid
			}
			if !db.SameLine(prim.Line, sec.Line) {
				prim.Line = sec.Line
				prim.Price = sec.Price
			}
		case prim.Link != nil:
			if prim.Sid == nil && db.SameLine(prim.Line, sec.Line) {
				prim.Sid = sec.Sid
			}
			if prim.EventSid == nil {
				prim.EventSid = sec.EventSid
			}
		}
	}
	return rows
}

// publishOdds writes the odds sync's rows and signals the week when anything
// changed. Below-floor calls pass no rows but still store the free matches.
func (s *Service) publishOdds(
	ctx context.Context,
	weekID int64,
	matches map[string]string,
	rows []db.Odds,
	remaining *int,
	now time.Time,
	enriched bool,
) (bool, error) {
	changed, err := s.writeOdds(ctx, matches, rows, remaining, now, enriched)
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
// still stored, but no lines were bought. When enriched is true, it also
// records OddsLinesEnrichSource in the same transaction.
func (s *Service) writeOdds(
	ctx context.Context,
	matches map[string]string,
	rows []db.Odds,
	remaining *int,
	now time.Time,
	enriched bool,
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

		if err := tx.RecordSync(ctx, OddsLinesSource, now, remaining); err != nil {
			return err
		}
		if enriched {
			return tx.RecordSync(ctx, OddsLinesEnrichSource, now, nil)
		}
		return nil
	})
	if err != nil {
		return changed, err
	}
	return changed, nil
}

// oddsRow maps one line onto an odds row and converts the Parlay API display
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
		EventSid:   db.OptionalString(line.EventSid),
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
