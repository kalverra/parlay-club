package db

import (
	"database/sql"
	"io/fs"
	"path"
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

// TestMigration0002PreservesData applies 0001, seeds every table that points at
// users, then runs the real migrations. The users rebuild must keep rows and
// foreign keys intact.
func TestMigration0002PreservesData(t *testing.T) {
	t.Parallel()

	database := openTestDB(t)
	script, err := fs.ReadFile(migrations, path.Join(migrationsDir, "0001_init.sql"))
	require.NoError(t, err)
	_, err = database.ExecContext(t.Context(), string(script))
	require.NoError(t, err)
	setUserVersion(t, database, 1)

	_, err = database.ExecContext(t.Context(), seed0001Data)
	require.NoError(t, err)

	require.NoError(t, Migrate(t.Context(), database))

	assert.Equal(t, 14, userVersionOf(t, database))
	counts := map[string]int{
		"SELECT count(*) FROM users":                2,
		"SELECT count(*) FROM groups":               1,
		"SELECT count(*) FROM group_members":        2,
		"SELECT count(*) FROM group_slip_schedules": 1,
		"SELECT count(*) FROM weeks":                1,
		"SELECT count(*) FROM games":                1,
		"SELECT count(*) FROM slips":                1,
		"SELECT count(*) FROM slip_members":         2,
		"SELECT count(*) FROM legs":                 2,
		"SELECT count(*) FROM wagers":               2,
		"SELECT count(*) FROM activity":             2,
	}
	for query, want := range counts {
		assert.Equal(t, want, countRows(t, database, query), query)
	}

	var (
		name      string
		color     string
		homeBook  sql.NullString
		active    bool
		createdAt string
		googleSub sql.NullString
		email     sql.NullString
		picture   sql.NullString
	)
	require.NoError(t, database.QueryRowContext(t.Context(), `
		SELECT name, avatar_color, home_book, active, created_at, google_sub, email, picture_url
		FROM users
		WHERE id = 1`).Scan(
		&name, &color, &homeBook, &active, &createdAt, &googleSub, &email, &picture,
	))
	assert.Equal(t, "Ada", name)
	assert.Equal(t, "sky", color)
	assert.False(t, homeBook.Valid)
	assert.True(t, active)
	assert.Equal(t, "2026-01-02T03:04:05Z", createdAt)
	assert.False(t, googleSub.Valid)
	assert.False(t, email.Valid)
	assert.False(t, picture.Valid)

	var selection string
	require.NoError(t, database.QueryRowContext(t.Context(),
		"SELECT selection FROM legs WHERE id = 2").Scan(&selection))
	assert.Equal(t, "OVER 44.5", selection)

	var stake, boost float64
	require.NoError(t, database.QueryRowContext(t.Context(),
		"SELECT stake, boost_pct FROM wagers WHERE slip_id = 1 AND user_id = 2").Scan(&stake, &boost))
	assert.InDelta(t, 25.0, stake, 0)
	assert.InDelta(t, 10.0, boost, 0)

	var message string
	require.NoError(t, database.QueryRowContext(t.Context(),
		"SELECT message FROM activity WHERE user_id IS NULL").Scan(&message))
	assert.Equal(t, "system note", message)

	var slateScope string
	var scheduleID sql.NullInt64
	require.NoError(t, database.QueryRowContext(t.Context(),
		"SELECT slate_scope, schedule_id FROM slips WHERE id = 1").Scan(&slateScope, &scheduleID))
	assert.Equal(t, "sunday_main", slateScope)
	assert.True(t, scheduleID.Valid, "backfill must link existing slip to schedule")

	var schedName, schedScope string
	var schedLegs, schedActive int
	require.NoError(t, database.QueryRowContext(t.Context(),
		"SELECT name, slate_scope, legs_per_member, is_active FROM group_slip_schedules WHERE group_id = 1").Scan(
		&schedName, &schedScope, &schedLegs, &schedActive,
	))
	assert.Equal(t, "Main Sunday Slip", schedName)
	assert.Equal(t, "sunday_main", schedScope)
	assert.Equal(t, 2, schedLegs)
	assert.Equal(t, 1, schedActive)

	assert.Empty(t, foreignKeyProblems(t, database), "the rebuild must keep every foreign key valid")

	_, err = database.ExecContext(t.Context(),
		"INSERT INTO users (name, avatar_color) VALUES ('ada', 'amber')")
	require.NoError(t, err, "display names may now repeat, ignoring case")

	_, err = database.ExecContext(t.Context(),
		"UPDATE users SET google_sub = 'google-1' WHERE id = 1")
	require.NoError(t, err)
	_, err = database.ExecContext(t.Context(),
		"UPDATE users SET google_sub = 'google-1' WHERE id = 2")
	require.Error(t, err, "google_sub is unique")

	_, err = database.ExecContext(t.Context(), "UPDATE users SET email = 'ada@example.com' WHERE id = 1")
	require.NoError(t, err)
	_, err = database.ExecContext(t.Context(), "UPDATE users SET email = 'ADA@EXAMPLE.COM' WHERE id = 2")
	require.Error(t, err, "email is unique ignoring case")
}

// seed0001Data fills every table that references users so the rebuild has rows
// to preserve.
const seed0001Data = `
INSERT INTO users (id, name, avatar_color, home_book, active, created_at) VALUES
  (1, 'Ada', 'sky', NULL, 1, '2026-01-02T03:04:05Z'),
  (2, 'Bob', 'rose', 'draftkings', 0, '2026-01-03T04:05:06Z');

INSERT INTO groups (id, name, invite_token) VALUES
  (1, 'Sunday Parlay Club', '00112233445566778899aabbccddeeff');

INSERT INTO group_members (group_id, user_id) VALUES (1, 1), (1, 2);

INSERT INTO weeks (id, season, season_type, week_num) VALUES (2026001, 2026, 2, 1);

INSERT INTO games (
  id, week_id, home_team, away_team, home_name, away_name, home_espn_id, away_espn_id,
  kickoff_utc, status, is_indoor, updated_at
) VALUES (
  'game-1', 2026001, 'BUF', 'MIA', 'Buffalo Bills', 'Miami Dolphins', '2', '15',
  '2026-09-10T17:00:00Z', 'SCHEDULED', 1, '2026-09-01T00:00:00Z'
);

INSERT INTO slips (id, group_id, week_id, name) VALUES (1, 1, 2026001, 'Sunday Six');

INSERT INTO slip_members (slip_id, user_id, locked_at) VALUES
  (1, 1, '2026-09-10T16:00:00Z'),
  (1, 2, NULL);

INSERT INTO legs (id, slip_id, user_id, game_id, market_type, selection, odds) VALUES
  (1, 1, 1, 'game-1', 'MONEYLINE', 'BUF', -110),
  (2, 1, 2, 'game-1', 'TOTAL', 'OVER 44.5', -105);

INSERT INTO wagers (slip_id, user_id, book, stake, odds, boost_pct) VALUES
  (1, 1, 'draftkings', 50, -110, 0),
  (1, 2, NULL, 25, 105, 10);

INSERT INTO activity (slip_id, user_id, message) VALUES
  (1, 1, 'created the slip'),
  (1, NULL, 'system note');
`

// countRows runs a count query and returns its value.
func countRows(t *testing.T, database *sql.DB, query string) int {
	t.Helper()

	var count int
	require.NoError(t, database.QueryRowContext(t.Context(), query).Scan(&count))
	return count
}

// foreignKeyProblems returns every row PRAGMA foreign_key_check reports.
func foreignKeyProblems(t *testing.T, database *sql.DB) []string {
	t.Helper()

	rows, err := database.QueryContext(t.Context(), "PRAGMA foreign_key_check")
	require.NoError(t, err)
	defer func() { assert.NoError(t, rows.Close()) }()

	var problems []string
	for rows.Next() {
		var (
			table  string
			rowID  int64
			parent string
			fkID   int64
		)
		require.NoError(t, rows.Scan(&table, &rowID, &parent, &fkID))
		problems = append(problems, table)
	}
	require.NoError(t, rows.Err())
	return problems
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
