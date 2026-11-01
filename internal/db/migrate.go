package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
)

// migrations holds every schema migration, applied in file-name order.
//
//go:embed migrations/*.sql
var migrations embed.FS

const migrationsDir = "migrations"

// migration is one embedded SQL file.
type migration struct {
	version int
	name    string
}

// Migrate applies every embedded migration newer than PRAGMA user_version, in
// number order, one transaction each. A second run changes nothing.
func Migrate(ctx context.Context, database *sql.DB) error {
	return migrate(ctx, database, migrations, migrationsDir)
}

// migrate is Migrate with the migration source passed in, so tests can use an
// in-memory file system.
func migrate(ctx context.Context, database *sql.DB, fsys fs.FS, dir string) error {
	list, err := migrationList(fsys, dir)
	if err != nil {
		return err
	}

	current, err := userVersion(ctx, database)
	if err != nil {
		return err
	}

	for _, next := range list {
		if next.version <= current {
			continue
		}
		if err := apply(ctx, database, fsys, dir, next); err != nil {
			return err
		}
		current = next.version
	}
	return nil
}

// migrationList returns the migrations in fsys sorted by number. A name that
// does not start with a positive number, or a repeated number, is an error.
func migrationList(fsys fs.FS, dir string) ([]migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}

	list := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || path.Ext(entry.Name()) != ".sql" {
			continue
		}
		prefix, _, found := strings.Cut(entry.Name(), "_")
		version, err := strconv.Atoi(prefix)
		if !found || err != nil || version < 1 {
			return nil, fmt.Errorf("migration %q must be named NNNN_name.sql", entry.Name())
		}
		list = append(list, migration{version: version, name: entry.Name()})
	}

	sort.Slice(list, func(i, j int) bool { return list[i].version < list[j].version })
	for i := 1; i < len(list); i++ {
		if list[i].version == list[i-1].version {
			return nil, fmt.Errorf(
				"migrations %q and %q share version %d",
				list[i-1].name,
				list[i].name,
				list[i].version,
			)
		}
	}
	return list, nil
}

// userVersion reads the schema version SQLite keeps in the database header.
func userVersion(ctx context.Context, database *sql.DB) (int, error) {
	var version int
	if err := database.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	return version, nil
}

// apply runs one migration and records its version, committing only when both
// succeed.
func apply(ctx context.Context, database *sql.DB, fsys fs.FS, dir string, next migration) error {
	name := path.Join(dir, next.name)
	script, err := fs.ReadFile(fsys, name)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", name, err)
	}

	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", name, err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, string(script)); err != nil {
		return fmt.Errorf("apply migration %s: %w", name, err)
	}
	// PRAGMA values cannot be bound; the version comes from a parsed file name.
	if _, err := tx.ExecContext(ctx, "PRAGMA user_version = "+strconv.Itoa(next.version)); err != nil {
		return fmt.Errorf("record migration %s: %w", name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", name, err)
	}
	return nil
}
