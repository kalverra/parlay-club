package db

import (
	"context"
	"database/sql"
	"fmt"
)

// Store runs the app's hand-written SQL against the database.
type Store struct {
	db     *sql.DB
	readDB *sql.DB
	tx     *sql.Tx
}

// NewStore wraps database in a Store. For file-backed databases it opens a
// multi-connection query_only=ON read pool alongside the single-writer handle;
// for :memory: databases it reuses the single handle. The caller keeps
// ownership of the writer handle.
func NewStore(database *sql.DB) *Store {
	readDB := database
	if file := dbFilePath(database); file != "" {
		if pool, err := openReadDB(file); err == nil {
			readDB = pool
		}
	}
	return &Store{db: database, readDB: readDB}
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

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := fn(&Store{db: s.db, readDB: s.readDB, tx: tx}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
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
