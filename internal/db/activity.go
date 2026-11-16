package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrActivityMessageRequired is returned when AddActivity gets a blank message.
var ErrActivityMessageRequired = errors.New("activity message required")

// Activity is one line in a slip's history. UserID is nil for system entries.
type Activity struct {
	ID        int64
	SlipID    int64
	UserID    *int64
	Message   string
	CreatedAt time.Time
}

// activityColumns is the column list every activity query scans, in scanActivity
// order.
const activityColumns = `
	id, slip_id, user_id, message, created_at`

// AddActivity writes one history entry for a slip. A nil userID records a
// system entry. The message is trimmed, and a blank one is refused.
func (s *Store) AddActivity(
	ctx context.Context,
	slipID int64,
	userID *int64,
	message string,
) (Activity, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return Activity{}, ErrActivityMessageRequired
	}

	result, err := s.conn().ExecContext(ctx, `
		INSERT INTO activity (slip_id, user_id, message) VALUES (?, ?, ?)`,
		slipID, nullableInt64(userID), message)
	if err != nil {
		return Activity{}, fmt.Errorf("add activity to slip %d: %w", slipID, err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Activity{}, fmt.Errorf("add activity to slip %d: %w", slipID, err)
	}
	return s.activityByID(ctx, id)
}

// ActivityForSlip returns a slip's history, newest first.
func (s *Store) ActivityForSlip(ctx context.Context, slipID int64) ([]Activity, error) {
	rows, err := s.readConn().QueryContext(ctx,
		"SELECT"+activityColumns+" FROM activity WHERE slip_id = ? ORDER BY id DESC", slipID)
	if err != nil {
		return nil, fmt.Errorf("list activity for slip %d: %w", slipID, err)
	}
	defer func() { _ = rows.Close() }()

	entries := make([]Activity, 0)
	for rows.Next() {
		entry, err := scanActivity(rows)
		if err != nil {
			return nil, fmt.Errorf("list activity for slip %d: %w", slipID, err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list activity for slip %d: %w", slipID, err)
	}
	return entries, nil
}

// activityByID returns the entry AddActivity just wrote.
func (s *Store) activityByID(ctx context.Context, id int64) (Activity, error) {
	entry, err := scanActivity(s.conn().QueryRowContext(ctx,
		"SELECT"+activityColumns+" FROM activity WHERE id = ?", id))
	if err != nil {
		return Activity{}, fmt.Errorf("activity %d: %w", id, err)
	}
	return entry, nil
}

// scanActivity reads the columns activityColumns selects, in order.
func scanActivity(row rowScanner) (Activity, error) {
	var (
		entry     Activity
		userID    sql.NullInt64
		createdAt string
	)
	if err := row.Scan(&entry.ID, &entry.SlipID, &userID, &entry.Message, &createdAt); err != nil {
		return Activity{}, err
	}

	parsed, err := scanTimestamp(createdAt)
	if err != nil {
		return Activity{}, err
	}
	if userID.Valid {
		entry.UserID = &userID.Int64
	}
	entry.CreatedAt = parsed
	return entry, nil
}

// nullableInt64 turns a pointer into the value or NULL for a query argument.
func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}
