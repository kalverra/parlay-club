package refresh

import (
	"context"
	"fmt"
	"time"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/nfelo"
)

// nfeloSource syncs the latest regular-season week's model projections.
func (s *Service) nfeloSource(week db.Week, games []db.Game, now time.Time) source {
	return source{
		key: NfeloProjectionsSource,
		due: func(_ context.Context, state db.SyncState) (bool, error) {
			return nfeloStale(state.FetchedAt, now, week, games), nil
		},
		run: func(ctx context.Context) (bool, error) {
			return s.syncNfelo(ctx, week, games, now)
		},
	}
}

// syncNfelo fetches the whole CSV, writes the latest week's matching rows, and
// records the fetch. A failed fetch leaves every stored row and fetched_at
// alone, so the caller backs off and tries again.
func (s *Service) syncNfelo(
	ctx context.Context,
	week db.Week,
	games []db.Game,
	now time.Time,
) (bool, error) {
	if week.SeasonType != regularSeasonType || len(games) == 0 || allKickedOff(games, now) {
		return false, nil
	}

	projections, err := s.projections.Projections(ctx)
	if err != nil {
		return false, fmt.Errorf("nfelo projections: %w", err)
	}

	rows := s.nfeloRows(projections, week, games, now)
	changed := false
	err = s.store.Tx(ctx, func(tx *db.Store) error {
		rowsChanged, err := tx.UpsertNfeloGames(ctx, rows)
		if err != nil {
			return err
		}
		changed = rowsChanged
		return tx.RecordSync(ctx, NfeloProjectionsSource, now, nil)
	})
	if err != nil {
		return changed, err
	}
	if changed {
		s.publishWeek(ctx, week.ID)
	}
	return changed, nil
}

// nfeloProjectionKey identifies one game in the CSV. Matching on all four parts
// keeps another season's or week's similar matchup from landing on this week.
type nfeloProjectionKey struct {
	season, week int
	home, away   string
}

// nfeloRows maps the CSV's rows onto the week's games. A game with no matching
// row writes nothing and keeps whatever projection it already had. Rows for
// this week that match no game are logged at debug, which is how a new nfelo
// team code shows up.
func (s *Service) nfeloRows(
	projections []nfelo.Projection,
	week db.Week,
	games []db.Game,
	now time.Time,
) []db.NfeloGame {
	byKey := make(map[nfeloProjectionKey]nfelo.Projection, len(projections))
	for _, projection := range projections {
		byKey[nfeloProjectionKey{
			season: projection.Season,
			week:   projection.Week,
			home:   projection.Home,
			away:   projection.Away,
		}] = projection
	}

	matched := make(map[nfeloProjectionKey]bool, len(games))
	rows := make([]db.NfeloGame, 0, len(games))
	for _, game := range games {
		key := nfeloProjectionKey{
			season: week.Season,
			week:   week.WeekNum,
			home:   game.HomeTeam,
			away:   game.AwayTeam,
		}
		projection, ok := byKey[key]
		if !ok {
			continue
		}
		matched[key] = true
		rows = append(rows, db.NfeloGame{
			GameID:      game.ID,
			HomeSpread:  projection.HomeSpread,
			HomeWinProb: projection.HomeWinProb,
			HomeQB:      projection.HomeQB,
			AwayQB:      projection.AwayQB,
			HomeQBAdj:   projection.HomeQBAdj,
			AwayQBAdj:   projection.AwayQBAdj,
			UpdatedAt:   now,
		})
	}

	for key := range byKey {
		if key.season != week.Season || key.week != week.WeekNum || matched[key] {
			continue
		}
		s.logger.Debug().
			Int("season", key.season).
			Int("week", key.week).
			Str("home", key.home).
			Str("away", key.away).
			Msg("nfelo projection has no game in the week")
	}
	return rows
}
