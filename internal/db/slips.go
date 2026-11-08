package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kalverra/parlay-club/internal/parlay"
)

// Errors a caller can act on.
var (
	// ErrSlipNotFound is returned when a slip lookup matches no row.
	ErrSlipNotFound = errors.New("slip not found")
	// ErrSlipNameRequired is returned when CreateSlip or UpdateSlip gets a blank name.
	ErrSlipNameRequired = errors.New("slip name required")
	// ErrLegsPerMemberRange is returned when legs_per_member is outside 1-5.
	ErrLegsPerMemberRange = errors.New("legs per member must be between 1 and 5")
	// ErrSlipMembersRequired is returned when a new slip has no members.
	ErrSlipMembersRequired = errors.New("slip needs at least one member")
	// ErrSlipMemberNotFound is returned when a person is not on the slip.
	ErrSlipMemberNotFound = errors.New("slip member not found")
	// ErrSlipStatusInvalid is returned when a status is not one the schema allows.
	ErrSlipStatusInvalid = errors.New("invalid slip status")
	// ErrSlipResultInvalid is returned when a result is not one a slip can carry.
	ErrSlipResultInvalid = errors.New("invalid slip result")
)

// SlipStatus is where a slip sits between open editing and grading.
type SlipStatus string

// The statuses a slip can carry.
const (
	SlipOpen   SlipStatus = "OPEN"
	SlipLocked SlipStatus = "LOCKED"
	SlipGraded SlipStatus = "GRADED"
)

// Valid reports whether the status is one of SlipOpen, SlipLocked, or
// SlipGraded.
func (s SlipStatus) Valid() bool {
	switch s {
	case SlipOpen, SlipLocked, SlipGraded:
		return true
	default:
		return false
	}
}

// Slip is one shared parlay inside a group and a week.
type Slip struct {
	ID            int64
	GroupID       int64
	WeekID        int64
	Name          string
	LegsPerMember int
	Status        SlipStatus
	Result        parlay.Result
	CreatedAt     time.Time
}

// SlipMember is one person on a slip, with the time they locked in if they
// have.
type SlipMember struct {
	User     User
	LockedAt *time.Time
}

// slipColumns is the column list every slip query scans, in scanSlip order.
const slipColumns = `
	id, group_id, week_id, name, legs_per_member, status, result, created_at`

// CreateSlip adds a slip and its members in one transaction. The name is
// trimmed, and a blank one is refused. At least one member is required. Repeat
// member IDs are ignored.
func (s *Store) CreateSlip(
	ctx context.Context,
	groupID, weekID int64,
	name string,
	legsPerMember int,
	memberIDs []int64,
) (Slip, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Slip{}, ErrSlipNameRequired
	}
	if legsPerMember < 1 || legsPerMember > 5 {
		return Slip{}, ErrLegsPerMemberRange
	}
	if len(memberIDs) == 0 {
		return Slip{}, ErrSlipMembersRequired
	}

	var slip Slip
	err := s.Tx(ctx, func(tx *Store) error {
		result, err := tx.conn().ExecContext(ctx, `
			INSERT INTO slips (group_id, week_id, name, legs_per_member)
			VALUES (?, ?, ?, ?)`, groupID, weekID, name, legsPerMember)
		if err != nil {
			return fmt.Errorf("insert slip: %w", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("insert slip: %w", err)
		}
		for _, userID := range memberIDs {
			if _, err := tx.conn().ExecContext(ctx, `
				INSERT INTO slip_members (slip_id, user_id) VALUES (?, ?)
				ON CONFLICT (slip_id, user_id) DO NOTHING`, id, userID); err != nil {
				return fmt.Errorf("add member %d to slip %d: %w", userID, id, err)
			}
		}

		slip, err = tx.SlipByID(ctx, id)
		return err
	})
	if err != nil {
		return Slip{}, fmt.Errorf("create slip %q: %w", name, err)
	}
	return slip, nil
}

// SlipByID returns the slip with the given ID. A missing row wraps
// ErrSlipNotFound.
func (s *Store) SlipByID(ctx context.Context, id int64) (Slip, error) {
	slip, err := scanSlip(s.conn().QueryRowContext(ctx,
		"SELECT"+slipColumns+" FROM slips WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return Slip{}, fmt.Errorf("slip %d: %w", id, ErrSlipNotFound)
	}
	if err != nil {
		return Slip{}, fmt.Errorf("slip %d: %w", id, err)
	}
	return slip, nil
}

// SlipsForWeek returns a group's slips in one week, newest first.
func (s *Store) SlipsForWeek(ctx context.Context, groupID, weekID int64) ([]Slip, error) {
	rows, err := s.conn().QueryContext(ctx,
		"SELECT"+slipColumns+`
		FROM slips
		WHERE group_id = ? AND week_id = ?
		ORDER BY created_at DESC, id DESC`, groupID, weekID)
	if err != nil {
		return nil, fmt.Errorf("list slips for group %d in week %d: %w", groupID, weekID, err)
	}
	defer func() { _ = rows.Close() }()

	slips := make([]Slip, 0)
	for rows.Next() {
		slip, err := scanSlip(rows)
		if err != nil {
			return nil, fmt.Errorf("list slips for group %d in week %d: %w", groupID, weekID, err)
		}
		slips = append(slips, slip)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list slips for group %d in week %d: %w", groupID, weekID, err)
	}
	return slips, nil
}

// SlipIDsForWeek returns every slip ID in a week across all groups, oldest
// first. Refresh uses it to signal every affected slip after a board change.
func (s *Store) SlipIDsForWeek(ctx context.Context, weekID int64) ([]int64, error) {
	rows, err := s.conn().QueryContext(ctx,
		"SELECT id FROM slips WHERE week_id = ? ORDER BY id", weekID)
	if err != nil {
		return nil, fmt.Errorf("list slip ids for week %d: %w", weekID, err)
	}
	defer func() { _ = rows.Close() }()

	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("list slip ids for week %d: %w", weekID, err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list slip ids for week %d: %w", weekID, err)
	}
	return ids, nil
}

// NewestSlipForWeek returns the most recently created slip in a group's week.
// A missing row wraps ErrSlipNotFound.
func (s *Store) NewestSlipForWeek(ctx context.Context, groupID, weekID int64) (Slip, error) {
	slip, err := scanSlip(s.conn().QueryRowContext(ctx,
		"SELECT"+slipColumns+`
		FROM slips
		WHERE group_id = ? AND week_id = ?
		ORDER BY created_at DESC, id DESC
		LIMIT 1`, groupID, weekID))
	if errors.Is(err, sql.ErrNoRows) {
		return Slip{}, fmt.Errorf("newest slip for group %d in week %d: %w", groupID, weekID, ErrSlipNotFound)
	}
	if err != nil {
		return Slip{}, fmt.Errorf("newest slip for group %d in week %d: %w", groupID, weekID, err)
	}
	return slip, nil
}

// PreviousSlip returns the latest slip for a group in weekID or any earlier
// week. It is the slip a create form copies its defaults from, so a second slip
// in the same week copies that week's first slip. A group with no slip that old
// wraps ErrSlipNotFound.
func (s *Store) PreviousSlip(ctx context.Context, groupID, weekID int64) (Slip, error) {
	slip, err := scanSlip(s.conn().QueryRowContext(ctx,
		"SELECT"+slipColumns+`
		FROM slips
		WHERE group_id = ? AND week_id <= ?
		ORDER BY week_id DESC, created_at DESC, id DESC
		LIMIT 1`, groupID, weekID))
	if errors.Is(err, sql.ErrNoRows) {
		return Slip{}, fmt.Errorf("previous slip for group %d before week %d: %w", groupID, weekID, ErrSlipNotFound)
	}
	if err != nil {
		return Slip{}, fmt.Errorf("previous slip for group %d before week %d: %w", groupID, weekID, err)
	}
	return slip, nil
}

// SlipMembers returns a slip's people and their lock times, ordered by user ID.
func (s *Store) SlipMembers(ctx context.Context, slipID int64) ([]SlipMember, error) {
	rows, err := s.conn().QueryContext(ctx, `
		SELECT u.id, u.name, u.avatar_color, u.home_book, u.active, u.email, u.picture_url, m.locked_at
		FROM users u
		JOIN slip_members m ON m.user_id = u.id
		WHERE m.slip_id = ?
		ORDER BY u.id`, slipID)
	if err != nil {
		return nil, fmt.Errorf("list members of slip %d: %w", slipID, err)
	}
	defer func() { _ = rows.Close() }()

	members := make([]SlipMember, 0)
	for rows.Next() {
		var (
			member   SlipMember
			color    string
			homeBook sql.NullString
			email    sql.NullString
			picture  sql.NullString
			lockedAt sql.NullString
		)
		if err := rows.Scan(
			&member.User.ID,
			&member.User.Name,
			&color,
			&homeBook,
			&member.User.Active,
			&email,
			&picture,
			&lockedAt,
		); err != nil {
			return nil, fmt.Errorf("list members of slip %d: %w", slipID, err)
		}
		member.User.AvatarColor = AvatarColor(color)
		if homeBook.Valid {
			member.User.HomeBook = &homeBook.String
		}
		if email.Valid {
			member.User.Email = email.String
		}
		if picture.Valid {
			member.User.PictureURL = picture.String
		}
		if lockedAt.Valid {
			locked, err := scanTimestamp(lockedAt.String)
			if err != nil {
				return nil, fmt.Errorf("list members of slip %d: %w", slipID, err)
			}
			member.LockedAt = &locked
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list members of slip %d: %w", slipID, err)
	}
	return members, nil
}

// SlipMemberIDs returns a slip's user IDs in order.
func (s *Store) SlipMemberIDs(ctx context.Context, slipID int64) ([]int64, error) {
	rows, err := s.conn().QueryContext(ctx, `
		SELECT user_id FROM slip_members WHERE slip_id = ? ORDER BY user_id`, slipID)
	if err != nil {
		return nil, fmt.Errorf("list member IDs of slip %d: %w", slipID, err)
	}
	defer func() { _ = rows.Close() }()

	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("list member IDs of slip %d: %w", slipID, err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list member IDs of slip %d: %w", slipID, err)
	}
	return ids, nil
}

// AddSlipMember puts a person on a slip. Adding someone already there changes
// nothing.
func (s *Store) AddSlipMember(ctx context.Context, slipID, userID int64) error {
	if _, err := s.conn().ExecContext(ctx, `
		INSERT INTO slip_members (slip_id, user_id) VALUES (?, ?)
		ON CONFLICT (slip_id, user_id) DO NOTHING`, slipID, userID); err != nil {
		return fmt.Errorf("add user %d to slip %d: %w", userID, slipID, err)
	}
	return nil
}

// RemoveSlipMember takes a person off a slip and leaves their legs alone. A
// person who is not on the slip wraps ErrSlipMemberNotFound.
func (s *Store) RemoveSlipMember(ctx context.Context, slipID, userID int64) error {
	result, err := s.conn().ExecContext(ctx,
		"DELETE FROM slip_members WHERE slip_id = ? AND user_id = ?", slipID, userID)
	if err != nil {
		return fmt.Errorf("remove user %d from slip %d: %w", userID, slipID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("remove user %d from slip %d: %w", userID, slipID, err)
	}
	if affected == 0 {
		return fmt.Errorf("remove user %d from slip %d: %w", userID, slipID, ErrSlipMemberNotFound)
	}
	return nil
}

// SetSlipMemberLock sets or clears one member's locked_at. A nil time clears
// it. A person who is not on the slip wraps ErrSlipMemberNotFound.
func (s *Store) SetSlipMemberLock(ctx context.Context, slipID, userID int64, lockedAt *time.Time) error {
	var value any
	if lockedAt != nil {
		value = lockedAt.UTC().Format(time.RFC3339)
	}

	result, err := s.conn().ExecContext(ctx,
		"UPDATE slip_members SET locked_at = ? WHERE slip_id = ? AND user_id = ?", value, slipID, userID)
	if err != nil {
		return fmt.Errorf("set lock for user %d on slip %d: %w", userID, slipID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("set lock for user %d on slip %d: %w", userID, slipID, err)
	}
	if affected == 0 {
		return fmt.Errorf("set lock for user %d on slip %d: %w", userID, slipID, ErrSlipMemberNotFound)
	}
	return nil
}

// UpdateSlip changes a slip's name and legs per member. The name is trimmed and
// checked like CreateSlip. A missing slip wraps ErrSlipNotFound.
func (s *Store) UpdateSlip(ctx context.Context, slipID int64, name string, legsPerMember int) (Slip, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Slip{}, ErrSlipNameRequired
	}
	if legsPerMember < 1 || legsPerMember > 5 {
		return Slip{}, ErrLegsPerMemberRange
	}

	result, err := s.conn().ExecContext(ctx, `
		UPDATE slips SET name = ?, legs_per_member = ?
		WHERE id = ?`, name, legsPerMember, slipID)
	if err != nil {
		return Slip{}, fmt.Errorf("update slip %d: %w", slipID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Slip{}, fmt.Errorf("update slip %d: %w", slipID, err)
	}
	if affected == 0 {
		return Slip{}, fmt.Errorf("update slip %d: %w", slipID, ErrSlipNotFound)
	}
	return s.SlipByID(ctx, slipID)
}

// SetSlipStatus writes a slip's status. An unknown status returns
// ErrSlipStatusInvalid, and a missing slip wraps ErrSlipNotFound.
func (s *Store) SetSlipStatus(ctx context.Context, slipID int64, status SlipStatus) error {
	if !status.Valid() {
		return fmt.Errorf("set status on slip %d: %w", slipID, ErrSlipStatusInvalid)
	}

	result, err := s.conn().ExecContext(ctx,
		"UPDATE slips SET status = ? WHERE id = ?", string(status), slipID)
	if err != nil {
		return fmt.Errorf("set status on slip %d: %w", slipID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("set status on slip %d: %w", slipID, err)
	}
	if affected == 0 {
		return fmt.Errorf("set status on slip %d: %w", slipID, ErrSlipNotFound)
	}
	return nil
}

// SetSlipGrade writes a slip's result and status together. Grading always
// recomputes both from the legs, so they belong in one write. An unknown result
// or status is refused, and a missing slip wraps ErrSlipNotFound.
func (s *Store) SetSlipGrade(
	ctx context.Context,
	slipID int64,
	result parlay.Result,
	status SlipStatus,
) error {
	if !result.IsSlipResult() {
		return fmt.Errorf("set grade on slip %d: %w", slipID, ErrSlipResultInvalid)
	}
	if !status.Valid() {
		return fmt.Errorf("set grade on slip %d: %w", slipID, ErrSlipStatusInvalid)
	}

	exec, err := s.conn().ExecContext(ctx,
		"UPDATE slips SET result = ?, status = ? WHERE id = ?", string(result), string(status), slipID)
	if err != nil {
		return fmt.Errorf("set grade on slip %d: %w", slipID, err)
	}
	affected, err := exec.RowsAffected()
	if err != nil {
		return fmt.Errorf("set grade on slip %d: %w", slipID, err)
	}
	if affected == 0 {
		return fmt.Errorf("set grade on slip %d: %w", slipID, ErrSlipNotFound)
	}
	return nil
}

// scanSlip reads the columns slipColumns selects, in order.
func scanSlip(row rowScanner) (Slip, error) {
	var (
		slip      Slip
		status    string
		result    string
		createdAt string
	)
	if err := row.Scan(
		&slip.ID,
		&slip.GroupID,
		&slip.WeekID,
		&slip.Name,
		&slip.LegsPerMember,
		&status,
		&result,
		&createdAt,
	); err != nil {
		return Slip{}, err
	}

	parsed, err := scanTimestamp(createdAt)
	if err != nil {
		return Slip{}, err
	}
	slip.Status = SlipStatus(status)
	slip.Result = parlay.Result(result)
	slip.CreatedAt = parsed
	return slip, nil
}

// scanTimestamp reads one of the RFC 3339 UTC strings the schema stores.
func scanTimestamp(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse timestamp %q: %w", value, err)
	}
	return parsed, nil
}
