package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// NFLversePlayerID maps one player's ESPN athlete ID to nflverse's GSIS and
// PFR identifiers.
type NFLversePlayerID struct {
	ESPNID    string
	GSISID    string
	PFRID     string
	UpdatedAt time.Time
}

// NFLverseTeamWeek holds one team's offensive and defensive-allowed efficiency
// and pace for one week of a season.
type NFLverseTeamWeek struct {
	Season            int
	SeasonType        int // 2 = regular, 3 = postseason
	WeekNum           int
	Team              string // ESPN abbreviation
	OffPassEPA        float64
	OffRushEPA        float64
	DefPassEPAAllowed float64
	DefRushEPAAllowed float64
	PlaysPerGame      float64
	PassRate          float64
	UpdatedAt         time.Time
}

// NFLversePlayerWeek holds one player's weekly snap share, target/air-yards
// usage, and EPA from nflverse.
type NFLversePlayerWeek struct {
	ESPNID        string
	Season        int
	SeasonType    int // 2 = regular, 3 = postseason
	WeekNum       int
	Team          string // ESPN abbreviation
	OffenseSnaps  int
	OffensePct    float64
	TargetShare   float64
	AirYardsShare float64
	WOPR          float64
	Carries       int
	Targets       int
	TotalEPA      float64
	RZOpps        int
	UpdatedAt     time.Time
}

const nflverseTeamWeekColumns = `
	season, season_type, week_num, team, off_pass_epa, off_rush_epa,
	def_pass_epa_allowed, def_rush_epa_allowed, plays_per_game, pass_rate,
	updated_at`

const nflversePlayerWeekColumns = `
	espn_id, season, season_type, week_num, team, offense_snaps, offense_pct,
	target_share, air_yards_share, wopr, carries, targets, total_epa, rz_opps,
	updated_at`

// UpsertNFLversePlayerIDs writes each player crosswalk row and reports whether
// any row changed. Identical rows keep their existing updated_at.
func (s *Store) UpsertNFLversePlayerIDs(ctx context.Context, rows []NFLversePlayerID) (bool, error) {
	changed := false
	err := s.Tx(ctx, func(tx *Store) error {
		for _, row := range rows {
			result, err := tx.conn().ExecContext(ctx, `
				INSERT INTO nflverse_player_ids (
					espn_id, gsis_id, pfr_id, updated_at
				) VALUES (?, ?, ?, ?)
				ON CONFLICT (espn_id) DO UPDATE SET
					gsis_id = excluded.gsis_id,
					pfr_id = excluded.pfr_id,
					updated_at = excluded.updated_at
				WHERE nflverse_player_ids.gsis_id IS NOT excluded.gsis_id
					OR nflverse_player_ids.pfr_id IS NOT excluded.pfr_id`,
				row.ESPNID,
				row.GSISID,
				row.PFRID,
				formatTimestamp(row.UpdatedAt),
			)
			if err != nil {
				return fmt.Errorf("upsert nflverse player id %s: %w", row.ESPNID, err)
			}
			affected, err := result.RowsAffected()
			if err != nil {
				return fmt.Errorf("upsert nflverse player id %s: %w", row.ESPNID, err)
			}
			if affected > 0 {
				changed = true
			}
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return changed, nil
}

// NFLversePlayerIDByESPN returns the crosswalk row for one ESPN athlete ID.
func (s *Store) NFLversePlayerIDByESPN(ctx context.Context, espnID string) (NFLversePlayerID, bool, error) {
	var (
		row       NFLversePlayerID
		updatedAt string
	)
	err := s.conn().QueryRowContext(ctx, `
		SELECT espn_id, gsis_id, pfr_id, updated_at
		FROM nflverse_player_ids
		WHERE espn_id = ?`, espnID).Scan(
		&row.ESPNID,
		&row.GSISID,
		&row.PFRID,
		&updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return NFLversePlayerID{}, false, nil
	}
	if err != nil {
		return NFLversePlayerID{}, false, fmt.Errorf("nflverse player id %s: %w", espnID, err)
	}
	parsed, err := scanTimestamp(updatedAt)
	if err != nil {
		return NFLversePlayerID{}, false, fmt.Errorf("nflverse player id %s: %w", espnID, err)
	}
	row.UpdatedAt = parsed
	return row, true, nil
}

// UpsertNFLverseTeamWeeks writes each team-week row and reports whether any row
// changed. Identical rows keep their existing updated_at.
func (s *Store) UpsertNFLverseTeamWeeks(ctx context.Context, rows []NFLverseTeamWeek) (bool, error) {
	changed := false
	err := s.Tx(ctx, func(tx *Store) error {
		for _, row := range rows {
			result, err := tx.conn().ExecContext(ctx, `
				INSERT INTO nflverse_team_weeks (
					season, season_type, week_num, team, off_pass_epa, off_rush_epa,
					def_pass_epa_allowed, def_rush_epa_allowed, plays_per_game,
					pass_rate, updated_at
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT (season, season_type, week_num, team) DO UPDATE SET
					off_pass_epa = excluded.off_pass_epa,
					off_rush_epa = excluded.off_rush_epa,
					def_pass_epa_allowed = excluded.def_pass_epa_allowed,
					def_rush_epa_allowed = excluded.def_rush_epa_allowed,
					plays_per_game = excluded.plays_per_game,
					pass_rate = excluded.pass_rate,
					updated_at = excluded.updated_at
				WHERE nflverse_team_weeks.off_pass_epa IS NOT excluded.off_pass_epa
					OR nflverse_team_weeks.off_rush_epa IS NOT excluded.off_rush_epa
					OR nflverse_team_weeks.def_pass_epa_allowed IS NOT excluded.def_pass_epa_allowed
					OR nflverse_team_weeks.def_rush_epa_allowed IS NOT excluded.def_rush_epa_allowed
					OR nflverse_team_weeks.plays_per_game IS NOT excluded.plays_per_game
					OR nflverse_team_weeks.pass_rate IS NOT excluded.pass_rate`,
				row.Season,
				row.SeasonType,
				row.WeekNum,
				row.Team,
				row.OffPassEPA,
				row.OffRushEPA,
				row.DefPassEPAAllowed,
				row.DefRushEPAAllowed,
				row.PlaysPerGame,
				row.PassRate,
				formatTimestamp(row.UpdatedAt),
			)
			if err != nil {
				return fmt.Errorf(
					"upsert nflverse team week %d/%d/w%d/%s: %w",
					row.Season,
					row.SeasonType,
					row.WeekNum,
					row.Team,
					err,
				)
			}
			affected, err := result.RowsAffected()
			if err != nil {
				return fmt.Errorf(
					"upsert nflverse team week %d/%d/w%d/%s: %w",
					row.Season,
					row.SeasonType,
					row.WeekNum,
					row.Team,
					err,
				)
			}
			if affected > 0 {
				changed = true
			}
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return changed, nil
}

// NFLverseTeamWeeksForSeason returns all stored team-week rows for one season
// and season type, ordered by week ascending then team.
func (s *Store) NFLverseTeamWeeksForSeason(ctx context.Context, season, seasonType int) ([]NFLverseTeamWeek, error) {
	rows, err := s.conn().QueryContext(ctx, "SELECT"+nflverseTeamWeekColumns+`
		FROM nflverse_team_weeks
		WHERE season = ? AND season_type = ?
		ORDER BY week_num ASC, team ASC`, season, seasonType)
	if err != nil {
		return nil, fmt.Errorf("nflverse team weeks %d/%d: %w", season, seasonType, err)
	}
	defer func() { _ = rows.Close() }()

	var out []NFLverseTeamWeek
	for rows.Next() {
		var (
			item      NFLverseTeamWeek
			updatedAt string
		)
		if err := rows.Scan(
			&item.Season,
			&item.SeasonType,
			&item.WeekNum,
			&item.Team,
			&item.OffPassEPA,
			&item.OffRushEPA,
			&item.DefPassEPAAllowed,
			&item.DefRushEPAAllowed,
			&item.PlaysPerGame,
			&item.PassRate,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan nflverse team week: %w", err)
		}
		parsed, err := scanTimestamp(updatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan nflverse team week: %w", err)
		}
		item.UpdatedAt = parsed
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("nflverse team weeks %d/%d: %w", season, seasonType, err)
	}
	return out, nil
}

// UpsertNFLversePlayerWeeks writes each player-week row and reports whether
// any row changed. Identical rows keep their existing updated_at.
func (s *Store) UpsertNFLversePlayerWeeks(ctx context.Context, rows []NFLversePlayerWeek) (bool, error) {
	changed := false
	err := s.Tx(ctx, func(tx *Store) error {
		for _, row := range rows {
			result, err := tx.conn().ExecContext(ctx, `
				INSERT INTO nflverse_player_weeks (
					espn_id, season, season_type, week_num, team, offense_snaps,
					offense_pct, target_share, air_yards_share, wopr, carries,
					targets, total_epa, rz_opps, updated_at
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT (espn_id, season, season_type, week_num) DO UPDATE SET
					team = excluded.team,
					offense_snaps = excluded.offense_snaps,
					offense_pct = excluded.offense_pct,
					target_share = excluded.target_share,
					air_yards_share = excluded.air_yards_share,
					wopr = excluded.wopr,
					carries = excluded.carries,
					targets = excluded.targets,
					total_epa = excluded.total_epa,
					rz_opps = excluded.rz_opps,
					updated_at = excluded.updated_at
				WHERE nflverse_player_weeks.team IS NOT excluded.team
					OR nflverse_player_weeks.offense_snaps IS NOT excluded.offense_snaps
					OR nflverse_player_weeks.offense_pct IS NOT excluded.offense_pct
					OR nflverse_player_weeks.target_share IS NOT excluded.target_share
					OR nflverse_player_weeks.air_yards_share IS NOT excluded.air_yards_share
					OR nflverse_player_weeks.wopr IS NOT excluded.wopr
					OR nflverse_player_weeks.carries IS NOT excluded.carries
					OR nflverse_player_weeks.targets IS NOT excluded.targets
					OR nflverse_player_weeks.total_epa IS NOT excluded.total_epa
					OR nflverse_player_weeks.rz_opps IS NOT excluded.rz_opps`,
				row.ESPNID,
				row.Season,
				row.SeasonType,
				row.WeekNum,
				row.Team,
				row.OffenseSnaps,
				row.OffensePct,
				row.TargetShare,
				row.AirYardsShare,
				row.WOPR,
				row.Carries,
				row.Targets,
				row.TotalEPA,
				row.RZOpps,
				formatTimestamp(row.UpdatedAt),
			)
			if err != nil {
				return fmt.Errorf(
					"upsert nflverse player week %s/%d/%d/w%d: %w",
					row.ESPNID,
					row.Season,
					row.SeasonType,
					row.WeekNum,
					err,
				)
			}
			affected, err := result.RowsAffected()
			if err != nil {
				return fmt.Errorf(
					"upsert nflverse player week %s/%d/%d/w%d: %w",
					row.ESPNID,
					row.Season,
					row.SeasonType,
					row.WeekNum,
					err,
				)
			}
			if affected > 0 {
				changed = true
			}
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return changed, nil
}

// NFLversePlayerWeeksForPlayer returns one player's weekly nflverse rows for
// a season and season type, ordered most recent week first.
func (s *Store) NFLversePlayerWeeksForPlayer(
	ctx context.Context,
	espnID string,
	season, seasonType int,
) ([]NFLversePlayerWeek, error) {
	rows, err := s.conn().QueryContext(ctx, "SELECT"+nflversePlayerWeekColumns+`
		FROM nflverse_player_weeks
		WHERE espn_id = ? AND season = ? AND season_type = ?
		ORDER BY week_num DESC`, espnID, season, seasonType)
	if err != nil {
		return nil, fmt.Errorf("nflverse player weeks %s/%d/%d: %w", espnID, season, seasonType, err)
	}
	defer func() { _ = rows.Close() }()
	return scanNFLversePlayerWeeks(rows)
}

// NFLversePlayerWeeksForSeason returns all stored player-week rows for a
// season and season type, ordered most recent week first then ESPN ID.
func (s *Store) NFLversePlayerWeeksForSeason(
	ctx context.Context,
	season, seasonType int,
) ([]NFLversePlayerWeek, error) {
	rows, err := s.conn().QueryContext(ctx, "SELECT"+nflversePlayerWeekColumns+`
		FROM nflverse_player_weeks
		WHERE season = ? AND season_type = ?
		ORDER BY week_num DESC, espn_id ASC`, season, seasonType)
	if err != nil {
		return nil, fmt.Errorf("nflverse player weeks %d/%d: %w", season, seasonType, err)
	}
	defer func() { _ = rows.Close() }()
	return scanNFLversePlayerWeeks(rows)
}

func scanNFLversePlayerWeeks(rows *sql.Rows) ([]NFLversePlayerWeek, error) {
	var out []NFLversePlayerWeek
	for rows.Next() {
		var (
			item      NFLversePlayerWeek
			updatedAt string
		)
		if err := rows.Scan(
			&item.ESPNID,
			&item.Season,
			&item.SeasonType,
			&item.WeekNum,
			&item.Team,
			&item.OffenseSnaps,
			&item.OffensePct,
			&item.TargetShare,
			&item.AirYardsShare,
			&item.WOPR,
			&item.Carries,
			&item.Targets,
			&item.TotalEPA,
			&item.RZOpps,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan nflverse player week: %w", err)
		}
		parsed, err := scanTimestamp(updatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan nflverse player week: %w", err)
		}
		item.UpdatedAt = parsed
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("nflverse player weeks: %w", err)
	}
	return out, nil
}
