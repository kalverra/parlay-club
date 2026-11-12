package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// Errors a caller can act on.
var (
	// ErrGroupNameRequired is returned when Create gets a blank name.
	ErrGroupNameRequired = errors.New("group name required")
	// ErrGroupNotFound is returned when a group lookup matches no row.
	ErrGroupNotFound = errors.New("group not found")
	// ErrInviteNotFound is returned when an invite token names no group.
	ErrInviteNotFound = errors.New("invite not found")
	// ErrLastGroup is returned when someone tries to leave their only group.
	ErrLastGroup = errors.New("cannot leave your only group")
	// ErrNotMember is returned when someone tries to leave a group they are
	// not in.
	ErrNotMember = errors.New("not a group member")
)

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
		if _, err := tx.conn().ExecContext(ctx, `
			INSERT INTO group_slip_schedules (group_id, name, slate_scope, legs_per_member, is_active)
			VALUES (?, 'Main Sunday Slip', 'sunday_main', 2, 1)`, id); err != nil {
			return fmt.Errorf("seed default schedule for group %d: %w", id, err)
		}
		group = Group{ID: id, Name: name, InviteToken: token}
		return nil
	})
	if err != nil {
		return Group{}, fmt.Errorf("create group %q: %w", name, err)
	}
	return group, nil
}

// FirstGroup returns the group created first. The history import targets it
// when the file names no group. A missing row wraps ErrGroupNotFound.
func (s *Store) FirstGroup(ctx context.Context) (Group, error) {
	var group Group
	err := s.conn().QueryRowContext(ctx, `
		SELECT id, name, invite_token
		FROM groups
		ORDER BY id
		LIMIT 1`).
		Scan(&group.ID, &group.Name, &group.InviteToken)
	if errors.Is(err, sql.ErrNoRows) {
		return Group{}, fmt.Errorf("first group: %w", ErrGroupNotFound)
	}
	if err != nil {
		return Group{}, fmt.Errorf("first group: %w", err)
	}
	return group, nil
}

// GroupByID returns one group. A missing row wraps ErrGroupNotFound.
func (s *Store) GroupByID(ctx context.Context, id int64) (Group, error) {
	var group Group
	err := s.conn().QueryRowContext(ctx, `
		SELECT id, name, invite_token
		FROM groups
		WHERE id = ?`, id).
		Scan(&group.ID, &group.Name, &group.InviteToken)
	if errors.Is(err, sql.ErrNoRows) {
		return Group{}, fmt.Errorf("group %d: %w", id, ErrGroupNotFound)
	}
	if err != nil {
		return Group{}, fmt.Errorf("group %d: %w", id, err)
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

// ByToken returns the group an invite token names. A missing or blank token
// wraps ErrInviteNotFound.
func (s *Store) ByToken(ctx context.Context, token string) (Group, error) {
	if token == "" {
		return Group{}, fmt.Errorf("look up invite: %w", ErrInviteNotFound)
	}

	var group Group
	err := s.conn().QueryRowContext(ctx, `
		SELECT id, name, invite_token
		FROM groups
		WHERE invite_token = ?`, token).
		Scan(&group.ID, &group.Name, &group.InviteToken)
	if errors.Is(err, sql.ErrNoRows) {
		// Do not echo the token: it is a bearer secret.
		return Group{}, fmt.Errorf("look up invite: %w", ErrInviteNotFound)
	}
	if err != nil {
		return Group{}, fmt.Errorf("look up invite: %w", err)
	}
	return group, nil
}

// JoinByToken adds userID to the group an invite token names and returns it.
// Joining twice changes nothing and still returns the group. A missing or blank
// token wraps ErrInviteNotFound.
func (s *Store) JoinByToken(ctx context.Context, userID int64, token string) (Group, error) {
	var group Group
	err := s.Tx(ctx, func(tx *Store) error {
		found, err := tx.ByToken(ctx, token)
		if err != nil {
			return err
		}
		if _, err := tx.conn().ExecContext(ctx, `
			INSERT INTO group_members (group_id, user_id) VALUES (?, ?)
			ON CONFLICT (group_id, user_id) DO NOTHING`, found.ID, userID); err != nil {
			return fmt.Errorf("add member %d to group %d: %w", userID, found.ID, err)
		}
		group = found
		return nil
	})
	if err != nil {
		return Group{}, fmt.Errorf("join group by invite: %w", err)
	}
	return group, nil
}

// ResetInvite replaces a group's invite token and returns the new one. The old
// link stops working at once. A missing group wraps ErrGroupNotFound.
func (s *Store) ResetInvite(ctx context.Context, groupID int64) (string, error) {
	token, err := newInviteToken()
	if err != nil {
		return "", err
	}

	result, err := s.conn().ExecContext(ctx,
		"UPDATE groups SET invite_token = ? WHERE id = ?", token, groupID)
	if err != nil {
		return "", fmt.Errorf("reset invite for group %d: %w", groupID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return "", fmt.Errorf("reset invite for group %d: %w", groupID, err)
	}
	if affected == 0 {
		return "", fmt.Errorf("reset invite for group %d: %w", groupID, ErrGroupNotFound)
	}
	return token, nil
}

// Leave removes userID's membership in groupID and nothing else, so their legs
// stay on the group's slips. It refuses to remove someone's last group and
// reports ErrNotMember when they are not in it.
func (s *Store) Leave(ctx context.Context, groupID, userID int64) error {
	err := s.Tx(ctx, func(tx *Store) error {
		member, err := tx.IsMember(ctx, groupID, userID)
		if err != nil {
			return err
		}
		if !member {
			return ErrNotMember
		}

		var count int
		if err := tx.conn().QueryRowContext(ctx,
			"SELECT count(*) FROM group_members WHERE user_id = ?", userID).Scan(&count); err != nil {
			return fmt.Errorf("count groups for user %d: %w", userID, err)
		}
		if count <= 1 {
			return ErrLastGroup
		}

		if _, err := tx.conn().ExecContext(ctx,
			"DELETE FROM group_members WHERE group_id = ? AND user_id = ?", groupID, userID); err != nil {
			return fmt.Errorf("remove user %d from group %d: %w", userID, groupID, err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("leave group %d: %w", groupID, err)
	}
	return nil
}

// Members returns the active people in groupID, oldest membership first.
func (s *Store) Members(ctx context.Context, groupID int64) ([]User, error) {
	rows, err := s.conn().QueryContext(ctx, `
		SELECT u.id, u.name, u.avatar_color, u.home_book, u.active, u.email, u.picture_url
		FROM users u
		JOIN group_members m ON m.user_id = u.id
		WHERE m.group_id = ? AND u.active = 1
		ORDER BY m.joined_at, u.id`, groupID)
	if err != nil {
		return nil, fmt.Errorf("list members of group %d: %w", groupID, err)
	}
	defer func() { _ = rows.Close() }()

	members := make([]User, 0)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("list members of group %d: %w", groupID, err)
		}
		members = append(members, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list members of group %d: %w", groupID, err)
	}
	return members, nil
}

// GroupMembers returns every person in groupID, active or not, oldest
// membership first. The profile uses it so an inactive person can be turned
// back on; pickers and new slips still use Members.
func (s *Store) GroupMembers(ctx context.Context, groupID int64) ([]User, error) {
	rows, err := s.conn().QueryContext(ctx, `
		SELECT u.id, u.name, u.avatar_color, u.home_book, u.active, u.email, u.picture_url
		FROM users u
		JOIN group_members m ON m.user_id = u.id
		WHERE m.group_id = ?
		ORDER BY m.joined_at, u.id`, groupID)
	if err != nil {
		return nil, fmt.Errorf("list all members of group %d: %w", groupID, err)
	}
	defer func() { _ = rows.Close() }()

	members := make([]User, 0)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("list all members of group %d: %w", groupID, err)
		}
		members = append(members, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list all members of group %d: %w", groupID, err)
	}
	return members, nil
}

// newInviteToken makes a 128-bit random token as lowercase hex.
func newInviteToken() (string, error) {
	raw := make([]byte, inviteTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate invite token: %w", err)
	}
	return hex.EncodeToString(raw), nil
}
