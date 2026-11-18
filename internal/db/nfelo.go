package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// NfeloGame is nfelo's pre-game model projection for one game. HomeSpread is
// from the home side, so a negative number favors the home team. The QB fields
// name the starters the model used, and the adjustments are Elo points.
type NfeloGame struct {
	GameID      string
	HomeSpread  float64
	HomeWinProb float64
	HomeQB      string
	AwayQB      string
	HomeQBAdj   float64
	AwayQBAdj   float64
	UpdatedAt   time.Time
}

// nfeloColumns is the column list every nfelo query scans, in scan order.
const nfeloColumns = `
	game_id, home_spread, home_win_prob, home_qb, away_qb, home_qb_adj,
	away_qb_adj, updated_at`

// UpsertNfeloGames writes each projection and reports whether any row changed.
// A row whose values are identical is left alone, updated_at included, so a
// rerun with the same CSV changes nothing.
func (s *Store) UpsertNfeloGames(ctx context.Context, rows []NfeloGame) (bool, error) {
	changed := false
	err := s.Tx(ctx, func(tx *Store) error {
		for _, row := range rows {
			result, err := tx.conn().ExecContext(ctx, `
				INSERT INTO nfelo_games (
					game_id, home_spread, home_win_prob, home_qb, away_qb,
					home_qb_adj, away_qb_adj, updated_at
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT (game_id) DO UPDATE SET
					home_spread = excluded.home_spread,
					home_win_prob = excluded.home_win_prob,
					home_qb = excluded.home_qb,
					away_qb = excluded.away_qb,
					home_qb_adj = excluded.home_qb_adj,
					away_qb_adj = excluded.away_qb_adj,
					updated_at = excluded.updated_at
				WHERE nfelo_games.home_spread IS NOT excluded.home_spread
					OR nfelo_games.home_win_prob IS NOT excluded.home_win_prob
					OR nfelo_games.home_qb IS NOT excluded.home_qb
					OR nfelo_games.away_qb IS NOT excluded.away_qb
					OR nfelo_games.home_qb_adj IS NOT excluded.home_qb_adj
					OR nfelo_games.away_qb_adj IS NOT excluded.away_qb_adj`,
				row.GameID,
				row.HomeSpread,
				row.HomeWinProb,
				row.HomeQB,
				row.AwayQB,
				row.HomeQBAdj,
				row.AwayQBAdj,
				formatTimestamp(row.UpdatedAt),
			)
			if err != nil {
				return fmt.Errorf("upsert nfelo game %s: %w", row.GameID, err)
			}
			affected, err := result.RowsAffected()
			if err != nil {
				return fmt.Errorf("upsert nfelo game %s: %w", row.GameID, err)
			}
			if affected > 0 {
				changed = true
			}
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return changed, nil
}

// NfeloForGame returns one game's nfelo projection. The bool is false when the
// game has no row, which is normal before the first sync or when the CSV has no
// matching game.
func (s *Store) NfeloForGame(ctx context.Context, gameID string) (NfeloGame, bool, error) {
	row := s.readConn().QueryRowContext(ctx, "SELECT"+nfeloColumns+`
		FROM nfelo_games
		WHERE game_id = ?`, gameID)
	projection, err := scanNfeloGame(row)
	if errors.Is(err, sql.ErrNoRows) {
		return NfeloGame{}, false, nil
	}
	if err != nil {
		return NfeloGame{}, false, fmt.Errorf("nfelo game %s: %w", gameID, err)
	}
	return projection, true, nil
}

// NfeloForGames returns nfelo projections for multiple game IDs indexed by
// game_id. Missing games are omitted from the returned map.
func (s *Store) NfeloForGames(ctx context.Context, gameIDs []string) (map[string]NfeloGame, error) {
	if len(gameIDs) == 0 {
		return map[string]NfeloGame{}, nil
	}
	placeholders := strings.Repeat("?,", len(gameIDs))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(gameIDs))
	for i, id := range gameIDs {
		args[i] = id
	}
	rows, err := s.readConn().QueryContext(ctx,
		"SELECT"+nfeloColumns+" FROM nfelo_games WHERE game_id IN ("+placeholders+")", args...)
	if err != nil {
		return nil, fmt.Errorf("nfelo games: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]NfeloGame, len(gameIDs))
	for rows.Next() {
		projection, err := scanNfeloGame(rows)
		if err != nil {
			return nil, fmt.Errorf("nfelo games: %w", err)
		}
		result[projection.GameID] = projection
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("nfelo games: %w", err)
	}
	return result, nil
}

func scanNfeloGame(scanner interface{ Scan(...any) error }) (NfeloGame, error) {
	var (
		projection NfeloGame
		updatedAt  string
	)
	if err := scanner.Scan(
		&projection.GameID,
		&projection.HomeSpread,
		&projection.HomeWinProb,
		&projection.HomeQB,
		&projection.AwayQB,
		&projection.HomeQBAdj,
		&projection.AwayQBAdj,
		&updatedAt,
	); err != nil {
		return NfeloGame{}, err
	}
	parsed, err := scanTimestamp(updatedAt)
	if err != nil {
		return NfeloGame{}, err
	}
	projection.UpdatedAt = parsed
	return projection, nil
}
