package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

var (
	// ErrPlayerNotFound is returned when a roster player lookup matches no row.
	ErrPlayerNotFound = errors.New("roster player not found")
	// ErrGameLogNotFound is returned when a player gamelog lookup matches no row.
	ErrGameLogNotFound = errors.New("player gamelog not found")
)

// RosterPlayer is one athlete on an NFL team's ESPN roster.
type RosterPlayer struct {
	AthleteID   string
	Team        string
	ESPNTeamID  string
	FullName    string
	NormName    string
	Position    string
	HeadshotURL string
	UpdatedAt   time.Time
}

const rosterColumns = `athlete_id, team, espn_team_id, full_name, norm_name, position, headshot_url, updated_at`

// PlayerGameLog is one athlete's cached season game log and market stats.
type PlayerGameLog struct {
	AthleteID   string
	GamesPlayed int
	StatsJSON   string
	UpdatedAt   time.Time
}

const gameLogColumns = `athlete_id, games_played, stats_json, updated_at`

// PlayerMatch connects a normalized sportsbook player name and team to an ESPN athlete ID.
type PlayerMatch struct {
	OddsName      string
	Team          string
	ESPNAthleteID string
	Manual        bool
}

const matchColumns = `odds_name, team, espn_athlete_id, manual`

// StripPlayerTag removes a trailing parenthetical tag such as " (MIA)" from a
// sportsbook player name. If the inner token is a 2-4 letter uppercase team
// code, it is also returned as teamHint.
func StripPlayerTag(name string) (cleaned, teamHint string) {
	trimmed := strings.TrimSpace(name)
	if !strings.HasSuffix(trimmed, ")") {
		return trimmed, ""
	}
	open := strings.LastIndexByte(trimmed, '(')
	if open <= 0 {
		return trimmed, ""
	}
	prefix := strings.TrimSpace(trimmed[:open])
	inner := strings.TrimSpace(trimmed[open+1 : len(trimmed)-1])
	if prefix == "" || inner == "" {
		return trimmed, ""
	}
	if isTeamTag(inner) {
		teamHint = inner
	}
	return prefix, teamHint
}

func isTeamTag(s string) bool {
	if len(s) < 2 || len(s) > 4 {
		return false
	}
	for i := range len(s) {
		if s[i] < 'A' || s[i] > 'Z' {
			return false
		}
	}
	return true
}

// NormalizePlayerName strips trailing parenthetical tags, folds common Latin
// accents, lowercases, strips '.', ”', '-', and ',', collapses whitespace,
// and drops a trailing generational suffix (jr, sr, ii, iii, iv, v) when
// preceded by a space.
func NormalizePlayerName(name string) string {
	cleaned, _ := StripPlayerTag(name)
	cleaned = strings.ToLower(cleaned)
	var b strings.Builder
	b.Grow(len(cleaned))
	for _, r := range cleaned {
		switch r {
		case '.', '\'', '-', ',':
			// drop punctuation
		default:
			b.WriteRune(plainRune(r))
		}
	}
	fields := strings.Fields(b.String())
	if len(fields) == 0 {
		return ""
	}
	last := fields[len(fields)-1]
	switch last {
	case "jr", "sr", "ii", "iii", "iv", "v":
		if len(fields) > 1 {
			fields = fields[:len(fields)-1]
		}
	}
	return strings.Join(fields, " ")
}

// plainRune maps common Latin accented letters to their unaccented ASCII form.
func plainRune(r rune) rune {
	switch r {
	case 'á', 'à', 'â', 'ä', 'ã', 'å', 'Á', 'À', 'Â', 'Ä', 'Ã', 'Å':
		return 'a'
	case 'ç', 'Ç':
		return 'c'
	case 'é', 'è', 'ê', 'ë', 'É', 'È', 'Ê', 'Ë':
		return 'e'
	case 'í', 'ì', 'î', 'ï', 'Í', 'Ì', 'Î', 'Ï':
		return 'i'
	case 'ñ', 'Ñ':
		return 'n'
	case 'ó', 'ò', 'ô', 'ö', 'õ', 'Ó', 'Ò', 'Ô', 'Ö', 'Õ':
		return 'o'
	case 'ú', 'ù', 'û', 'ü', 'Ú', 'Ù', 'Û', 'Ü':
		return 'u'
	case 'ý', 'ÿ', 'Ý', 'Ÿ':
		return 'y'
	default:
		return r
	}
}

// UpsertRosterPlayers inserts or updates players on a team's roster.
func (s *Store) UpsertRosterPlayers(ctx context.Context, players []RosterPlayer) error {
	if len(players) == 0 {
		return nil
	}
	return s.Tx(ctx, func(tx *Store) error {
		for _, p := range players {
			norm := p.NormName
			if norm == "" {
				norm = NormalizePlayerName(p.FullName)
			}
			_, err := tx.conn().ExecContext(ctx, `
				INSERT INTO roster_players (
					athlete_id, team, espn_team_id, full_name, norm_name, position, headshot_url, updated_at
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT (athlete_id) DO UPDATE SET
					team = excluded.team,
					espn_team_id = excluded.espn_team_id,
					full_name = excluded.full_name,
					norm_name = excluded.norm_name,
					position = excluded.position,
					headshot_url = excluded.headshot_url,
					updated_at = excluded.updated_at`,
				p.AthleteID,
				p.Team,
				p.ESPNTeamID,
				p.FullName,
				norm,
				p.Position,
				p.HeadshotURL,
				formatTimestamp(p.UpdatedAt),
			)
			if err != nil {
				return fmt.Errorf("upsert roster player %s: %w", p.AthleteID, err)
			}
		}
		return nil
	})
}

// RosterPlayerByID returns one athlete by ESPN athlete ID.
func (s *Store) RosterPlayerByID(ctx context.Context, athleteID string) (RosterPlayer, error) {
	row := s.readConn().QueryRowContext(ctx,
		"SELECT "+rosterColumns+" FROM roster_players WHERE athlete_id = ?", athleteID)
	player, err := scanRosterPlayer(row)
	if errors.Is(err, sql.ErrNoRows) {
		return RosterPlayer{}, fmt.Errorf("athlete %s: %w", athleteID, ErrPlayerNotFound)
	}
	if err != nil {
		return RosterPlayer{}, fmt.Errorf("athlete %s: %w", athleteID, err)
	}
	return player, nil
}

// RosterPlayersByIDs returns roster players for multiple athletes indexed by
// athlete ID.
func (s *Store) RosterPlayersByIDs(ctx context.Context, athleteIDs []string) (map[string]RosterPlayer, error) {
	if len(athleteIDs) == 0 {
		return map[string]RosterPlayer{}, nil
	}
	placeholders := strings.Repeat("?,", len(athleteIDs))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(athleteIDs))
	for i, id := range athleteIDs {
		args[i] = id
	}
	rows, err := s.readConn().QueryContext(ctx,
		"SELECT "+rosterColumns+" FROM roster_players WHERE athlete_id IN ("+placeholders+")", args...)
	if err != nil {
		return nil, fmt.Errorf("roster players by ids: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]RosterPlayer, len(athleteIDs))
	for rows.Next() {
		p, err := scanRosterPlayer(rows)
		if err != nil {
			return nil, fmt.Errorf("roster players by ids: %w", err)
		}
		result[p.AthleteID] = p
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("roster players by ids: %w", err)
	}
	return result, nil
}

// RosterForTeam returns all roster players for an NFL team.
func (s *Store) RosterForTeam(ctx context.Context, team string) ([]RosterPlayer, error) {
	rows, err := s.readConn().QueryContext(ctx,
		"SELECT "+rosterColumns+" FROM roster_players WHERE team = ? ORDER BY full_name", team)
	if err != nil {
		return nil, fmt.Errorf("roster for team %s: %w", team, err)
	}
	defer func() { _ = rows.Close() }()

	var result []RosterPlayer
	for rows.Next() {
		p, err := scanRosterPlayer(rows)
		if err != nil {
			return nil, fmt.Errorf("roster for team %s: %w", team, err)
		}
		result = append(result, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("roster for team %s: %w", team, err)
	}
	return result, nil
}

// RosterForTeams returns all roster players for the given NFL teams.
func (s *Store) RosterForTeams(ctx context.Context, teams []string) ([]RosterPlayer, error) {
	if len(teams) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(teams))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(teams))
	for i, t := range teams {
		args[i] = t
	}
	rows, err := s.readConn().QueryContext(ctx,
		"SELECT "+rosterColumns+" FROM roster_players WHERE team IN ("+placeholders+") ORDER BY team, full_name", args...)
	if err != nil {
		return nil, fmt.Errorf("rosters for teams: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []RosterPlayer
	for rows.Next() {
		p, err := scanRosterPlayer(rows)
		if err != nil {
			return nil, fmt.Errorf("rosters for teams: %w", err)
		}
		result = append(result, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rosters for teams: %w", err)
	}
	return result, nil
}

// WeekPlayerNames returns a sorted, deduplicated slice of player names appearing
// on rosters or in prop odds for games in the given week.
func (s *Store) WeekPlayerNames(ctx context.Context, weekID int64) ([]string, error) {
	games, err := s.GamesForWeek(ctx, weekID)
	if err != nil {
		return nil, fmt.Errorf("week player names: games: %w", err)
	}
	if len(games) == 0 {
		return nil, nil
	}

	teams := make([]string, 0, len(games)*2)
	gameIDs := make([]string, 0, len(games))
	for _, g := range games {
		gameIDs = append(gameIDs, g.ID)
		if g.HomeTeam != "" {
			teams = append(teams, g.HomeTeam)
		}
		if g.AwayTeam != "" {
			teams = append(teams, g.AwayTeam)
		}
	}

	unique := make(map[string]struct{})

	rosterPlayers, err := s.RosterForTeams(ctx, teams)
	if err != nil {
		return nil, fmt.Errorf("week player names: rosters: %w", err)
	}
	for _, p := range rosterPlayers {
		if name := strings.TrimSpace(p.FullName); name != "" {
			unique[name] = struct{}{}
		}
	}

	if len(gameIDs) > 0 {
		placeholders := strings.Repeat("?,", len(gameIDs))
		placeholders = placeholders[:len(placeholders)-1]
		args := make([]any, len(gameIDs))
		for i, id := range gameIDs {
			args[i] = id
		}
		rows, err := s.readConn().QueryContext(ctx,
			"SELECT DISTINCT player_name FROM odds WHERE game_id IN ("+placeholders+") AND player_name != ''",
			args...,
		)
		if err != nil {
			return nil, fmt.Errorf("week player names: odds: %w", err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return nil, fmt.Errorf("week player names: scan: %w", err)
			}
			if name = strings.TrimSpace(name); name != "" {
				unique[name] = struct{}{}
			}
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("week player names: odds rows: %w", err)
		}
	}

	names := make([]string, 0, len(unique))
	for name := range unique {
		names = append(names, name)
	}
	slices.Sort(names)
	return names, nil
}

// UpsertPlayerGameLog inserts or updates an athlete's game log.
func (s *Store) UpsertPlayerGameLog(ctx context.Context, log PlayerGameLog) error {
	statsJSON := log.StatsJSON
	if statsJSON == "" {
		statsJSON = "{}"
	}
	_, err := s.conn().ExecContext(ctx, `
		INSERT INTO player_gamelogs (athlete_id, games_played, stats_json, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (athlete_id) DO UPDATE SET
			games_played = excluded.games_played,
			stats_json = excluded.stats_json,
			updated_at = excluded.updated_at`,
		log.AthleteID,
		log.GamesPlayed,
		statsJSON,
		formatTimestamp(log.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("upsert player gamelog %s: %w", log.AthleteID, err)
	}
	return nil
}

// PlayerGameLog returns one athlete's game log.
func (s *Store) PlayerGameLog(ctx context.Context, athleteID string) (PlayerGameLog, error) {
	row := s.readConn().QueryRowContext(ctx,
		"SELECT "+gameLogColumns+" FROM player_gamelogs WHERE athlete_id = ?", athleteID)
	log, err := scanPlayerGameLog(row)
	if errors.Is(err, sql.ErrNoRows) {
		return PlayerGameLog{}, fmt.Errorf("athlete %s: %w", athleteID, ErrGameLogNotFound)
	}
	if err != nil {
		return PlayerGameLog{}, fmt.Errorf("athlete %s: %w", athleteID, err)
	}
	return log, nil
}

// PlayerGameLogs returns game logs for multiple athletes indexed by athlete ID.
func (s *Store) PlayerGameLogs(ctx context.Context, athleteIDs []string) (map[string]PlayerGameLog, error) {
	if len(athleteIDs) == 0 {
		return map[string]PlayerGameLog{}, nil
	}
	placeholders := strings.Repeat("?,", len(athleteIDs))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(athleteIDs))
	for i, id := range athleteIDs {
		args[i] = id
	}
	rows, err := s.readConn().QueryContext(ctx,
		"SELECT "+gameLogColumns+" FROM player_gamelogs WHERE athlete_id IN ("+placeholders+")", args...)
	if err != nil {
		return nil, fmt.Errorf("player gamelogs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]PlayerGameLog, len(athleteIDs))
	for rows.Next() {
		log, err := scanPlayerGameLog(rows)
		if err != nil {
			return nil, fmt.Errorf("player gamelogs: %w", err)
		}
		result[log.AthleteID] = log
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("player gamelogs: %w", err)
	}
	return result, nil
}

// ResolvePlayerMatch matches a sportsbook odds player name against home and away teams
// using BatchPlayerResolver (honoring StripPlayerTag team hints and collision safety).
func (s *Store) ResolvePlayerMatch(
	ctx context.Context,
	oddsName, homeTeam, awayTeam string,
) (athleteID, team string, err error) {
	cleaned, _ := StripPlayerTag(oddsName)
	if NormalizePlayerName(cleaned) == "" {
		return "", "", nil
	}

	teams := []string{homeTeam, awayTeam}
	rosters, err := s.RosterForTeams(ctx, teams)
	if err != nil {
		return "", "", fmt.Errorf("resolve player match roster: %w", err)
	}
	matches, err := s.PlayerMatchesForTeams(ctx, teams)
	if err != nil {
		return "", "", fmt.Errorf("resolve player match: %w", err)
	}

	resolver := NewBatchPlayerResolver(rosters, matches)
	resolved := resolver.ResolveGamePlayer("", oddsName, "", homeTeam, awayTeam)
	if newMatches := resolver.NewMatches(); len(newMatches) > 0 {
		if err := s.UpsertPlayerMatches(ctx, newMatches); err != nil {
			return "", "", fmt.Errorf("resolve player match upsert: %w", err)
		}
	}
	if !resolved.Linked {
		return "", "", nil
	}
	return resolved.AthleteID, resolved.Team, nil
}

// SaveManualPlayerMatch writes manual = 1 in player_matches, removes any
// conflicting non-manual row for the other team in that matchup if needed,
// and backfills team and player_id on matching odds and legs rows.
func (s *Store) SaveManualPlayerMatch(ctx context.Context, oddsName, team, athleteID string) error {
	norm := NormalizePlayerName(oddsName)
	if norm == "" {
		return errors.New("empty player name")
	}

	return s.Tx(ctx, func(tx *Store) error {
		_, err := tx.conn().ExecContext(ctx, `
			INSERT INTO player_matches (odds_name, team, espn_athlete_id, manual)
			VALUES (?, ?, ?, 1)
			ON CONFLICT (odds_name, team) DO UPDATE SET
				espn_athlete_id = excluded.espn_athlete_id,
				manual = 1`,
			norm, team, athleteID,
		)
		if err != nil {
			return fmt.Errorf("save manual match: %w", err)
		}

		_, err = tx.conn().ExecContext(ctx, `
			DELETE FROM player_matches
			WHERE odds_name = ?
				AND manual = 0
				AND (
					team IN (
						SELECT CASE WHEN home_team = ? THEN away_team ELSE home_team END
						FROM games
						WHERE home_team = ? OR away_team = ?
					)
					OR NOT EXISTS (SELECT 1 FROM games WHERE home_team = ? OR away_team = ?)
				)
				AND team != ?`,
			norm, team, team, team, team, team, team,
		)
		if err != nil {
			return fmt.Errorf("clean conflicting match: %w", err)
		}

		_, err = tx.conn().ExecContext(ctx, `
			UPDATE OR REPLACE odds
			SET team = ?
			WHERE (player_name = ? OR player_name = ? COLLATE NOCASE)
				AND (
					game_id IN (SELECT id FROM games WHERE home_team = ? OR away_team = ?)
					OR NOT EXISTS (SELECT 1 FROM games WHERE home_team = ? OR away_team = ?)
				)`,
			team, oddsName, norm, team, team, team, team,
		)
		if err != nil {
			return fmt.Errorf("backfill odds: %w", err)
		}

		_, err = tx.conn().ExecContext(ctx, `
			UPDATE legs
			SET team = ?, player_id = ?
			WHERE (player_name = ? OR player_name = ? COLLATE NOCASE)
				AND (
					game_id IS NULL
					OR game_id IN (SELECT id FROM games WHERE home_team = ? OR away_team = ?)
					OR NOT EXISTS (SELECT 1 FROM games WHERE home_team = ? OR away_team = ?)
				)`,
			team, athleteID, oddsName, norm, team, team, team, team,
		)
		if err != nil {
			return fmt.Errorf("backfill legs: %w", err)
		}

		return nil
	})
}

// UpsertPlayerMatches batch-inserts or updates player matches in one transaction.
// Existing manual = 1 rows are never overwritten by non-manual matches.
func (s *Store) UpsertPlayerMatches(ctx context.Context, matches []PlayerMatch) error {
	if len(matches) == 0 {
		return nil
	}
	return s.Tx(ctx, func(tx *Store) error {
		stmt, err := tx.conn().PrepareContext(ctx, `
			INSERT INTO player_matches (odds_name, team, espn_athlete_id, manual)
			VALUES (?, ?, ?, ?)
			ON CONFLICT (odds_name, team) DO UPDATE SET
				espn_athlete_id = excluded.espn_athlete_id,
				manual = CASE WHEN excluded.manual = 1 THEN 1 ELSE player_matches.manual END
			WHERE player_matches.manual = 0 OR excluded.manual = 1`)
		if err != nil {
			return fmt.Errorf("prepare upsert player matches: %w", err)
		}
		defer func() { _ = stmt.Close() }()

		for _, m := range matches {
			manual := 0
			if m.Manual {
				manual = 1
			}
			if _, err := stmt.ExecContext(ctx, m.OddsName, m.Team, m.ESPNAthleteID, manual); err != nil {
				return fmt.Errorf("upsert player match %s %s: %w", m.OddsName, m.Team, err)
			}
		}
		return nil
	})
}

// PlayerMatchesForTeams returns all player matches for the given teams.
func (s *Store) PlayerMatchesForTeams(ctx context.Context, teams []string) ([]PlayerMatch, error) {
	if len(teams) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(teams))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(teams))
	for i, t := range teams {
		args[i] = t
	}
	rows, err := s.readConn().QueryContext(ctx,
		"SELECT "+matchColumns+" FROM player_matches WHERE team IN ("+placeholders+") ORDER BY team, odds_name", args...)
	if err != nil {
		return nil, fmt.Errorf("player matches for teams: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var matches []PlayerMatch
	for rows.Next() {
		var m PlayerMatch
		var manual int
		if err := rows.Scan(&m.OddsName, &m.Team, &m.ESPNAthleteID, &manual); err != nil {
			return nil, fmt.Errorf("player matches for teams: %w", err)
		}
		m.Manual = manual == 1
		matches = append(matches, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("player matches for teams: %w", err)
	}
	return matches, nil
}

func scanRosterPlayer(row rowScanner) (RosterPlayer, error) {
	var (
		p         RosterPlayer
		updatedAt string
	)
	if err := row.Scan(
		&p.AthleteID,
		&p.Team,
		&p.ESPNTeamID,
		&p.FullName,
		&p.NormName,
		&p.Position,
		&p.HeadshotURL,
		&updatedAt,
	); err != nil {
		return RosterPlayer{}, err
	}
	t, err := scanTimestamp(updatedAt)
	if err != nil {
		return RosterPlayer{}, err
	}
	p.UpdatedAt = t
	return p, nil
}

func scanPlayerGameLog(row rowScanner) (PlayerGameLog, error) {
	var (
		g         PlayerGameLog
		updatedAt string
	)
	if err := row.Scan(&g.AthleteID, &g.GamesPlayed, &g.StatsJSON, &updatedAt); err != nil {
		return PlayerGameLog{}, err
	}
	t, err := scanTimestamp(updatedAt)
	if err != nil {
		return PlayerGameLog{}, err
	}
	g.UpdatedAt = t
	return g, nil
}
