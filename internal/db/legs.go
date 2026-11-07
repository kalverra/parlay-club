package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/kalverra/parlay-club/internal/parlay"
)

// ErrLegNotFound is returned when a leg lookup matches no row.
var ErrLegNotFound = errors.New("leg not found")

// DefaultLegOdds is the American price the leg form starts with.
const DefaultLegOdds = -110

// Leg is one pick on a slip.
type Leg struct {
	ID         int64
	SlipID     int64
	UserID     int64
	GameID     *string
	MarketType string
	Selection  string
	Team       *string
	PlayerName *string
	PlayerID   *string
	Side       *string
	Line       *float64
	Odds       int
	Result     parlay.Result
	GradedBy   *string
	CreatedAt  time.Time
}

// legColumns is the column list every leg query scans, in scanLeg order.
const legColumns = `
	id, slip_id, user_id, game_id, market_type, selection, team, player_name,
	player_id, side, line, odds, result, graded_by, created_at`

// CreateLeg adds one pick to a slip. An odds value of 0 takes DefaultLegOdds,
// and a new leg always starts PENDING. The returned leg has its stored ID and
// creation time.
func (s *Store) CreateLeg(ctx context.Context, leg Leg) (Leg, error) {
	odds := leg.Odds
	if odds == 0 {
		odds = DefaultLegOdds
	}

	result, err := s.conn().ExecContext(ctx, `
		INSERT INTO legs (
			slip_id, user_id, game_id, market_type, selection, team, player_name,
			player_id, side, line, odds
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		leg.SlipID,
		leg.UserID,
		nullableString(leg.GameID),
		leg.MarketType,
		leg.Selection,
		nullableString(leg.Team),
		nullableString(leg.PlayerName),
		nullableString(leg.PlayerID),
		nullableString(leg.Side),
		nullableFloat(leg.Line),
		odds,
	)
	if err != nil {
		return Leg{}, fmt.Errorf("insert leg on slip %d: %w", leg.SlipID, err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Leg{}, fmt.Errorf("insert leg on slip %d: %w", leg.SlipID, err)
	}
	return s.LegByID(ctx, id)
}

// LegByID returns the leg with the given ID. A missing row wraps
// ErrLegNotFound.
func (s *Store) LegByID(ctx context.Context, id int64) (Leg, error) {
	leg, err := scanLeg(s.conn().QueryRowContext(ctx,
		"SELECT"+legColumns+" FROM legs WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return Leg{}, fmt.Errorf("leg %d: %w", id, ErrLegNotFound)
	}
	if err != nil {
		return Leg{}, fmt.Errorf("leg %d: %w", id, err)
	}
	return leg, nil
}

// LegsForSlip returns a slip's legs in the order they were added.
func (s *Store) LegsForSlip(ctx context.Context, slipID int64) ([]Leg, error) {
	rows, err := s.conn().QueryContext(ctx,
		"SELECT"+legColumns+" FROM legs WHERE slip_id = ? ORDER BY id", slipID)
	if err != nil {
		return nil, fmt.Errorf("list legs of slip %d: %w", slipID, err)
	}
	defer func() { _ = rows.Close() }()

	legs := make([]Leg, 0)
	for rows.Next() {
		leg, err := scanLeg(rows)
		if err != nil {
			return nil, fmt.Errorf("list legs of slip %d: %w", slipID, err)
		}
		legs = append(legs, leg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list legs of slip %d: %w", slipID, err)
	}
	return legs, nil
}

// UpdateLeg changes a leg's pick fields. The owner, slip, result, and graded_by
// stay put. A missing leg wraps ErrLegNotFound.
func (s *Store) UpdateLeg(ctx context.Context, leg Leg) (Leg, error) {
	result, err := s.conn().ExecContext(ctx, `
		UPDATE legs SET
			game_id = ?, market_type = ?, selection = ?, team = ?, player_name = ?,
			player_id = ?, side = ?, line = ?, odds = ?
		WHERE id = ?`,
		nullableString(leg.GameID),
		leg.MarketType,
		leg.Selection,
		nullableString(leg.Team),
		nullableString(leg.PlayerName),
		nullableString(leg.PlayerID),
		nullableString(leg.Side),
		nullableFloat(leg.Line),
		leg.Odds,
		leg.ID,
	)
	if err != nil {
		return Leg{}, fmt.Errorf("update leg %d: %w", leg.ID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Leg{}, fmt.Errorf("update leg %d: %w", leg.ID, err)
	}
	if affected == 0 {
		return Leg{}, fmt.Errorf("update leg %d: %w", leg.ID, ErrLegNotFound)
	}
	return s.LegByID(ctx, leg.ID)
}

// DeleteLeg removes one leg. A missing leg wraps ErrLegNotFound.
func (s *Store) DeleteLeg(ctx context.Context, id int64) error {
	result, err := s.conn().ExecContext(ctx, "DELETE FROM legs WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete leg %d: %w", id, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete leg %d: %w", id, err)
	}
	if affected == 0 {
		return fmt.Errorf("delete leg %d: %w", id, ErrLegNotFound)
	}
	return nil
}

// scanLeg reads the columns legColumns selects, in order.
func scanLeg(row rowScanner) (Leg, error) {
	var (
		leg       Leg
		gameID    sql.NullString
		team      sql.NullString
		player    sql.NullString
		playerID  sql.NullString
		side      sql.NullString
		line      sql.NullFloat64
		result    string
		gradedBy  sql.NullString
		createdAt string
	)
	if err := row.Scan(
		&leg.ID,
		&leg.SlipID,
		&leg.UserID,
		&gameID,
		&leg.MarketType,
		&leg.Selection,
		&team,
		&player,
		&playerID,
		&side,
		&line,
		&leg.Odds,
		&result,
		&gradedBy,
		&createdAt,
	); err != nil {
		return Leg{}, err
	}

	parsed, err := scanTimestamp(createdAt)
	if err != nil {
		return Leg{}, err
	}
	leg.GameID = nullableStringPtr(gameID)
	leg.Team = nullableStringPtr(team)
	leg.PlayerName = nullableStringPtr(player)
	leg.PlayerID = nullableStringPtr(playerID)
	leg.Side = nullableStringPtr(side)
	if line.Valid {
		leg.Line = &line.Float64
	}
	leg.Result = parlay.Result(result)
	leg.GradedBy = nullableStringPtr(gradedBy)
	leg.CreatedAt = parsed
	return leg, nil
}

// nullableString turns a pointer into the value or NULL for a query argument.
func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

// nullableFloat turns a pointer into the value or NULL for a query argument.
func nullableFloat(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

// nullableStringPtr turns a scanned NULL-able string back into a pointer.
func nullableStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}
