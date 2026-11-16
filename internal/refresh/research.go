package refresh

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/archive"
	"github.com/kalverra/parlay-club/internal/ingest/odds"
)

// rosterSource syncs one team's roster.
func (s *Service) rosterSource(teamAbbr, espnTeamID string, now time.Time) source {
	return source{
		key: RosterSource(espnTeamID),
		due: func(_ context.Context, state db.SyncState) (bool, error) {
			return rosterStale(state.FetchedAt, now), nil
		},
		run: func(ctx context.Context) (bool, error) {
			return s.syncRoster(ctx, teamAbbr, espnTeamID, now)
		},
	}
}

// syncRoster fetches and writes one team's roster.
func (s *Service) syncRoster(ctx context.Context, teamAbbr, espnTeamID string, now time.Time) (bool, error) {
	if s.espn == nil {
		return false, nil
	}
	players, err := s.espn.Roster(ctx, espnTeamID)
	if err != nil {
		return false, fmt.Errorf("espn roster %s: %w", espnTeamID, err)
	}

	dbPlayers := make([]db.RosterPlayer, 0, len(players))
	for _, p := range players {
		dbPlayers = append(dbPlayers, db.RosterPlayer{
			AthleteID:   p.ID,
			Team:        teamAbbr,
			ESPNTeamID:  espnTeamID,
			FullName:    p.FullName,
			Position:    p.Position,
			HeadshotURL: p.HeadshotURL,
			UpdatedAt:   now,
		})
	}

	err = s.store.Tx(ctx, func(tx *db.Store) error {
		if err := tx.UpsertRosterPlayers(ctx, dbPlayers); err != nil {
			return err
		}
		return tx.RecordSync(ctx, RosterSource(espnTeamID), now, nil)
	})
	if err != nil {
		return false, err
	}
	return len(dbPlayers) > 0, nil
}

// summarySource syncs one game's ESPN summary and injuries.
func (s *Service) summarySource(game db.Game, now time.Time) source {
	return source{
		key: SummarySource(game.ID),
		due: func(_ context.Context, state db.SyncState) (bool, error) {
			return summaryStale(state.FetchedAt, now, game), nil
		},
		run: func(ctx context.Context) (bool, error) {
			return s.syncSummary(ctx, game, now)
		},
	}
}

// syncSummary fetches one game's summary and writes injuries_json.
func (s *Service) syncSummary(ctx context.Context, game db.Game, now time.Time) (bool, error) {
	if s.espn == nil || game.Status == db.GameFinal {
		return false, nil
	}
	rawInjuries, err := s.espn.SummaryInjuries(ctx, game.ID)
	if err != nil {
		return false, fmt.Errorf("summary %s: %w", game.ID, err)
	}

	changed := false
	err = s.store.Tx(ctx, func(tx *db.Store) error {
		injuriesChanged, err := tx.SetGameInjuries(ctx, game.ID, rawInjuries)
		if err != nil {
			return err
		}
		changed = injuriesChanged
		return tx.RecordSync(ctx, SummarySource(game.ID), now, nil)
	})
	if err != nil {
		return changed, err
	}
	if changed {
		s.publishWeek(ctx, game.WeekID)
	}
	return changed, nil
}

// gameLogSource syncs one athlete's game log.
func (s *Service) gameLogSource(athleteID string, now time.Time) source {
	return source{
		key: GameLogSource(athleteID),
		due: func(_ context.Context, state db.SyncState) (bool, error) {
			return gameLogStale(state.FetchedAt, now), nil
		},
		run: func(ctx context.Context) (bool, error) {
			return s.syncGameLog(ctx, athleteID, now)
		},
	}
}

// syncGameLog fetches and writes one athlete's game log.
func (s *Service) syncGameLog(ctx context.Context, athleteID string, now time.Time) (bool, error) {
	if s.espn == nil {
		return false, nil
	}
	log, err := s.espn.GameLog(ctx, athleteID)
	if err != nil {
		return false, fmt.Errorf("espn gamelog %s: %w", athleteID, err)
	}

	statsBytes, err := json.Marshal(log.Markets)
	if err != nil {
		return false, fmt.Errorf("marshal gamelog stats %s: %w", athleteID, err)
	}

	dbLog := db.PlayerGameLog{
		AthleteID:   athleteID,
		GamesPlayed: log.GamesPlayed,
		StatsJSON:   string(statsBytes),
		UpdatedAt:   now,
	}

	err = s.store.Tx(ctx, func(tx *db.Store) error {
		if err := tx.UpsertPlayerGameLog(ctx, dbLog); err != nil {
			return err
		}
		return tx.RecordSync(ctx, GameLogSource(athleteID), now, nil)
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

// oddsPropsSource syncs one game's prop odds.
func (s *Service) oddsPropsSource(game db.Game, now time.Time) source {
	return source{
		key: OddsPropsSource(game.ID),
		due: func(_ context.Context, state db.SyncState) (bool, error) {
			return oddsPropsStale(state.FetchedAt, now, game), nil
		},
		run: func(ctx context.Context) (bool, error) {
			return s.syncOddsProps(ctx, game, now)
		},
	}
}

// propsFetch is one provider's prop fetch result.
type propsFetch struct {
	props    []odds.Prop
	bought   bool
	disabled bool
	err      error
}

// fetchParlayProps fetches bulk props from Parlay API if configured and balance permits.
func (s *Service) fetchParlayProps(ctx context.Context, game *db.Game) propsFetch {
	if s.props == nil {
		return propsFetch{disabled: true}
	}

	linesState, err := s.store.SyncState(ctx, OddsLinesSource)
	hasBalance := (err == nil && linesState.OddsCreditsRemaining != nil)
	eventID := ""
	if game.OddsEventID != nil {
		eventID = *game.OddsEventID
	}

	if eventID == "" || !hasBalance {
		events, credits, err := s.props.Events(ctx)
		switch {
		case errors.Is(err, odds.ErrNoAPIKey):
			return propsFetch{disabled: true}
		case err != nil:
			return propsFetch{err: fmt.Errorf("parlay api events: %w", err)}
		default:
			_ = s.store.SetSyncCredits(ctx, OddsLinesSource, credits.Remaining)
			linesState.OddsCreditsRemaining = &credits.Remaining
			matches, _ := odds.MatchEvents(events, matchGames([]db.Game{*game}))
			if mID, ok := matches[game.ID]; ok {
				_, _ = s.store.SetGameOddsEvent(ctx, game.ID, mID)
				eventID = mID
				game.OddsEventID = &mID
			}
		}
	}

	remaining := 0
	if linesState.OddsCreditsRemaining != nil {
		remaining = *linesState.OddsCreditsRemaining
	}
	switch {
	case remaining < OddsPropsFloor:
		s.logger.Warn().
			Int("remaining", remaining).
			Int("floor", OddsPropsFloor).
			Msg("odds props paused below credit floor")
		return propsFetch{}
	case eventID == "":
		s.logger.Warn().
			Str("game_id", game.ID).
			Msg("odds props skipped: no Parlay API event matched")
		return propsFetch{}
	default:
		props, lineCredits, err := s.props.Props(ctx, eventID)
		if err != nil {
			return propsFetch{err: fmt.Errorf("parlay api props: %w", err)}
		}
		_ = s.store.SetSyncCredits(ctx, OddsLinesSource, lineCredits.Remaining)
		return propsFetch{props: props, bought: true}
	}
}

// fetchFallbackProps fetches per-event props from The Odds API if configured and balance permits.
func (s *Service) fetchFallbackProps(ctx context.Context, game db.Game, floor int, now time.Time) propsFetch {
	if s.fallbackProps == nil {
		return propsFetch{disabled: true}
	}

	fallbackCtx := archive.WithKey(ctx, TheOddsAPISource)
	events, credits, err := s.fallbackProps.Events(fallbackCtx)
	switch {
	case errors.Is(err, odds.ErrNoAPIKey):
		return propsFetch{disabled: true}
	case err != nil:
		return propsFetch{err: fmt.Errorf("fallback events: %w", err)}
	default:
		_ = s.store.RecordSync(fallbackCtx, TheOddsAPISource, now, &credits.Remaining)
		matches, _ := odds.MatchEvents(events, matchGames([]db.Game{game}))
		toaEventID, hasToa := matches[game.ID]

		switch {
		case credits.Remaining < floor:
			s.logger.Warn().
				Int("remaining", credits.Remaining).
				Int("floor", floor).
				Msg("fallback props paused below credit floor")
			return propsFetch{}
		case !hasToa:
			s.logger.Warn().
				Str("game_id", game.ID).
				Msg("fallback props skipped: no fallback event matched")
			return propsFetch{}
		default:
			secProps, secCredits, err := s.fallbackProps.EventProps(fallbackCtx, toaEventID)
			if err != nil {
				return propsFetch{err: fmt.Errorf("fallback event props: %w", err)}
			}
			_ = s.store.RecordSync(fallbackCtx, TheOddsAPISource, now, &secCredits.Remaining)
			return propsFetch{props: secProps, bought: true}
		}
	}
}

// mapPropsToOdds maps prop outcome rows onto db.Odds records.
func (s *Service) mapPropsToOdds(ctx context.Context, game db.Game, props []odds.Prop, now time.Time) []db.Odds {
	rows := make([]db.Odds, 0, len(props))
	for _, prop := range props {
		row := db.Odds{
			GameID:     game.ID,
			Book:       prop.Book,
			MarketType: prop.MarketType,
			Side:       prop.Side,
			Line:       prop.Line,
			Price:      prop.Price,
			Link:       db.OptionalString(prop.Link),
			Sid:        db.OptionalString(prop.Sid),
			EventSid:   db.OptionalString(prop.EventSid),
			FetchedAt:  now,
		}
		if prop.MarketType == odds.MarketTeamTotal {
			switch prop.Team {
			case game.HomeName, game.HomeTeam:
				row.Team = game.HomeTeam
			case game.AwayName, game.AwayTeam:
				row.Team = game.AwayTeam
			default:
				s.logger.Warn().
					Str("game_id", game.ID).
					Str("team", prop.Team).
					Msg("team total subject matches neither home nor away team")
				continue
			}
		} else {
			row.PlayerName = prop.PlayerName
			_, team, err := s.store.ResolvePlayerMatch(ctx, prop.PlayerName, game.HomeTeam, game.AwayTeam)
			if err != nil {
				s.logger.Warn().Err(err).Str("player", prop.PlayerName).Msg("resolve player match")
			}
			row.Team = team
		}
		rows = append(rows, row)
	}
	return rows
}

// syncOddsProps fetches and writes props for one game.
func (s *Service) syncOddsProps(ctx context.Context, game db.Game, now time.Time) (bool, error) {
	if !now.Before(game.Kickoff) {
		return false, nil
	}
	start := time.Now()

	// 1. Sync rosters for both teams before matching props.
	if s.espn != nil {
		if game.HomeESPNID != "" && game.HomeTeam != "" {
			if err := s.refreshSync(ctx, s.rosterSource(game.HomeTeam, game.HomeESPNID, now), now); err != nil {
				s.logger.Warn().Err(err).Str("team", game.HomeTeam).Msg("sync home roster")
			}
		}
		if game.AwayESPNID != "" && game.AwayTeam != "" {
			if err := s.refreshSync(ctx, s.rosterSource(game.AwayTeam, game.AwayESPNID, now), now); err != nil {
				s.logger.Warn().Err(err).Str("team", game.AwayTeam).Msg("sync away roster")
			}
		}
	}

	// 2. Primary Parlay API props.
	primary := s.fetchParlayProps(ctx, &game)

	// 3. Secondary The Odds API enrichment or fallback.
	fallbackFloor := TheOddsAPIFallbackFloor
	if primary.bought && len(primary.props) > 0 {
		fallbackFloor = TheOddsAPIEnrichFloor
	}
	fallback := s.fetchFallbackProps(ctx, game, fallbackFloor, now)

	// 4. Combine props.
	var finalProps []odds.Prop
	switch {
	case primary.bought && len(primary.props) > 0:
		if fallback.bought {
			finalProps = odds.MergeProps(primary.props, fallback.props)
		} else {
			finalProps = primary.props
		}
	case fallback.bought:
		finalProps = fallback.props
	}

	// Check error conditions.
	configured := 0
	var failures []error
	for _, fetch := range []propsFetch{primary, fallback} {
		if fetch.disabled {
			continue
		}
		configured++
		if fetch.err != nil {
			failures = append(failures, fetch.err)
		}
	}
	if configured == 0 {
		if s.props == nil && s.fallbackProps == nil {
			return false, nil
		}
		return false, odds.ErrNoAPIKey
	}
	if len(failures) == configured {
		return false, errors.Join(failures...)
	}
	for _, failure := range failures {
		s.logger.Warn().Err(failure).Msg("props provider unavailable; using surviving provider's props")
	}

	// 5. Convert to db.Odds rows and write.
	rows := s.mapPropsToOdds(ctx, game, finalProps, now)
	changed, err := s.writeOddsProps(ctx, game, rows, now)
	s.logger.Info().
		Float64("duration_ms", durationMS(time.Since(start))).
		Str("game_id", game.ID).
		Int("rows", len(rows)).
		Bool("changed", changed).
		Err(err).
		Msg("refresh sync odds props")
	return changed, err
}

// writeOddsProps writes prop rows and records the game's props sync state.
func (s *Service) writeOddsProps(ctx context.Context, game db.Game, rows []db.Odds, now time.Time) (bool, error) {
	changed := false
	err := s.store.Tx(ctx, func(tx *db.Store) error {
		if len(rows) > 0 {
			rowsChanged, err := tx.UpsertOdds(ctx, rows)
			if err != nil {
				return err
			}
			changed = rowsChanged
		}
		return tx.RecordSync(ctx, OddsPropsSource(game.ID), now, nil)
	})
	if err != nil {
		return changed, err
	}
	if changed {
		s.publishWeek(ctx, game.WeekID)
	}
	return changed, nil
}

// prefillProps warms Parlay API bulk props for upcoming games using a bounded
// worker pool. It never calls The Odds API fallback EventProps and respects the
// 15-minute idempotency window.
func (s *Service) prefillProps(
	ctx context.Context,
	games []db.Game,
	now time.Time,
) (synced, skipped, failed int, errs []error) {
	workers := min(prefillConcurrency, len(games))
	jobs := make(chan db.Game, len(games))
	for _, game := range games {
		jobs <- game
	}
	close(jobs)

	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)
	for range workers {
		wg.Go(func() {
			for game := range jobs {
				if ctx.Err() != nil {
					return
				}
				bought := false
				src := source{
					key: OddsPropsSource(game.ID),
					due: func(_ context.Context, state db.SyncState) (bool, error) {
						return prefillStale(state.FetchedAt, now), nil
					},
					run: func(ctx context.Context) (bool, error) {
						primary := s.fetchParlayProps(ctx, &game)
						if primary.disabled || !primary.bought {
							return false, primary.err
						}
						rows := s.mapPropsToOdds(ctx, game, primary.props, now)
						changed, err := s.writeOddsProps(ctx, game, rows, now)
						if err != nil {
							return false, err
						}
						bought = true
						return changed, nil
					},
				}
				err := s.refreshWithTimeout(ctx, src, now)
				mu.Lock()
				switch {
				case err != nil:
					failed++
					errs = append(errs, err)
				case bought:
					synced++
				default:
					skipped++
				}
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	return synced, skipped, failed, errs
}
