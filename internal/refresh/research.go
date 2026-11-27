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
	"github.com/kalverra/parlay-club/internal/ingest/espn"
	"github.com/kalverra/parlay-club/internal/ingest/odds"
)

// rosterSource syncs one team's roster.
func (s *Service) rosterSource(teamAbbr, espnTeamID string, games []db.Game, now time.Time) source {
	return source{
		key: RosterSource(espnTeamID),
		due: func(_ context.Context, state db.SyncState) (bool, error) {
			return rosterStale(state.FetchedAt, now, games), nil
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
			AthleteID:    p.ID,
			Team:         teamAbbr,
			ESPNTeamID:   espnTeamID,
			FullName:     p.FullName,
			Position:     p.Position,
			HeadshotURL:  p.HeadshotURL,
			InjuryStatus: p.InjuryStatus,
			InjuryDate:   p.InjuryDate,
			UpdatedAt:    now,
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

// summarySource syncs one game's ESPN summary, injuries, and (when live) box score.
func (s *Service) summarySource(game db.Game, now time.Time, hasSlipLegs bool) source {
	return source{
		key: SummarySource(game.ID),
		due: func(_ context.Context, state db.SyncState) (bool, error) {
			return summaryStale(state.FetchedAt, now, game, hasSlipLegs), nil
		},
		run: func(ctx context.Context) (bool, error) {
			return s.syncSummary(ctx, game, now)
		},
	}
}

// syncSummary fetches one game's summary and writes injuries_json (and
// boxscore_json when the game is live).
func (s *Service) syncSummary(ctx context.Context, game db.Game, now time.Time) (bool, error) {
	if s.espn == nil || game.Status == db.GameFinal {
		return false, nil
	}

	var (
		rawInjuries  string
		boxscoreJSON *string
	)
	if sc, ok := s.espn.(summaryClient); ok {
		summary, err := sc.Summary(ctx, game.ID)
		if err != nil {
			return false, fmt.Errorf("summary %s: %w", game.ID, err)
		}
		rawInjuries = summary.InjuriesJSON
		if game.Status == db.GameLive {
			encoded, err := json.Marshal(summary.BoxScore)
			if err != nil {
				return false, fmt.Errorf("marshal boxscore %s: %w", game.ID, err)
			}
			encodedStr := string(encoded)
			boxscoreJSON = &encodedStr
		}
	} else {
		var err error
		rawInjuries, err = s.espn.SummaryInjuries(ctx, game.ID)
		if err != nil {
			return false, fmt.Errorf("summary %s: %w", game.ID, err)
		}
	}

	changed := false
	err := s.store.Tx(ctx, func(tx *db.Store) error {
		summaryChanged, err := tx.SetGameSummary(ctx, game.ID, rawInjuries, boxscoreJSON)
		if err != nil {
			return err
		}
		changed = summaryChanged
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
func (s *Service) gameLogSource(athleteID string, games []db.Game, now time.Time) source {
	return source{
		key: GameLogSource(athleteID),
		due: func(_ context.Context, state db.SyncState) (bool, error) {
			return gameLogStale(state.FetchedAt, now, games), nil
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

	entries := log.Entries
	if entries == nil {
		entries = []espn.GameLogEntry{}
	}
	gamesBytes, err := json.Marshal(entries)
	if err != nil {
		return false, fmt.Errorf("marshal gamelog entries %s: %w", athleteID, err)
	}

	dbLog := db.PlayerGameLog{
		AthleteID:   athleteID,
		GamesPlayed: log.GamesPlayed,
		StatsJSON:   string(statsBytes),
		GamesJSON:   string(gamesBytes),
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

// oddsPropsEnrichSource enriches one game's existing props with The Odds API
// deep links when odds:props:{gameID} is already fresh (for example, after
// Parlay-only RunPrefill) and enrichment has not run for that sync yet.
func (s *Service) oddsPropsEnrichSource(game db.Game, now time.Time, force bool) source {
	return source{
		key: OddsPropsEnrichSource(game.ID),
		due: func(ctx context.Context, state db.SyncState) (bool, error) {
			if !now.Before(game.Kickoff) || s.fallbackProps == nil {
				return false, nil
			}
			propsState, err := s.store.SyncState(ctx, OddsPropsSource(game.ID))
			if errors.Is(err, db.ErrSyncStateNotFound) {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			if state.FetchedAt.IsZero() || state.FetchedAt.Before(propsState.FetchedAt) {
				return true, nil
			}
			if force {
				return pickCooldownStale(state.FetchedAt, now), nil
			}
			return oddsPropsStale(state.FetchedAt, now, game), nil
		},
		run: func(ctx context.Context) (bool, error) {
			return s.enrichGameProps(ctx, game, now)
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
	if len(props) == 0 {
		return nil
	}

	resolver := s.newBatchPlayerResolver(ctx, game)

	type oddsKey struct {
		gameID     string
		book       string
		marketType string
		team       string
		playerName string
		side       string
	}

	type mappedOdds struct {
		row  db.Odds
		pref db.OddsCandidate
	}

	mapped := make([]mappedOdds, 0, len(props))
	indexByKey := make(map[oddsKey]int, len(props))

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
		linked := false
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
			resolved := resolver.ResolveGamePlayer(game.ID, prop.PlayerName, "", game.HomeTeam, game.AwayTeam)
			row.PlayerName = resolved.Name
			row.Team = resolved.Team
			linked = resolved.Linked
		}

		cand := mappedOdds{
			row: row,
			pref: db.OddsCandidate{
				Linked:   linked,
				Richness: db.OddsRichness(row),
			},
		}
		key := oddsKey{
			gameID:     row.GameID,
			book:       row.Book,
			marketType: row.MarketType,
			team:       row.Team,
			playerName: row.PlayerName,
			side:       row.Side,
		}
		if idx, exists := indexByKey[key]; exists {
			if db.PreferOddsCandidate(cand.pref, mapped[idx].pref) {
				mapped[idx] = cand
			}
			continue
		}
		indexByKey[key] = len(mapped)
		mapped = append(mapped, cand)
	}

	if matches := resolver.NewMatches(); len(matches) > 0 {
		if err := s.store.UpsertPlayerMatches(ctx, matches); err != nil {
			s.logger.Warn().Err(err).Str("game_id", game.ID).Msg("upsert player matches")
		}
	}

	rows := make([]db.Odds, len(mapped))
	for i, m := range mapped {
		rows[i] = m.row
	}
	return rows
}

func (s *Service) newBatchPlayerResolver(ctx context.Context, game db.Game) *db.BatchPlayerResolver {
	var teams []string
	if game.HomeTeam != "" {
		teams = append(teams, game.HomeTeam)
	}
	if game.AwayTeam != "" && game.AwayTeam != game.HomeTeam {
		teams = append(teams, game.AwayTeam)
	}

	rosterPlayers, err := s.store.RosterForTeams(ctx, teams)
	if err != nil {
		s.logger.Warn().Err(err).Str("game_id", game.ID).Msg("load rosters for prop matching")
	}
	playerMatches, err := s.store.PlayerMatchesForTeams(ctx, teams)
	if err != nil {
		s.logger.Warn().Err(err).Str("game_id", game.ID).Msg("load player matches for prop matching")
	}

	return db.NewBatchPlayerResolver(rosterPlayers, playerMatches)
}

// syncOddsProps fetches and writes props for one game.
func (s *Service) syncOddsProps(ctx context.Context, game db.Game, now time.Time) (bool, error) {
	if !now.Before(game.Kickoff) {
		return false, nil
	}
	start := time.Now()

	// 1. Sync rosters for both teams before matching props.
	if s.espn != nil {
		gameSlice := []db.Game{game}
		if game.HomeESPNID != "" && game.HomeTeam != "" {
			if err := s.refreshSync(
				ctx,
				s.rosterSource(game.HomeTeam, game.HomeESPNID, gameSlice, now),
				now,
			); err != nil {
				s.logger.Warn().Err(err).Str("team", game.HomeTeam).Msg("sync home roster")
			}
		}
		if game.AwayESPNID != "" && game.AwayTeam != "" {
			if err := s.refreshSync(
				ctx,
				s.rosterSource(game.AwayTeam, game.AwayESPNID, gameSlice, now),
				now,
			); err != nil {
				s.logger.Warn().Err(err).Str("team", game.AwayTeam).Msg("sync away roster")
			}
		}
	}

	// 2. Primary Parlay API props.
	primary := s.fetchParlayProps(ctx, &game)

	// 3. Secondary The Odds API fallback when Parlay API is unconfigured,
	// failed, below its floor, or returned zero props.
	fallback := propsFetch{disabled: true}
	if !primary.bought || len(primary.props) == 0 {
		fallback = s.fetchFallbackProps(ctx, game, TheOddsAPIFallbackFloor, now)
	}

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
	changed, err := s.writeOddsProps(ctx, game, rows, now, fallback.bought)
	s.logger.Info().
		Float64("duration_ms", durationMS(time.Since(start))).
		Str("game_id", game.ID).
		Int("rows", len(rows)).
		Bool("changed", changed).
		Err(err).
		Msg("refresh sync odds props")
	return changed, err
}

// enrichGameProps fetches per-event props from The Odds API only (never Parlay
// API), merges them into the game's existing current prop rows at a unified
// FetchedAt, and records both odds:props:{gameID} and odds:props:enrich:{gameID}.
func (s *Service) enrichGameProps(ctx context.Context, game db.Game, now time.Time) (bool, error) {
	if !now.Before(game.Kickoff) || s.fallbackProps == nil {
		return false, nil
	}
	start := time.Now()

	fallback := s.fetchFallbackProps(ctx, game, TheOddsAPIEnrichFloor, now)
	if fallback.disabled {
		return false, nil
	}
	if fallback.err != nil {
		return false, fallback.err
	}
	if !fallback.bought {
		return false, nil
	}

	changed, rows, err := s.commitGamePropsEnrich(ctx, game, fallback.props, now)
	s.logger.Info().
		Float64("duration_ms", durationMS(time.Since(start))).
		Str("game_id", game.ID).
		Int("rows", rows).
		Bool("changed", changed).
		Err(err).
		Msg("refresh enrich game props")
	return changed, err
}

// enrichGamePropsForMarkets fetches only the requested prop markets for one
// game from The Odds API for draftkings and fanduel using a pre-matched event
// ID, then merges them into the game's existing current prop rows.
func (s *Service) enrichGamePropsForMarkets(
	ctx context.Context,
	game db.Game,
	toaEventID string,
	markets []string,
	now time.Time,
) (bool, error) {
	if !now.Before(game.Kickoff) || s.fallbackProps == nil || toaEventID == "" || len(markets) == 0 {
		return false, nil
	}
	start := time.Now()
	fallbackCtx := archive.WithKey(ctx, TheOddsAPISource)

	var (
		props   []odds.Prop
		credits odds.Credits
		err     error
	)
	if scoped, ok := s.fallbackProps.(marketPropsClient); ok {
		props, credits, err = scoped.EventPropsForMarkets(fallbackCtx, toaEventID, markets, enrichBookmakers)
	} else {
		props, credits, err = s.fallbackProps.EventProps(fallbackCtx, toaEventID)
	}
	if err != nil {
		return false, fmt.Errorf("fallback event props: %w", err)
	}
	_ = s.store.RecordSync(fallbackCtx, TheOddsAPISource, now, &credits.Remaining)

	changed, rows, err := s.commitGamePropsEnrich(ctx, game, props, now)
	s.logger.Info().
		Float64("duration_ms", durationMS(time.Since(start))).
		Str("game_id", game.ID).
		Strs("markets", markets).
		Int("rows", rows).
		Bool("changed", changed).
		Err(err).
		Msg("refresh enrich game props")
	return changed, err
}

func (s *Service) commitGamePropsEnrich(
	ctx context.Context,
	game db.Game,
	props []odds.Prop,
	now time.Time,
) (bool, int, error) {
	existingOdds, err := s.store.OddsForGame(ctx, game.ID)
	if err != nil {
		return false, 0, fmt.Errorf("load existing props for %s: %w", game.ID, err)
	}
	var existingProps []db.Odds
	for _, row := range existingOdds {
		if db.IsPropRow(row) {
			existingProps = append(existingProps, row)
		}
	}

	enrichedRows := s.mapPropsToOdds(ctx, game, props, now)
	merged := mergeOdds(existingProps, enrichedRows)
	for i := range merged {
		merged[i].FetchedAt = now
	}

	changed, err := s.writeOddsProps(ctx, game, merged, now, true)
	return changed, len(merged), err
}

// writeOddsProps writes prop rows and records the game's props sync state.
// When enriched is true, it also records OddsPropsEnrichSource(game.ID) in the
// same transaction.
func (s *Service) writeOddsProps(
	ctx context.Context,
	game db.Game,
	rows []db.Odds,
	now time.Time,
	enriched bool,
) (bool, error) {
	changed := false
	err := s.store.Tx(ctx, func(tx *db.Store) error {
		if len(rows) > 0 {
			rowsChanged, err := tx.UpsertOdds(ctx, rows)
			if err != nil {
				return err
			}
			changed = rowsChanged
		}
		if err := tx.RecordSync(ctx, OddsPropsSource(game.ID), now, nil); err != nil {
			return err
		}
		if enriched {
			return tx.RecordSync(ctx, OddsPropsEnrichSource(game.ID), now, nil)
		}
		return nil
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
						changed, err := s.writeOddsProps(ctx, game, rows, now, false)
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
