package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/kalverra/parlay-club/internal/parlay"
)

// GradedSlip is a GRADED slip paired with its week coordinates for season
// ordering in the Record Book.
type GradedSlip struct {
	Slip Slip
	Week Week
}

// Seasons returns every NFL season that has at least one slip across all
// groups, newest first.
func (s *Store) Seasons(ctx context.Context) ([]int, error) {
	return s.seasons(ctx, 0, false)
}

// GradedSeasons returns every NFL season that has at least one GRADED slip
// across all groups, newest first.
func (s *Store) GradedSeasons(ctx context.Context) ([]int, error) {
	return s.seasons(ctx, 0, true)
}

// SeasonsForGroup returns every NFL season that has at least one slip in
// groupID, newest first.
func (s *Store) SeasonsForGroup(ctx context.Context, groupID int64) ([]int, error) {
	return s.seasons(ctx, groupID, false)
}

// GradedSeasonsForGroup returns every NFL season that has at least one GRADED
// slip in groupID, newest first.
func (s *Store) GradedSeasonsForGroup(ctx context.Context, groupID int64) ([]int, error) {
	return s.seasons(ctx, groupID, true)
}

func (s *Store) seasons(ctx context.Context, groupID int64, gradedOnly bool) ([]int, error) {
	query := `
		SELECT DISTINCT wk.season
		FROM slips s
		JOIN weeks wk ON wk.id = s.week_id`
	var clauses []string
	var args []any
	if groupID > 0 {
		clauses = append(clauses, "s.group_id = ?")
		args = append(args, groupID)
	}
	if gradedOnly {
		clauses = append(clauses, "s.status = ?")
		args = append(args, string(SlipGraded))
	}
	if len(clauses) > 0 {
		query += "\n\t\tWHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY wk.season DESC"

	rows, err := s.readConn().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list seasons for group %d: %w", groupID, err)
	}
	defer func() { _ = rows.Close() }()

	seasons := make([]int, 0)
	for rows.Next() {
		var season int
		if err := rows.Scan(&season); err != nil {
			return nil, fmt.Errorf("list seasons for group %d: %w", groupID, err)
		}
		seasons = append(seasons, season)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list seasons for group %d: %w", groupID, err)
	}
	return seasons, nil
}

// GradedSlipsForGroupSeason returns every GRADED slip in groupID for season,
// ordered chronologically by week, creation time, and slip ID.
func (s *Store) GradedSlipsForGroupSeason(ctx context.Context, groupID int64, season int) ([]GradedSlip, error) {
	rows, err := s.readConn().QueryContext(ctx, `
		SELECT `+prefixedSlipColumns("s")+`,
			wk.id, wk.season, wk.season_type, wk.week_num
		FROM slips s
		JOIN weeks wk ON wk.id = s.week_id
		WHERE s.group_id = ? AND wk.season = ? AND s.status = ?
		ORDER BY wk.season ASC, wk.season_type ASC, wk.week_num ASC, s.created_at ASC, s.id ASC`,
		groupID, season, string(SlipGraded))
	if err != nil {
		return nil, fmt.Errorf("list graded slips for group %d season %d: %w", groupID, season, err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]GradedSlip, 0)
	for rows.Next() {
		gs, err := scanGradedSlip(rows)
		if err != nil {
			return nil, fmt.Errorf("list graded slips for group %d season %d: %w", groupID, season, err)
		}
		out = append(out, gs)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list graded slips for group %d season %d: %w", groupID, season, err)
	}
	return out, nil
}

// GradedLegsForGroupSeason returns every leg on GRADED slips in groupID for
// season, ordered by slip ID and leg ID.
func (s *Store) GradedLegsForGroupSeason(ctx context.Context, groupID int64, season int) ([]Leg, error) {
	rows, err := s.readConn().QueryContext(ctx, `
		SELECT `+prefixedLegColumns("l")+`
		FROM legs l
		JOIN slips s ON s.id = l.slip_id
		JOIN weeks wk ON wk.id = s.week_id
		WHERE s.group_id = ? AND wk.season = ? AND s.status = ?
		ORDER BY l.slip_id ASC, l.id ASC`,
		groupID, season, string(SlipGraded))
	if err != nil {
		return nil, fmt.Errorf("list graded legs for group %d season %d: %w", groupID, season, err)
	}
	defer func() { _ = rows.Close() }()

	legs := make([]Leg, 0)
	for rows.Next() {
		leg, err := scanLeg(rows)
		if err != nil {
			return nil, fmt.Errorf("list graded legs for group %d season %d: %w", groupID, season, err)
		}
		legs = append(legs, leg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list graded legs for group %d season %d: %w", groupID, season, err)
	}
	return legs, nil
}

// RecordUsersForGroupSeason returns every user who is a member of groupID or
// who authored a leg on a GRADED slip in groupID during season.
func (s *Store) RecordUsersForGroupSeason(ctx context.Context, groupID int64, season int) ([]User, error) {
	rows, err := s.readConn().QueryContext(ctx, `
		SELECT u.id, u.name, u.avatar_color, u.home_book, u.active, u.email, u.picture_url
		FROM users u
		WHERE u.id IN (
			SELECT user_id FROM group_members WHERE group_id = ?
			UNION
			SELECT l.user_id
			FROM legs l
			JOIN slips s ON s.id = l.slip_id
			JOIN weeks wk ON wk.id = s.week_id
			WHERE s.group_id = ? AND wk.season = ? AND s.status = ?
		)
		ORDER BY u.id`,
		groupID, groupID, season, string(SlipGraded))
	if err != nil {
		return nil, fmt.Errorf("list record users for group %d season %d: %w", groupID, season, err)
	}
	defer func() { _ = rows.Close() }()

	users := make([]User, 0)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("list record users for group %d season %d: %w", groupID, season, err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list record users for group %d season %d: %w", groupID, season, err)
	}
	return users, nil
}

func scanGradedSlip(row rowScanner) (GradedSlip, error) {
	var (
		gs         GradedSlip
		scheduleID sql.NullInt64
		slateScope string
		status     string
		result     string
		createdAt  string
	)
	if err := row.Scan(
		&gs.Slip.ID,
		&gs.Slip.GroupID,
		&gs.Slip.WeekID,
		&scheduleID,
		&gs.Slip.Name,
		&slateScope,
		&gs.Slip.LegsPerMember,
		&status,
		&result,
		&createdAt,
		&gs.Week.ID,
		&gs.Week.Season,
		&gs.Week.SeasonType,
		&gs.Week.WeekNum,
	); err != nil {
		return GradedSlip{}, err
	}
	if scheduleID.Valid {
		gs.Slip.ScheduleID = &scheduleID.Int64
	}
	parsed, err := scanTimestamp(createdAt)
	if err != nil {
		return GradedSlip{}, err
	}
	gs.Slip.SlateScope = SlateScope(slateScope)
	gs.Slip.Status = SlipStatus(status)
	gs.Slip.Result = parlay.Result(result)
	gs.Slip.CreatedAt = parsed
	return gs, nil
}

func prefixedSlipColumns(alias string) string {
	return prefixColumns(slipColumns, alias)
}

func prefixedLegColumns(alias string) string {
	return prefixColumns(legColumns, alias)
}

func prefixColumns(cols, alias string) string {
	parts := strings.Split(cols, ",")
	for i, part := range parts {
		parts[i] = alias + "." + strings.TrimSpace(part)
	}
	return strings.Join(parts, ", ")
}
