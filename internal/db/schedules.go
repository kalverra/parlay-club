package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Errors for group slip schedules.
var (
	// ErrScheduleNotFound is returned when a schedule lookup matches no row.
	ErrScheduleNotFound = errors.New("schedule not found")
	// ErrScheduleNameRequired is returned when a schedule name is blank.
	ErrScheduleNameRequired = errors.New("schedule name required")
	// ErrScheduleNameTaken is returned when a schedule with that name already exists in the group.
	ErrScheduleNameTaken = errors.New("schedule name already taken")
)

// GroupSlipSchedule represents a recurring slip template for a group.
type GroupSlipSchedule struct {
	ID            int64
	GroupID       int64
	Name          string
	SlateScope    SlateScope
	LegsPerMember int
	IsActive      bool
	CreatedAt     time.Time
}

const scheduleColumns = ` id, group_id, name, slate_scope, legs_per_member, is_active, created_at `

// scanSchedule reads one schedule row from a query result.
func scanSchedule(row rowScanner) (GroupSlipSchedule, error) {
	var (
		sched      GroupSlipSchedule
		slateScope string
		createdAt  string
	)
	if err := row.Scan(
		&sched.ID,
		&sched.GroupID,
		&sched.Name,
		&slateScope,
		&sched.LegsPerMember,
		&sched.IsActive,
		&createdAt,
	); err != nil {
		return GroupSlipSchedule{}, err
	}

	parsed, err := scanTimestamp(createdAt)
	if err != nil {
		return GroupSlipSchedule{}, err
	}
	sched.SlateScope = SlateScope(slateScope)
	sched.CreatedAt = parsed
	return sched, nil
}

// GroupSlipScheduleByID returns one schedule by its ID. A missing row wraps
// ErrScheduleNotFound.
func (s *Store) GroupSlipScheduleByID(ctx context.Context, id int64) (GroupSlipSchedule, error) {
	sched, err := scanSchedule(s.conn().QueryRowContext(ctx, `
		SELECT`+scheduleColumns+`
		FROM group_slip_schedules
		WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return GroupSlipSchedule{}, fmt.Errorf("schedule %d: %w", id, ErrScheduleNotFound)
	}
	if err != nil {
		return GroupSlipSchedule{}, fmt.Errorf("schedule %d: %w", id, err)
	}
	return sched, nil
}

// GroupSlipSchedules returns all schedules for a group, ordered by ID.
func (s *Store) GroupSlipSchedules(ctx context.Context, groupID int64) ([]GroupSlipSchedule, error) {
	return s.groupSlipSchedules(ctx, groupID, false)
}

// ActiveGroupSlipSchedules returns all active schedules for a group, ordered by ID.
func (s *Store) ActiveGroupSlipSchedules(ctx context.Context, groupID int64) ([]GroupSlipSchedule, error) {
	return s.groupSlipSchedules(ctx, groupID, true)
}

func (s *Store) groupSlipSchedules(ctx context.Context, groupID int64, onlyActive bool) ([]GroupSlipSchedule, error) {
	query := `SELECT` + scheduleColumns + `FROM group_slip_schedules WHERE group_id = ?`
	if onlyActive {
		query += ` AND is_active = 1`
	}
	query += ` ORDER BY id`

	rows, err := s.conn().QueryContext(ctx, query, groupID)
	if err != nil {
		return nil, fmt.Errorf("list schedules for group %d: %w", groupID, err)
	}
	defer func() { _ = rows.Close() }()

	schedules := make([]GroupSlipSchedule, 0)
	for rows.Next() {
		sched, err := scanSchedule(rows)
		if err != nil {
			return nil, fmt.Errorf("list schedules for group %d: %w", groupID, err)
		}
		schedules = append(schedules, sched)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list schedules for group %d: %w", groupID, err)
	}
	return schedules, nil
}

// CreateGroupSlipSchedule adds an active recurring slip schedule for a group.
// The name is trimmed and cannot be blank. If slateScope is empty, it defaults
// to SlateScopeSundayMain.
func (s *Store) CreateGroupSlipSchedule(
	ctx context.Context,
	groupID int64,
	name string,
	slateScope SlateScope,
	legsPerMember int,
) (GroupSlipSchedule, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return GroupSlipSchedule{}, ErrScheduleNameRequired
	}
	if slateScope == "" {
		slateScope = SlateScopeSundayMain
	}
	if !slateScope.Valid() {
		return GroupSlipSchedule{}, ErrSlateScopeInvalid
	}
	if legsPerMember < 1 || legsPerMember > 5 {
		return GroupSlipSchedule{}, ErrLegsPerMemberRange
	}

	result, err := s.conn().ExecContext(ctx, `
		INSERT INTO group_slip_schedules (group_id, name, slate_scope, legs_per_member, is_active)
		VALUES (?, ?, ?, ?, 1)`,
		groupID, name, string(slateScope), legsPerMember,
	)
	if err != nil {
		if isConstraintUnique(err) {
			return GroupSlipSchedule{}, fmt.Errorf(
				"create schedule %q for group %d: %w",
				name,
				groupID,
				ErrScheduleNameTaken,
			)
		}
		return GroupSlipSchedule{}, fmt.Errorf("create schedule %q for group %d: %w", name, groupID, err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return GroupSlipSchedule{}, fmt.Errorf("create schedule %q for group %d: %w", name, groupID, err)
	}

	return s.GroupSlipScheduleByID(ctx, id)
}

// UpdateGroupSlipSchedule updates the name, slate scope, and legs per member of
// a schedule. A missing schedule wraps ErrScheduleNotFound.
func (s *Store) UpdateGroupSlipSchedule(
	ctx context.Context,
	id int64,
	name string,
	slateScope SlateScope,
	legsPerMember int,
) (GroupSlipSchedule, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return GroupSlipSchedule{}, ErrScheduleNameRequired
	}
	if slateScope == "" {
		slateScope = SlateScopeSundayMain
	}
	if !slateScope.Valid() {
		return GroupSlipSchedule{}, ErrSlateScopeInvalid
	}
	if legsPerMember < 1 || legsPerMember > 5 {
		return GroupSlipSchedule{}, ErrLegsPerMemberRange
	}

	result, err := s.conn().ExecContext(ctx, `
		UPDATE group_slip_schedules
		SET name = ?, slate_scope = ?, legs_per_member = ?
		WHERE id = ?`,
		name, string(slateScope), legsPerMember, id,
	)
	if err != nil {
		if isConstraintUnique(err) {
			return GroupSlipSchedule{}, fmt.Errorf("update schedule %d: %w", id, ErrScheduleNameTaken)
		}
		return GroupSlipSchedule{}, fmt.Errorf("update schedule %d: %w", id, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return GroupSlipSchedule{}, fmt.Errorf("update schedule %d: %w", id, err)
	}
	if affected == 0 {
		return GroupSlipSchedule{}, fmt.Errorf("update schedule %d: %w", id, ErrScheduleNotFound)
	}

	return s.GroupSlipScheduleByID(ctx, id)
}

// SetGroupSlipScheduleActive sets the is_active flag of a schedule.
// A missing schedule wraps ErrScheduleNotFound.
func (s *Store) SetGroupSlipScheduleActive(ctx context.Context, id int64, active bool) (GroupSlipSchedule, error) {
	activeInt := 0
	if active {
		activeInt = 1
	}
	result, err := s.conn().ExecContext(ctx, `
		UPDATE group_slip_schedules
		SET is_active = ?
		WHERE id = ?`, activeInt, id)
	if err != nil {
		return GroupSlipSchedule{}, fmt.Errorf("set schedule %d active: %w", id, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return GroupSlipSchedule{}, fmt.Errorf("set schedule %d active: %w", id, err)
	}
	if affected == 0 {
		return GroupSlipSchedule{}, fmt.Errorf("schedule %d: %w", id, ErrScheduleNotFound)
	}

	return s.GroupSlipScheduleByID(ctx, id)
}

// DeleteGroupSlipSchedule removes a schedule. A missing schedule wraps ErrScheduleNotFound.
func (s *Store) DeleteGroupSlipSchedule(ctx context.Context, id int64) error {
	result, err := s.conn().ExecContext(ctx, `
		DELETE FROM group_slip_schedules WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete schedule %d: %w", id, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete schedule %d: %w", id, err)
	}
	if affected == 0 {
		return fmt.Errorf("delete schedule %d: %w", id, ErrScheduleNotFound)
	}
	return nil
}
