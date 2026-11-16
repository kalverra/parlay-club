package db_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

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

func TestStoreOpensReadPoolForFileDB(t *testing.T) {
	t.Parallel()

	database := openTestDB(t)
	require.NoError(t, db.Migrate(t.Context(), database))

	store := db.NewStore(database)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })

	readDB := store.ReadDB()
	require.NotSame(t, database, readDB, "file-backed store must use a separate read pool")
	assert.Equal(t, 4, readDB.Stats().MaxOpenConnections)

	var queryOnly string
	require.NoError(t, readDB.QueryRowContext(t.Context(), "PRAGMA query_only").Scan(&queryOnly))
	assert.Equal(t, "1", queryOnly)

	_, err := readDB.ExecContext(t.Context(), "CREATE TABLE should_fail (id INTEGER PRIMARY KEY)")
	require.Error(t, err, "read pool must reject writes")
}

func TestStoreFallsBackToSingleHandleForMemoryDB(t *testing.T) {
	t.Parallel()

	database, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))

	store := db.NewStore(database)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })

	assert.Same(t, database, store.ReadDB(), "in-memory store must reuse the single handle")
	assert.Equal(t, 1, store.ReadDB().Stats().MaxOpenConnections)

	week, err := store.UpsertWeek(t.Context(), 2026, 2, 1)
	require.NoError(t, err)
	_, err = store.UpsertGame(t.Context(), db.Game{
		ID:       "mem-game-1",
		WeekID:   week.ID,
		HomeTeam: "BUF",
		AwayTeam: "KC",
		Kickoff:  time.Date(2026, 9, 13, 17, 0, 0, 0, time.UTC),
		Status:   db.GameScheduled,
	})
	require.NoError(t, err)

	changed, err := store.UpsertOdds(t.Context(), []db.Odds{{
		GameID:     "mem-game-1",
		Book:       "draftkings",
		MarketType: "spread",
		Team:       "BUF",
		Side:       "home",
		Price:      -110,
		FetchedAt:  time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC),
	}})
	require.NoError(t, err)
	assert.True(t, changed)

	got, err := store.OddsForWeek(t.Context(), week.ID)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, -110, got[0].Price)
}

func TestConcurrentReadsDuringWrite(t *testing.T) {
	t.Parallel()

	database := openTestDB(t)
	require.NoError(t, db.Migrate(t.Context(), database))

	store := db.NewStore(database)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })

	week, err := store.UpsertWeek(t.Context(), 2026, 2, 1)
	require.NoError(t, err)
	_, err = store.UpsertGame(t.Context(), db.Game{
		ID:       "wal-game-1",
		WeekID:   week.ID,
		HomeTeam: "BUF",
		AwayTeam: "KC",
		Kickoff:  time.Date(2026, 9, 13, 17, 0, 0, 0, time.UTC),
		Status:   db.GameScheduled,
	})
	require.NoError(t, err)

	initialFetchedAt := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	changed, err := store.UpsertOdds(t.Context(), []db.Odds{{
		GameID:     "wal-game-1",
		Book:       "draftkings",
		MarketType: "spread",
		Team:       "BUF",
		Side:       "home",
		Price:      -110,
		FetchedAt:  initialFetchedAt,
	}})
	require.NoError(t, err)
	require.True(t, changed)

	txStarted := make(chan struct{})
	releaseTx := make(chan struct{})
	txDone := make(chan error, 1)

	go func() {
		txDone <- store.Tx(t.Context(), func(tx *db.Store) error {
			_, err := tx.UpsertOdds(t.Context(), []db.Odds{{
				GameID:     "wal-game-1",
				Book:       "draftkings",
				MarketType: "spread",
				Team:       "BUF",
				Side:       "home",
				Price:      -115,
				FetchedAt:  initialFetchedAt.Add(time.Minute),
			}})
			close(txStarted)
			<-releaseTx
			return err
		})
	}()

	<-txStarted

	// Parallel reads must complete without waiting for the open write tx.
	const readers = 8
	var wg sync.WaitGroup
	readErrs := make(chan error, readers)
	readPrices := make(chan int, readers)

	readCtx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	for range readers {
		wg.Go(func() {
			rows, err := store.OddsForWeek(readCtx, week.ID)
			if err != nil {
				readErrs <- err
				return
			}
			if len(rows) != 1 {
				readErrs <- assert.AnError
				return
			}
			readPrices <- rows[0].Price
		})
	}

	wg.Wait()
	close(readErrs)
	close(readPrices)

	for err := range readErrs {
		require.NoError(t, err)
	}
	for price := range readPrices {
		assert.Equal(t, -110, price, "readers must see the committed pre-tx snapshot")
	}

	close(releaseTx)
	require.NoError(t, <-txDone)

	after, err := store.OddsForWeek(t.Context(), week.ID)
	require.NoError(t, err)
	require.Len(t, after, 1)
	assert.Equal(t, -115, after[0].Price, "readers must see the new price once committed")
}
