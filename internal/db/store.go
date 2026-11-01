package db

import "database/sql"

// Store runs the app's hand-written SQL against the database.
type Store struct {
	db *sql.DB
}

// NewStore wraps database in a Store. The caller keeps ownership of the
// database handle.
func NewStore(database *sql.DB) *Store {
	return &Store{db: database}
}
