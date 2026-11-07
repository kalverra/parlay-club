package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrGameNotFound is returned when a game lookup matches no row.
var ErrGameNotFound = errors.New("game not found")

// GameKickoff returns a game's kickoff time in UTC. A missing row wraps
// ErrGameNotFound.
func (s *Store) GameKickoff(ctx context.Context, id string) (time.Time, error) {
	var kickoff string
	err := s.conn().QueryRowContext(ctx,
		"SELECT kickoff_utc FROM games WHERE id = ?", id).Scan(&kickoff)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, fmt.Errorf("game %s: %w", id, ErrGameNotFound)
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("game %s: %w", id, err)
	}

	parsed, err := scanTimestamp(kickoff)
	if err != nil {
		return time.Time{}, fmt.Errorf("game %s: %w", id, err)
	}
	return parsed, nil
}

// WeekPast reports whether every game in the week has kicked off by now. A week
// with no games is not past: there is nothing to lock, so its legs stay
// editable. Past weeks skip the kickoff lock so history can be entered.
func (s *Store) WeekPast(ctx context.Context, weekID int64, now time.Time) (bool, error) {
	var (
		games  int
		latest sql.NullString
	)
	err := s.conn().QueryRowContext(ctx,
		"SELECT count(*), MAX(kickoff_utc) FROM games WHERE week_id = ?", weekID).
		Scan(&games, &latest)
	if err != nil {
		return false, fmt.Errorf("week %d kickoff: %w", weekID, err)
	}
	if games == 0 {
		return false, nil
	}

	kickoff, err := scanTimestamp(latest.String)
	if err != nil {
		return false, fmt.Errorf("week %d kickoff: %w", weekID, err)
	}
	return !now.Before(kickoff), nil
}
