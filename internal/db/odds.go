package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Odds is one book's price for one selection, shaped like a row of the odds
// table. PlayerName is empty for game markets, and Team is empty for a total.
type Odds struct {
	GameID     string
	Book       string
	MarketType string
	Team       string
	PlayerName string
	Side       string
	Line       *float64
	Price      int
	FetchedAt  time.Time
}

// oddsColumns is the column list every odds query scans, in scanOdds order.
const oddsColumns = `
	game_id, book, market_type, team, player_name, side, line, price, fetched_at`

// UpsertOdds writes each row and reports whether any row changed. A row whose
// values are identical apart from nothing is left alone; a new fetch time is a
// real change because the board shows how fresh the lines are.
func (s *Store) UpsertOdds(ctx context.Context, rows []Odds) (bool, error) {
	changed := false
	err := s.Tx(ctx, func(tx *Store) error {
		for _, row := range rows {
			result, err := tx.conn().ExecContext(ctx, `
				INSERT INTO odds (
					game_id, book, market_type, team, player_name, side,
					line, price, fetched_at
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT (game_id, book, market_type, team, player_name, side)
				DO UPDATE SET
					line = excluded.line,
					price = excluded.price,
					fetched_at = excluded.fetched_at
				WHERE odds.line IS NOT excluded.line
					OR odds.price IS NOT excluded.price
					OR odds.fetched_at IS NOT excluded.fetched_at`,
				row.GameID,
				row.Book,
				row.MarketType,
				row.Team,
				row.PlayerName,
				row.Side,
				nullableFloat(row.Line),
				row.Price,
				formatTimestamp(row.FetchedAt),
			)
			if err != nil {
				return fmt.Errorf(
					"upsert odds %s %s %s %s %s: %w",
					row.GameID, row.Book, row.MarketType, row.Team, row.Side, err,
				)
			}
			affected, err := result.RowsAffected()
			if err != nil {
				return fmt.Errorf("upsert odds %s: %w", row.GameID, err)
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

// OddsForGame returns every stored price for one game, ordered by book, market,
// team, player, and side so callers get a stable list.
func (s *Store) OddsForGame(ctx context.Context, gameID string) ([]Odds, error) {
	rows, err := s.conn().QueryContext(ctx, "SELECT"+oddsColumns+`
		FROM odds
		WHERE game_id = ?
		ORDER BY book, market_type, team, player_name, side`, gameID)
	if err != nil {
		return nil, fmt.Errorf("list odds for game %s: %w", gameID, err)
	}
	defer func() { _ = rows.Close() }()

	odds := make([]Odds, 0)
	for rows.Next() {
		row, err := scanOdds(rows)
		if err != nil {
			return nil, fmt.Errorf("list odds for game %s: %w", gameID, err)
		}
		odds = append(odds, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list odds for game %s: %w", gameID, err)
	}
	return odds, nil
}

// scanOdds reads the odds columns in oddsColumns order.
func scanOdds(row rowScanner) (Odds, error) {
	var (
		odds      Odds
		line      sql.NullFloat64
		fetchedAt string
	)
	if err := row.Scan(
		&odds.GameID,
		&odds.Book,
		&odds.MarketType,
		&odds.Team,
		&odds.PlayerName,
		&odds.Side,
		&line,
		&odds.Price,
		&fetchedAt,
	); err != nil {
		return Odds{}, err
	}

	parsed, err := scanTimestamp(fetchedAt)
	if err != nil {
		return Odds{}, err
	}
	odds.Line = nullableFloatPtr(line)
	odds.FetchedAt = parsed
	return odds, nil
}
