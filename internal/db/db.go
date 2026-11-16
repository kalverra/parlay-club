// Package db opens the SQLite database and runs its migrations.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	// Registers the pure-Go SQLite driver under the name "sqlite".
	_ "modernc.org/sqlite"
)

// pragmas are applied to every new connection in the order given.
//
// WAL lets reads proceed during writes; foreign_keys is off by default in
// SQLite, so it must be turned on per connection; synchronous NORMAL is the
// safe pairing with WAL; busy_timeout retries locks instead of failing.
var pragmas = []string{
	"journal_mode(WAL)",
	"busy_timeout(5000)",
	"foreign_keys(1)",
	"synchronous(NORMAL)",
}

// readPragmas are applied to every connection in the read-only pool.
var readPragmas = []string{
	"busy_timeout(5000)",
	"foreign_keys(1)",
	"query_only(ON)",
}

// Open opens the SQLite database at path, creating its directory when missing,
// and verifies the connection works. The connection pool is capped at one
// connection: traffic is tiny, and a single writer rules out SQLITE_BUSY.
//
// The caller owns the returned handle and must close it.
func Open(path string) (*sql.DB, error) {
	dataSource, err := openDSN(path)
	if err != nil {
		return nil, err
	}

	database, err := sql.Open("sqlite", dataSource)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	database.SetMaxOpenConns(1)

	if err := database.PingContext(context.Background()); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	return database, nil
}

// openReadDB opens a multi-connection query_only=ON pool against an existing
// file-backed SQLite database in WAL mode.
func openReadDB(path string) (*sql.DB, error) {
	database, err := sql.Open("sqlite", readDSN(path))
	if err != nil {
		return nil, fmt.Errorf("open sqlite read pool %s: %w", path, err)
	}
	database.SetMaxOpenConns(4)
	database.SetMaxIdleConns(4)

	if err := database.PingContext(context.Background()); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("open sqlite read pool %s: %w", path, err)
	}
	return database, nil
}

func isMemoryPath(path string) bool {
	return path == ":memory:" || strings.HasPrefix(path, "file::memory:") || strings.Contains(path, "mode=memory")
}

func openDSN(path string) (string, error) {
	if isMemoryPath(path) {
		if path == ":memory:" {
			return memoryDSN(), nil
		}
		return path, nil
	}

	// SQLite reads a file: URI's "./"-relative path as an authority. Resolve to
	// an absolute path so relative DB_PATH values work.
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve database path %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o750); err != nil {
		return "", fmt.Errorf("create database directory: %w", err)
	}
	return dsn(absPath), nil
}

func memoryDSN() string {
	params := url.Values{"_txlock": {"immediate"}}
	for _, pragma := range pragmas {
		params.Add("_pragma", pragma)
	}
	return "file::memory:?" + params.Encode()
}

// dsn builds a file URI with the pragmas as query parameters. Modernc's driver
// reads them on every connection, so each pooled connection gets them.
func dsn(path string) string {
	params := url.Values{"_txlock": {"immediate"}}
	for _, pragma := range pragmas {
		params.Add("_pragma", pragma)
	}
	uri := url.URL{Scheme: "file", Path: path, RawQuery: params.Encode()}
	return uri.String()
}

// readDSN builds a file URI for the concurrent read-only pool.
func readDSN(path string) string {
	params := url.Values{}
	for _, pragma := range readPragmas {
		params.Add("_pragma", pragma)
	}
	uri := url.URL{Scheme: "file", Path: path, RawQuery: params.Encode()}
	return uri.String()
}

// isConstraintUnique reports whether err represents a SQLite UNIQUE constraint violation.
func isConstraintUnique(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}
