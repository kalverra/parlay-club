package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// Wager is one group member's logged bet on a slip.
type Wager struct {
	SlipID   int64
	UserID   int64
	Book     *string
	Stake    float64
	Odds     int
	BoostPct float64
}

const wagerColumns = "slip_id, user_id, book, stake, odds, boost_pct"

// UpsertWager inserts or updates one member's logged bet on a slip.
func (s *Store) UpsertWager(ctx context.Context, wager Wager) (Wager, error) {
	wager.Book = normalizeOptionalText(wager.Book)
	_, err := s.conn().ExecContext(ctx, `
		INSERT INTO wagers (slip_id, user_id, book, stake, odds, boost_pct)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (slip_id, user_id) DO UPDATE SET
			book = excluded.book,
			stake = excluded.stake,
			odds = excluded.odds,
			boost_pct = excluded.boost_pct`,
		wager.SlipID, wager.UserID, wager.Book, wager.Stake, wager.Odds, wager.BoostPct)
	if err != nil {
		return Wager{}, fmt.Errorf("upsert wager for slip %d user %d: %w", wager.SlipID, wager.UserID, err)
	}
	return wager, nil
}

// DeleteWager removes one member's logged bet on a slip. Removing a wager that
// does not exist changes nothing.
func (s *Store) DeleteWager(ctx context.Context, slipID, userID int64) error {
	_, err := s.conn().ExecContext(ctx,
		"DELETE FROM wagers WHERE slip_id = ? AND user_id = ?", slipID, userID)
	if err != nil {
		return fmt.Errorf("delete wager for slip %d user %d: %w", slipID, userID, err)
	}
	return nil
}

// WagerForUser returns one member's logged bet on a slip, and reports whether
// one exists.
func (s *Store) WagerForUser(ctx context.Context, slipID, userID int64) (Wager, bool, error) {
	row := s.conn().QueryRowContext(ctx, `
		SELECT `+wagerColumns+`
		FROM wagers
		WHERE slip_id = ? AND user_id = ?`, slipID, userID)
	wager, err := scanWager(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Wager{}, false, nil
	}
	if err != nil {
		return Wager{}, false, fmt.Errorf("get wager for slip %d user %d: %w", slipID, userID, err)
	}
	return wager, true, nil
}

// WagersForSlip returns every logged wager on a slip, ordered by user ID.
func (s *Store) WagersForSlip(ctx context.Context, slipID int64) ([]Wager, error) {
	rows, err := s.conn().QueryContext(ctx, `
		SELECT `+wagerColumns+`
		FROM wagers
		WHERE slip_id = ?
		ORDER BY user_id`, slipID)
	if err != nil {
		return nil, fmt.Errorf("list wagers for slip %d: %w", slipID, err)
	}
	defer func() { _ = rows.Close() }()

	wagers := make([]Wager, 0)
	for rows.Next() {
		wager, err := scanWager(rows)
		if err != nil {
			return nil, fmt.Errorf("list wagers for slip %d: %w", slipID, err)
		}
		wagers = append(wagers, wager)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list wagers for slip %d: %w", slipID, err)
	}
	return wagers, nil
}

// WagersForGroupSeason returns every logged wager on slips in groupID during
// season, ordered by slip ID and user ID.
func (s *Store) WagersForGroupSeason(ctx context.Context, groupID int64, season int) ([]Wager, error) {
	rows, err := s.conn().QueryContext(ctx, `
		SELECT w.slip_id, w.user_id, w.book, w.stake, w.odds, w.boost_pct
		FROM wagers w
		JOIN slips s ON s.id = w.slip_id
		JOIN weeks wk ON wk.id = s.week_id
		WHERE s.group_id = ? AND wk.season = ?
		ORDER BY w.slip_id, w.user_id`, groupID, season)
	if err != nil {
		return nil, fmt.Errorf("list wagers for group %d season %d: %w", groupID, season, err)
	}
	defer func() { _ = rows.Close() }()

	wagers := make([]Wager, 0)
	for rows.Next() {
		wager, err := scanWager(rows)
		if err != nil {
			return nil, fmt.Errorf("list wagers for group %d season %d: %w", groupID, season, err)
		}
		wagers = append(wagers, wager)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list wagers for group %d season %d: %w", groupID, season, err)
	}
	return wagers, nil
}

func scanWager(scanner interface{ Scan(...any) error }) (Wager, error) {
	var (
		wager Wager
		book  sql.NullString
	)
	if err := scanner.Scan(
		&wager.SlipID,
		&wager.UserID,
		&book,
		&wager.Stake,
		&wager.Odds,
		&wager.BoostPct,
	); err != nil {
		return Wager{}, err
	}
	wager.Book = nullableStringPtr(book)
	return wager, nil
}

func normalizeOptionalText(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
