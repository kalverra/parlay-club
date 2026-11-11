// Package refresh decides when outside data is fetched and writes the results
// through the store. It owns the per-source staleness rules, single-flight
// keyed by source, the in-memory backoff after a failure, and the Parlay API
// credit floors. Callers kick it from a page load; it returns at once and does
// the work in the background.
package refresh

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	// Embed the zone database so America/New_York resolves in a container
	// without a separate tzdata package.
	_ "time/tzdata"

	"github.com/rs/zerolog"
	"golang.org/x/sync/singleflight"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/archive"
	"github.com/kalverra/parlay-club/internal/ingest/espn"
	"github.com/kalverra/parlay-club/internal/ingest/nfelo"
	"github.com/kalverra/parlay-club/internal/ingest/odds"
	"github.com/kalverra/parlay-club/internal/ingest/weather"
)

// Source keys written to sync_state.
const (
	// ScoreboardSource is the current week's slate, scores, and status.
	ScoreboardSource = "espn:scoreboard"
	// OddsLinesSource is the week's game lines.
	OddsLinesSource = "odds:lines"
	// TheOddsAPISource is The Odds API's event list, lines, and props. Its
	// "odds:" prefix makes the archive label its responses as odds.
	TheOddsAPISource = "odds:theoddsapi"
	// NfeloProjectionsSource is nfelo's model spreads and QB adjustments.
	NfeloProjectionsSource = "nfelo:projections"
	// weatherSourcePrefix keys per-game forecasts.
	weatherSourcePrefix = "weather:"
	// oddsPropsSourcePrefix keys per-game prop lines.
	oddsPropsSourcePrefix = "odds:props:"
	// rosterSourcePrefix keys per-team rosters.
	rosterSourcePrefix = "espn:roster:"
	// gameLogSourcePrefix keys per-athlete season game logs.
	gameLogSourcePrefix = "espn:gamelog:"
	// summarySourcePrefix keys per-game ESPN summaries and injury lists.
	summarySourcePrefix = "espn:summary:"
)

// WeatherSource returns the sync_state key for one game's forecast.
func WeatherSource(gameID string) string {
	return weatherSourcePrefix + gameID
}

// OddsPropsSource returns the sync_state key for one game's props.
func OddsPropsSource(gameID string) string {
	return oddsPropsSourcePrefix + gameID
}

// RosterSource returns the sync_state key for one team's roster.
func RosterSource(teamID string) string {
	return rosterSourcePrefix + teamID
}

// GameLogSource returns the sync_state key for one athlete's game log.
func GameLogSource(athleteID string) string {
	return gameLogSourcePrefix + athleteID
}

// SummarySource returns the sync_state key for one game's summary.
func SummarySource(gameID string) string {
	return summarySourcePrefix + gameID
}

// Staleness windows and the failure backoff. The odds windows match the credit
// budget in docs/reference/data-sources.md.
const (
	scoreboardLiveAge    = 10 * time.Minute
	scoreboardGameDayAge = 30 * time.Minute
	scoreboardIdleAge    = 6 * time.Hour
	weatherAge           = time.Hour
	weatherLookahead     = 48 * time.Hour
	oddsWeekdayAge       = 24 * time.Hour
	oddsSaturdayAge      = 12 * time.Hour
	oddsSundayAge        = 45 * time.Minute
	oddsLateAge          = 3 * time.Hour
	oddsPropsAge         = 2 * time.Hour
	rosterAge            = 24 * time.Hour
	gameLogAge           = 12 * time.Hour
	summaryAge           = 2 * time.Hour
	nfeloAge             = 12 * time.Hour
	failureBackoff       = 5 * time.Minute
	syncTimeout          = 30 * time.Second
	requestTimeout       = 10 * time.Second
)

// Credit floors. The odds client uses the free event list first, so refresh
// can check a fresh balance before spending on game lines.
const (
	// OddsLinesFloor is the least remaining balance that permits the lines call.
	OddsLinesFloor = 50
	// OddsPropsFloor is the least remaining balance that permits the props call.
	OddsPropsFloor = 150
	// TheOddsAPIEnrichFloor is the least remaining The Odds API balance that
	// permits enrichment while Parlay API supplied the week's lines or game props.
	TheOddsAPIEnrichFloor = 100
	// TheOddsAPIFallbackFloor is the least remaining The Odds API balance that
	// lets it serve the week's lines or game props when Parlay API is unavailable.
	TheOddsAPIFallbackFloor = 25
)

// Clock returns the current time. The server passes time.Now; tests pass a
// clock they move by hand.
type Clock func() time.Time

// ScoreboardClient is the slice of *espn.Client refresh needs.
type ScoreboardClient interface {
	Scoreboard(ctx context.Context, params espn.ScoreboardParams) (espn.Scoreboard, error)
}

// ESPNResearchClient is the slice of *espn.Client refresh needs for rosters, summaries, and game logs.
type ESPNResearchClient interface {
	Roster(ctx context.Context, espnTeamID string) ([]espn.RosterPlayer, error)
	SummaryInjuries(ctx context.Context, eventID string) (string, error)
	GameLog(ctx context.Context, athleteID string) (espn.GameLog, error)
}

// ForecastClient is the slice of *weather.Client refresh needs.
type ForecastClient interface {
	Forecast(ctx context.Context, venue weather.Venue, kickoff time.Time) (weather.Forecast, error)
}

// LinesClient is the slice of *odds.Client refresh needs.
type LinesClient interface {
	Events(ctx context.Context) ([]odds.Event, odds.Credits, error)
	Lines(ctx context.Context) ([]odds.Line, odds.Credits, error)
}

// PropsClient is the slice of *odds.Client refresh needs for Parlay API bulk props.
type PropsClient interface {
	Events(ctx context.Context) ([]odds.Event, odds.Credits, error)
	Props(ctx context.Context, eventID string) ([]odds.Prop, odds.Credits, error)
}

// FallbackPropsClient is the slice of *odds.Client refresh needs for The Odds API per-event props.
type FallbackPropsClient interface {
	Events(ctx context.Context) ([]odds.Event, odds.Credits, error)
	EventProps(ctx context.Context, eventID string) ([]odds.Prop, odds.Credits, error)
}

// ProjectionsClient is the slice of *nfelo.Client refresh needs.
type ProjectionsClient interface {
	Projections(ctx context.Context) ([]nfelo.Projection, error)
}

// Publisher receives live change signals. *live.Hub satisfies it.
type Publisher interface {
	Publish(topic string)
}

// Deps are the services refresh works with.
type Deps struct {
	Store         *db.Store
	Scoreboard    ScoreboardClient
	ESPN          ESPNResearchClient
	Forecast      ForecastClient
	Lines         LinesClient
	FallbackLines LinesClient
	Props         PropsClient
	FallbackProps FallbackPropsClient
	Projections   ProjectionsClient
	Publisher     Publisher
	Now           Clock
	Logger        zerolog.Logger
}

// Service runs background syncs. Its zero value is not usable; build one with
// New.
type Service struct {
	store         *db.Store
	scoreboard    ScoreboardClient
	espn          ESPNResearchClient
	forecast      ForecastClient
	lines         LinesClient
	fallbackLines LinesClient
	props         PropsClient
	fallbackProps FallbackPropsClient
	projections   ProjectionsClient
	publisher     Publisher
	now           Clock
	logger        zerolog.Logger

	flight singleflight.Group

	mu      sync.Mutex
	backoff map[string]time.Time

	wg sync.WaitGroup
}

// source is one unit of refresh work. due reads the source's sync_state and
// says whether it needs a fetch. run fetches and writes, and reports whether
// any row changed.
type source struct {
	key string
	due func(context.Context, db.SyncState) (bool, error)
	run func(context.Context) (bool, error)
}

// New builds a refresh service. A nil clock means time.Now.
func New(deps Deps) *Service {
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	espn := deps.ESPN
	if espn == nil {
		if ec, ok := deps.Scoreboard.(ESPNResearchClient); ok {
			espn = ec
		}
	}
	props := deps.Props
	if props == nil {
		if pc, ok := deps.Lines.(PropsClient); ok {
			props = pc
		}
	}
	fallbackProps := deps.FallbackProps
	if fallbackProps == nil {
		if fpc, ok := deps.FallbackLines.(FallbackPropsClient); ok {
			fallbackProps = fpc
		}
	}
	return &Service{
		store:         deps.Store,
		scoreboard:    deps.Scoreboard,
		espn:          espn,
		forecast:      deps.Forecast,
		lines:         deps.Lines,
		fallbackLines: deps.FallbackLines,
		props:         props,
		fallbackProps: fallbackProps,
		projections:   deps.Projections,
		publisher:     deps.Publisher,
		now:           now,
		logger:        deps.Logger,
		backoff:       make(map[string]time.Time),
	}
}

// Kick starts a background refresh pass and returns at once. It detaches from
// the caller's context so a finished page request does not cancel a sync.
func (s *Service) Kick() {
	s.KickContext(context.Background())
}

// KickContext is Kick with a parent context. The work still detaches from its
// cancellation, so it runs until its own timeout.
func (s *Service) KickContext(ctx context.Context) {
	ctx = context.WithoutCancel(ctx)
	s.wg.Go(func() {
		s.kick(ctx)
	})
}

// kick runs the slate first, then the sources that depend on it. The slate
// sync may add or change the week's games, so the weather and odds pass reads
// them again.
func (s *Service) kick(ctx context.Context) {
	now := s.now()

	_, games, err := s.currentWeek(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("read current games")
		return
	}
	s.refresh(ctx, s.scoreboardSource(games, now), now)

	week, games, err := s.currentWeek(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("read current games")
		return
	}

	sources, err := s.sourcesFor(ctx, week, games, now)
	if err != nil {
		s.logger.Error().Err(err).Msg("build refresh sources")
	}
	s.runSources(ctx, now, sources)
}

// RunOnce is the synchronous counterpart to Kick. It runs the same
// scoreboard-first pass, waits for every source it starts, and returns the
// failures joined. The sandbox bootstrap uses it so startup finishes with a
// complete slate; normal page loads keep using Kick.
func (s *Service) RunOnce(ctx context.Context) error {
	now := s.now()

	_, games, err := s.currentWeek(ctx)
	if err != nil {
		return fmt.Errorf("read current games: %w", err)
	}

	var errs []error
	if err := s.refreshWithTimeout(ctx, s.scoreboardSource(games, now), now); err != nil {
		errs = append(errs, err)
	}

	week, games, err := s.currentWeek(ctx)
	if err != nil {
		return errors.Join(append(errs, fmt.Errorf("read current games: %w", err))...)
	}
	sources, err := s.sourcesFor(ctx, week, games, now)
	if err != nil {
		errs = append(errs, err)
	}

	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		runErr []error
	)
	for _, src := range sources {
		wg.Go(func() {
			if err := s.refreshWithTimeout(ctx, src, now); err != nil {
				mu.Lock()
				runErr = append(runErr, err)
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	return errors.Join(append(errs, runErr...)...)
}

// sourcesFor builds the sources that depend on the slate. A store read failure
// is joined but does not hide the sources that were built.
func (s *Service) sourcesFor(
	ctx context.Context,
	week db.Week,
	games []db.Game,
	now time.Time,
) ([]source, error) {
	sources := make([]source, 0, len(games)+2)
	for _, game := range games {
		sources = append(sources, s.weatherSource(game, now))
	}
	sources = append(sources, s.oddsSource(games, now))
	if s.projections != nil && week.ID != 0 {
		sources = append(sources, s.nfeloSource(week, games, now))
	}
	if s.espn == nil || week.ID == 0 {
		return sources, nil
	}

	var errs []error
	slipIDs, err := s.store.SlipIDsForWeek(ctx, week.ID)
	if err != nil {
		errs = append(errs, fmt.Errorf("list slips for gamelogs: %w", err))
		return sources, errors.Join(errs...)
	}
	athletes := make(map[string]struct{})
	for _, slipID := range slipIDs {
		legs, err := s.store.LegsForSlip(ctx, slipID)
		if err != nil {
			errs = append(errs, fmt.Errorf("read slip %d legs for gamelogs: %w", slipID, err))
			continue
		}
		for _, leg := range legs {
			if leg.PlayerID != nil && *leg.PlayerID != "" {
				athletes[*leg.PlayerID] = struct{}{}
			}
		}
	}
	for athleteID := range athletes {
		sources = append(sources, s.gameLogSource(athleteID, now))
	}
	return sources, errors.Join(errs...)
}

// currentWeek returns the latest week and its games. Before the first slate
// sync there is no week, which is not an error.
func (s *Service) currentWeek(ctx context.Context) (db.Week, []db.Game, error) {
	week, err := s.store.LatestWeek(ctx)
	if errors.Is(err, db.ErrWeekNotFound) {
		return db.Week{}, nil, nil
	}
	if err != nil {
		return db.Week{}, nil, fmt.Errorf("latest week: %w", err)
	}

	games, err := s.store.GamesForWeek(ctx, week.ID)
	if err != nil {
		return db.Week{}, nil, fmt.Errorf("week %d games: %w", week.ID, err)
	}
	return week, games, nil
}

// runSources starts one goroutine per source.
func (s *Service) runSources(ctx context.Context, now time.Time, sources []source) {
	for _, src := range sources {
		s.wg.Go(func() {
			s.refresh(ctx, src, now)
		})
	}
}

// wait blocks until every sync this service started has finished. It exists
// for tests; the server never waits.
func (s *Service) wait() {
	s.wg.Wait()
}

// refresh runs one source when it is due, unless a failure has paused it.
func (s *Service) refresh(parent context.Context, src source, now time.Time) {
	_ = s.refreshWithTimeout(parent, src, now)
}

// refreshWithTimeout runs one source under the shared per-source timeout. Kick
// and RunOnce both come through here, so neither can hold a caller's lock
// forever on a stuck store call or fetch.
func (s *Service) refreshWithTimeout(parent context.Context, src source, now time.Time) error {
	ctx, cancel := context.WithTimeout(parent, syncTimeout)
	defer cancel()
	return s.refreshSync(ctx, src, now)
}

// refreshSync runs one source synchronously when it is due, respecting the failure
// backoff and single-flight deduplication.
func (s *Service) refreshSync(parent context.Context, src source, now time.Time) error {
	if s.paused(src.key, now) {
		return nil
	}

	// Label every outside call with the same key sync_state uses, so the
	// archive and the sync record join cleanly.
	ctx := archive.WithKey(parent, src.key)

	due, err := s.shouldRun(ctx, src)
	if err != nil {
		s.logger.Error().Err(err).Str("source", src.key).Msg("check refresh source")
		s.backOff(src.key, now)
		return err
	}
	if !due {
		return nil
	}

	_, err, _ = s.flight.Do(src.key, func() (any, error) {
		// A second page load may have joined the flight after the first run
		// finished. Read the state again so only a still-stale source fetches.
		due, err := s.shouldRun(ctx, src)
		if err != nil || !due {
			return false, err
		}
		return src.run(ctx)
	})
	if err != nil {
		s.logger.Error().Err(err).Str("source", src.key).Msg("refresh failed")
		s.backOff(src.key, now)
		return err
	}
	s.clearBackoff(src.key)
	return nil
}

// SyncGameProps fetches and writes props for one game if stale.
func (s *Service) SyncGameProps(ctx context.Context, gameID string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	game, err := s.store.GameByID(ctx, gameID)
	if err != nil {
		return fmt.Errorf("game %s: %w", gameID, err)
	}

	now := s.now()
	return s.refreshSync(ctx, s.oddsPropsSource(game, now), now)
}

// SyncGameResearch syncs injuries, rosters, and props for one game if stale.
func (s *Service) SyncGameResearch(ctx context.Context, gameID string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	game, err := s.store.GameByID(ctx, gameID)
	if err != nil {
		return fmt.Errorf("game %s: %w", gameID, err)
	}

	now := s.now()
	var errs []error
	if err := s.refreshSync(ctx, s.summarySource(game, now), now); err != nil {
		errs = append(errs, err)
	}
	if err := s.refreshSync(ctx, s.oddsPropsSource(game, now), now); err != nil && !errors.Is(err, odds.ErrNoAPIKey) {
		errs = append(errs, err)
	}
	if !now.Before(game.Kickoff) {
		if game.HomeESPNID != "" && game.HomeTeam != "" {
			if err := s.refreshSync(ctx, s.rosterSource(game.HomeTeam, game.HomeESPNID, now), now); err != nil {
				errs = append(errs, err)
			}
		}
		if game.AwayESPNID != "" && game.AwayTeam != "" {
			if err := s.refreshSync(ctx, s.rosterSource(game.AwayTeam, game.AwayESPNID, now), now); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// SyncPlayerResearch syncs one athlete's game log if stale.
func (s *Service) SyncPlayerResearch(ctx context.Context, athleteID string) error {
	if athleteID == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	now := s.now()
	return s.refreshSync(ctx, s.gameLogSource(athleteID, now), now)
}

// shouldRun reads a source's state and applies its due rule. Credit floors are
// checked by the odds sync itself, after the free event list refreshes the
// balance; a stored low balance must not pause the source forever.
func (s *Service) shouldRun(ctx context.Context, src source) (bool, error) {
	state, err := s.store.SyncState(ctx, src.key)
	if errors.Is(err, db.ErrSyncStateNotFound) {
		state = db.SyncState{Source: src.key}
	} else if err != nil {
		return false, err
	}
	return src.due(ctx, state)
}

// paused reports whether a source is still inside its failure backoff.
func (s *Service) paused(key string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	until, ok := s.backoff[key]
	return ok && now.Before(until)
}

// backOff pauses a source for five minutes after a failure.
func (s *Service) backOff(key string, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.backoff[key] = now.Add(failureBackoff)
}

// clearBackoff forgets a source's failure after a successful run.
func (s *Service) clearBackoff(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.backoff, key)
}

// ClearBackoff forgets every source's failure pause. A sandbox reset calls it
// after clearing sync_state so an old failure cannot skip the fresh pass.
func (s *Service) ClearBackoff() {
	s.mu.Lock()
	defer s.mu.Unlock()

	clear(s.backoff)
}
