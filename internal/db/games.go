package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Errors a caller can act on.
var (
	// ErrGameNotFound is returned when a game lookup matches no row.
	ErrGameNotFound = errors.New("game not found")
	// ErrGameStatusInvalid is returned when a status is not one the schema allows.
	ErrGameStatusInvalid = errors.New("invalid game status")
)

// GameStatus is where a game sits in its lifecycle.
type GameStatus string

// The statuses a game can carry.
const (
	GameScheduled GameStatus = "SCHEDULED"
	GameLive      GameStatus = "LIVE"
	GameFinal     GameStatus = "FINAL"
)

// Valid reports whether the status is one of GameScheduled, GameLive, or
// GameFinal.
func (s GameStatus) Valid() bool {
	switch s {
	case GameScheduled, GameLive, GameFinal:
		return true
	default:
		return false
	}
}

// Game is one NFL game. The slate sync owns the team, kickoff, status, venue,
// and score fields. Odds, weather, and injuries come from their own sources,
// so UpsertGame leaves them alone.
type Game struct {
	ID           string
	WeekID       int64
	OddsEventID  *string
	HomeTeam     string
	AwayTeam     string
	HomeName     string
	AwayName     string
	HomeESPNID   string
	AwayESPNID   string
	Kickoff      time.Time
	Status       GameStatus
	VenueID      *string
	VenueName    *string
	Indoor       bool
	TempF        *float64
	WindMPH      *float64
	WindGustMPH  *float64
	PrecipIN     *float64
	HomeScore    *int
	AwayScore    *int
	InjuriesJSON *string
	UpdatedAt    time.Time
}

// gameColumns is the column list every game query scans, in scanGame order.
const gameColumns = `
	id, week_id, odds_event_id, home_team, away_team, home_name, away_name,
	home_espn_id, away_espn_id, kickoff_utc, status, venue_id, venue_name,
	is_indoor, temp_f, wind_mph, wind_gust_mph, precip_in, home_score,
	away_score, injuries_json, updated_at`

// UpsertGame inserts a game, or updates the fields the slate sync owns when
// they changed. A rerun with the same data leaves the row and its updated_at
// untouched. Weather, injuries, and the odds event match stay put.
func (s *Store) UpsertGame(ctx context.Context, game Game) (Game, error) {
	if !game.Status.Valid() {
		return Game{}, fmt.Errorf("game %s: %w", game.ID, ErrGameStatusInvalid)
	}

	_, err := s.conn().ExecContext(ctx, `
		INSERT INTO games (
			id, week_id, home_team, away_team, home_name, away_name,
			home_espn_id, away_espn_id, kickoff_utc, status, venue_id,
			venue_name, is_indoor, home_score, away_score, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			week_id = excluded.week_id,
			home_team = excluded.home_team,
			away_team = excluded.away_team,
			home_name = excluded.home_name,
			away_name = excluded.away_name,
			home_espn_id = excluded.home_espn_id,
			away_espn_id = excluded.away_espn_id,
			kickoff_utc = excluded.kickoff_utc,
			status = excluded.status,
			venue_id = excluded.venue_id,
			venue_name = excluded.venue_name,
			is_indoor = excluded.is_indoor,
			home_score = excluded.home_score,
			away_score = excluded.away_score,
			updated_at = excluded.updated_at
		WHERE games.week_id IS NOT excluded.week_id
			OR games.home_team IS NOT excluded.home_team
			OR games.away_team IS NOT excluded.away_team
			OR games.home_name IS NOT excluded.home_name
			OR games.away_name IS NOT excluded.away_name
			OR games.home_espn_id IS NOT excluded.home_espn_id
			OR games.away_espn_id IS NOT excluded.away_espn_id
			OR games.kickoff_utc IS NOT excluded.kickoff_utc
			OR games.status IS NOT excluded.status
			OR games.venue_id IS NOT excluded.venue_id
			OR games.venue_name IS NOT excluded.venue_name
			OR games.is_indoor IS NOT excluded.is_indoor
			OR games.home_score IS NOT excluded.home_score
			OR games.away_score IS NOT excluded.away_score`,
		game.ID,
		game.WeekID,
		game.HomeTeam,
		game.AwayTeam,
		game.HomeName,
		game.AwayName,
		game.HomeESPNID,
		game.AwayESPNID,
		formatTimestamp(game.Kickoff),
		game.Status,
		nullableString(game.VenueID),
		nullableString(game.VenueName),
		game.Indoor,
		nullableInt(game.HomeScore),
		nullableInt(game.AwayScore),
		formatTimestamp(game.UpdatedAt),
	)
	if err != nil {
		return Game{}, fmt.Errorf("upsert game %s: %w", game.ID, err)
	}
	return s.GameByID(ctx, game.ID)
}

// GameByID returns the game with the given ID. A missing row wraps
// ErrGameNotFound.
func (s *Store) GameByID(ctx context.Context, id string) (Game, error) {
	game, err := scanGame(s.conn().QueryRowContext(ctx,
		"SELECT"+gameColumns+" FROM games WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return Game{}, fmt.Errorf("game %s: %w", id, ErrGameNotFound)
	}
	if err != nil {
		return Game{}, fmt.Errorf("game %s: %w", id, err)
	}
	return game, nil
}

// GameKickoff returns a game's kickoff time in UTC. A missing row wraps
// ErrGameNotFound.
func (s *Store) GameKickoff(ctx context.Context, id string) (time.Time, error) {
	var kickoff string
	err := s.conn().QueryRowContext(ctx,
		"SELECT kickoff_utc FROM games WHERE id = ?", id).Scan(&kickoff)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, fmt.Errorf("game %s: %w", id, ErrGameNotFound)
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("game %s: %w", id, err)
	}

	parsed, err := scanTimestamp(kickoff)
	if err != nil {
		return time.Time{}, fmt.Errorf("game %s: %w", id, err)
	}
	return parsed, nil
}

// WeekPast reports whether every game in the week has kicked off by now. A week
// with no games is not past: there is nothing to lock, so its legs stay
// editable. Past weeks skip the kickoff lock so history can be entered.
func (s *Store) WeekPast(ctx context.Context, weekID int64, now time.Time) (bool, error) {
	var (
		games  int
		latest sql.NullString
	)
	err := s.conn().QueryRowContext(ctx,
		"SELECT count(*), MAX(kickoff_utc) FROM games WHERE week_id = ?", weekID).
		Scan(&games, &latest)
	if err != nil {
		return false, fmt.Errorf("week %d kickoff: %w", weekID, err)
	}
	if games == 0 {
		return false, nil
	}

	kickoff, err := scanTimestamp(latest.String)
	if err != nil {
		return false, fmt.Errorf("week %d kickoff: %w", weekID, err)
	}
	return !now.Before(kickoff), nil
}

// scanGame reads the columns every game query selects, in order.
func scanGame(row rowScanner) (Game, error) {
	var (
		game        Game
		oddsEventID sql.NullString
		kickoff     string
		status      string
		venueID     sql.NullString
		venueName   sql.NullString
		tempF       sql.NullFloat64
		windMPH     sql.NullFloat64
		windGustMPH sql.NullFloat64
		precipIN    sql.NullFloat64
		homeScore   sql.NullInt64
		awayScore   sql.NullInt64
		injuries    sql.NullString
		updatedAt   string
	)
	if err := row.Scan(
		&game.ID,
		&game.WeekID,
		&oddsEventID,
		&game.HomeTeam,
		&game.AwayTeam,
		&game.HomeName,
		&game.AwayName,
		&game.HomeESPNID,
		&game.AwayESPNID,
		&kickoff,
		&status,
		&venueID,
		&venueName,
		&game.Indoor,
		&tempF,
		&windMPH,
		&windGustMPH,
		&precipIN,
		&homeScore,
		&awayScore,
		&injuries,
		&updatedAt,
	); err != nil {
		return Game{}, err
	}

	kickoffTime, err := scanTimestamp(kickoff)
	if err != nil {
		return Game{}, err
	}
	updateTime, err := scanTimestamp(updatedAt)
	if err != nil {
		return Game{}, err
	}

	game.OddsEventID = nullableStringPtr(oddsEventID)
	game.Kickoff = kickoffTime
	game.Status = GameStatus(status)
	game.VenueID = nullableStringPtr(venueID)
	game.VenueName = nullableStringPtr(venueName)
	game.TempF = nullableFloatPtr(tempF)
	game.WindMPH = nullableFloatPtr(windMPH)
	game.WindGustMPH = nullableFloatPtr(windGustMPH)
	game.PrecipIN = nullableFloatPtr(precipIN)
	game.HomeScore = nullableIntPtr(homeScore)
	game.AwayScore = nullableIntPtr(awayScore)
	game.InjuriesJSON = nullableStringPtr(injuries)
	game.UpdatedAt = updateTime
	return game, nil
}

// formatTimestamp writes one of the RFC 3339 UTC strings the schema stores.
func formatTimestamp(value time.Time) string {
	return value.UTC().Format(time.RFC3339)
}

// nullableInt turns a pointer into the value or NULL for a query argument.
func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

// nullableIntPtr turns a scanned NULL-able integer back into a pointer.
func nullableIntPtr(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	converted := int(value.Int64)
	return &converted
}

// nullableFloatPtr turns a scanned NULL-able float back into a pointer.
func nullableFloatPtr(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	return &value.Float64
}
