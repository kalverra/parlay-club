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
			row.PlayerName, row.Team, linked = resolver.resolve(prop.PlayerName)
		}

		cand := mappedOdds{row: row, linked: linked}
		key := oddsKey{
			gameID:     row.GameID,
			book:       row.Book,
			marketType: row.MarketType,
			team:       row.Team,
			playerName: row.PlayerName,
			side:       row.Side,
		}
		if idx, exists := indexByKey[key]; exists {
			if preferMappedOdds(cand, mapped[idx]) {
				mapped[idx] = cand
			}
			continue
		}
		indexByKey[key] = len(mapped)
		mapped = append(mapped, cand)
	}

	if err := resolver.flush(ctx); err != nil {
		s.logger.Warn().Err(err).Str("game_id", game.ID).Msg("upsert player matches")
	}

	rows := make([]db.Odds, len(mapped))
	for i, m := range mapped {
		rows[i] = m.row
	}
	return rows
}

type mappedOdds struct {
	row    db.Odds
	linked bool
}

func preferMappedOdds(cand, existing mappedOdds) bool {
	if cand.linked != existing.linked {
		return cand.linked
	}
	return oddsRichness(cand.row) > oddsRichness(existing.row)
}

func oddsRichness(row db.Odds) int {
	score := 0
	if row.Link != nil {
		score++
	}
	if row.Sid != nil {
		score++
	}
	if row.EventSid != nil {
		score++
	}
	return score
}

type playerMatchKey struct {
	oddsName string
	team     string
}

type batchPlayerResolver struct {
	store             *db.Store
	homeTeam          string
	awayTeam          string
	rosterByAthleteID map[string]db.RosterPlayer
	rosterByNorm      map[string][]db.RosterPlayer
	matchesByNorm     map[string][]db.PlayerMatch
	unmatchedSpelling map[string]string
	newMatches        map[playerMatchKey]db.PlayerMatch
}

func (s *Service) newBatchPlayerResolver(ctx context.Context, game db.Game) *batchPlayerResolver {
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

	r := &batchPlayerResolver{
		store:             s.store,
		homeTeam:          game.HomeTeam,
		awayTeam:          game.AwayTeam,
		rosterByAthleteID: make(map[string]db.RosterPlayer, len(rosterPlayers)),
		rosterByNorm:      make(map[string][]db.RosterPlayer, len(rosterPlayers)),
		matchesByNorm:     make(map[string][]db.PlayerMatch, len(playerMatches)),
		unmatchedSpelling: make(map[string]string),
		newMatches:        make(map[playerMatchKey]db.PlayerMatch),
	}

	for _, p := range rosterPlayers {
		r.rosterByAthleteID[p.AthleteID] = p
		norm := p.NormName
		if norm == "" {
			norm = db.NormalizePlayerName(p.FullName)
		}
		if norm != "" {
			r.rosterByNorm[norm] = append(r.rosterByNorm[norm], p)
		}
	}
	for _, m := range playerMatches {
		norm := db.NormalizePlayerName(m.OddsName)
		if norm != "" {
			r.matchesByNorm[norm] = append(r.matchesByNorm[norm], m)
		}
	}
	return r
}

func (r *batchPlayerResolver) resolve(rawName string) (canonicalName, team string, linked bool) {
	cleaned, teamHint := db.StripPlayerTag(rawName)
	validHint := teamHint != "" && (teamHint == r.homeTeam || teamHint == r.awayTeam)
	norm := db.NormalizePlayerName(cleaned)
	if norm == "" {
		return cleaned, "", false
	}

	if m, ok := r.matchFromCache(norm, teamHint, validHint); ok {
		if rp, found := r.rosterByAthleteID[m.ESPNAthleteID]; found && rp.FullName != "" {
			return rp.FullName, m.Team, true
		}
		return r.unifySpelling(norm, cleaned), m.Team, true
	}

	if rp, ok := r.matchFromRoster(norm, teamHint, validHint); ok {
		r.newMatches[playerMatchKey{oddsName: norm, team: rp.Team}] = db.PlayerMatch{
			OddsName:      norm,
			Team:          rp.Team,
			ESPNAthleteID: rp.AthleteID,
			Manual:        false,
		}
		return rp.FullName, rp.Team, true
	}

	if validHint {
		team = teamHint
	}
	return r.unifySpelling(norm, cleaned), team, false
}

func (r *batchPlayerResolver) matchFromCache(norm, teamHint string, validHint bool) (db.PlayerMatch, bool) {
	matches := r.matchesByNorm[norm]
	if len(matches) == 0 {
		return db.PlayerMatch{}, false
	}

	var manual []db.PlayerMatch
	for _, m := range matches {
		if m.Manual {
			manual = append(manual, m)
		}
	}
	if len(manual) > 0 {
		return pickPlayerMatch(manual, teamHint, validHint, true)
	}
	return pickPlayerMatch(matches, teamHint, validHint, false)
}

func pickPlayerMatch(
	candidates []db.PlayerMatch,
	teamHint string,
	validHint, allowSingleMismatch bool,
) (db.PlayerMatch, bool) {
	if validHint {
		var hinted []db.PlayerMatch
		for _, c := range candidates {
			if c.Team == teamHint {
				hinted = append(hinted, c)
			}
		}
		if len(hinted) == 1 {
			return hinted[0], true
		}
		if len(hinted) > 1 || !allowSingleMismatch {
			return db.PlayerMatch{}, false
		}
	}
	if len(candidates) == 1 {
		return candidates[0], true
	}
	return db.PlayerMatch{}, false
}

func (r *batchPlayerResolver) matchFromRoster(norm, teamHint string, validHint bool) (db.RosterPlayer, bool) {
	candidates := r.rosterByNorm[norm]
	if len(candidates) == 0 {
		return db.RosterPlayer{}, false
	}
	if validHint {
		var hinted []db.RosterPlayer
		for _, c := range candidates {
			if c.Team == teamHint {
				hinted = append(hinted, c)
			}
		}
		if len(hinted) == 1 {
			return hinted[0], true
		}
		return db.RosterPlayer{}, false
	}
	if len(candidates) == 1 {
		return candidates[0], true
	}
	return db.RosterPlayer{}, false
}

func (r *batchPlayerResolver) unifySpelling(norm, cleaned string) string {
	if existing, ok := r.unmatchedSpelling[norm]; ok {
		return existing
	}
	r.unmatchedSpelling[norm] = cleaned
	return cleaned
}

func (r *batchPlayerResolver) flush(ctx context.Context) error {
	if len(r.newMatches) == 0 {
		return nil
	}
	matches := make([]db.PlayerMatch, 0, len(r.newMatches))
	for _, m := range r.newMatches {
		matches = append(matches, m)
	}
	return r.store.UpsertPlayerMatches(ctx, matches)
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
