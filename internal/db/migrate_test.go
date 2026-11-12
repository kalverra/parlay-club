package db_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/slips"
)

// latestSchemaVersion is the version the last migration records.
const latestSchemaVersion = 9

func TestMigrateFreshDatabase(t *testing.T) {
	t.Parallel()

	database := openTestDB(t)
	require.NoError(t, db.Migrate(t.Context(), database))

	assert.Equal(t, latestSchemaVersion, schemaVersion(t, database))
	assert.ElementsMatch(t, []string{
		"activity",
		"api_bodies",
		"api_responses",
		"games",
		"group_members",
		"group_slip_schedules",
		"groups",
		"leg_votes",
		"legs",
		"nfelo_games",
		"odds",
		"player_gamelogs",
		"player_matches",
		"roster_players",
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

	// Run migration to apply 0009
	require.NoError(t, db.Migrate(t.Context(), database))
	assert.Equal(t, 9, schemaVersion(t, database))

	// Verify backfilled schedule is created and linked to the existing slip
	store := db.NewStore(database)
	schedules, err := store.GroupSlipSchedules(t.Context(), 1)
	require.NoError(t, err)
	require.Len(t, schedules, 1)
	assert.Equal(t, "Main Sunday Slip", schedules[0].Name)

	slip, err := store.SlipByID(t.Context(), 10)
	require.NoError(t, err)
	require.NotNil(t, slip.ScheduleID, "existing newest slip must be linked to backfilled schedule")
	assert.Equal(t, schedules[0].ID, *slip.ScheduleID)

	// Now run EnsureWeekSlips on the week
	slipsList, err := slips.EnsureWeekSlips(t.Context(), store, 1, 2026209)
	require.NoError(t, err)
	assert.Len(t, slipsList, 1, "week must still have exactly one slip, no duplicate created")
	assert.Equal(t, int64(10), slipsList[0].ID)
	assert.Equal(t, "Week 9 Sandbox", slipsList[0].Name)
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
