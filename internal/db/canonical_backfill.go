package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type gameMatchup struct {
	homeTeam string
	awayTeam string
}

type oddsTargetKey struct {
	gameID     string
	book       string
	marketType string
	team       string
	playerName string
	side       string
}

type oddsBackfillCandidate struct {
	rowID      int64
	gameID     string
	book       string
	marketType string
	origTeam   string
	origName   string
	side       string
	canonTeam  string
	canonName  string
	pref       OddsCandidate
}

type legBackfillRow struct {
	id         int64
	gameID     string
	team       string
	playerName string
	playerID   string
}

// BackfillCanonicalPlayers deduplicates and canonicalizes stored odds rows,
// backfills missing legs.player_id and legs.team values, and refreshes
// roster_players.norm_name. It is safe to run multiple times.
func BackfillCanonicalPlayers(ctx context.Context, database *sql.DB) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin canonical player backfill: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := backfillCanonicalPlayersTx(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit canonical player backfill: %w", err)
	}
	return nil
}

func backfillCanonicalPlayersTx(ctx context.Context, tx *sql.Tx) error {
	state, err := loadBackfillState(ctx, tx)
	if err != nil {
		return err
	}
	if err := state.backfillOdds(ctx, tx); err != nil {
		return err
	}
	if err := state.backfillLegs(ctx, tx); err != nil {
		return err
	}
	return flushNewMatches(ctx, tx, state.resolver.NewMatches())
}

type backfillState struct {
	resolver  *BatchPlayerResolver
	gamesByID map[string]gameMatchup
}

func loadBackfillState(ctx context.Context, tx *sql.Tx) (*backfillState, error) {
	rosters, err := loadAndRefreshRosters(ctx, tx)
	if err != nil {
		return nil, err
	}
	matches, err := loadPlayerMatchesTx(ctx, tx)
	if err != nil {
		return nil, err
	}
	games, err := loadGamesTx(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &backfillState{
		resolver:  NewBatchPlayerResolver(rosters, matches),
		gamesByID: games,
	}, nil
}

func loadAndRefreshRosters(ctx context.Context, tx *sql.Tx) ([]RosterPlayer, error) {
	rows, err := tx.QueryContext(ctx, `SELECT athlete_id, team, full_name, norm_name FROM roster_players`)
	if err != nil {
		return nil, fmt.Errorf("load roster_players for backfill: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type normUpdate struct {
		athleteID string
		normName  string
	}
	var updates []normUpdate
	var rosters []RosterPlayer

	for rows.Next() {
		var p RosterPlayer
		if err := rows.Scan(&p.AthleteID, &p.Team, &p.FullName, &p.NormName); err != nil {
			return nil, fmt.Errorf("scan roster_players for backfill: %w", err)
		}
		wantNorm := NormalizePlayerName(p.FullName)
		if wantNorm != "" && p.NormName != wantNorm {
			updates = append(updates, normUpdate{athleteID: p.AthleteID, normName: wantNorm})
			p.NormName = wantNorm
		}
		rosters = append(rosters, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate roster_players for backfill: %w", err)
	}

	for _, u := range updates {
		if _, err := tx.ExecContext(
			ctx,
			`UPDATE roster_players SET norm_name = ? WHERE athlete_id = ?`,
			u.normName,
			u.athleteID,
		); err != nil {
			return nil, fmt.Errorf("update roster_players norm_name %s: %w", u.athleteID, err)
		}
	}
	return rosters, nil
}

func loadPlayerMatchesTx(ctx context.Context, tx *sql.Tx) ([]PlayerMatch, error) {
	rows, err := tx.QueryContext(ctx, `SELECT odds_name, team, espn_athlete_id, manual FROM player_matches`)
	if err != nil {
		return nil, fmt.Errorf("load player_matches for backfill: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var matches []PlayerMatch
	for rows.Next() {
		var (
			m      PlayerMatch
			manual int
		)
		if err := rows.Scan(&m.OddsName, &m.Team, &m.ESPNAthleteID, &manual); err != nil {
			return nil, fmt.Errorf("scan player_matches for backfill: %w", err)
		}
		m.Manual = manual == 1
		matches = append(matches, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate player_matches for backfill: %w", err)
	}
	return matches, nil
}

func loadGamesTx(ctx context.Context, tx *sql.Tx) (map[string]gameMatchup, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id, home_team, away_team FROM games`)
	if err != nil {
		return nil, fmt.Errorf("load games for backfill: %w", err)
	}
	defer func() { _ = rows.Close() }()

	games := make(map[string]gameMatchup)
	for rows.Next() {
		var id string
		var g gameMatchup
		if err := rows.Scan(&id, &g.homeTeam, &g.awayTeam); err != nil {
			return nil, fmt.Errorf("scan games for backfill: %w", err)
		}
		games[id] = g
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate games for backfill: %w", err)
	}
	return games, nil
}

func (s *backfillState) backfillOdds(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT rowid, game_id, book, market_type, team, player_name, side,
			link, sid, event_sid, fetched_at
		FROM odds
		WHERE TRIM(player_name) != ''
		ORDER BY rowid`)
	if err != nil {
		return fmt.Errorf("query odds for backfill: %w", err)
	}
	defer func() { _ = rows.Close() }()

	groups := make(map[oddsTargetKey][]oddsBackfillCandidate)
	var groupOrder []oddsTargetKey

	for rows.Next() {
		var (
			c             oddsBackfillCandidate
			link, sid, es sql.NullString
			fetchedAt     string
		)
		if err := rows.Scan(
			&c.rowID, &c.gameID, &c.book, &c.marketType, &c.origTeam, &c.origName, &c.side,
			&link, &sid, &es, &fetchedAt,
		); err != nil {
			return fmt.Errorf("scan odds for backfill: %w", err)
		}
		richness := 0
		if link.Valid {
			richness++
		}
		if sid.Valid {
			richness++
		}
		if es.Valid {
			richness++
		}

		game, hasGame := s.gamesByID[c.gameID]
		resolved := s.resolver.ResolvePlayer(c.gameID, c.origName, c.origTeam, game.homeTeam, game.awayTeam, hasGame)
		if resolved.Name == "" {
			continue
		}
		c.canonName = resolved.Name
		c.canonTeam = resolved.Team
		c.pref = OddsCandidate{
			Linked:        resolved.Linked,
			Richness:      richness,
			AlreadyCanon:  c.origTeam == c.canonTeam && c.origName == c.canonName,
			FetchedAt:     fetchedAt,
			TieBreakRowID: c.rowID,
		}

		key := oddsTargetKey{
			gameID:     c.gameID,
			book:       c.book,
			marketType: c.marketType,
			team:       c.canonTeam,
			playerName: c.canonName,
			side:       c.side,
		}
		if _, exists := groups[key]; !exists {
			groupOrder = append(groupOrder, key)
		}
		groups[key] = append(groups[key], c)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate odds for backfill: %w", err)
	}

	var deleteIDs []int64
	var updates []oddsBackfillCandidate
	for _, key := range groupOrder {
		cands := groups[key]
		winner := cands[0]
		for _, c := range cands[1:] {
			if PreferOddsCandidate(c.pref, winner.pref) {
				winner = c
			}
		}
		for _, c := range cands {
			if c.rowID != winner.rowID {
				deleteIDs = append(deleteIDs, c.rowID)
			}
		}
		if winner.origTeam != winner.canonTeam || winner.origName != winner.canonName {
			updates = append(updates, winner)
		}
	}

	for _, rowID := range deleteIDs {
		if _, err := tx.ExecContext(ctx, `DELETE FROM odds WHERE rowid = ?`, rowID); err != nil {
			return fmt.Errorf("delete duplicate odds rowid %d: %w", rowID, err)
		}
	}
	for _, u := range updates {
		if _, err := tx.ExecContext(ctx,
			`UPDATE odds SET team = ?, player_name = ? WHERE rowid = ?`,
			u.canonTeam, u.canonName, u.rowID,
		); err != nil {
			return fmt.Errorf("canonicalize odds rowid %d: %w", u.rowID, err)
		}
	}
	return nil
}

func (s *backfillState) backfillLegs(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, COALESCE(game_id, ''), COALESCE(team, ''), player_name, COALESCE(player_id, '')
		FROM legs
		WHERE player_name IS NOT NULL AND TRIM(player_name) != ''
		ORDER BY id`)
	if err != nil {
		return fmt.Errorf("query legs for backfill: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var legs []legBackfillRow
	for rows.Next() {
		var l legBackfillRow
		if err := rows.Scan(&l.id, &l.gameID, &l.team, &l.playerName, &l.playerID); err != nil {
			return fmt.Errorf("scan legs for backfill: %w", err)
		}
		legs = append(legs, l)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate legs for backfill: %w", err)
	}

	for _, l := range legs {
		newTeam, newName, newPlayerID := s.resolveLeg(l)
		if newTeam == l.team && newName == l.playerName && newPlayerID == l.playerID {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE legs
			SET team = ?, player_name = ?, player_id = ?
			WHERE id = ?`,
			nullableStringValue(newTeam),
			nullableStringValue(newName),
			nullableStringValue(newPlayerID),
			l.id,
		); err != nil {
			return fmt.Errorf("backfill leg %d: %w", l.id, err)
		}
	}
	return nil
}

func (s *backfillState) resolveLeg(l legBackfillRow) (team, name, playerID string) {
	team = l.team
	name = l.playerName
	playerID = l.playerID

	if playerID != "" {
		if rp, ok := s.resolver.RosterPlayerByID(playerID); ok {
			if team == "" {
				team = rp.Team
			}
			if rp.FullName != "" {
				name = rp.FullName
			}
		}
		return team, name, playerID
	}

	game, hasGame := s.gamesByID[l.gameID]
	resolved := s.resolver.ResolvePlayer(l.gameID, l.playerName, l.team, game.homeTeam, game.awayTeam, hasGame)
	if team == "" && resolved.Team != "" {
		team = resolved.Team
	}
	if resolved.AthleteID != "" {
		playerID = resolved.AthleteID
	}
	if resolved.Linked && resolved.Name != "" {
		name = resolved.Name
	} else if cleaned, _ := StripPlayerTag(l.playerName); cleaned != "" && cleaned != l.playerName {
		name = cleaned
	}
	return team, name, playerID
}

func flushNewMatches(ctx context.Context, tx *sql.Tx, matches []PlayerMatch) error {
	if len(matches) == 0 {
		return nil
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO player_matches (odds_name, team, espn_athlete_id, manual)
		VALUES (?, ?, ?, 0)
		ON CONFLICT (odds_name, team) DO UPDATE SET
			espn_athlete_id = excluded.espn_athlete_id
		WHERE player_matches.manual = 0`)
	if err != nil {
		return fmt.Errorf("prepare backfill player_matches: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, m := range matches {
		if _, err := stmt.ExecContext(ctx, m.OddsName, m.Team, m.ESPNAthleteID); err != nil {
			return fmt.Errorf("upsert backfill player_match %s %s: %w", m.OddsName, m.Team, err)
		}
	}
	return nil
}

func nullableStringValue(val string) sql.NullString {
	trimmed := strings.TrimSpace(val)
	if trimmed == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: trimmed, Valid: true}
}
