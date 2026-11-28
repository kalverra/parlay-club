package refresh

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
)

// liveLead is how long before kickoff a week enters its live window. Inside
// that window an SSE stream gets the longer deadline.
const liveLead = 30 * time.Minute

// liveWindowTimeout bounds the store read behind LiveWindow. A stream opening
// must not wait on a slow query.
const liveWindowTimeout = 5 * time.Second

const (
	// liveFastAge is the staleness window for viewer-driven live ESPN polling.
	liveFastAge = 20 * time.Second
	// liveRefreshCooldown prevents repeated manual Refresh clicks on a slip.
	liveRefreshCooldown = 10 * time.Second
	// liveSyncTimeout bounds one viewer-driven live refresh pass.
	liveSyncTimeout = 10 * time.Second
	// liveSummaryConcurrency limits concurrent ESPN summary requests per pass.
	liveSummaryConcurrency = 4
)

// liveWindow reports whether any game in the week is live or kicks off within
// liveLead. A final game is never live, even when its kickoff was recent.
func liveWindow(games []db.Game, now time.Time) bool {
	for _, game := range games {
		if game.Status == db.GameLive {
			return true
		}
		if game.Status != db.GameScheduled {
			continue
		}
		if !now.Before(game.Kickoff.Add(-liveLead)) && now.Before(game.Kickoff) {
			return true
		}
	}
	return false
}

// LiveWindow reports whether weekID is in its live window. It is the rule
// behind live.Stream's IsLive. A read error fails closed to the idle deadline,
// because a stream must never stay open on a guess.
func (s *Service) LiveWindow(weekID int64) bool {
	ctx, cancel := context.WithTimeout(context.Background(), liveWindowTimeout)
	defer cancel()

	games, err := s.store.GamesForWeek(ctx, weekID)
	if err != nil {
		s.logger.Error().Err(err).Int64("week", weekID).Msg("read week for live window")
		return false
	}
	return liveWindow(games, s.now())
}

// SyncLive runs a fast, free-API live refresh for one slip (or week when
// slipID is 0): scoreboard, concurrent summaries for LIVE slip games at a
// 20-second staleness window, and immediate auto-grading for FINAL games with
// pending legs. It makes zero paid odds calls.
func (s *Service) SyncLive(ctx context.Context, slipID, weekID int64) error {
	return s.syncLive(ctx, slipID, weekID, s.now(), false)
}

// KickLive starts a non-blocking SyncLive pass in the background, detached from
// any request context, cancelled on Shutdown, and tracked in s.wg.
func (s *Service) KickLive(slipID, weekID int64) {
	if slipID == 0 && weekID == 0 {
		return
	}
	s.kickAsync(
		context.Background(),
		func(err error) {
			s.logger.Warn().
				Err(err).
				Int64("slip_id", slipID).
				Int64("week_id", weekID).
				Msg("kick live")
		},
		func(runCtx context.Context) error {
			return s.SyncLive(runCtx, slipID, weekID)
		},
	)
}

// ForceLiveRefresh runs a live refresh for slipID, bypassing liveFastAge
// subject to a per-slip 10-second cooldown.
func (s *Service) ForceLiveRefresh(ctx context.Context, slipID, weekID int64) error {
	now := s.now()
	if !s.tryClaimLiveCooldown(slipID, now) {
		return nil
	}
	return s.syncLive(ctx, slipID, weekID, now, true)
}

func (s *Service) syncLive(ctx context.Context, slipID, weekID int64, now time.Time, force bool) error {
	if s.stopping() {
		return context.Canceled
	}
	ctx, cancel := context.WithTimeout(ctx, liveSyncTimeout)
	defer cancel()

	ctx, stopWork := context.WithCancel(ctx)
	defer stopWork()
	unregister := context.AfterFunc(s.stopCtx, stopWork)
	defer unregister()

	var errs []error
	if s.scoreboard != nil {
		sbSource := source{
			key: ScoreboardSource,
			due: func(_ context.Context, state db.SyncState) (bool, error) {
				if force {
					return true, nil
				}
				return fastStale(state.FetchedAt, now, liveFastAge), nil
			},
			run: func(ctx context.Context) (bool, error) {
				return s.syncScoreboard(ctx, now)
			},
		}
		if err := s.refreshSync(ctx, sbSource, now); err != nil {
			errs = append(errs, err)
		}
	}

	games, resolvedWeekID, err := s.liveWeekGames(ctx, slipID, weekID)
	if err != nil {
		errs = append(errs, err)
		return errors.Join(errs...)
	}

	gameByID := make(map[string]db.Game, len(games))
	for _, g := range games {
		gameByID[g.ID] = g
	}

	targetGameIDs, needsGrade, err := s.liveSlipTargets(ctx, slipID, resolvedWeekID, games, gameByID)
	if err != nil {
		errs = append(errs, err)
		return errors.Join(errs...)
	}

	if s.espn != nil {
		var summarySources []source
		for gameID := range targetGameIDs {
			game, ok := gameByID[gameID]
			if !ok || game.Status != db.GameLive {
				continue
			}
			summarySources = append(summarySources, s.liveSummarySource(game, now, force))
		}
		if len(summarySources) > 0 {
			errs = append(errs, s.runBoundedSources(ctx, now, summarySources, liveSummaryConcurrency)...)
		}
	}

	if needsGrade {
		if err := s.refreshSync(ctx, s.gradeSource(now), now); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (s *Service) liveWeekGames(ctx context.Context, slipID, weekID int64) ([]db.Game, int64, error) {
	if weekID == 0 && slipID != 0 {
		slip, err := s.store.SlipByID(ctx, slipID)
		if err != nil {
			return nil, 0, fmt.Errorf("slip %d: %w", slipID, err)
		}
		weekID = slip.WeekID
	}
	if weekID == 0 {
		week, games, err := s.currentWeek(ctx)
		if err != nil {
			return nil, 0, err
		}
		return games, week.ID, nil
	}
	games, err := s.store.GamesForWeek(ctx, weekID)
	if err != nil {
		return nil, weekID, fmt.Errorf("week %d games: %w", weekID, err)
	}
	return games, weekID, nil
}

func (s *Service) liveSlipTargets(
	ctx context.Context,
	slipID, weekID int64,
	games []db.Game,
	gameByID map[string]db.Game,
) (map[string]bool, bool, error) {
	if slipID == 0 {
		var liveWithLegs map[string]bool
		if weekID != 0 {
			var err error
			liveWithLegs, err = s.store.LiveGameIDsWithSlipLegs(ctx, weekID)
			if err != nil {
				return nil, false, fmt.Errorf("live games with slip legs: %w", err)
			}
		}
		needsGrade := false
		for _, g := range games {
			if g.Status == db.GameFinal {
				needsGrade = true
				break
			}
		}
		return liveWithLegs, needsGrade, nil
	}

	legs, err := s.store.LegsForSlip(ctx, slipID)
	if err != nil {
		return nil, false, fmt.Errorf("slip %d legs: %w", slipID, err)
	}
	targets := make(map[string]bool, len(legs))
	needsGrade := false
	for _, leg := range legs {
		if leg.GameID == nil || *leg.GameID == "" {
			continue
		}
		gameID := *leg.GameID
		targets[gameID] = true
		if leg.Result == parlay.Pending {
			if g, ok := gameByID[gameID]; ok && g.Status == db.GameFinal {
				needsGrade = true
			}
		}
	}
	return targets, needsGrade, nil
}

func (s *Service) liveSummarySource(game db.Game, now time.Time, force bool) source {
	return source{
		key: SummarySource(game.ID),
		due: func(_ context.Context, state db.SyncState) (bool, error) {
			if force {
				return true, nil
			}
			return fastStale(state.FetchedAt, now, liveFastAge), nil
		},
		run: func(ctx context.Context) (bool, error) {
			return s.syncSummary(ctx, game, now)
		},
	}
}

func (s *Service) tryClaimLiveCooldown(slipID int64, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.liveCooldown == nil {
		s.liveCooldown = make(map[int64]time.Time)
	}
	if until, ok := s.liveCooldown[slipID]; ok && now.Before(until) {
		return false
	}
	s.liveCooldown[slipID] = now.Add(liveRefreshCooldown)
	return true
}

func fastStale(last, now time.Time, maxAge time.Duration) bool {
	if last.IsZero() {
		return true
	}
	return !now.Before(last.Add(maxAge))
}
