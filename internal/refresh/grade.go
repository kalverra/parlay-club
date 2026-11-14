package refresh

import (
	"context"
	"time"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/grader"
	"github.com/kalverra/parlay-club/internal/ingest/archive"
	"github.com/kalverra/parlay-club/internal/ingest/espn"
	"github.com/kalverra/parlay-club/internal/parlay"
	"github.com/kalverra/parlay-club/internal/slips"
)

// gradeSource auto-grades any PENDING legs on FINAL games.
func (s *Service) gradeSource(now time.Time) source {
	return source{
		key: GradeSource,
		due: func(ctx context.Context, _ db.SyncState) (bool, error) {
			pending, err := s.store.PendingLegsOnFinalGames(ctx)
			if err != nil {
				return false, err
			}
			return len(pending) > 0, nil
		},
		run: func(ctx context.Context) (bool, error) {
			return s.syncGrade(ctx, now)
		},
	}
}

// syncGrade grades all pending legs on final games, fetching ESPN box scores
// only for games that have at least one pending player-prop leg.
func (s *Service) syncGrade(ctx context.Context, now time.Time) (bool, error) {
	if s.slips == nil {
		return false, nil
	}

	pending, err := s.store.PendingLegsOnFinalGames(ctx)
	if err != nil {
		return false, err
	}
	if len(pending) == 0 {
		return false, nil
	}

	byGame := make(map[string][]db.PendingFinalLeg)
	var gameOrder []string
	for _, item := range pending {
		if _, seen := byGame[item.GameID]; !seen {
			gameOrder = append(gameOrder, item.GameID)
		}
		byGame[item.GameID] = append(byGame[item.GameID], item)
	}

	var grades []slips.AutoGrade
	for _, gameID := range gameOrder {
		items := byGame[gameID]
		var box *espn.BoxScore
		if hasPlayerProp(items) {
			if s.boxScore == nil {
				continue
			}
			boxKey := SummarySource(gameID)
			if s.paused(boxKey, now) {
				continue
			}
			val, err, _ := s.flight.Do(boxKey, func() (any, error) {
				return s.boxScore.BoxScore(archive.WithKey(ctx, boxKey), gameID)
			})
			if err != nil {
				if !s.stopping() {
					s.logger.Error().Err(err).Str("game_id", gameID).Msg("grade boxscore failed")
					s.backOff(boxKey, now)
				}
				continue
			}
			s.clearBackoff(boxKey)
			fetched := val.(espn.BoxScore)
			box = &fetched
		}

		first := items[0]
		gameScore := grader.GameScore{
			HomeTeam:  first.HomeTeam,
			AwayTeam:  first.AwayTeam,
			HomeScore: first.HomeScore,
			AwayScore: first.AwayScore,
			Final:     true,
		}
		for _, item := range items {
			legInput := grader.LegInput{
				Market:     item.Leg.MarketType,
				Team:       derefString(item.Leg.Team),
				PlayerID:   derefString(item.Leg.PlayerID),
				PlayerName: derefString(item.Leg.PlayerName),
				Side:       derefString(item.Leg.Side),
				Line:       item.Leg.Line,
			}
			result := grader.GradeLeg(legInput, gameScore, box)
			if result != parlay.Pending {
				grades = append(grades, slips.AutoGrade{
					LegID:  item.Leg.ID,
					Result: result,
				})
			}
		}
	}

	if len(grades) == 0 {
		return false, nil
	}
	if err := s.slips.AutoGradeLegs(ctx, grades); err != nil {
		return false, err
	}
	if err := s.store.RecordSync(ctx, GradeSource, now, nil); err != nil {
		return true, err
	}
	return true, nil
}

func hasPlayerProp(items []db.PendingFinalLeg) bool {
	for _, item := range items {
		if grader.IsPlayerMarket(item.Leg.MarketType) {
			return true
		}
	}
	return false
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
