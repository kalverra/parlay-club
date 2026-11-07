package db

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// ErrGroupNameRequired is returned when Create gets a blank name.
var ErrGroupNameRequired = errors.New("group name required")

// inviteTokenBytes is the size of a group invite token: 16 bytes, or 128 bits.
const inviteTokenBytes = 16

// Group is one boundary for slips and their members.
type Group struct {
	ID          int64
	Name        string
	InviteToken string
}

// Create adds a group and makes userID its first member. The name is trimmed,
// and a blank one is refused.
func (s *Store) Create(ctx context.Context, userID int64, name string) (Group, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Group{}, ErrGroupNameRequired
	}
	token, err := newInviteToken()
	if err != nil {
		return Group{}, err
	}

	var group Group
	err = s.Tx(ctx, func(tx *Store) error {
		result, err := tx.conn().ExecContext(ctx,
			"INSERT INTO groups (name, invite_token) VALUES (?, ?)", name, token)
		if err != nil {
			return fmt.Errorf("insert group: %w", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("insert group: %w", err)
		}
		if _, err := tx.conn().ExecContext(ctx,
			"INSERT INTO group_members (group_id, user_id) VALUES (?, ?)", id, userID); err != nil {
			return fmt.Errorf("add member %d to group %d: %w", userID, id, err)
		}
		group = Group{ID: id, Name: name, InviteToken: token}
		return nil
	})
	if err != nil {
		return Group{}, fmt.Errorf("create group %q: %w", name, err)
	}
	return group, nil
}

// ForUser returns the groups userID belongs to, oldest membership first. A
// user with no groups gets an empty slice, not nil.
func (s *Store) ForUser(ctx context.Context, userID int64) ([]Group, error) {
	rows, err := s.conn().QueryContext(ctx, `
		SELECT g.id, g.name, g.invite_token
		FROM groups g
		JOIN group_members m ON m.group_id = g.id
		WHERE m.user_id = ?
		ORDER BY m.joined_at, g.id`, userID)
	if err != nil {
		return nil, fmt.Errorf("list groups for user %d: %w", userID, err)
	}
	defer func() { _ = rows.Close() }()

	groups := make([]Group, 0)
	for rows.Next() {
		var group Group
		if err := rows.Scan(&group.ID, &group.Name, &group.InviteToken); err != nil {
			return nil, fmt.Errorf("list groups for user %d: %w", userID, err)
		}
		groups = append(groups, group)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list groups for user %d: %w", userID, err)
	}
	return groups, nil
}

// IsMember reports whether userID belongs to groupID.
func (s *Store) IsMember(ctx context.Context, groupID, userID int64) (bool, error) {
	var member bool
	err := s.conn().QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM group_members WHERE group_id = ? AND user_id = ?
		)`, groupID, userID).Scan(&member)
	if err != nil {
		return false, fmt.Errorf("check group %d membership for user %d: %w", groupID, userID, err)
	}
	return member, nil
}

// newInviteToken makes a 128-bit random token as lowercase hex.
func newInviteToken() (string, error) {
	raw := make([]byte, inviteTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate invite token: %w", err)
	}
	return hex.EncodeToString(raw), nil
}
