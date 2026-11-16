package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Odds is one book's price for one selection, shaped like a row of the odds
// table. PlayerName is empty for game markets, and Team is empty for a total.
// Link and Sid are the book's deep link and selection ID for this outcome, and
// are nil when the book supplied none. EventSid is the book's own ID for the
// event, used by books whose combined link needs an event page.
type Odds struct {
	GameID     string
	Book       string
	MarketType string
	Team       string
	PlayerName string
	Side       string
	Line       *float64
	Price      int
	Link       *string
	Sid        *string
	EventSid   *string
	FetchedAt  time.Time
}

// MatchesSelection reports whether o prices the same selection as leg, ignoring
// book, line, price, and link. It is the one definition of "the same
// selection" shared by the games board, the slip reference prices, and export.
func (o Odds) MatchesSelection(leg Leg) bool {
	if leg.GameID == nil || o.GameID != *leg.GameID {
		return false
	}
	return o.MarketType == leg.MarketType &&
		o.Team == StringValue(leg.Team) &&
		o.PlayerName == StringValue(leg.PlayerName) &&
		o.Side == StringValue(leg.Side)
}

// SameLine compares two optional lines for an exact match. Two nil lines are
// the same selection, such as a moneyline.
func SameLine(left, right *float64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

// NormalizeBook trims and lowercases a book key so a display value still finds
// the stored odds rows.
func NormalizeBook(book string) string {
	return strings.ToLower(strings.TrimSpace(book))
}

// OptionalString trims a blank-able value, returning nil when it is blank so
// the database stores NULL.
func OptionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

// StringValue reads an optional field as the empty string the odds table stores
// for an absent team, player, or side.
func StringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// oddsColumns is the column list every odds query scans, in scanOdds order.
const oddsColumns = `
	game_id, book, market_type, team, player_name, side, line, price, link,
	sid, event_sid, fetched_at`

// UpsertOdds writes each row and reports whether any row changed. A row whose
// values are identical apart from nothing is left alone; a new fetch time is a
// real change because the board shows how fresh the lines are.
func (s *Store) UpsertOdds(ctx context.Context, rows []Odds) (bool, error) {
	if len(rows) == 0 {
		return false, nil
	}
	changed := false
	err := s.Tx(ctx, func(tx *Store) error {
		stmt, err := tx.conn().PrepareContext(ctx, `
			INSERT INTO odds (
				game_id, book, market_type, team, player_name, side,
				line, price, link, sid, event_sid, fetched_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (game_id, book, market_type, team, player_name, side)
			DO UPDATE SET
				line = excluded.line,
				price = excluded.price,
				link = excluded.link,
				sid = excluded.sid,
				event_sid = excluded.event_sid,
				fetched_at = excluded.fetched_at
			WHERE odds.line IS NOT excluded.line
				OR odds.price IS NOT excluded.price
				OR odds.link IS NOT excluded.link
				OR odds.sid IS NOT excluded.sid
				OR odds.event_sid IS NOT excluded.event_sid
				OR odds.fetched_at IS NOT excluded.fetched_at`)
		if err != nil {
			return fmt.Errorf("prepare upsert odds: %w", err)
		}
		defer func() { _ = stmt.Close() }()

		for _, row := range rows {
			result, err := stmt.ExecContext(ctx,
				row.GameID,
				row.Book,
				row.MarketType,
				row.Team,
				row.PlayerName,
				row.Side,
				nullableFloat(row.Line),
				row.Price,
				nullableString(row.Link),
				nullableString(row.Sid),
				nullableString(row.EventSid),
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
	rows, err := s.readConn().QueryContext(ctx, "SELECT"+oddsColumns+`
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

// OddsForWeek returns every stored price for one week's games, read in one
// query so the pick board does not ask per game. Rows are ordered by game,
// book, market, team, player, and side so callers get a stable list.
func (s *Store) OddsForWeek(ctx context.Context, weekID int64) ([]Odds, error) {
	rows, err := s.readConn().QueryContext(ctx, "SELECT"+oddsColumns+`
		FROM odds
		WHERE game_id IN (SELECT id FROM games WHERE week_id = ?)
		ORDER BY game_id, book, market_type, team, player_name, side`, weekID)
	if err != nil {
		return nil, fmt.Errorf("list odds for week %d: %w", weekID, err)
	}
	defer func() { _ = rows.Close() }()

	odds := make([]Odds, 0)
	for rows.Next() {
		row, err := scanOdds(rows)
		if err != nil {
			return nil, fmt.Errorf("list odds for week %d: %w", weekID, err)
		}
		odds = append(odds, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list odds for week %d: %w", weekID, err)
	}
	return odds, nil
}

// scanOdds reads the odds columns in oddsColumns order.
func scanOdds(row rowScanner) (Odds, error) {
	var (
		odds      Odds
		line      sql.NullFloat64
		link      sql.NullString
		sid       sql.NullString
		eventSid  sql.NullString
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
		&link,
		&sid,
		&eventSid,
		&fetchedAt,
	); err != nil {
		return Odds{}, err
	}

	parsed, err := scanTimestamp(fetchedAt)
	if err != nil {
		return Odds{}, err
	}
	odds.Line = nullableFloatPtr(line)
	odds.Link = nullableStringPtr(link)
	odds.Sid = nullableStringPtr(sid)
	odds.EventSid = nullableStringPtr(eventSid)
	odds.FetchedAt = parsed
	return odds, nil
}

// LatestPropForPlayer returns the newest prop odds row for a player with a line.
func (s *Store) LatestPropForPlayer(ctx context.Context, athleteID, playerName, team string) (Odds, bool, error) {
	candidates := make([]string, 0, 4)
	if playerName != "" {
		candidates = append(candidates, playerName)
		norm := NormalizePlayerName(playerName)
		if norm != "" && norm != playerName {
			candidates = append(candidates, norm)
		}
	}
	if athleteID != "" {
		rows, err := s.readConn().QueryContext(ctx,
			"SELECT odds_name FROM player_matches WHERE espn_athlete_id = ?", athleteID)
		if err == nil {
			defer func() { _ = rows.Close() }()
			for rows.Next() {
				var name string
				if err := rows.Scan(&name); err == nil && name != "" {
					candidates = append(candidates, name)
				}
			}
		}
	}
	if len(candidates) == 0 {
		return Odds{}, false, nil
	}

	placeholders := strings.Repeat("?,", len(candidates))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, 0, len(candidates)+1)
	for _, c := range candidates {
		args = append(args, c)
	}

	query := "SELECT " + oddsColumns + `
		FROM odds
		WHERE player_name COLLATE NOCASE IN (` + placeholders + `)
			AND line IS NOT NULL`
	if team != "" {
		query += " AND (team = ? OR team = '')"
		args = append(args, team)
	}
	query += " ORDER BY fetched_at DESC LIMIT 1"

	row := s.readConn().QueryRowContext(ctx, query, args...)
	o, err := scanOdds(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Odds{}, false, nil
	}
	if err != nil {
		return Odds{}, false, fmt.Errorf("latest prop for player %s: %w", playerName, err)
	}
	return o, true, nil
}
