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
	// weatherSourcePrefix keys per-game forecasts.
	weatherSourcePrefix = "weather:"
)

// WeatherSource returns the sync_state key for one game's forecast.
func WeatherSource(gameID string) string {
	return weatherSourcePrefix + gameID
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
	failureBackoff       = 5 * time.Minute
	syncTimeout          = 30 * time.Second
)

// Credit floors. The odds client uses the free event list first, so refresh
// can check a fresh balance before spending on game lines.
const (
	// OddsLinesFloor is the least remaining balance that permits the lines call.
	OddsLinesFloor = 50
	// TheOddsAPIEnrichFloor is the least remaining The Odds API balance that
	// permits enrichment while Parlay API supplied the week's lines.
	TheOddsAPIEnrichFloor = 100
	// TheOddsAPIFallbackFloor is the least remaining The Odds API balance that
	// lets it serve the week's lines when Parlay API is unavailable.
	TheOddsAPIFallbackFloor = 25
)

// Clock returns the current time. The server passes time.Now; tests pass a
// clock they move by hand.
type Clock func() time.Time

// ScoreboardClient is the slice of *espn.Client refresh needs.
type ScoreboardClient interface {
	Scoreboard(ctx context.Context, params espn.ScoreboardParams) (espn.Scoreboard, error)
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

// Publisher receives live change signals. *live.Hub satisfies it.
type Publisher interface {
	Publish(topic string)
}

// Deps are the services refresh works with.
type Deps struct {
	Store         *db.Store
	Scoreboard    ScoreboardClient
	Forecast      ForecastClient
	Lines         LinesClient
	FallbackLines LinesClient
	Publisher     Publisher
	Now           Clock
	Logger        zerolog.Logger
}

// Service runs background syncs. Its zero value is not usable; build one with
// New.
type Service struct {
	store         *db.Store
	scoreboard    ScoreboardClient
	forecast      ForecastClient
	lines         LinesClient
	fallbackLines LinesClient
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
	return &Service{
		store:         deps.Store,
		scoreboard:    deps.Scoreboard,
		forecast:      deps.Forecast,
		lines:         deps.Lines,
		fallbackLines: deps.FallbackLines,
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

	games, err := s.currentGames(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("read current games")
		return
	}
	s.refresh(ctx, s.scoreboardSource(games, now), now)

	games, err = s.currentGames(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("read current games")
		return
	}

	sources := make([]source, 0, len(games)+1)
	for _, game := range games {
		sources = append(sources, s.weatherSource(game, now))
	}
	sources = append(sources, s.oddsSource(games, now))
	s.runSources(ctx, now, sources)
}

// currentGames returns the latest week's games. Before the first slate sync
// there is no week, which is not an error.
func (s *Service) currentGames(ctx context.Context) ([]db.Game, error) {
	week, err := s.store.LatestWeek(ctx)
	if errors.Is(err, db.ErrWeekNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("latest week: %w", err)
	}

	games, err := s.store.GamesForWeek(ctx, week.ID)
	if err != nil {
		return nil, fmt.Errorf("week %d games: %w", week.ID, err)
	}
	return games, nil
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
	if s.paused(src.key, now) {
		return
	}

	ctx, cancel := context.WithTimeout(parent, syncTimeout)
	defer cancel()
	// Label every outside call with the same key sync_state uses, so the
	// archive and the sync record join cleanly.
	ctx = archive.WithKey(ctx, src.key)

	due, err := s.shouldRun(ctx, src)
	if err != nil {
		s.logger.Error().Err(err).Str("source", src.key).Msg("check refresh source")
		s.backOff(src.key, now)
		return
	}
	if !due {
		return
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
		return
	}
	s.clearBackoff(src.key)
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
