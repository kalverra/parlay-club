package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/rs/zerolog"
)

const slowQueryThreshold = 100 * time.Millisecond

func durationMS(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000.0
}

// Store runs the app's hand-written SQL against the database.
type Store struct {
	db          *sql.DB
	readDB      *sql.DB
	readPoolErr error
	tx          *sql.Tx
	logger      zerolog.Logger
}

// NewStore wraps database in a Store. For file-backed databases it opens a
// multi-connection query_only=ON read pool alongside the single-writer handle;
// for :memory: databases it reuses the single handle. The caller keeps
// ownership of the writer handle.
func NewStore(database *sql.DB) *Store {
	readDB := database
	var readPoolErr error
	if file := dbFilePath(database); file != "" {
		pool, err := openReadDB(file)
		if err != nil {
			readPoolErr = err
		} else {
			readDB = pool
		}
	}
	return &Store{
		db:          database,
		readDB:      readDB,
		readPoolErr: readPoolErr,
		logger:      zerolog.Nop(),
	}
}

// WithLogger returns a shallow copy of s configured to emit slow-query warnings
// to logger. If opening the separate read pool failed, it logs the fallback once.
func (s *Store) WithLogger(logger zerolog.Logger) *Store {
	if s.readPoolErr != nil {
		logger.Warn().Err(s.readPoolErr).Msg("open sqlite read pool; falling back to writer handle")
	}
	return &Store{
		db:     s.db,
		readDB: s.readDB,
		tx:     s.tx,
		logger: logger,
	}
}

// Close closes the read pool when the store opened one separate from the
// writer handle. The caller keeps ownership of the writer handle.
func (s *Store) Close() error {
	if s.readDB != nil && s.readDB != s.db {
		return s.readDB.Close()
	}
	return nil
}

// ReadDB returns the read pool, or the writer handle when sharing one handle
// (such as :memory: databases).
func (s *Store) ReadDB() *sql.DB {
	if s.readDB != nil {
		return s.readDB
	}
	return s.db
}

// Tx runs fn in one transaction and commits when fn returns nil. The Store
// passed to fn uses that transaction, and it must not outlive fn. Calling Tx on
// a store already inside a transaction reuses it.
func (s *Store) Tx(ctx context.Context, fn func(*Store) error) error {
	if s.tx != nil {
		return fn(s)
	}

	start := time.Now()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := fn(&Store{db: s.db, readDB: s.readDB, tx: tx, logger: s.logger}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	if elapsed := time.Since(start); elapsed >= slowQueryThreshold {
		s.logger.Warn().
			Float64("duration_ms", durationMS(elapsed)).
			Msg("slow sqlite tx")
	}
	return nil
}

// conn returns the transaction in progress, or the writer database when there is none.
func (s *Store) conn() queryer {
	if s.tx != nil {
		return s.tx
	}
	return s.db
}

// readConn returns the transaction in progress, or the read pool when there is none.
func (s *Store) readConn() queryer {
	if s.tx != nil {
		return s.tx
	}
	if s.readDB != nil {
		return s.readDB
	}
	return s.db
}

// queryer is the slice of *sql.DB and *sql.Tx the store's queries use.
type queryer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func dbFilePath(database *sql.DB) string {
	rows, err := database.QueryContext(context.Background(), "PRAGMA database_list")
	if err != nil {
		return ""
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var seq int
		var name, file string
		if err := rows.Scan(&seq, &name, &file); err != nil {
			return ""
		}
		if name == "main" {
			return file
		}
	}
	return ""
}
