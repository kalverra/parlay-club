package db_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/slips"
)

// latestSchemaVersion is the version the last migration records.
const latestSchemaVersion = 22

func TestMigrateFreshDatabase(t *testing.T) {
	t.Parallel()

	database := openTestDB(t)
	require.NoError(t, db.Migrate(t.Context(), database))

	assert.Equal(t, latestSchemaVersion, schemaVersion(t, database))
	assert.ElementsMatch(t, []string{
		"activity",
		"api_bodies",
		"api_responses",
		"bug_reports",
		"games",
		"group_members",
		"group_slip_schedules",
		"groups",
		"leg_votes",
		"legs",
		"nfelo_games",
		"nflverse_player_ids",
		"nflverse_player_weeks",
		"nflverse_team_weeks",
		"odds",
		"player_gamelogs",
		"player_matches",
		"push_log",
		"push_subscriptions",
		"reminder_tasks",
		"roster_players",
		"slip_games",
		"slip_members",
		"slips",
		"sync_state",
		"users",
		"wagers",
		"weeks",
	}, tableNames(t, database))
}

func TestMigrateRerunKeepsVersionAndData(t *testing.T) {
	t.Parallel()

	database := openTestDB(t)
	require.NoError(t, db.Migrate(t.Context(), database))
	_, err := database.ExecContext(t.Context(), "INSERT INTO users (name, avatar_color) VALUES ('Ada', 'sky')")
	require.NoError(t, err)

	require.NoError(t, db.Migrate(t.Context(), database))

	assert.Equal(t, latestSchemaVersion, schemaVersion(t, database))
	assert.Equal(t, 1, userCount(t, database))
}

func TestMigration0009MultiSlipEvents(t *testing.T) {
	t.Parallel()

	database := openTestDB(t)
	require.NoError(t, db.Migrate(t.Context(), database))

	// Create group, week, and slip without specifying slate_scope.
	_, err := database.ExecContext(t.Context(),
		"INSERT INTO groups (id, name, invite_token) VALUES (10, 'Club 10', 'token-10')")
	require.NoError(t, err)
	_, err = database.ExecContext(t.Context(),
		"INSERT INTO weeks (id, season, season_type, week_num) VALUES (2026201, 2026, 2, 1)")
	require.NoError(t, err)
	_, err = database.ExecContext(t.Context(),
		"INSERT INTO slips (id, group_id, week_id, name) VALUES (100, 10, 2026201, 'Week 1 Slip')")
	require.NoError(t, err)

	// slate_scope must default to 'sunday_main'.
	var slateScope string
	require.NoError(t, database.QueryRowContext(t.Context(),
		"SELECT slate_scope FROM slips WHERE id = 100").Scan(&slateScope))
	assert.Equal(t, "sunday_main", slateScope)

	// Inserting custom slate_scope on slips works.
	_, err = database.ExecContext(
		t.Context(),
		"INSERT INTO slips (id, group_id, week_id, name, slate_scope) VALUES (101, 10, 2026201, 'Thursday Slip', 'thursday')",
	)
	require.NoError(t, err)
	require.NoError(t, database.QueryRowContext(t.Context(),
		"SELECT slate_scope FROM slips WHERE id = 101").Scan(&slateScope))
	assert.Equal(t, "thursday", slateScope)

	// Insert into group_slip_schedules.
	_, err = database.ExecContext(t.Context(), `
		INSERT INTO group_slip_schedules (group_id, name, slate_scope, legs_per_member, is_active)
		VALUES (10, 'Main Sunday Slip', 'sunday_main', 2, 1)`)
	require.NoError(t, err)

	// UNIQUE (group_id, name) constraint prevents duplicate names.
	_, err = database.ExecContext(t.Context(), `
		INSERT INTO group_slip_schedules (group_id, name, slate_scope, legs_per_member, is_active)
		VALUES (10, 'Main Sunday Slip', 'sunday_main', 2, 1)`)
	require.Error(t, err, "duplicate schedule name in same group must fail")

	// legs_per_member CHECK constraint (1 to 5).
	_, err = database.ExecContext(t.Context(), `
		INSERT INTO group_slip_schedules (group_id, name, slate_scope, legs_per_member, is_active)
		VALUES (10, 'Too Few Legs', 'sunday_main', 0, 1)`)
	require.Error(t, err, "legs_per_member < 1 must violate CHECK")

	_, err = database.ExecContext(t.Context(), `
		INSERT INTO group_slip_schedules (group_id, name, slate_scope, legs_per_member, is_active)
		VALUES (10, 'Too Many Legs', 'sunday_main', 6, 1)`)
	require.Error(t, err, "legs_per_member > 5 must violate CHECK")

	// is_active CHECK constraint (0 or 1).
	_, err = database.ExecContext(t.Context(), `
		INSERT INTO group_slip_schedules (group_id, name, slate_scope, legs_per_member, is_active)
		VALUES (10, 'Invalid Active', 'sunday_main', 2, 2)`)
	require.Error(t, err, "is_active not in (0, 1) must violate CHECK")

	// CASCADE on group delete.
	_, err = database.ExecContext(t.Context(),
		"INSERT INTO groups (id, name, invite_token) VALUES (11, 'Club 11', 'token-11')")
	require.NoError(t, err)
	_, err = database.ExecContext(t.Context(), `
		INSERT INTO group_slip_schedules (group_id, name, slate_scope, legs_per_member, is_active)
		VALUES (11, 'Club 11 Schedule', 'sunday_main', 2, 1)`)
	require.NoError(t, err)

	_, err = database.ExecContext(t.Context(), "DELETE FROM groups WHERE id = 11")
	require.NoError(t, err)

	var schedCount int
	require.NoError(t, database.QueryRowContext(t.Context(),
		"SELECT count(*) FROM group_slip_schedules WHERE group_id = 11").Scan(&schedCount))
	assert.Equal(t, 0, schedCount, "deleting group must cascade to schedules")
}

func TestMigrationUpgradePathEnsureWeekSlipsSingleSlip(t *testing.T) {
	t.Parallel()

	database := openTestDB(t)

	// Apply migrations 0001 through 0008 manually
	entries, err := os.ReadDir("migrations")
	require.NoError(t, err)
	for _, entry := range entries {
		if entry.Name() < "0009" && filepath.Ext(entry.Name()) == ".sql" {
			content, err := os.ReadFile(filepath.Join("migrations", entry.Name()))
			require.NoError(t, err)
			_, err = database.ExecContext(t.Context(), string(content))
			require.NoError(t, err)
		}
	}
	_, err = database.ExecContext(t.Context(), "PRAGMA user_version = 8")
	require.NoError(t, err)

	// Seed pre-0009 data: group, user, week, and a slip with a differently-named slip "Week 9 Sandbox"
	_, err = database.ExecContext(t.Context(), `
		INSERT INTO users (id, name, avatar_color, active) VALUES (1, 'Ada', 'sky', 1);
		INSERT INTO groups (id, name, invite_token) VALUES (1, 'Sunday Club', 'token-1');
		INSERT INTO group_members (group_id, user_id) VALUES (1, 1);
		INSERT INTO weeks (id, season, season_type, week_num) VALUES (2026209, 2026, 2, 9);
		INSERT INTO slips (id, group_id, week_id, name) VALUES (10, 1, 2026209, 'Week 9 Sandbox');
		INSERT INTO slip_members (slip_id, user_id) VALUES (10, 1);
	`)
	require.NoError(t, err)

	// Run migration to apply 0009 and 0010
	require.NoError(t, db.Migrate(t.Context(), database))
	assert.Equal(t, latestSchemaVersion, schemaVersion(t, database))

	// Verify backfilled schedule is created and linked to the existing slip
	store := db.NewStore(database)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })
	schedules, err := store.GroupSlipSchedules(t.Context(), 1)
	require.NoError(t, err)
	require.Len(t, schedules, 1)
	assert.Equal(t, "Main Sunday Slip", schedules[0].Name)

	slip, err := store.SlipByID(t.Context(), 10)
	require.NoError(t, err)
	require.NotNil(t, slip.ScheduleID, "existing newest slip must be linked to backfilled schedule")
	assert.Equal(t, schedules[0].ID, *slip.ScheduleID)

	// Now run EnsureWeekSlips on the week
	slipsList, err := slips.EnsureWeekSlips(
		t.Context(),
		store,
		1,
		2026209,
		time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC),
	)
	require.NoError(t, err)
	assert.Len(t, slipsList, 1, "week must still have exactly one slip, no duplicate created")
	assert.Equal(t, int64(10), slipsList[0].ID)
	assert.Equal(t, "Week 9 Sandbox", slipsList[0].Name)
}

func TestMigration0010BugReports(t *testing.T) {
	t.Parallel()

	database := openTestDB(t)
	require.NoError(t, db.Migrate(t.Context(), database))

	// Insert user for foreign key
	_, err := database.ExecContext(t.Context(),
		"INSERT INTO users (id, name, avatar_color) VALUES (1, 'Ada', 'sky')")
	require.NoError(t, err)

	_, err = database.ExecContext(t.Context(), `
		INSERT INTO bug_reports (
			token, user_id, page_url, description, user_agent, screen_size, screenshot
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"0123456789abcdef0123456789abcdef",
		1,
		"https://example.com/slips/1",
		"Button does not respond",
		"Mozilla/5.0",
		"390x844",
		[]byte("fake-jpeg-bytes"),
	)
	require.NoError(t, err)

	var (
		token          string
		description    string
		screenshotMIME string
	)
	require.NoError(t, database.QueryRowContext(t.Context(),
		"SELECT token, description, screenshot_mime FROM bug_reports WHERE user_id = 1").
		Scan(&token, &description, &screenshotMIME))

	assert.Equal(t, "0123456789abcdef0123456789abcdef", token)
	assert.Equal(t, "Button does not respond", description)
	assert.Equal(t, "image/jpeg", screenshotMIME)
}

func TestMigration0011LogosPortraits(t *testing.T) {
	t.Parallel()

	database := openTestDB(t)
	require.NoError(t, db.Migrate(t.Context(), database))

	_, err := database.ExecContext(t.Context(), `
		INSERT INTO weeks (id, season, season_type, week_num)
		VALUES (2026209, 2026, 2, 9);
		INSERT INTO games (
			id, week_id, home_team, away_team, home_name, away_name,
			home_espn_id, away_espn_id, home_logo, away_logo, kickoff_utc,
			status, is_indoor, updated_at
		) VALUES (
			'401772001', 2026209, 'BUF', 'MIA', 'Buffalo Bills', 'Miami Dolphins',
			'2', '15', 'https://a.espncdn.com/buf.png', NULL, '2026-11-08T18:00:00Z',
			'SCHEDULED', 0, '2026-11-07T12:00:00Z'
		);
		INSERT INTO roster_players (
			athlete_id, team, espn_team_id, full_name, norm_name, position, updated_at
		) VALUES (
			'3918298', 'BUF', '2', 'Josh Allen', 'josh allen', 'QB', '2026-11-07T12:00:00Z'
		);`)
	require.NoError(t, err)

	var (
		homeLogo    sql.NullString
		awayLogo    sql.NullString
		headshotURL string
	)
	require.NoError(t, database.QueryRowContext(t.Context(),
		"SELECT home_logo, away_logo FROM games WHERE id = '401772001'").
		Scan(&homeLogo, &awayLogo))
	assert.True(t, homeLogo.Valid)
	assert.Equal(t, "https://a.espncdn.com/buf.png", homeLogo.String)
	assert.False(t, awayLogo.Valid)

	require.NoError(t, database.QueryRowContext(t.Context(),
		"SELECT headshot_url FROM roster_players WHERE athlete_id = '3918298'").
		Scan(&headshotURL))
	assert.Empty(t, headshotURL)
}

func TestMigration0013CanonicalPlayers(t *testing.T) {
	t.Parallel()

	database := openTestDB(t)

	// Apply migrations 0001 through 0012 manually.
	entries, err := os.ReadDir("migrations")
	require.NoError(t, err)
	for _, entry := range entries {
		if entry.Name() < "0013" && filepath.Ext(entry.Name()) == ".sql" {
			content, err := os.ReadFile(filepath.Join("migrations", entry.Name()))
			require.NoError(t, err)
			_, err = database.ExecContext(t.Context(), string(content))
			require.NoError(t, err)
		}
	}
	_, err = database.ExecContext(t.Context(), "PRAGMA user_version = 12")
	require.NoError(t, err)

	// Seed prod-shaped pre-0013 rows with colliding player name variants, stale
	// roster norm_name, and legs missing player_id/team.
	_, err = database.ExecContext(t.Context(), `
		INSERT INTO users (id, name, avatar_color, active) VALUES (1, 'Ada', 'sky', 1);
		INSERT INTO groups (id, name, invite_token) VALUES (1, 'Sunday Club', 'token-1');
		INSERT INTO weeks (id, season, season_type, week_num) VALUES (2026209, 2026, 2, 9);
		INSERT INTO slips (id, group_id, week_id, name) VALUES (10, 1, 2026209, 'Week 9 Slip');
		INSERT INTO games (
			id, week_id, home_team, away_team, home_name, away_name,
			home_espn_id, away_espn_id, kickoff_utc, status, is_indoor, updated_at
		) VALUES (
			'g1', 2026209, 'BUF', 'MIA', 'Buffalo Bills', 'Miami Dolphins',
			'2', '15', '2026-11-08T18:00:00Z', 'SCHEDULED', 0, '2026-11-07T12:00:00Z'
		);
		INSERT INTO roster_players (
			athlete_id, team, espn_team_id, full_name, norm_name, position, updated_at
		) VALUES
			('ollie-mia', 'MIA', '15', 'Ollie Gordon II', 'ollie gordon', 'RB', '2026-11-07T12:00:00Z'),
			('jose-mia', 'MIA', '15', 'José Alvarado', 'josé alvarado', 'WR', '2026-11-07T12:00:00Z');

		-- Colliding Bovada rows for Ollie Gordon: the (MIA) row has link+sid and should win.
		INSERT INTO odds (
			game_id, book, market_type, team, player_name, side, line, price, link, sid, fetched_at
		) VALUES (
			'g1', 'bovada', 'RUSH_YDS', '', 'Ollie Gordon II (MIA)', 'OVER',
			45.5, -110, 'https://bovada.lv/1', 'sid-1', '2026-11-08T10:00:00Z'
		);
		INSERT INTO odds (
			game_id, book, market_type, team, player_name, side, line, price, fetched_at
		) VALUES
			('g1', 'bovada', 'RUSH_YDS', 'MIA', 'Ollie Gordon II', 'OVER', 45.5, -112, '2026-11-08T09:00:00Z'),
			('g1', 'bovada', 'RUSH_YDS', '', 'Ollie Gordon', 'OVER', 45.5, -115, '2026-11-08T08:00:00Z'),
			('g1', 'sleeper', 'RUSH_YDS', '', 'Ollie Gordon', 'OVER', 46.5, -105, '2026-11-08T10:00:00Z'),
			('g1', 'draftkings', 'REC_YDS', '', 'DeMario Douglas', 'OVER', 35.5, -110, '2026-11-08T10:00:00Z'),
			('g1', 'fanduel', 'REC_YDS', '', 'Demario Douglas (MIA)', 'OVER', 35.5, -115, '2026-11-08T10:00:00Z'),
			('g1', 'draftkings', 'ML', 'BUF', '', 'TEAM', NULL, -140, '2026-11-08T10:00:00Z');

		INSERT INTO legs (id, slip_id, user_id, game_id, market_type, selection, team, player_name, side, line, odds)
		VALUES
			(1, 10, 1, 'g1', 'RUSH_YDS', 'Ollie Gordon II (MIA) Over 45.5', NULL, 'Ollie Gordon II (MIA)', 'OVER', 45.5, -110),
			(2, 10, 1, 'g1', 'RUSH_YDS', 'Ollie Gordon Over 46.5', '', 'Ollie Gordon', 'OVER', 46.5, -105),
			(3, 10, 1, NULL, 'REC_YDS', 'José Alvarado Over 25.5', NULL, 'José Alvarado', 'OVER', 25.5, -110);
		INSERT INTO legs (id, slip_id, user_id, game_id, market_type, selection, team, side, odds)
		VALUES (4, 10, 1, 'g1', 'ML', 'BUF ML', 'BUF', 'TEAM', -140);
	`)
	require.NoError(t, err)

	require.NoError(t, db.Migrate(t.Context(), database))
	assert.Equal(t, latestSchemaVersion, schemaVersion(t, database))

	// Verify roster norm_name accent folding was refreshed.
	var joseNorm string
	require.NoError(t, database.QueryRowContext(t.Context(),
		"SELECT norm_name FROM roster_players WHERE athlete_id = 'jose-mia'").Scan(&joseNorm))
	assert.Equal(t, "jose alvarado", joseNorm)

	// Verify odds deduplication & canonicalization.
	var totalOdds int
	require.NoError(t, database.QueryRowContext(t.Context(), "SELECT count(*) FROM odds").Scan(&totalOdds))
	assert.Equal(t, 5, totalOdds, "3 colliding bovada rows must collapse to 1")

	var (
		bovadaTeam, bovadaPlayer string
		bovadaPrice              int
		bovadaLink               sql.NullString
	)
	require.NoError(t, database.QueryRowContext(t.Context(),
		"SELECT team, player_name, price, link FROM odds WHERE book = 'bovada' AND market_type = 'RUSH_YDS'").
		Scan(&bovadaTeam, &bovadaPlayer, &bovadaPrice, &bovadaLink))
	assert.Equal(t, "MIA", bovadaTeam)
	assert.Equal(t, "Ollie Gordon II", bovadaPlayer)
	assert.Equal(t, -110, bovadaPrice, "richer row with link+sid must win")
	assert.Equal(t, "https://bovada.lv/1", bovadaLink.String)

	var sleeperTeam, sleeperPlayer string
	require.NoError(t, database.QueryRowContext(t.Context(),
		"SELECT team, player_name FROM odds WHERE book = 'sleeper' AND market_type = 'RUSH_YDS'").
		Scan(&sleeperTeam, &sleeperPlayer))
	assert.Equal(t, "MIA", sleeperTeam)
	assert.Equal(t, "Ollie Gordon II", sleeperPlayer)

	var dkPlayer, fdTeam, fdPlayer string
	require.NoError(t, database.QueryRowContext(t.Context(),
		"SELECT player_name FROM odds WHERE book = 'draftkings' AND market_type = 'REC_YDS'").
		Scan(&dkPlayer))
	require.NoError(t, database.QueryRowContext(t.Context(),
		"SELECT team, player_name FROM odds WHERE book = 'fanduel' AND market_type = 'REC_YDS'").
		Scan(&fdTeam, &fdPlayer))
	assert.Equal(t, "DeMario Douglas", dkPlayer)
	assert.Equal(t, "MIA", fdTeam)
	assert.Equal(t, dkPlayer, fdPlayer, "unmatched casing variants must unify across books")

	// Verify legs backfill.
	type legRow struct {
		team, playerName, playerID sql.NullString
	}
	readLeg := func(id int64) legRow {
		var r legRow
		require.NoError(t, database.QueryRowContext(t.Context(),
			"SELECT team, player_name, player_id FROM legs WHERE id = ?", id).
			Scan(&r.team, &r.playerName, &r.playerID))
		return r
	}

	leg1 := readLeg(1)
	assert.Equal(t, "MIA", leg1.team.String)
	assert.Equal(t, "Ollie Gordon II", leg1.playerName.String)
	assert.Equal(t, "ollie-mia", leg1.playerID.String)

	leg2 := readLeg(2)
	assert.Equal(t, "MIA", leg2.team.String)
	assert.Equal(t, "Ollie Gordon II", leg2.playerName.String)
	assert.Equal(t, "ollie-mia", leg2.playerID.String)

	leg3 := readLeg(3)
	assert.Equal(t, "MIA", leg3.team.String)
	assert.Equal(t, "José Alvarado", leg3.playerName.String)
	assert.Equal(t, "jose-mia", leg3.playerID.String)

	leg4 := readLeg(4)
	assert.Equal(t, "BUF", leg4.team.String)
	assert.False(t, leg4.playerName.Valid)
	assert.False(t, leg4.playerID.Valid)

	// Re-running BackfillCanonicalPlayers directly must be idempotent.
	require.NoError(t, db.BackfillCanonicalPlayers(t.Context(), database))
	require.NoError(t, database.QueryRowContext(t.Context(), "SELECT count(*) FROM odds").Scan(&totalOdds))
	assert.Equal(t, 5, totalOdds)
}

func TestMigration0014RosterInjuries(t *testing.T) {
	t.Parallel()

	database := openTestDB(t)

	// Apply migrations 0001 through 0013 manually.
	entries, err := os.ReadDir("migrations")
	require.NoError(t, err)
	for _, entry := range entries {
		if entry.Name() < "0014" && filepath.Ext(entry.Name()) == ".sql" {
			content, err := os.ReadFile(filepath.Join("migrations", entry.Name()))
			require.NoError(t, err)
			_, err = database.ExecContext(t.Context(), string(content))
			require.NoError(t, err)
		}
	}
	_, err = database.ExecContext(t.Context(), "PRAGMA user_version = 13")
	require.NoError(t, err)

	// Seed a pre-0014 roster player.
	_, err = database.ExecContext(t.Context(), `
		INSERT INTO roster_players (
			athlete_id, team, espn_team_id, full_name, norm_name, position, updated_at
		) VALUES (
			'4385690', 'BUF', '2', 'Dalton Kincaid', 'dalton kincaid', 'TE', '2026-11-07T12:00:00Z'
		)`)
	require.NoError(t, err)

	require.NoError(t, db.Migrate(t.Context(), database))
	assert.Equal(t, latestSchemaVersion, schemaVersion(t, database))

	// Existing row must default both injury columns to ''.
	var status, date string
	require.NoError(t, database.QueryRowContext(t.Context(),
		"SELECT injury_status, injury_date FROM roster_players WHERE athlete_id = '4385690'").
		Scan(&status, &date))
	assert.Empty(t, status)
	assert.Empty(t, date)

	// Inserting a row with injury_status and injury_date works.
	_, err = database.ExecContext(t.Context(), `
		INSERT INTO roster_players (
			athlete_id, team, espn_team_id, full_name, norm_name, position,
			injury_status, injury_date, updated_at
		) VALUES (
			'3918298', 'BUF', '2', 'Josh Allen', 'josh allen', 'QB',
			'Questionable', '2026-11-08T12:00:00Z', '2026-11-08T12:00:00Z'
		)`)
	require.NoError(t, err)
	require.NoError(t, database.QueryRowContext(t.Context(),
		"SELECT injury_status, injury_date FROM roster_players WHERE athlete_id = '3918298'").
		Scan(&status, &date))
	assert.Equal(t, "Questionable", status)
	assert.Equal(t, "2026-11-08T12:00:00Z", date)
}

func TestMigration0015ResearchContext(t *testing.T) {
	t.Parallel()

	database := openTestDB(t)
	require.NoError(t, db.Migrate(t.Context(), database))

	_, err := database.ExecContext(t.Context(), `
		INSERT INTO player_gamelogs (athlete_id, games_played, stats_json, updated_at)
		VALUES ('3918298', 1, '{}', '2026-11-08T12:00:00Z')`)
	require.NoError(t, err)

	var gamesJSON string
	require.NoError(t, database.QueryRowContext(t.Context(),
		"SELECT games_json FROM player_gamelogs WHERE athlete_id = '3918298'").Scan(&gamesJSON))
	assert.JSONEq(t, "[]", gamesJSON)

	_, err = database.ExecContext(t.Context(), `
		INSERT INTO nflverse_player_weeks (
			espn_id, season, season_type, week_num, team,
			offense_snaps, offense_pct, target_share, air_yards_share, wopr,
			carries, targets, total_epa, rz_opps, updated_at
		) VALUES (
			'4362628', 2026, 2, 9, 'CIN',
			50, 0.9, 0.3, 0.4, 0.7,
			0, 10, 5.2, 2, '2026-11-08T12:00:00Z'
		)`)
	require.NoError(t, err)

	var (
		opponent   string
		playerName string
		position   string
		recYds     int
		kickerPts  int
	)
	require.NoError(t, database.QueryRowContext(t.Context(),
		`SELECT opponent, player_name, position, rec_yds, kicker_pts
		 FROM nflverse_player_weeks WHERE espn_id = '4362628'`).
		Scan(&opponent, &playerName, &position, &recYds, &kickerPts))
	assert.Empty(t, opponent)
	assert.Empty(t, playerName)
	assert.Empty(t, position)
	assert.Zero(t, recYds)
	assert.Zero(t, kickerPts)

	var idxName string
	require.NoError(t, database.QueryRowContext(t.Context(),
		`SELECT name FROM sqlite_master
		 WHERE type = 'index' AND name = 'idx_nflverse_player_weeks_opp_pos'`).
		Scan(&idxName))
	assert.Equal(t, "idx_nflverse_player_weeks_opp_pos", idxName)
}

func TestMigration0018LiveGameState(t *testing.T) {
	t.Parallel()

	database := openTestDB(t)
	require.NoError(t, db.Migrate(t.Context(), database))

	_, err := database.ExecContext(t.Context(),
		"INSERT INTO weeks (id, season, season_type, week_num) VALUES (2026209, 2026, 2, 9)")
	require.NoError(t, err)
	_, err = database.ExecContext(t.Context(), `
		INSERT INTO games (
			id, week_id, home_team, away_team, home_name, away_name,
			home_espn_id, away_espn_id, kickoff_utc, status, is_indoor, updated_at
		) VALUES (
			'401772001', 2026209, 'BUF', 'MIA', 'Buffalo Bills', 'Miami Dolphins',
			'2', '15', '2026-11-08T18:00:00Z', 'SCHEDULED', 0, '2026-11-07T12:00:00Z'
		)`)
	require.NoError(t, err)

	var (
		period   sql.NullInt64
		clock    sql.NullString
		boxscore sql.NullString
	)
	require.NoError(t, database.QueryRowContext(t.Context(),
		"SELECT period, clock, boxscore_json FROM games WHERE id = '401772001'").
		Scan(&period, &clock, &boxscore))
	assert.False(t, period.Valid)
	assert.False(t, clock.Valid)
	assert.False(t, boxscore.Valid)

	_, err = database.ExecContext(t.Context(), `
		UPDATE games
		SET period = 3, clock = '8:42', boxscore_json = '{"hasStats":true}'
		WHERE id = '401772001'`)
	require.NoError(t, err)

	require.NoError(t, database.QueryRowContext(t.Context(),
		"SELECT period, clock, boxscore_json FROM games WHERE id = '401772001'").
		Scan(&period, &clock, &boxscore))
	assert.True(t, period.Valid)
	assert.Equal(t, int64(3), period.Int64)
	assert.Equal(t, "8:42", clock.String)
	assert.Equal(t, `{"hasStats":true}`, boxscore.String)
}

func TestMigration0019V2OpportunitySignals(t *testing.T) {
	t.Parallel()

	database := openTestDB(t)
	require.NoError(t, db.Migrate(t.Context(), database))

	_, err := database.ExecContext(t.Context(), `
		INSERT INTO nflverse_player_weeks (
			espn_id, season, season_type, week_num, team,
			offense_snaps, offense_pct, target_share, air_yards_share, wopr,
			carries, targets, total_epa, rz_opps, updated_at
		) VALUES (
			'4362628', 2026, 2, 9, 'CIN',
			50, 0.9, 0.3, 0.4, 0.7,
			1, 10, 5.2, 2, '2026-11-08T12:00:00Z'
		)`)
	require.NoError(t, err)

	var (
		expPassYds, expRushYds, expRecYds, expReceptions float64
		expPassTDs, expRushTDs, expRecTDs                float64
		carryShare, cpoe                                 float64
	)
	require.NoError(t, database.QueryRowContext(t.Context(), `
		SELECT exp_pass_yds, exp_rush_yds, exp_rec_yds, exp_receptions,
		       exp_pass_tds, exp_rush_tds, exp_rec_tds, carry_share, cpoe
		FROM nflverse_player_weeks WHERE espn_id = '4362628'`).
		Scan(
			&expPassYds, &expRushYds, &expRecYds, &expReceptions,
			&expPassTDs, &expRushTDs, &expRecTDs, &carryShare, &cpoe,
		))
	assert.Zero(t, expPassYds)
	assert.Zero(t, expRushYds)
	assert.Zero(t, expRecYds)
	assert.Zero(t, expReceptions)
	assert.Zero(t, expPassTDs)
	assert.Zero(t, expRushTDs)
	assert.Zero(t, expRecTDs)
	assert.Zero(t, carryShare)
	assert.Zero(t, cpoe)
}

func TestMigration0022PruneBulkArchive(t *testing.T) {
	t.Parallel()

	database := openTestDB(t)

	// Apply migrations 0001 through 0021 manually.
	entries, err := os.ReadDir("migrations")
	require.NoError(t, err)
	for _, entry := range entries {
		if entry.Name() < "0022" && filepath.Ext(entry.Name()) == ".sql" {
			content, err := os.ReadFile(filepath.Join("migrations", entry.Name()))
			require.NoError(t, err)
			_, err = database.ExecContext(t.Context(), string(content))
			require.NoError(t, err)
		}
	}
	_, err = database.ExecContext(t.Context(), "PRAGMA user_version = 21")
	require.NoError(t, err)

	// Seed api_bodies and api_responses across kept and pruned sources,
	// including a shared body referenced by both espn and nfelo.
	_, err = database.ExecContext(t.Context(), `
		INSERT INTO api_bodies (sha256, encoding, size, body) VALUES
			('sha-espn', 'gzip', 100, X'1f8b0800'),
			('sha-odds', 'gzip', 120, X'1f8b0801'),
			('sha-weather', 'gzip', 80, X'1f8b0802'),
			('sha-shared', 'gzip', 50, X'1f8b0803'),
			('sha-nflverse', 'gzip', 5000, X'1f8b0804'),
			('sha-nfelo', 'gzip', 2000, X'1f8b0805');

		INSERT INTO api_responses (
			source, key, method, url, status, fetched_at, duration_ms, body_sha256
		) VALUES
			('espn', 'espn:summary:401', 'GET', 'https://site.api.espn.com/summary', 200, '2026-11-08T17:00:00Z', 40, 'sha-espn'),
			('espn', 'espn:roster:buf', 'GET', 'https://site.api.espn.com/roster', 200, '2026-11-08T17:01:00Z', 35, 'sha-shared'),
			('odds', 'odds:lines', 'GET', 'https://api.the-odds-api.com/v4/sports/americanfootball_nfl/odds', 200, '2026-11-08T17:02:00Z', 55, 'sha-odds'),
			('weather', 'weather:buf', 'GET', 'https://api.open-meteo.com/v1/forecast', 200, '2026-11-08T17:03:00Z', 25, 'sha-weather'),
			('nflverse', 'nflverse:player_stats:2026', 'GET', 'https://github.com/nflverse/nflverse-data/releases', 200, '2026-11-08T17:04:00Z', 300, 'sha-nflverse'),
			('nfelo', 'nfelo:projections', 'GET', 'https://raw.githubusercontent.com/greerreNFL/nfelo/main/nfelo.csv', 200, '2026-11-08T17:05:00Z', 150, 'sha-nfelo'),
			('nfelo', 'nfelo:dup', 'GET', 'https://raw.githubusercontent.com/greerreNFL/nfelo/main/dup.csv', 200, '2026-11-08T17:06:00Z', 90, 'sha-shared');
	`)
	require.NoError(t, err)

	require.NoError(t, db.Migrate(t.Context(), database))
	assert.Equal(t, latestSchemaVersion, schemaVersion(t, database))

	// Pruned sources must be gone from api_responses.
	var prunedCount int
	require.NoError(t, database.QueryRowContext(t.Context(),
		"SELECT count(*) FROM api_responses WHERE source IN ('nflverse', 'nfelo')").Scan(&prunedCount))
	assert.Zero(t, prunedCount)

	// Kept sources must survive intact.
	var keptSources []string
	rows, err := database.QueryContext(t.Context(),
		"SELECT source || ':' || key || ':' || body_sha256 FROM api_responses ORDER BY id")
	require.NoError(t, err)
	defer func() { assert.NoError(t, rows.Close()) }()
	for rows.Next() {
		var entry string
		require.NoError(t, rows.Scan(&entry))
		keptSources = append(keptSources, entry)
	}
	require.NoError(t, rows.Err())
	assert.Equal(t, []string{
		"espn:espn:summary:401:sha-espn",
		"espn:espn:roster:buf:sha-shared",
		"odds:odds:lines:sha-odds",
		"weather:weather:buf:sha-weather",
	}, keptSources)

	// Only unreferenced bodies are removed; shared body referenced by espn survives.
	var keptBodies []string
	bodyRows, err := database.QueryContext(t.Context(),
		"SELECT sha256 FROM api_bodies ORDER BY sha256")
	require.NoError(t, err)
	defer func() { assert.NoError(t, bodyRows.Close()) }()
	for bodyRows.Next() {
		var sha string
		require.NoError(t, bodyRows.Scan(&sha))
		keptBodies = append(keptBodies, sha)
	}
	require.NoError(t, bodyRows.Err())
	assert.Equal(t, []string{
		"sha-espn",
		"sha-odds",
		"sha-shared",
		"sha-weather",
	}, keptBodies)
}

// schemaVersion reads PRAGMA user_version, the migration runner's record of the
// schema it applied.
func schemaVersion(t *testing.T, database *sql.DB) int {
	t.Helper()

	var version int
	require.NoError(t, database.QueryRowContext(t.Context(), "PRAGMA user_version").Scan(&version))
	return version
}

func tableNames(t *testing.T, database *sql.DB) []string {
	t.Helper()

	rows, err := database.QueryContext(t.Context(), `
		SELECT name FROM sqlite_master
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%'
		ORDER BY name`)
	require.NoError(t, err)
	defer func() { assert.NoError(t, rows.Close()) }()

	var names []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		names = append(names, name)
	}
	require.NoError(t, rows.Err())
	return names
}

func userCount(t *testing.T, database *sql.DB) int {
	t.Helper()

	var count int
	require.NoError(t, database.QueryRowContext(t.Context(), "SELECT count(*) FROM users").Scan(&count))
	return count
}
