package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrWeekNotFound is returned when a week lookup matches no row.
var ErrWeekNotFound = errors.New("week not found")

// Week is one ESPN NFL week.
type Week struct {
	ID         int64
	Season     int
	SeasonType int
	WeekNum    int
}

// WeekID returns the primary key the schema assigns a week:
// season*1000 + season_type*100 + week_num.
func WeekID(season, seasonType, weekNum int) int64 {
	return int64(season)*1000 + int64(seasonType)*100 + int64(weekNum)
}

// UpsertWeek inserts a week, or returns the existing one with the same season,
// season type, and week number. The parts identify the row, so a second sync of
// the same slate changes nothing.
func (s *Store) UpsertWeek(ctx context.Context, season, seasonType, weekNum int) (Week, error) {
	id := WeekID(season, seasonType, weekNum)
	if _, err := s.conn().ExecContext(ctx, `
		INSERT INTO weeks (id, season, season_type, week_num)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (season, season_type, week_num) DO NOTHING`,
		id, season, seasonType, weekNum); err != nil {
		return Week{}, fmt.Errorf("upsert week %d-%d-%d: %w", season, seasonType, weekNum, err)
	}
	return s.WeekByID(ctx, id)
}

// WeekByID returns the week with the given ID. A missing row wraps
// ErrWeekNotFound.
func (s *Store) WeekByID(ctx context.Context, id int64) (Week, error) {
	var week Week
	err := s.conn().QueryRowContext(ctx, `
		SELECT id, season, season_type, week_num
		FROM weeks
		WHERE id = ?`, id).
		Scan(&week.ID, &week.Season, &week.SeasonType, &week.WeekNum)
	if errors.Is(err, sql.ErrNoRows) {
		return Week{}, fmt.Errorf("week %d: %w", id, ErrWeekNotFound)
	}
	if err != nil {
		return Week{}, fmt.Errorf("week %d: %w", id, err)
	}
	return week, nil
}
