package db_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
)

// openTestDB opens a fresh database file under the test's temp dir.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })

	return database
}

func TestOpenAppliesPragmas(t *testing.T) {
	t.Parallel()

	database := openTestDB(t)

	tests := map[string]string{
		"journal_mode": "wal",
		"foreign_keys": "1",
		"busy_timeout": "5000",
		"synchronous":  "1",
	}
	for pragma, want := range tests {
		var got string
		require.NoError(t, database.QueryRowContext(t.Context(), "PRAGMA "+pragma).Scan(&got), pragma)
		assert.Equal(t, want, got, pragma)
	}
}

func TestOpenEnforcesForeignKeys(t *testing.T) {
	t.Parallel()

	database := openTestDB(t)
	_, err := database.ExecContext(t.Context(), "CREATE TABLE parent (id INTEGER PRIMARY KEY)")
	require.NoError(t, err)
	_, err = database.ExecContext(
		t.Context(),
		"CREATE TABLE child (parent_id INTEGER NOT NULL REFERENCES parent(id))",
	)
	require.NoError(t, err)

	_, err = database.ExecContext(t.Context(), "INSERT INTO child (parent_id) VALUES (1)")
	require.Error(t, err, "a child row without a parent must be rejected")
}

func TestOpenCapsPoolAtOneConnection(t *testing.T) {
	t.Parallel()

	database := openTestDB(t)

	assert.Equal(t, 1, database.Stats().MaxOpenConnections)
}

func TestOpenCreatesMissingDirectory(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", "data", "parlay.db")
	database, err := db.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })

	_, err = os.Stat(path)
	require.NoError(t, err, "the database file must exist after Open")
}

func TestOpenAcceptsRelativePath(t *testing.T) {
	t.Parallel()

	abs := filepath.Join(t.TempDir(), "data", "parlay.db")
	cwd, err := os.Getwd()
	require.NoError(t, err)
	rel, err := filepath.Rel(cwd, abs)
	require.NoError(t, err)

	database, err := db.Open(rel)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })

	_, err = os.Stat(abs)
	require.NoError(t, err, "the database must land at the path Open was given")
}

func TestOpenFailsWhenDirectoryCannotBeCreated(t *testing.T) {
	t.Parallel()

	blocker := filepath.Join(t.TempDir(), "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("not a directory"), 0o600))

	_, err := db.Open(filepath.Join(blocker, "parlay.db"))
	require.Error(t, err)
}
