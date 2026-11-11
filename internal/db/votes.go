package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// LegVote is one person's vote on one leg. Value is 1 for an up vote and -1 for
// a down vote. A cleared vote has no row.
type LegVote struct {
	LegID     int64
	UserID    int64
	Value     int
	UpdatedAt time.Time
}

// LegVoteValue returns one person's vote on a leg, or 0 when they have not
// voted.
func (s *Store) LegVoteValue(ctx context.Context, legID, userID int64) (int, error) {
	var value int
	err := s.conn().QueryRowContext(ctx,
		"SELECT value FROM leg_votes WHERE leg_id = ? AND user_id = ?", legID, userID).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("leg %d vote by user %d: %w", legID, userID, err)
	}
	return value, nil
}

// SetLegVote writes one person's vote on a leg, replacing an earlier one. The
// table checks that value is -1 or 1.
func (s *Store) SetLegVote(ctx context.Context, legID, userID int64, value int, at time.Time) error {
	_, err := s.conn().ExecContext(ctx, `
		INSERT INTO leg_votes (leg_id, user_id, value, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (leg_id, user_id) DO UPDATE SET
			value = excluded.value,
			updated_at = excluded.updated_at`,
		legID, userID, value, at.UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("vote leg %d by user %d: %w", legID, userID, err)
	}
	return nil
}

// ClearLegVote removes one person's vote on a leg. Clearing a vote that is not
// there changes nothing.
func (s *Store) ClearLegVote(ctx context.Context, legID, userID int64) error {
	_, err := s.conn().ExecContext(ctx,
		"DELETE FROM leg_votes WHERE leg_id = ? AND user_id = ?", legID, userID)
	if err != nil {
		return fmt.Errorf("clear leg %d vote by user %d: %w", legID, userID, err)
	}
	return nil
}

// DeleteLegVotes removes every vote on a leg, in the same transaction as the
// edit that changed its pick.
func (s *Store) DeleteLegVotes(ctx context.Context, legID int64) error {
	_, err := s.conn().ExecContext(ctx, "DELETE FROM leg_votes WHERE leg_id = ?", legID)
	if err != nil {
		return fmt.Errorf("delete leg %d votes: %w", legID, err)
	}
	return nil
}

// DeleteUserVotesForSlip removes every vote one person cast on a slip's legs.
// Removing a member calls it in the same transaction so their tally leaves with
// them and rejoining does not bring the old votes back.
func (s *Store) DeleteUserVotesForSlip(ctx context.Context, slipID, userID int64) error {
	_, err := s.conn().ExecContext(ctx, `
		DELETE FROM leg_votes
		WHERE user_id = ? AND leg_id IN (SELECT id FROM legs WHERE slip_id = ?)`,
		userID, slipID)
	if err != nil {
		return fmt.Errorf("delete user %d votes on slip %d: %w", userID, slipID, err)
	}
	return nil
}

// VotesForSlip returns every vote on a slip's legs, ordered by leg and voter.
func (s *Store) VotesForSlip(ctx context.Context, slipID int64) ([]LegVote, error) {
	rows, err := s.conn().QueryContext(ctx, `
		SELECT v.leg_id, v.user_id, v.value, v.updated_at
		FROM leg_votes v
		JOIN legs l ON l.id = v.leg_id
		WHERE l.slip_id = ?
		ORDER BY v.leg_id, v.user_id`, slipID)
	if err != nil {
		return nil, fmt.Errorf("list votes on slip %d: %w", slipID, err)
	}
	defer func() { _ = rows.Close() }()

	votes := make([]LegVote, 0)
	for rows.Next() {
		var (
			vote      LegVote
			updatedAt string
		)
		if err := rows.Scan(&vote.LegID, &vote.UserID, &vote.Value, &updatedAt); err != nil {
			return nil, fmt.Errorf("list votes on slip %d: %w", slipID, err)
		}
		vote.UpdatedAt, err = scanTimestamp(updatedAt)
		if err != nil {
			return nil, fmt.Errorf("list votes on slip %d: %w", slipID, err)
		}
		votes = append(votes, vote)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list votes on slip %d: %w", slipID, err)
	}
	return votes, nil
}
