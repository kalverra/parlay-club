package db

import (
	"database/sql"
	"path/filepath"
	"strconv"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openTestDB opens a fresh database file under the test's temp dir.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	database, err := Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })

	return database
}

func TestMigrateAppliesMigrationsInOrder(t *testing.T) {
	t.Parallel()

	database := openTestDB(t)
	fsys := fstest.MapFS{
		"m/0001_one.sql": {Data: []byte("CREATE TABLE one (id INTEGER PRIMARY KEY);")},
		"m/0002_two.sql": {Data: []byte("CREATE TABLE two (id INTEGER PRIMARY KEY);")},
		"m/0003_three.sql": {Data: []byte(
			"CREATE TABLE three (id INTEGER PRIMARY KEY, two_id INTEGER REFERENCES two(id));",
		)},
	}

	require.NoError(t, migrate(t.Context(), database, fsys, "m"))

	// three has a foreign key to two, so it only applies after two.
	for _, table := range []string{"one", "two", "three"} {
		assert.True(t, hasTable(t, database, table), table)
	}
	assert.Equal(t, 3, userVersionOf(t, database))
}

func TestMigrateSkipsAppliedMigrations(t *testing.T) {
	t.Parallel()

	database := openTestDB(t)
	setUserVersion(t, database, 1)
	fsys := fstest.MapFS{
		"m/0001_one.sql": {Data: []byte("CREATE TABLE one (id INTEGER PRIMARY KEY);")},
		"m/0002_two.sql": {Data: []byte("CREATE TABLE two (id INTEGER PRIMARY KEY);")},
	}

	require.NoError(t, migrate(t.Context(), database, fsys, "m"))

	assert.False(t, hasTable(t, database, "one"), "an applied migration must not run again")
	assert.True(t, hasTable(t, database, "two"))
	assert.Equal(t, 2, userVersionOf(t, database))
}

func TestMigrateRollsBackFailedMigration(t *testing.T) {
	t.Parallel()

	database := openTestDB(t)
	fsys := fstest.MapFS{
		"m/0001_broken.sql": {Data: []byte(
			"CREATE TABLE half (id INTEGER PRIMARY KEY); THIS IS NOT SQL;",
		)},
	}

	require.Error(t, migrate(t.Context(), database, fsys, "m"))

	assert.False(t, hasTable(t, database, "half"), "a failed migration must leave no tables behind")
	assert.Equal(t, 0, userVersionOf(t, database))
}

func setUserVersion(t *testing.T, database *sql.DB, version int) {
	t.Helper()

	_, err := database.ExecContext(t.Context(), "PRAGMA user_version = "+strconv.Itoa(version))
	require.NoError(t, err)
}

func userVersionOf(t *testing.T, database *sql.DB) int {
	t.Helper()

	var version int
	require.NoError(t, database.QueryRowContext(t.Context(), "PRAGMA user_version").Scan(&version))
	return version
}

func hasTable(t *testing.T, database *sql.DB, name string) bool {
	t.Helper()

	var count int
	err := database.QueryRowContext(
		t.Context(),
		"SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?",
		name,
	).Scan(&count)
	require.NoError(t, err)
	return count == 1
}
