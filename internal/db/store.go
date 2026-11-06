package db

import (
	"context"
	"database/sql"
	"fmt"
)

// Store runs the app's hand-written SQL against the database.
type Store struct {
	db *sql.DB
	tx *sql.Tx
}

// NewStore wraps database in a Store. The caller keeps ownership of the
// database handle.
func NewStore(database *sql.DB) *Store {
	return &Store{db: database}
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

	if err := fn(&Store{db: s.db, tx: tx}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// conn returns the transaction in progress, or the database when there is none.
func (s *Store) conn() queryer {
	if s.tx != nil {
		return s.tx
	}
	return s.db
}

// queryer is the slice of *sql.DB and *sql.Tx the store's queries use.
type queryer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}
