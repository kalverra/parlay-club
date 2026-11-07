package refresh

import (
	"context"
	"time"

	"github.com/kalverra/parlay-club/internal/db"
)

// liveLead is how long before kickoff a week enters its live window. Inside
// that window an SSE stream gets the longer deadline.
const liveLead = 30 * time.Minute

// liveWindowTimeout bounds the store read behind LiveWindow. A stream opening
// must not wait on a slow query.
const liveWindowTimeout = 5 * time.Second

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
