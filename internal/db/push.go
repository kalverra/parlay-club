package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// PushSubscription is one browser push endpoint registered by a user. A person
// can register several devices, so user_id is not unique.
type PushSubscription struct {
	ID        int64
	UserID    int64
	Endpoint  string
	P256dh    string
	Auth      string
	CreatedAt time.Time
}

// pushSubscriptionColumns is the column list for push subscription queries,
// matching scanPushSubscription order.
const pushSubscriptionColumns = `id, user_id, endpoint, p256dh, auth, created_at`

// SavePushSubscription writes sub, replacing the row with the same endpoint.
// An endpoint that already exists is reassigned to sub.UserID, so the last
// person to subscribe on a shared device owns it. When the user then has more
// than limit endpoints, the oldest ones are deleted. A non-positive limit
// leaves every row in place.
func (s *Store) SavePushSubscription(ctx context.Context, sub PushSubscription, limit int) error {
	return s.Tx(ctx, func(tx *Store) error {
		if _, err := tx.conn().ExecContext(ctx, `
			INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(endpoint) DO UPDATE SET
				user_id    = excluded.user_id,
				p256dh     = excluded.p256dh,
				auth       = excluded.auth,
				created_at = strftime('%Y-%m-%dT%H:%M:%SZ','now')`,
			sub.UserID, sub.Endpoint, sub.P256dh, sub.Auth); err != nil {
			return fmt.Errorf("save push subscription: %w", err)
		}
		if limit <= 0 {
			return nil
		}
		if _, err := tx.conn().ExecContext(ctx, `
			DELETE FROM push_subscriptions
			WHERE user_id = ?
			  AND id NOT IN (
				SELECT id FROM push_subscriptions
				WHERE user_id = ?
				ORDER BY created_at DESC, id DESC
				LIMIT ?
			  )`, sub.UserID, sub.UserID, limit); err != nil {
			return fmt.Errorf("trim push subscriptions: %w", err)
		}
		return nil
	})
}

// DeletePushSubscription removes endpoint when it belongs to userID and reports
// whether a row went away. Another user's endpoint is left alone.
func (s *Store) DeletePushSubscription(ctx context.Context, userID int64, endpoint string) (bool, error) {
	result, err := s.conn().ExecContext(ctx, `
		DELETE FROM push_subscriptions WHERE user_id = ? AND endpoint = ?`, userID, endpoint)
	if err != nil {
		return false, fmt.Errorf("delete push subscription: %w", err)
	}
	removed, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("delete push subscription: %w", err)
	}
	return removed > 0, nil
}

// DeletePushSubscriptionByID removes one subscription, which is how a push
// service's 404 or 410 retires an endpoint.
func (s *Store) DeletePushSubscriptionByID(ctx context.Context, id int64) error {
	if _, err := s.conn().ExecContext(ctx, `DELETE FROM push_subscriptions WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete push subscription %d: %w", id, err)
	}
	return nil
}

// ListPushSubscriptions returns a user's endpoints, oldest first.
func (s *Store) ListPushSubscriptions(ctx context.Context, userID int64) ([]PushSubscription, error) {
	rows, err := s.readConn().QueryContext(ctx, `
		SELECT `+pushSubscriptionColumns+`
		FROM push_subscriptions
		WHERE user_id = ?
		ORDER BY id`, userID)
	if err != nil {
		return nil, fmt.Errorf("list push subscriptions: %w", err)
	}
	return scanPushSubscriptions(rows)
}

// PushSubscriptionsForUsers returns the endpoints of the given users who are
// still active. Inactive people keep their history but get no pushes.
func (s *Store) PushSubscriptionsForUsers(
	ctx context.Context,
	userIDs []int64,
) ([]PushSubscription, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(userIDs)), ",")
	args := make([]any, 0, len(userIDs))
	for _, id := range userIDs {
		args = append(args, id)
	}
	// #nosec G202 -- placeholders contains only static "?,..." tokens.
	query := `SELECT ps.id, ps.user_id, ps.endpoint, ps.p256dh, ps.auth, ps.created_at
		FROM push_subscriptions ps
		JOIN users ON users.id = ps.user_id
		WHERE ps.user_id IN (` + placeholders + `)
		  AND users.active = 1
		ORDER BY ps.id`
	rows, err := s.readConn().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list push subscriptions for users: %w", err)
	}
	return scanPushSubscriptions(rows)
}

// scanPushSubscriptions reads every row the push subscription queries select.
func scanPushSubscriptions(rows *sql.Rows) ([]PushSubscription, error) {
	defer func() { _ = rows.Close() }()

	subs := make([]PushSubscription, 0)
	for rows.Next() {
		var (
			sub       PushSubscription
			createdAt string
		)
		if err := rows.Scan(
			&sub.ID,
			&sub.UserID,
			&sub.Endpoint,
			&sub.P256dh,
			&sub.Auth,
			&createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan push subscription: %w", err)
		}
		parsed, err := scanTimestamp(createdAt)
		if err != nil {
			return nil, err
		}
		sub.CreatedAt = parsed
		subs = append(subs, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan push subscription: %w", err)
	}
	return subs, nil
}
