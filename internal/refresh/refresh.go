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
	"strings"
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
	"github.com/kalverra/parlay-club/internal/ingest/nflverse"
	"github.com/kalverra/parlay-club/internal/ingest/odds"
	"github.com/kalverra/parlay-club/internal/ingest/weather"
	"github.com/kalverra/parlay-club/internal/slips"
)

// Source keys written to sync_state.
const (
	// ScoreboardSource is the current week's slate, scores, and status.
	ScoreboardSource = "espn:scoreboard"
	// OddsLinesSource is the week's game lines.
	OddsLinesSource = db.ArchiveOddsLinesKey
	// TheOddsAPISource is The Odds API's event list, lines, and props. Its
	// "odds:" prefix makes the archive label its responses as odds.
	TheOddsAPISource = "odds:theoddsapi"
	// OddsLinesEnrichSource is the sync_state key for game-line link enrichment
	// from The Odds API.
	OddsLinesEnrichSource = "odds:lines:enrich"
	// NfeloProjectionsSource is nfelo's model spreads and QB adjustments.
	NfeloProjectionsSource = "nfelo:projections"
	// GradeSource is the sync_state key for auto-grading pending legs on final games.
	GradeSource = "grade"
	// GameDayPrefillSource is the sync_state key for the one-shot per-game-day pre-fill.
	GameDayPrefillSource = "prefill:gameday"
	// weatherSourcePrefix keys per-game forecasts.
	weatherSourcePrefix = "weather:"
	// oddsPropsEnrichSourcePrefix keys per-game prop link enrichment from The Odds API.
	oddsPropsEnrichSourcePrefix = "odds:props:enrich:"
	// nflverseSourcePrefix keys per-season nflverse efficiency and usage syncs.
	nflverseSourcePrefix = "nflverse:season:"
)

// WeatherSource returns the sync_state key for one game's forecast.
func WeatherSource(gameID string) string {
	return weatherSourcePrefix + gameID
}

// OddsPropsSource returns the sync_state key for one game's props.
func OddsPropsSource(gameID string) string {
	return db.ArchiveOddsPropsKey(gameID)
}

// OddsPropsEnrichSource returns the sync_state key for one game's prop link enrichment.
func OddsPropsEnrichSource(gameID string) string {
	return oddsPropsEnrichSourcePrefix + gameID
}

// RosterSource returns the sync_state key for one team's roster.
func RosterSource(teamID string) string {
	return db.ArchiveRosterKey(teamID)
}

// GameLogSource returns the sync_state key for one athlete's game log.
func GameLogSource(athleteID string) string {
	return db.ArchiveGameLogKey(athleteID)
}

// SummarySource returns the sync_state key for one game's summary.
func SummarySource(gameID string) string {
	return db.ArchiveSummaryKey(gameID)
}

// NFLverseSource returns the sync_state key for one season's nflverse sync.
func NFLverseSource(season int) string {
	return fmt.Sprintf("%s%d", nflverseSourcePrefix, season)
}

// Staleness windows and the failure backoff. The odds windows match the credit
// budget in docs/reference/data-sources.md.
const (
	scoreboardLiveAge       = 2 * time.Minute
	scoreboardGameDayAge    = 5 * time.Minute
	scoreboardIdleAge       = 15 * time.Minute
	weatherNearAge          = 15 * time.Minute
	weatherNearLookahead    = 6 * time.Hour
	weatherAge              = 30 * time.Minute
	weatherLookahead        = 48 * time.Hour
	oddsWeekdayAge          = 24 * time.Hour
	oddsEveAge              = 12 * time.Hour
	oddsGameDayAge          = 45 * time.Minute
	oddsLateAge             = 3 * time.Hour
	oddsPickLinesGameDayAge = 15 * time.Minute
	oddsPickLinesIdleAge    = 2 * time.Hour
	oddsPickLinesLookahead  = 2 * time.Hour
	oddsPropsAge            = 2 * time.Hour
	oddsPropsGameDayAge     = 30 * time.Minute
	oddsPropsNearKickoff    = 6 * time.Hour
	pickRefreshCooldown     = 2 * time.Minute
	rosterGameDayAge        = 2 * time.Hour
	rosterIdleAge           = 6 * time.Hour
	gameLogGameDayAge       = time.Hour
	gameLogIdleAge          = 4 * time.Hour
	summaryLiveAge          = 2 * time.Minute
	summaryGameDayAge       = 15 * time.Minute
	summaryIdleAge          = time.Hour
	summaryNearKickoff      = 4 * time.Hour
	nfeloGameDayAge         = time.Hour
	nfeloIdleAge            = 4 * time.Hour
	nflverseActiveAge       = 6 * time.Hour
	nflverseIdleAge         = 24 * time.Hour
	nflversePostFinalWindow = 12 * time.Hour
	prefillIdempotencyAge   = 15 * time.Minute
	prefillConcurrency      = 4
	prefillKickTimeout      = 2 * time.Minute
	prefillStartHour        = 6
	failureBackoff          = 5 * time.Minute
	syncTimeout             = 30 * time.Second
	requestTimeout          = 10 * time.Second
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
	TheOddsAPIEnrichFloor = 40
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

// summaryClient is optionally implemented by ESPNResearchClient (such as
// *espn.Client) to fetch both injuries and box score from one /summary call.
type summaryClient interface {
	Summary(ctx context.Context, eventID string) (espn.GameSummary, error)
}

// BoxScoreClient is the slice of *espn.Client refresh needs to auto-grade
// player prop legs on final games.
type BoxScoreClient interface {
	BoxScore(ctx context.Context, eventID string) (espn.BoxScore, error)
}

// SlipGrader applies automatic leg grades through the slip service.
type SlipGrader interface {
	AutoGradeLegs(ctx context.Context, grades []slips.AutoGrade) error
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

// NFLverseClient is the slice of *nflverse.Client refresh needs.
type NFLverseClient interface {
	Players(ctx context.Context) ([]nflverse.Player, error)
	TeamStats(ctx context.Context, season int) ([]nflverse.TeamWeek, error)
	PlayerStats(ctx context.Context, season int) ([]nflverse.PlayerWeek, error)
	SnapCounts(ctx context.Context, season int) ([]nflverse.SnapCount, error)
	FFOpportunity(ctx context.Context, season int) ([]nflverse.PlayerOpportunity, error)
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
	BoxScore      BoxScoreClient
	Forecast      ForecastClient
	Lines         LinesClient
	FallbackLines LinesClient
	Props         PropsClient
	FallbackProps FallbackPropsClient
	Projections   ProjectionsClient
	NFLverse      NFLverseClient
	Slips         SlipGrader
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
	boxScore      BoxScoreClient
	forecast      ForecastClient
	lines         LinesClient
	fallbackLines LinesClient
	props         PropsClient
	fallbackProps FallbackPropsClient
	projections   ProjectionsClient
	nflverse      NFLverseClient
	slips         SlipGrader
	publisher     Publisher
	now           Clock
	logger        zerolog.Logger

	flight singleflight.Group

	mu                 sync.Mutex
	backoff            map[string]time.Time
	onMarketDataChange func()
	afterPass          func(context.Context)

	// stopCtx cancels the work Shutdown must not outlive: a sync still
	// fetching or writing while the database closes. stopMu makes the check
	// in KickContext and the goroutine it starts one step, so Shutdown cannot
	// wait on a pass that starts just after the wait begins.
	stopCtx    context.Context
	stopCancel context.CancelFunc
	stopMu     sync.Mutex

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
	boxScore := deps.BoxScore
	if boxScore == nil {
		if bc, ok := deps.Scoreboard.(BoxScoreClient); ok {
			boxScore = bc
		} else if bc, ok := espn.(BoxScoreClient); ok {
			boxScore = bc
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
	stopCtx, stopCancel := context.WithCancel(context.Background())
	return &Service{
		store:         deps.Store,
		scoreboard:    deps.Scoreboard,
		espn:          espn,
		boxScore:      boxScore,
		forecast:      deps.Forecast,
		lines:         deps.Lines,
		fallbackLines: deps.FallbackLines,
		props:         props,
		fallbackProps: fallbackProps,
		projections:   deps.Projections,
		nflverse:      deps.NFLverse,
		slips:         deps.Slips,
		publisher:     deps.Publisher,
		now:           now,
		logger:        deps.Logger,
		backoff:       make(map[string]time.Time),
		stopCtx:       stopCtx,
		stopCancel:    stopCancel,
	}
}

// SetOnMarketDataChange registers a callback invoked after scoreboard or
// nflverse syncs commit new final-game or nflverse data.
func (s *Service) SetOnMarketDataChange(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onMarketDataChange = fn
}

func (s *Service) notifyMarketDataChange() {
	s.mu.Lock()
	fn := s.onMarketDataChange
	s.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// SetAfterPass registers a callback that runs after a refresh pass has read and
// written its sources. Reminder scheduling uses it: a kickoff change lands in
// the slate sync, and the callback runs once the pass has done its work. A nil
// fn disables the callback.
func (s *Service) SetAfterPass(fn func(context.Context)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.afterPass = fn
}

// notifyAfterPass hands the finished pass to the callback under its own
// timeout. Shutdown cancels it, and the work detaches from the caller because a
// page request may already have returned.
func (s *Service) notifyAfterPass(parent context.Context) {
	s.mu.Lock()
	fn := s.afterPass
	s.mu.Unlock()
	if fn == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), syncTimeout)
	defer cancel()
	unregister := context.AfterFunc(s.stopCtx, cancel)
	defer unregister()
	fn(ctx)
}

// Kick starts a background refresh pass and returns at once. It detaches from
// the caller's context so a finished page request does not cancel a sync.
func (s *Service) Kick() {
	s.KickContext(context.Background())
}

// KickContext is Kick with a parent context. The work still detaches from its
// cancellation, so it runs until its own timeout. Shutdown does reach it, so a
// stopped service starts no new pass and an in-flight one ends promptly.
func (s *Service) KickContext(ctx context.Context) {
	s.stopMu.Lock()
	defer s.stopMu.Unlock()

	if s.stopping() {
		return // the server is stopping, so a new pass would only be cut short.
	}

	ctx = context.WithoutCancel(ctx)
	s.wg.Go(func() {
		s.kick(ctx)
	})
}

// Shutdown stops starting passes, cancels the ones in flight, and waits for
// them to finish. It returns ctx's error when the wait outlives the caller's
// deadline, so a source that ignores its context cannot hold the process open
// past Cloud Run's kill window. Calling it twice is safe.
func (s *Service) Shutdown(ctx context.Context) error {
	s.stopMu.Lock()
	s.stopCancel()
	s.stopMu.Unlock()

	return waitFor(ctx, &s.wg)
}

// stopping reports whether Shutdown has begun.
func (s *Service) stopping() bool {
	return s.stopCtx.Err() != nil
}

// waitFor blocks until wg is done or ctx is cancelled. An expired ctx leaves
// one goroutine behind until the work it waits on finishes.
func waitFor(ctx context.Context, wg *sync.WaitGroup) error {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func durationMS(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000.0
}

// kick runs the slate first, then the free sources that depend on it. The
// slate sync may add or change the week's games, so the free-source pass reads
// them again. Page-load kicks omit odds:lines to avoid spending credits outside
// RunPrefill, RunOnce, and pick-time refreshes.
func (s *Service) kick(ctx context.Context) {
	start := time.Now()
	sourcesCount := 0
	defer func() {
		s.logger.Debug().
			Float64("duration_ms", durationMS(time.Since(start))).
			Int("sources", sourcesCount).
			Msg("refresh kick")
	}()
	defer s.notifyAfterPass(ctx)

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

	if s.maybeRunGameDayPrefill(ctx, games, now) {
		return
	}

	sources, err := s.freeSourcesFor(ctx, week, games, now)
	if err != nil {
		s.logger.Error().Err(err).Msg("build refresh sources")
	}
	sourcesCount = len(sources)
	s.runSources(ctx, now, sources)
}

// maybeRunGameDayPrefill triggers RunPrefill once per Eastern game date when
// upcoming games remain on that date, deduplicating concurrent page loads with
// singleflight and backing off for 5 minutes on zero-progress failures so
// kick can still fall back to freeSourcesFor.
func (s *Service) maybeRunGameDayPrefill(ctx context.Context, games []db.Game, now time.Time) bool {
	if !gameDayPrefillWindow(games, now) || s.paused(GameDayPrefillSource, now) {
		return false
	}

	src := source{
		key: GameDayPrefillSource,
		due: func(_ context.Context, state db.SyncState) (bool, error) {
			return gameDayPrefillStale(state.FetchedAt, now), nil
		},
	}
	due, err := s.shouldRun(ctx, src)
	if err != nil {
		if !s.stopping() {
			s.logger.Error().Err(err).Str("source", GameDayPrefillSource).Msg("check game-day prefill")
			s.backOff(GameDayPrefillSource, now)
		}
		return false
	}
	if !due {
		return false
	}

	res, err, _ := s.flight.Do(GameDayPrefillSource, func() (any, error) {
		due, err := s.shouldRun(ctx, src)
		if err != nil || !due {
			return false, err
		}

		prefillCtx, cancel := context.WithTimeout(ctx, prefillKickTimeout)
		defer cancel()

		report, err := s.RunPrefill(prefillCtx)
		if err != nil && !report.LinesSynced && report.PropsSynced == 0 {
			return false, err
		}
		if err != nil && !s.stopping() {
			s.logger.Error().Err(err).Str("source", GameDayPrefillSource).Msg("game-day prefill partial failure")
		}
		return true, nil
	})
	if err != nil {
		if !s.stopping() {
			s.logger.Error().Err(err).Str("source", GameDayPrefillSource).Msg("game-day prefill failed")
			s.backOff(GameDayPrefillSource, now)
		}
		return false
	}
	s.clearBackoff(GameDayPrefillSource)
	ran, _ := res.(bool)
	return ran
}

// RunOnce is the synchronous counterpart used by sandbox bootstrap and tests.
// It runs the scoreboard-first pass followed by all free sources plus
// odds:lines, waits for every source it starts, and returns the failures
// joined.
func (s *Service) RunOnce(ctx context.Context) error {
	now := s.now()
	defer s.notifyAfterPass(ctx)

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
	sources, err := s.freeSourcesFor(ctx, week, games, now)
	if err != nil {
		errs = append(errs, err)
	}
	sources = append(sources, s.oddsSource(games, now))

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

// PrefillReport summarizes one RunPrefill sweep.
type PrefillReport struct {
	WeekID          int64 `json:"week_id"`
	UpcomingGames   int   `json:"upcoming_games"`
	LinesSynced     bool  `json:"lines_synced"`
	PropsSynced     int   `json:"props_synced"`
	PropsSkipped    int   `json:"props_skipped"`
	PropsFailed     int   `json:"props_failed"`
	FreeSourcesRun  int   `json:"free_sources_run"`
	ParlayRemaining *int  `json:"parlay_remaining,omitempty"`
}

// RunPrefill runs the game-day pre-fill sweep synchronously: scoreboard first,
// then odds:lines (with 15m idempotency so retries do not double-spend), then
// free sources (weather, summaries, unique week-team rosters, nfelo, grade,
// gamelogs), and finally bounded-parallel Parlay-only bulk props for today's
// upcoming games in America/New_York (falling back to all upcoming week games
// when invoked manually with no upcoming games today, always with 15m
// idempotency and no fallback EventProps calls).
func (s *Service) RunPrefill(ctx context.Context) (PrefillReport, error) {
	s.stopMu.Lock()
	if s.stopping() {
		s.stopMu.Unlock()
		return PrefillReport{}, context.Canceled
	}
	s.wg.Add(1)
	s.stopMu.Unlock()
	defer s.wg.Done()

	ctx, stopWork := context.WithCancel(ctx)
	defer stopWork()
	unregister := context.AfterFunc(s.stopCtx, stopWork)
	defer unregister()

	now := s.now()
	var report PrefillReport
	var errs []error

	_, games, err := s.currentWeek(ctx)
	if err != nil {
		return report, fmt.Errorf("read current games: %w", err)
	}
	if err := s.refreshWithTimeout(ctx, s.scoreboardSource(games, now), now); err != nil {
		errs = append(errs, err)
	}

	week, games, err := s.currentWeek(ctx)
	if err != nil {
		return report, errors.Join(append(errs, fmt.Errorf("read current games: %w", err))...)
	}
	report.WeekID = week.ID

	if len(games) > 0 && !allKickedOff(games, now) {
		linesRan := false
		linesSrc := source{
			key: OddsLinesSource,
			due: func(_ context.Context, state db.SyncState) (bool, error) {
				return prefillStale(state.FetchedAt, now), nil
			},
			run: func(ctx context.Context) (bool, error) {
				linesRan = true
				return s.syncOdds(ctx, games, now)
			},
		}
		if err := s.refreshWithTimeout(ctx, linesSrc, now); err != nil {
			if !errors.Is(err, odds.ErrNoAPIKey) {
				errs = append(errs, err)
			}
		} else if linesRan {
			report.LinesSynced = true
		}

		if refreshedWeek, refreshedGames, err := s.currentWeek(ctx); err != nil {
			errs = append(errs, fmt.Errorf("reload games after lines: %w", err))
		} else {
			week, games = refreshedWeek, refreshedGames
		}
	}

	freeSources, err := s.freeSourcesFor(ctx, week, games, now)
	if err != nil {
		errs = append(errs, err)
	}
	report.FreeSourcesRun = len(freeSources)
	errs = append(errs, s.runBoundedSources(ctx, now, freeSources, prefillConcurrency)...)

	upcoming := upcomingGamesOnDate(games, now)
	if len(upcoming) == 0 {
		upcoming = upcomingWeekGames(games, now)
	}
	report.UpcomingGames = len(upcoming)

	if s.props != nil && len(upcoming) > 0 {
		synced, skipped, failed, propErrs := s.prefillProps(ctx, upcoming, now)
		report.PropsSynced = synced
		report.PropsSkipped = skipped
		report.PropsFailed = failed
		errs = append(errs, propErrs...)
	} else {
		report.PropsSkipped = len(upcoming)
	}

	if state, err := s.store.SyncState(ctx, OddsLinesSource); err == nil && state.OddsCreditsRemaining != nil {
		report.ParlayRemaining = state.OddsCreditsRemaining
	}

	if !s.stopping() && (len(errs) == 0 || report.LinesSynced || report.PropsSynced > 0) {
		recordCtx, cancelRecord := context.WithTimeout(s.stopCtx, syncTimeout)
		defer cancelRecord()
		if err := s.store.RecordSync(recordCtx, GameDayPrefillSource, now, nil); err != nil {
			errs = append(errs, fmt.Errorf("record %s: %w", GameDayPrefillSource, err))
		}
	}

	return report, errors.Join(errs...)
}

func upcomingWeekGames(games []db.Game, now time.Time) []db.Game {
	upcoming := make([]db.Game, 0, len(games))
	for _, game := range games {
		if game.Status != db.GameFinal && now.Before(game.Kickoff) {
			upcoming = append(upcoming, game)
		}
	}
	return upcoming
}

// runBoundedSources runs sources across at most limit concurrent workers.
func (s *Service) runBoundedSources(ctx context.Context, now time.Time, sources []source, limit int) []error {
	if len(sources) == 0 {
		return nil
	}
	if limit <= 0 || limit > len(sources) {
		limit = len(sources)
	}

	jobs := make(chan source, len(sources))
	for _, src := range sources {
		jobs <- src
	}
	close(jobs)

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)
	for range limit {
		wg.Go(func() {
			for src := range jobs {
				if ctx.Err() != nil {
					return
				}
				if err := s.refreshWithTimeout(ctx, src, now); err != nil {
					mu.Lock()
					errs = append(errs, err)
					mu.Unlock()
				}
			}
		})
	}
	wg.Wait()
	return errs
}

// freeSourcesFor builds the free/unmetered sources that depend on the slate.
// A store read failure is joined but does not hide the sources that were built.
func (s *Service) freeSourcesFor(
	ctx context.Context,
	week db.Week,
	games []db.Game,
	now time.Time,
) ([]source, error) {
	sources := make([]source, 0, len(games)*2+2)
	for _, game := range games {
		sources = append(sources, s.weatherSource(game, now))
	}
	var errs []error
	if s.espn != nil {
		var liveWithLegs map[string]bool
		if week.ID != 0 {
			var err error
			liveWithLegs, err = s.store.LiveGameIDsWithSlipLegs(ctx, week.ID)
			if err != nil {
				errs = append(errs, fmt.Errorf("live games with slip legs: %w", err))
			}
		}
		seenTeams := make(map[string]struct{}, len(games)*2)
		for _, game := range games {
			if game.Status == db.GameFinal {
				continue
			}
			sources = append(sources, s.summarySource(game, now, liveWithLegs[game.ID]))
			if game.HomeESPNID != "" && game.HomeTeam != "" {
				if _, seen := seenTeams[game.HomeESPNID]; !seen {
					seenTeams[game.HomeESPNID] = struct{}{}
					sources = append(sources, s.rosterSource(game.HomeTeam, game.HomeESPNID, games, now))
				}
			}
			if game.AwayESPNID != "" && game.AwayTeam != "" {
				if _, seen := seenTeams[game.AwayESPNID]; !seen {
					seenTeams[game.AwayESPNID] = struct{}{}
					sources = append(sources, s.rosterSource(game.AwayTeam, game.AwayESPNID, games, now))
				}
			}
		}
	}
	if s.projections != nil && week.ID != 0 {
		sources = append(sources, s.nfeloSource(week, games, now))
	}
	if s.nflverse != nil && week.ID != 0 {
		sources = append(sources, s.nflverseSource(week, games, now))
	}
	if week.ID != 0 {
		sources = append(sources, s.gradeSource(now))
	}
	if s.espn == nil || week.ID == 0 {
		return sources, errors.Join(errs...)
	}

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
		sources = append(sources, s.gameLogSource(athleteID, games, now))
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
// for tests that keep the service running; the server calls Shutdown instead.
func (s *Service) wait() {
	s.wg.Wait()
}

// refresh runs one source when it is due, unless a failure has paused it.
func (s *Service) refresh(parent context.Context, src source, now time.Time) {
	_ = s.refreshWithTimeout(parent, src, now)
}

// refreshWithTimeout runs one source under the shared per-source timeout. Kick
// and RunOnce both come through here, so neither can hold a caller's lock
// forever on a stuck store call or fetch. Shutdown cancels it too, so a stop
// ends the fetch instead of waiting out the timeout.
func (s *Service) refreshWithTimeout(parent context.Context, src source, now time.Time) error {
	if err := parent.Err(); err != nil {
		return fmt.Errorf("refresh %s: %w", src.key, err)
	}
	if s.stopping() {
		// A pass that outlived the stop starts no further source work.
		return fmt.Errorf("refresh %s: %w", src.key, context.Canceled)
	}
	ctx, cancel := context.WithTimeout(parent, syncTimeout)
	defer cancel()

	ctx, stopWork := context.WithCancel(ctx)
	defer stopWork()
	unregister := context.AfterFunc(s.stopCtx, stopWork)
	defer unregister()

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
		if s.stopping() {
			return err // a shutdown cancelled the read; do not pause the source.
		}
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
		if s.stopping() {
			// A shutdown cancelled the fetch. That is not a source failure, so
			// it neither logs an outage nor pauses a source the next boot retries.
			return err
		}
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
//
// No production route calls it now that research sheets read cached SQLite only.
// It stays for the sandbox replay tests, which seed a full research sheet.
func (s *Service) SyncGameResearch(ctx context.Context, gameID string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	game, err := s.store.GameByID(ctx, gameID)
	if err != nil {
		return fmt.Errorf("game %s: %w", gameID, err)
	}

	now := s.now()
	hasSlipLegs := false
	if game.Status == db.GameLive && game.WeekID != 0 {
		if liveWithLegs, err := s.store.LiveGameIDsWithSlipLegs(ctx, game.WeekID); err == nil {
			hasSlipLegs = liveWithLegs[game.ID]
		}
	}
	var errs []error
	if err := s.refreshSync(ctx, s.summarySource(game, now, hasSlipLegs), now); err != nil {
		errs = append(errs, err)
	}
	if err := s.refreshSync(ctx, s.oddsPropsSource(game, now), now); err != nil && !errors.Is(err, odds.ErrNoAPIKey) {
		errs = append(errs, err)
	}
	if !now.Before(game.Kickoff) {
		gameSlice := []db.Game{game}
		if game.HomeESPNID != "" && game.HomeTeam != "" {
			if err := s.refreshSync(
				ctx,
				s.rosterSource(game.HomeTeam, game.HomeESPNID, gameSlice, now),
				now,
			); err != nil {
				errs = append(errs, err)
			}
		}
		if game.AwayESPNID != "" && game.AwayTeam != "" {
			if err := s.refreshSync(
				ctx,
				s.rosterSource(game.AwayTeam, game.AwayESPNID, gameSlice, now),
				now,
			); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// SyncPlayerResearch syncs one athlete's game log if stale.
//
// No production route calls it now that player sheets read cached SQLite only.
// It stays for the sandbox replay tests, which seed a full research sheet.
func (s *Service) SyncPlayerResearch(ctx context.Context, athleteID string) error {
	if athleteID == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	_, games, err := s.currentWeek(ctx)
	if err != nil {
		return err
	}

	now := s.now()
	return s.refreshSync(ctx, s.gameLogSource(athleteID, games, now), now)
}

// SyncPickLines refreshes the week's game lines for an open pick screen if
// stale under pick-time rules (15m on game days or within 2h of kickoff, 2h
// midweek).
func (s *Service) SyncPickLines(ctx context.Context, weekID int64) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	return s.syncPickLines(ctx, weekID, s.now(), false)
}

// SyncPickGame refreshes pick-time lines for the week (if stale) and props for
// one upcoming game (if stale under 30m/2h rules).
func (s *Service) SyncPickGame(ctx context.Context, weekID int64, gameID string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	now := s.now()
	var errs []error
	if weekID != 0 {
		if err := s.syncPickLines(ctx, weekID, now, false); err != nil {
			errs = append(errs, err)
		}
	}
	if gameID != "" {
		if err := s.syncPickProps(ctx, weekID, gameID, now, false); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// KickPickLines starts a background pick-time lines refresh for weekID if stale
// and returns immediately. Concurrent callers share one upstream fetch via
// singleflight, and Shutdown waits for or cancels in-flight work.
func (s *Service) KickPickLines(ctx context.Context, weekID int64) {
	if weekID == 0 || (s.lines == nil && s.fallbackLines == nil) {
		return
	}
	s.kickPick(ctx, "kick pick lines", weekID, "", func(runCtx context.Context) error {
		return s.SyncPickLines(runCtx, weekID)
	})
}

// KickPickGame starts a background pick-time refresh for weekID's lines and
// gameID's props if stale, and returns immediately. Concurrent callers share
// upstream fetches via singleflight, and Shutdown waits for or cancels in-flight
// work.
func (s *Service) KickPickGame(ctx context.Context, weekID int64, gameID string) {
	if weekID == 0 && gameID == "" {
		return
	}
	s.kickPick(ctx, "kick pick game", weekID, gameID, func(runCtx context.Context) error {
		return s.SyncPickGame(runCtx, weekID, gameID)
	})
}

// KickSlipExportEnrich starts a background export-link enrichment pass for a
// locked slip's upcoming game-line and prop legs that lack DraftKings links or
// carry non-42 FanDuel marketId prefixes. Concurrent callers share upstream
// fetches via singleflight, and Shutdown waits for or cancels in-flight work.
func (s *Service) KickSlipExportEnrich(ctx context.Context, slipID int64) {
	if slipID == 0 || (s.fallbackProps == nil && s.fallbackLines == nil) {
		return
	}
	s.kickAsync(
		ctx,
		func(err error) {
			s.logger.Warn().
				Err(err).
				Int64("slip_id", slipID).
				Msg("kick slip export enrich")
		},
		func(runCtx context.Context) error {
			runCtx, cancel := context.WithTimeout(runCtx, syncTimeout)
			defer cancel()
			return s.syncSlipExportEnrich(runCtx, slipID)
		},
	)
}

func (s *Service) kickPick(
	ctx context.Context,
	msg string,
	weekID int64,
	gameID string,
	run func(context.Context) error,
) {
	s.kickAsync(
		ctx,
		func(err error) {
			s.logger.Warn().
				Err(err).
				Int64("week_id", weekID).
				Str("game_id", gameID).
				Msg(msg)
		},
		run,
	)
}

func (s *Service) kickAsync(
	ctx context.Context,
	onErr func(error),
	run func(context.Context) error,
) {
	s.stopMu.Lock()
	defer s.stopMu.Unlock()

	if s.stopping() {
		return
	}

	parent := context.WithoutCancel(ctx)
	s.wg.Go(func() {
		runCtx, stopWork := context.WithCancel(parent)
		defer stopWork()
		unregister := context.AfterFunc(s.stopCtx, stopWork)
		defer unregister()

		if err := run(runCtx); err != nil && !s.stopping() {
			onErr(err)
		}
	})
}

func (s *Service) syncSlipExportEnrich(ctx context.Context, slipID int64) error {
	slip, err := s.store.SlipByID(ctx, slipID)
	if err != nil {
		return fmt.Errorf("slip %d: %w", slipID, err)
	}
	if slip.Status != db.SlipLocked {
		return nil
	}

	legs, err := s.store.LegsForSlip(ctx, slipID)
	if err != nil {
		return fmt.Errorf("slip %d legs: %w", slipID, err)
	}
	games, err := s.store.GamesForWeek(ctx, slip.WeekID)
	if err != nil {
		return fmt.Errorf("week %d games: %w", slip.WeekID, err)
	}
	gameByID := make(map[string]db.Game, len(games))
	for _, game := range games {
		gameByID[game.ID] = game
	}

	now := s.now()
	oddsByGame := make(map[string][]db.Odds)
	seenPropGames := make(map[string]bool)
	var propsToEnrich []db.Game
	needsLinesEnrich := false
	for _, leg := range legs {
		if leg.GameID == nil || *leg.GameID == "" {
			continue
		}
		gameID := *leg.GameID
		game, ok := gameByID[gameID]
		if !ok || !now.Before(game.Kickoff) {
			continue
		}
		isProp := db.IsPropLeg(leg)
		if isProp && seenPropGames[gameID] {
			continue
		}
		if !isProp && needsLinesEnrich {
			continue
		}
		oddsRows, loaded := oddsByGame[gameID]
		if !loaded {
			oddsRows, err = s.store.OddsForGame(ctx, gameID)
			if err != nil {
				return fmt.Errorf("game %s odds: %w", gameID, err)
			}
			oddsByGame[gameID] = oddsRows
		}
		if !legNeedsExportEnrich(leg, oddsRows) {
			continue
		}
		if isProp {
			seenPropGames[gameID] = true
			propsToEnrich = append(propsToEnrich, game)
		} else {
			needsLinesEnrich = true
		}
	}

	var errs []error
	if needsLinesEnrich {
		err := s.refreshSync(ctx, s.oddsLinesEnrichSource(slip.WeekID, games, now), now)
		if err != nil && !errors.Is(err, odds.ErrNoAPIKey) {
			errs = append(errs, err)
		}
	}
	for _, game := range propsToEnrich {
		err := s.refreshSync(ctx, s.oddsPropsEnrichSource(game, now, false), now)
		if err != nil && !errors.Is(err, odds.ErrNoAPIKey) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func legNeedsExportEnrich(leg db.Leg, oddsRows []db.Odds) bool {
	for _, book := range []string{"draftkings", "fanduel"} {
		row, ok := db.MatchBookOdds(leg, oddsRows, book)
		if !ok || row.Link == nil || strings.TrimSpace(*row.Link) == "" {
			return true
		}
		if book == "fanduel" && !odds.HasPreferredFanDuelPrefix(*row.Link) {
			return true
		}
	}
	return false
}

// ForcePickRefresh refreshes the week's lines and, when gameID is non-empty,
// that game's props, subject to a 2-minute cooldown per source so rapid
// clicks cannot drain credits.
func (s *Service) ForcePickRefresh(ctx context.Context, weekID int64, gameID string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	now := s.now()
	var errs []error
	if weekID != 0 {
		if err := s.syncPickLines(ctx, weekID, now, true); err != nil {
			errs = append(errs, err)
		}
	}
	if gameID != "" {
		if err := s.syncPickProps(ctx, weekID, gameID, now, true); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *Service) syncPickLines(ctx context.Context, weekID int64, now time.Time, force bool) error {
	if weekID == 0 || (s.lines == nil && s.fallbackLines == nil) {
		return nil
	}
	games, err := s.store.GamesForWeek(ctx, weekID)
	if err != nil {
		return fmt.Errorf("week %d games: %w", weekID, err)
	}
	if len(games) == 0 || allKickedOff(games, now) {
		return nil
	}
	src := source{
		key: OddsLinesSource,
		due: func(_ context.Context, state db.SyncState) (bool, error) {
			if force {
				return pickCooldownStale(state.FetchedAt, now), nil
			}
			return oddsPickLinesStale(state.FetchedAt, now, games), nil
		},
		run: func(ctx context.Context) (bool, error) {
			return s.syncOdds(ctx, games, now)
		},
	}
	err = s.refreshSync(ctx, src, now)
	if errors.Is(err, odds.ErrNoAPIKey) {
		return nil
	}
	return err
}

func (s *Service) syncPickProps(ctx context.Context, weekID int64, gameID string, now time.Time, force bool) error {
	game, err := s.store.GameByID(ctx, gameID)
	if err != nil {
		return fmt.Errorf("game %s: %w", gameID, err)
	}
	if weekID != 0 && game.WeekID != weekID {
		return nil
	}
	if !now.Before(game.Kickoff) {
		return nil
	}
	src := source{
		key: OddsPropsSource(game.ID),
		due: func(_ context.Context, state db.SyncState) (bool, error) {
			if force {
				return pickCooldownStale(state.FetchedAt, now), nil
			}
			return oddsPropsStale(state.FetchedAt, now, game), nil
		},
		run: func(ctx context.Context) (bool, error) {
			return s.syncOddsProps(ctx, game, now)
		},
	}
	err = s.refreshSync(ctx, src, now)
	if errors.Is(err, odds.ErrNoAPIKey) {
		return nil
	}
	return err
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
