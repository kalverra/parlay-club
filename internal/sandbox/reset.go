package sandbox

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// resetStatements clear every disposable table, children before parents, so
// one transaction can empty the sandbox without tripping a foreign key. The
// sandbox_state row and the migration bookkeeping stay put.
var resetStatements = []string{
	"DELETE FROM api_responses",
	"DELETE FROM api_bodies",
	"DELETE FROM activity",
	"DELETE FROM wagers",
	"DELETE FROM legs",
	"DELETE FROM slip_members",
	"DELETE FROM slips",
	"DELETE FROM player_gamelogs",
	"DELETE FROM roster_players",
	"DELETE FROM player_matches",
	"DELETE FROM nfelo_games",
	"DELETE FROM nflverse_player_weeks",
	"DELETE FROM nflverse_team_weeks",
	"DELETE FROM nflverse_player_ids",
	"DELETE FROM odds",
	"DELETE FROM games",
	"DELETE FROM weeks",
	"DELETE FROM group_members",
	"DELETE FROM groups",
	"DELETE FROM users",
	"DELETE FROM sync_state",
}

// Clear deletes every disposable row in one transaction. It keeps the schema,
// the migration version, and the sandbox's own stage record, so a reset can
// rebuild the morning from the same file. Callers run the rebuild after the
// transaction commits; a failed rebuild leaves the clear behind and the next
// reset can try again.
func Clear(ctx context.Context, database *sql.DB) error {
	if database == nil {
		return errors.New("clear sandbox: no database")
	}

	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("clear sandbox: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, statement := range resetStatements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("clear sandbox: %s: %w", statement, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("clear sandbox: commit: %w", err)
	}
	return nil
}
