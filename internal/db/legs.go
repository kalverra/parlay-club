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

// ErrLegResultInvalid is returned when a leg result is not one the schema
// allows.
var ErrLegResultInvalid = errors.New("that result isn't valid")

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

// PendingFinalLeg pairs a PENDING leg with the FINAL game it belongs to.
type PendingFinalLeg struct {
	Leg       Leg
	GameID    string
	HomeTeam  string
	AwayTeam  string
	HomeScore *int
	AwayScore *int
}

// PendingLegsOnFinalGames returns every PENDING leg whose game is FINAL,
// ordered by game ID then leg ID.
func (s *Store) PendingLegsOnFinalGames(ctx context.Context) ([]PendingFinalLeg, error) {
	rows, err := s.conn().QueryContext(ctx, `
		SELECT
			l.id, l.slip_id, l.user_id, l.game_id, l.market_type, l.selection, l.team,
			l.player_name, l.player_id, l.side, l.line, l.odds, l.result, l.graded_by, l.created_at,
			g.id, g.home_team, g.away_team, g.home_score, g.away_score
		FROM legs l
		JOIN games g ON g.id = l.game_id
		WHERE g.status = ? AND l.result = ?
		ORDER BY g.id, l.id`,
		string(GameFinal), string(parlay.Pending),
	)
	if err != nil {
		return nil, fmt.Errorf("list pending legs on final games: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]PendingFinalLeg, 0)
	for rows.Next() {
		var (
			item      PendingFinalLeg
			gameID    sql.NullString
			team      sql.NullString
			player    sql.NullString
			playerID  sql.NullString
			side      sql.NullString
			line      sql.NullFloat64
			result    string
			gradedBy  sql.NullString
			createdAt string
			homeScore sql.NullInt64
			awayScore sql.NullInt64
		)
		if err := rows.Scan(
			&item.Leg.ID,
			&item.Leg.SlipID,
			&item.Leg.UserID,
			&gameID,
			&item.Leg.MarketType,
			&item.Leg.Selection,
			&team,
			&player,
			&playerID,
			&side,
			&line,
			&item.Leg.Odds,
			&result,
			&gradedBy,
			&createdAt,
			&item.GameID,
			&item.HomeTeam,
			&item.AwayTeam,
			&homeScore,
			&awayScore,
		); err != nil {
			return nil, fmt.Errorf("scan pending leg on final game: %w", err)
		}

		parsed, err := scanTimestamp(createdAt)
		if err != nil {
			return nil, fmt.Errorf("scan pending leg on final game: %w", err)
		}
		item.Leg.GameID = nullableStringPtr(gameID)
		item.Leg.Team = nullableStringPtr(team)
		item.Leg.PlayerName = nullableStringPtr(player)
		item.Leg.PlayerID = nullableStringPtr(playerID)
		item.Leg.Side = nullableStringPtr(side)
		if line.Valid {
			item.Leg.Line = &line.Float64
		}
		item.Leg.Result = parlay.Result(result)
		item.Leg.GradedBy = nullableStringPtr(gradedBy)
		item.Leg.CreatedAt = parsed
		item.HomeScore = nullableIntPtr(homeScore)
		item.AwayScore = nullableIntPtr(awayScore)
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list pending legs on final games: %w", err)
	}
	return out, nil
}

// SetLegResultIfPending updates a leg's result and grader only when the leg is
// still PENDING, returning true if the row was updated.
func (s *Store) SetLegResultIfPending(
	ctx context.Context,
	legID int64,
	result parlay.Result,
	gradedBy string,
) (bool, error) {
	if !result.IsLegResult() {
		return false, fmt.Errorf("set result on leg %d: %w", legID, ErrLegResultInvalid)
	}

	exec, err := s.conn().ExecContext(ctx,
		"UPDATE legs SET result = ?, graded_by = ? WHERE id = ? AND result = ?",
		string(result), gradedBy, legID, string(parlay.Pending),
	)
	if err != nil {
		return false, fmt.Errorf("set result on leg %d: %w", legID, err)
	}
	affected, err := exec.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("set result on leg %d: %w", legID, err)
	}
	return affected > 0, nil
}

// SetLegResult writes one leg's result and the name of who graded it. A
// result the schema does not allow returns ErrLegResultInvalid, and a missing
// leg wraps ErrLegNotFound.
func (s *Store) SetLegResult(
	ctx context.Context,
	legID int64,
	result parlay.Result,
	gradedBy string,
) error {
	if !result.IsLegResult() {
		return fmt.Errorf("set result on leg %d: %w", legID, ErrLegResultInvalid)
	}

	exec, err := s.conn().ExecContext(ctx,
		"UPDATE legs SET result = ?, graded_by = ? WHERE id = ?", string(result), gradedBy, legID)
	if err != nil {
		return fmt.Errorf("set result on leg %d: %w", legID, err)
	}
	affected, err := exec.RowsAffected()
	if err != nil {
		return fmt.Errorf("set result on leg %d: %w", legID, err)
	}
	if affected == 0 {
		return fmt.Errorf("set result on leg %d: %w", legID, ErrLegNotFound)
	}
	return nil
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
