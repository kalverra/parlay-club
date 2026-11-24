package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
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

// Weeks returns every known week, newest first.
func (s *Store) Weeks(ctx context.Context) ([]Week, error) {
	rows, err := s.readConn().QueryContext(ctx, `
		SELECT id, season, season_type, week_num
		FROM weeks
		ORDER BY season DESC, season_type DESC, week_num DESC`)
	if err != nil {
		return nil, fmt.Errorf("list weeks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	weeks := make([]Week, 0)
	for rows.Next() {
		var week Week
		if err := rows.Scan(&week.ID, &week.Season, &week.SeasonType, &week.WeekNum); err != nil {
			return nil, fmt.Errorf("list weeks: %w", err)
		}
		weeks = append(weeks, week)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list weeks: %w", err)
	}
	return weeks, nil
}

// UpcomingWeeks returns weeks that have at least one game with kickoff > now,
// ordered soonest-first (by id ASC).
func (s *Store) UpcomingWeeks(ctx context.Context, now time.Time) ([]Week, error) {
	rows, err := s.readConn().QueryContext(ctx, `
		SELECT w.id, w.season, w.season_type, w.week_num
		FROM weeks w
		WHERE EXISTS (
			SELECT 1 FROM games g
			WHERE g.week_id = w.id AND g.kickoff_utc > ?
		)
		ORDER BY w.id ASC`, formatTimestamp(now))
	if err != nil {
		return nil, fmt.Errorf("list upcoming weeks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	weeks := make([]Week, 0)
	for rows.Next() {
		var week Week
		if err := rows.Scan(&week.ID, &week.Season, &week.SeasonType, &week.WeekNum); err != nil {
			return nil, fmt.Errorf("list upcoming weeks: %w", err)
		}
		weeks = append(weeks, week)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list upcoming weeks: %w", err)
	}
	return weeks, nil
}

// LatestWeek returns the newest week on record. Until the slate sync names a
// current week, the home page opens the latest one. A missing row wraps
// ErrWeekNotFound.
func (s *Store) LatestWeek(ctx context.Context) (Week, error) {
	var week Week
	err := s.readConn().QueryRowContext(ctx, `
		SELECT id, season, season_type, week_num
		FROM weeks
		ORDER BY season DESC, season_type DESC, week_num DESC
		LIMIT 1`).
		Scan(&week.ID, &week.Season, &week.SeasonType, &week.WeekNum)
	if errors.Is(err, sql.ErrNoRows) {
		return Week{}, fmt.Errorf("latest week: %w", ErrWeekNotFound)
	}
	if err != nil {
		return Week{}, fmt.Errorf("latest week: %w", err)
	}
	return week, nil
}

// WeekByID returns the week with the given ID. A missing row wraps
// ErrWeekNotFound.
func (s *Store) WeekByID(ctx context.Context, id int64) (Week, error) {
	var week Week
	err := s.readConn().QueryRowContext(ctx, `
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
