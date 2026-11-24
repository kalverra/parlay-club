package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kalverra/parlay-club/internal/parlay"
	"github.com/kalverra/parlay-club/internal/tz"
)

// eastern is the zone the kickoff windows and slates use.
var eastern = tz.Eastern

// Errors a caller can act on.
var (
	// ErrSlipNotFound is returned when a slip lookup matches no row.
	ErrSlipNotFound = errors.New("slip not found")
	// ErrSlipNameRequired is returned when CreateSlip or UpdateSlip gets a blank name.
	ErrSlipNameRequired = errors.New("slip name required")
	// ErrSlipNameTaken is returned when a slip with that name already exists in the group and week.
	ErrSlipNameTaken = errors.New("slip name already taken")
	// ErrSlateScopeInvalid is returned when a slate scope is not one the schema allows.
	ErrSlateScopeInvalid = errors.New("invalid slate scope")
	// ErrLegsPerMemberRange is returned when legs_per_member is outside 1-5.
	ErrLegsPerMemberRange = errors.New("legs per member must be between 1 and 5")
	// ErrSlipMembersRequired is returned when a new slip has no members.
	ErrSlipMembersRequired = errors.New("slip needs at least one member")
	// ErrSlipGamesRequired is returned when a custom slip has no eligible games.
	ErrSlipGamesRequired = errors.New("select at least one game")
	// ErrSlipMemberNotFound is returned when a person is not on the slip.
	ErrSlipMemberNotFound = errors.New("slip member not found")
	// ErrSlipStatusInvalid is returned when a status is not one the schema allows.
	ErrSlipStatusInvalid = errors.New("invalid slip status")
	// ErrSlipResultInvalid is returned when a result is not one a slip can carry.
	ErrSlipResultInvalid = errors.New("invalid slip result")
)

// SlateScope identifies which games in a week belong on a slip.
type SlateScope string

// The slate scopes a slip can carry.
const (
	SlateScopeThursday   SlateScope = "thursday"
	SlateScopeSundayMain SlateScope = "sunday_main"
	SlateScopeWeekend    SlateScope = "weekend"
	SlateScopeFullWeek   SlateScope = "full_week"
	SlateScopeCustom     SlateScope = "custom"
)

// Valid reports whether the slate scope is one of SlateScopeThursday,
// SlateScopeSundayMain, SlateScopeWeekend, SlateScopeFullWeek, or
// SlateScopeCustom.
func (s SlateScope) Valid() bool {
	switch s {
	case SlateScopeThursday, SlateScopeSundayMain, SlateScopeWeekend, SlateScopeFullWeek, SlateScopeCustom:
		return true
	default:
		return false
	}
}

// ValidForSchedule reports whether the slate scope can be used on a recurring
// group slip schedule (excludes SlateScopeCustom).
func (s SlateScope) ValidForSchedule() bool {
	switch s {
	case SlateScopeThursday, SlateScopeSundayMain, SlateScopeWeekend, SlateScopeFullWeek:
		return true
	default:
		return false
	}
}

// Label returns the human-readable description of the slate scope.
func (s SlateScope) Label() string {
	switch s {
	case SlateScopeThursday:
		return "Thursday Slate"
	case SlateScopeWeekend:
		return "Weekend Slate"
	case SlateScopeFullWeek:
		return "Full Week"
	case SlateScopeCustom:
		return "Custom"
	case SlateScopeSundayMain:
		return "Sunday Slate"
	default:
		return SlateScopeSundayMain.Label()
	}
}

// Matches reports whether kickoff falls within this slate scope in Eastern
// time. If the slate scope is unset or unknown, it defaults to Sunday Main.
func (s SlateScope) Matches(kickoff time.Time) bool {
	day := kickoff.In(eastern).Weekday()
	switch s {
	case SlateScopeThursday:
		return day == time.Thursday
	case SlateScopeWeekend:
		return day == time.Saturday || day == time.Sunday || day == time.Monday
	case SlateScopeFullWeek:
		return true
	case SlateScopeSundayMain:
		return day == time.Sunday || day == time.Monday
	default:
		return SlateScopeSundayMain.Matches(kickoff)
	}
}

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
	ScheduleID    *int64
	Name          string
	SlateScope    SlateScope
	GameIDs       []string // populated when SlateScope == SlateScopeCustom
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
	id, group_id, week_id, schedule_id, name, slate_scope, legs_per_member, status, result, created_at`

// CreateSlip adds a slip and its members in one transaction. The name is
// trimmed, and a blank one is refused. If slateScope is empty, it defaults to
// SlateScopeSundayMain. An invalid slateScope returns ErrSlateScopeInvalid.
// At least one member is required. Repeat member IDs are ignored.
func (s *Store) CreateSlip(
	ctx context.Context,
	groupID, weekID int64,
	scheduleID *int64,
	name string,
	slateScope SlateScope,
	legsPerMember int,
	memberIDs []int64,
) (Slip, error) {
	return s.CreateSlipWithGames(ctx, groupID, weekID, scheduleID, name, slateScope, legsPerMember, memberIDs, nil)
}

// CreateSlipWithGames adds a slip, its members, and (when slateScope is
// SlateScopeCustom) its eligible game IDs in one transaction. Custom slips
// require at least one valid game in weekID, with duplicate game IDs ignored.
func (s *Store) CreateSlipWithGames(
	ctx context.Context,
	groupID, weekID int64,
	scheduleID *int64,
	name string,
	slateScope SlateScope,
	legsPerMember int,
	memberIDs []int64,
	gameIDs []string,
) (Slip, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Slip{}, ErrSlipNameRequired
	}
	if slateScope == "" {
		slateScope = SlateScopeSundayMain
	}
	if !slateScope.Valid() {
		return Slip{}, ErrSlateScopeInvalid
	}
	if legsPerMember < 1 || legsPerMember > 5 {
		return Slip{}, ErrLegsPerMemberRange
	}
	if len(memberIDs) == 0 {
		return Slip{}, ErrSlipMembersRequired
	}

	var dedupedGames []string
	if slateScope == SlateScopeCustom {
		dedupedGames = dedupeGameIDs(gameIDs)
		if len(dedupedGames) == 0 {
			return Slip{}, ErrSlipGamesRequired
		}
	}

	var slip Slip
	err := s.Tx(ctx, func(tx *Store) error {
		if slateScope == SlateScopeCustom {
			if err := tx.verifyGamesInWeek(ctx, weekID, dedupedGames); err != nil {
				return err
			}
		}

		result, err := tx.conn().ExecContext(ctx, `
			INSERT INTO slips (group_id, week_id, schedule_id, name, slate_scope, legs_per_member)
			VALUES (?, ?, ?, ?, ?, ?)`, groupID, weekID, scheduleID, name, string(slateScope), legsPerMember)
		if err != nil {
			if isConstraintUnique(err) {
				return fmt.Errorf("insert slip: %w", ErrSlipNameTaken)
			}
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
		for _, gameID := range dedupedGames {
			if _, err := tx.conn().ExecContext(ctx, `
				INSERT INTO slip_games (slip_id, game_id) VALUES (?, ?)
				ON CONFLICT (slip_id, game_id) DO NOTHING`, id, gameID); err != nil {
				return fmt.Errorf("add game %q to slip %d: %w", gameID, id, err)
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

// dedupeGameIDs trims, drops blanks, and deduplicates game IDs in order.
func dedupeGameIDs(gameIDs []string) []string {
	if len(gameIDs) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(gameIDs))
	out := make([]string, 0, len(gameIDs))
	for _, raw := range gameIDs {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// verifyGamesInWeek checks that every game ID exists in weekID.
func (s *Store) verifyGamesInWeek(ctx context.Context, weekID int64, gameIDs []string) error {
	rows, err := s.readConn().QueryContext(ctx, `SELECT id FROM games WHERE week_id = ?`, weekID)
	if err != nil {
		return fmt.Errorf("verify games in week %d: %w", weekID, err)
	}
	defer func() { _ = rows.Close() }()

	valid := make(map[string]struct{})
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("verify games in week %d: %w", weekID, err)
		}
		valid[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("verify games in week %d: %w", weekID, err)
	}

	for _, id := range gameIDs {
		if _, ok := valid[id]; !ok {
			return fmt.Errorf("game %q not in week %d: %w", id, weekID, ErrGameNotFound)
		}
	}
	return nil
}

// SlipByID returns the slip with the given ID. A missing row wraps
// ErrSlipNotFound.
func (s *Store) SlipByID(ctx context.Context, id int64) (Slip, error) {
	slip, err := scanSlip(s.readConn().QueryRowContext(ctx,
		"SELECT"+slipColumns+" FROM slips WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return Slip{}, fmt.Errorf("slip %d: %w", id, ErrSlipNotFound)
	}
	if err != nil {
		return Slip{}, fmt.Errorf("slip %d: %w", id, err)
	}
	if err := s.hydrateSlipGameIDs(ctx, &slip); err != nil {
		return Slip{}, fmt.Errorf("slip %d: %w", id, err)
	}
	return slip, nil
}

// SlipsForWeek returns a group's slips in one week, newest first.
func (s *Store) SlipsForWeek(ctx context.Context, groupID, weekID int64) ([]Slip, error) {
	rows, err := s.readConn().QueryContext(ctx,
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
	if err := s.hydrateSlipsGameIDs(ctx, slips); err != nil {
		return nil, fmt.Errorf("list slips for group %d in week %d: %w", groupID, weekID, err)
	}
	return slips, nil
}

// SlipWithLegs pairs a slip with its legs in added order.
type SlipWithLegs struct {
	Slip Slip
	Legs []Leg
}

// SlipsWithLegsForWeek returns slips in one week along with their legs in
// added order, ordered oldest slip first (matching SlipIDsForWeek). A zero
// groupID matches all groups, and an empty status matches all statuses. An
// unknown non-empty status returns ErrSlipStatusInvalid.
func (s *Store) SlipsWithLegsForWeek(
	ctx context.Context,
	weekID, groupID int64,
	status SlipStatus,
) ([]SlipWithLegs, error) {
	if status != "" && !status.Valid() {
		return nil, fmt.Errorf("list slips with legs for week %d: %w", weekID, ErrSlipStatusInvalid)
	}

	clauses := []string{"week_id = ?"}
	args := []any{weekID}
	if groupID != 0 {
		clauses = append(clauses, "group_id = ?")
		args = append(args, groupID)
	}
	if status != "" {
		clauses = append(clauses, "status = ?")
		args = append(args, string(status))
	}

	// #nosec G202 -- clauses contains only static column predicates.
	query := "SELECT" + slipColumns + " FROM slips WHERE " + strings.Join(clauses, " AND ") + " ORDER BY id"
	rows, err := s.readConn().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list slips with legs for week %d: %w", weekID, err)
	}
	defer func() { _ = rows.Close() }()

	slips := make([]Slip, 0)
	slipIDs := make([]int64, 0)
	for rows.Next() {
		slip, err := scanSlip(rows)
		if err != nil {
			return nil, fmt.Errorf("list slips with legs for week %d: %w", weekID, err)
		}
		slips = append(slips, slip)
		slipIDs = append(slipIDs, slip.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list slips with legs for week %d: %w", weekID, err)
	}
	if len(slips) == 0 {
		return []SlipWithLegs{}, nil
	}
	if err := s.hydrateSlipsGameIDs(ctx, slips); err != nil {
		return nil, fmt.Errorf("list slips with legs for week %d: %w", weekID, err)
	}

	legsBySlip, err := s.LegsForSlips(ctx, slipIDs)
	if err != nil {
		return nil, fmt.Errorf("list slips with legs for week %d: %w", weekID, err)
	}

	out := make([]SlipWithLegs, len(slips))
	for i, slip := range slips {
		legs := legsBySlip[slip.ID]
		if legs == nil {
			legs = []Leg{}
		}
		out[i] = SlipWithLegs{
			Slip: slip,
			Legs: legs,
		}
	}
	return out, nil
}

// SlipIDsForWeek returns every slip ID in a week across all groups, oldest
// first. Refresh uses it to signal every affected slip after a board change.
func (s *Store) SlipIDsForWeek(ctx context.Context, weekID int64) ([]int64, error) {
	rows, err := s.readConn().QueryContext(ctx,
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
	slip, err := scanSlip(s.readConn().QueryRowContext(ctx,
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
	if err := s.hydrateSlipGameIDs(ctx, &slip); err != nil {
		return Slip{}, fmt.Errorf("newest slip for group %d in week %d: %w", groupID, weekID, err)
	}
	return slip, nil
}

// PreviousSlip returns the latest slip for a group in weekID or any earlier
// week. It is the slip a create form copies its defaults from, so a second slip
// in the same week copies that week's first slip. A group with no slip that old
// wraps ErrSlipNotFound.
func (s *Store) PreviousSlip(ctx context.Context, groupID, weekID int64) (Slip, error) {
	slip, err := scanSlip(s.readConn().QueryRowContext(ctx,
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
	if err := s.hydrateSlipGameIDs(ctx, &slip); err != nil {
		return Slip{}, fmt.Errorf("previous slip for group %d before week %d: %w", groupID, weekID, err)
	}
	return slip, nil
}

// SlipMembers returns a slip's people and their lock times, ordered by user ID.
func (s *Store) SlipMembers(ctx context.Context, slipID int64) ([]SlipMember, error) {
	rows, err := s.readConn().QueryContext(ctx, `
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
	rows, err := s.readConn().QueryContext(ctx, `
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
		slip       Slip
		scheduleID sql.NullInt64
		slateScope string
		status     string
		result     string
		createdAt  string
	)
	if err := row.Scan(
		&slip.ID,
		&slip.GroupID,
		&slip.WeekID,
		&scheduleID,
		&slip.Name,
		&slateScope,
		&slip.LegsPerMember,
		&status,
		&result,
		&createdAt,
	); err != nil {
		return Slip{}, err
	}

	if scheduleID.Valid {
		slip.ScheduleID = &scheduleID.Int64
	}

	parsed, err := scanTimestamp(createdAt)
	if err != nil {
		return Slip{}, err
	}
	slip.SlateScope = SlateScope(slateScope)
	slip.Status = SlipStatus(status)
	slip.Result = parlay.Result(result)
	slip.CreatedAt = parsed
	return slip, nil
}

// hydrateSlipGameIDs populates GameIDs when slip has SlateScopeCustom.
func (s *Store) hydrateSlipGameIDs(ctx context.Context, slip *Slip) error {
	if slip.SlateScope != SlateScopeCustom {
		return nil
	}
	rows, err := s.readConn().QueryContext(ctx, `
		SELECT game_id FROM slip_games WHERE slip_id = ? ORDER BY game_id`, slip.ID)
	if err != nil {
		return fmt.Errorf("load slip games: %w", err)
	}
	defer func() { _ = rows.Close() }()

	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("load slip games: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("load slip games: %w", err)
	}
	slip.GameIDs = ids
	return nil
}

// hydrateSlipsGameIDs batch-populates GameIDs for any SlateScopeCustom slips.
func (s *Store) hydrateSlipsGameIDs(ctx context.Context, slips []Slip) error {
	customIDs := make([]int64, 0)
	for _, slip := range slips {
		if slip.SlateScope == SlateScopeCustom {
			customIDs = append(customIDs, slip.ID)
		}
	}
	if len(customIDs) == 0 {
		return nil
	}

	placeholders := make([]string, len(customIDs))
	args := make([]any, len(customIDs))
	for i, id := range customIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	// #nosec G202 -- placeholders contains only static "?" tokens.
	query := `SELECT slip_id, game_id FROM slip_games WHERE slip_id IN (` +
		strings.Join(placeholders, ",") + `) ORDER BY slip_id, game_id`
	rows, err := s.readConn().QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("batch load slip games: %w", err)
	}
	defer func() { _ = rows.Close() }()

	bySlip := make(map[int64][]string, len(customIDs))
	for rows.Next() {
		var (
			slipID int64
			gameID string
		)
		if err := rows.Scan(&slipID, &gameID); err != nil {
			return fmt.Errorf("batch load slip games: %w", err)
		}
		bySlip[slipID] = append(bySlip[slipID], gameID)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("batch load slip games: %w", err)
	}

	for i := range slips {
		if slips[i].SlateScope == SlateScopeCustom {
			slips[i].GameIDs = bySlip[slips[i].ID]
		}
	}
	return nil
}

// scanTimestamp reads one of the RFC 3339 UTC strings the schema stores.
func scanTimestamp(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse timestamp %q: %w", value, err)
	}
	return parsed, nil
}
