package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
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

// SelectionKey identifies one pickable selection across legs, odds rows, and
// board rows, ignoring book, line, price, and link.
type SelectionKey struct {
	GameID     string
	MarketType string
	Team       string
	PlayerName string
	PlayerID   string
	Side       string
}

// IsPlayerMarket reports whether a market type targets an athlete rather than
// a game or team.
func IsPlayerMarket(market string) bool {
	switch market {
	case "PASS_YDS", "PASS_TDS", "RUSH_YDS", "RUSH_ATT", "REC_YDS", "RECEPTIONS", "ANYTIME_TD", "KICKER_PTS":
		return true
	default:
		return false
	}
}

// MarketLabel returns the short display label for a player prop market type,
// or "" when the market has no stat label.
func MarketLabel(market string) string {
	switch market {
	case "PASS_YDS":
		return "Pass Yds"
	case "PASS_TDS":
		return "Pass TDs"
	case "RUSH_YDS":
		return "Rush Yds"
	case "RUSH_ATT":
		return "Rush Att"
	case "REC_YDS":
		return "Rec Yds"
	case "RECEPTIONS":
		return "Receptions"
	case "ANYTIME_TD":
		return "Anytime TD"
	case "KICKER_PTS":
		return "Kicking Pts"
	default:
		return ""
	}
}

// FormatSelection formats a selection's display label for board rows and legs,
// gracefully omitting missing lines on partial rows.
func FormatSelection(market, team, player, side string, line *float64) string {
	switch market {
	case "SPREAD":
		if line == nil {
			return team
		}
		return strings.TrimSpace(team + " " + signedLine(*line))
	case "ML":
		return strings.TrimSpace(team + " ML")
	case "TOTAL":
		s := titleSide(side)
		if line == nil {
			return s
		}
		return strings.TrimSpace(s + " " + formatLine(*line))
	case "TEAM_TOTAL":
		text := team + " Team Total " + titleSide(side)
		if line != nil {
			text += " " + formatLine(*line)
		}
		return strings.TrimSpace(text)
	case "ANYTIME_TD":
		return strings.TrimSpace(player + " Anytime TD")
	default:
		label := MarketLabel(market)
		text := player + " " + titleSide(side)
		if line != nil {
			text += " " + formatLine(*line)
		}
		if label != "" {
			text += " " + label
		}
		return strings.TrimSpace(text)
	}
}

// BuildLegSelection formats a leg's display text from form inputs, returning ""
// when required fields for the market are missing or invalid.
func BuildLegSelection(market, team, player, side string, line *float64) string {
	team = strings.TrimSpace(team)
	player = strings.TrimSpace(player)
	side = strictTitleSide(side)

	switch market {
	case "SPREAD":
		if team == "" || line == nil {
			return ""
		}
	case "ML":
		if team == "" {
			return ""
		}
	case "TOTAL":
		if side == "" || line == nil {
			return ""
		}
	case "TEAM_TOTAL":
		if team == "" || side == "" || line == nil {
			return ""
		}
	case "ANYTIME_TD":
		if player == "" {
			return ""
		}
	case "OTHER":
		return ""
	default:
		if player == "" || side == "" || line == nil {
			return ""
		}
	}
	return FormatSelection(market, team, player, side, line)
}

func titleSide(side string) string {
	side = strings.TrimSpace(side)
	if side == "" {
		return ""
	}
	return strings.ToUpper(side[:1]) + strings.ToLower(side[1:])
}

func strictTitleSide(raw string) string {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "OVER":
		return "Over"
	case "UNDER":
		return "Under"
	case "YES":
		return "Yes"
	default:
		return ""
	}
}

func signedLine(line float64) string {
	if line > 0 {
		return "+" + formatLine(line)
	}
	return formatLine(line)
}

func formatLine(line float64) string {
	return strconv.FormatFloat(line, 'f', -1, 64)
}

// LegSelection extracts the canonical selection key for a stored or draft leg.
func LegSelection(leg Leg) SelectionKey {
	return SelectionKey{
		GameID:     StringValue(leg.GameID),
		MarketType: leg.MarketType,
		Team:       StringValue(leg.Team),
		PlayerName: StringValue(leg.PlayerName),
		PlayerID:   StringValue(leg.PlayerID),
		Side:       StringValue(leg.Side),
	}
}

// OddsSelection extracts the selection key for a stored odds row.
func OddsSelection(o Odds) SelectionKey {
	return SelectionKey{
		GameID:     o.GameID,
		MarketType: o.MarketType,
		Team:       o.Team,
		PlayerName: o.PlayerName,
		Side:       o.Side,
	}
}

// SamePlayer reports whether two selection keys target the same athlete, using
// ESPN PlayerID when both carry one, or normalized PlayerName with
// non-conflicting Team otherwise.
func SamePlayer(left, right SelectionKey) bool {
	if left.PlayerID != "" && right.PlayerID != "" {
		return left.PlayerID == right.PlayerID
	}
	if left.Team != "" && right.Team != "" && left.Team != right.Team {
		return false
	}
	leftName := NormalizePlayerName(left.PlayerName)
	rightName := NormalizePlayerName(right.PlayerName)
	return leftName != "" && leftName == rightName
}

// SameSelection reports whether two selections represent the same pick,
// ignoring book, line, price, and link. Player props match via SamePlayer;
// game markets match on game, market, team, and side.
func SameSelection(left, right SelectionKey) bool {
	if left.GameID == "" || left.GameID != right.GameID {
		return false
	}
	if left.MarketType != right.MarketType || left.Side != right.Side {
		return false
	}
	if IsPlayerMarket(left.MarketType) || left.PlayerName != "" || right.PlayerName != "" {
		return SamePlayer(left, right)
	}
	return left.Team == right.Team
}

// SameLineGroup reports whether two selection keys belong to the same board
// line group: same game and market, any side/team for SPREAD/ML/TOTAL, same
// team for TEAM_TOTAL, and SamePlayer for player markets.
func SameLineGroup(left, right SelectionKey) bool {
	if left.GameID == "" || left.GameID != right.GameID || left.MarketType != right.MarketType {
		return false
	}
	switch left.MarketType {
	case "SPREAD", "ML", "TOTAL":
		return true
	case "TEAM_TOTAL":
		return left.Team == right.Team
	default:
		if IsPlayerMarket(left.MarketType) || left.PlayerName != "" || right.PlayerName != "" {
			return SamePlayer(left, right)
		}
		return left.Team == right.Team
	}
}

// MatchBookOdds finds the odds row for a leg's selection and book via
// MatchesSelection, preferring an exact line match when more than one row
// matches and falling back to the first matching row for that book.
func MatchBookOdds(leg Leg, odds []Odds, book string) (Odds, bool) {
	book = NormalizeBook(book)
	if book == "" {
		return Odds{}, false
	}
	var fallback Odds
	var found bool
	for _, row := range odds {
		if NormalizeBook(row.Book) != book {
			continue
		}
		if !row.MatchesSelection(leg) {
			continue
		}
		if SameLine(row.Line, leg.Line) {
			return row, true
		}
		if !found {
			fallback = row
			found = true
		}
	}
	return fallback, found
}

// MatchesSelection reports whether o prices the same selection as leg, ignoring
// book, line, price, and link. It is the one definition of "the same
// selection" shared by the games board, the slip reference prices, and export.
func (o Odds) MatchesSelection(leg Leg) bool {
	return SameSelection(LegSelection(leg), OddsSelection(o))
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
// values are identical is left alone; a new fetch time is a real change because
// the board shows how fresh the lines are. Incoming NULL link/sid/event_sid
// values preserve stored values when the line has not moved; when the line
// moves, stale link and sid are cleared while event_sid is preserved.
func (s *Store) UpsertOdds(ctx context.Context, rows []Odds) (bool, error) {
	if len(rows) == 0 {
		return false, nil
	}
	start := time.Now()
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
				link = CASE
					WHEN excluded.link IS NOT NULL THEN excluded.link
					WHEN odds.line IS excluded.line THEN odds.link
					ELSE NULL
				END,
				sid = CASE
					WHEN excluded.sid IS NOT NULL THEN excluded.sid
					WHEN odds.line IS excluded.line THEN odds.sid
					ELSE NULL
				END,
				event_sid = COALESCE(excluded.event_sid, odds.event_sid),
				fetched_at = excluded.fetched_at
			WHERE odds.line IS NOT excluded.line
				OR odds.price IS NOT excluded.price
				OR (excluded.link IS NOT NULL AND odds.link IS NOT excluded.link)
				OR (excluded.sid IS NOT NULL AND odds.sid IS NOT excluded.sid)
				OR (excluded.event_sid IS NOT NULL AND odds.event_sid IS NOT excluded.event_sid)
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
	if elapsed := time.Since(start); elapsed >= slowQueryThreshold {
		s.logger.Warn().
			Float64("duration_ms", durationMS(elapsed)).
			Int("rows", len(rows)).
			Bool("changed", changed).
			Msg("slow sqlite upsert odds")
	}
	return changed, nil
}

// IsPropRow reports whether an odds row belongs to the prop sync group.
func IsPropRow(row Odds) bool {
	return row.MarketType == "TEAM_TOTAL" || row.PlayerName != "" || IsPlayerMarket(row.MarketType)
}

// IsPropLeg reports whether a slip leg belongs to the prop sync group.
func IsPropLeg(leg Leg) bool {
	return leg.MarketType == "TEAM_TOTAL" || IsPlayerMarket(leg.MarketType) ||
		(leg.PlayerName != nil && strings.TrimSpace(*leg.PlayerName) != "")
}

// CurrentOdds drops rows left behind by an earlier sync. Game lines and props
// are fetched independently and stamped with different fetch times, and props
// are synced one game at a time, so CurrentOdds computes the newest FetchedAt
// separately for each game's game-line rows and prop rows. Keeping the game in
// the key means loading one game's props later does not hide another game's
// props.
func CurrentOdds(rows []Odds) []Odds {
	if len(rows) == 0 {
		return rows
	}

	type syncGroup struct {
		gameID string
		props  bool
	}
	newest := make(map[syncGroup]time.Time, len(rows))
	for _, row := range rows {
		group := syncGroup{row.GameID, IsPropRow(row)}
		if row.FetchedAt.After(newest[group]) {
			newest[group] = row.FetchedAt
		}
	}

	current := make([]Odds, 0, len(rows))
	for _, row := range rows {
		group := syncGroup{row.GameID, IsPropRow(row)}
		if !newest[group].IsZero() && row.FetchedAt.Equal(newest[group]) {
			current = append(current, row)
		}
	}
	return current
}

// OddsForGame returns the current stored prices for one game, ordered by book,
// market, team, player, and side so callers get a stable list.
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
	return CurrentOdds(odds), nil
}

// OddsForWeek returns the current stored prices for one week's games, read in
// one query so the pick board does not ask per game. Rows are ordered by game,
// book, market, team, player, and side so callers get a stable list.
func (s *Store) OddsForWeek(ctx context.Context, weekID int64) ([]Odds, error) {
	start := time.Now()
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
	if elapsed := time.Since(start); elapsed >= slowQueryThreshold {
		s.logger.Warn().
			Float64("duration_ms", durationMS(elapsed)).
			Int64("week_id", weekID).
			Int("rows", len(odds)).
			Msg("slow sqlite odds for week")
	}
	return CurrentOdds(odds), nil
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
	return s.LatestPropForPlayerMarket(ctx, athleteID, playerName, team, "")
}

// LatestPropForPlayerMarket returns the newest prop odds row for a player with
// a line, filtered to market when non-empty.
func (s *Store) LatestPropForPlayerMarket(
	ctx context.Context,
	athleteID, playerName, team, market string,
) (Odds, bool, error) {
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
	args := make([]any, 0, len(candidates)+2)
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
	if market = strings.TrimSpace(market); market != "" {
		query += " AND market_type = ?"
		args = append(args, market)
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
