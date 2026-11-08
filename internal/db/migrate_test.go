package db_test

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
)

// latestSchemaVersion is the version the last migration records.
const latestSchemaVersion = 5

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
		"groups",
		"legs",
		"odds",
		"player_matches",
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
