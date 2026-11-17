package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type playerTeamKey struct {
	normName string
	team     string
}

type gameNormKey struct {
	gameID   string
	normName string
}

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
	linked     bool
	richness   int
	fetchedAt  string
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
	return state.flushNewMatches(ctx, tx)
}

type backfillState struct {
	rosterByAthleteID map[string]RosterPlayer
	rosterByNorm      map[string][]RosterPlayer
	matchesByNorm     map[string][]PlayerMatch
	existingMatches   map[playerTeamKey]struct{}
	newMatches        map[playerTeamKey]PlayerMatch
	gamesByID         map[string]gameMatchup
	unmatchedSpelling map[gameNormKey]string
}

func loadBackfillState(ctx context.Context, tx *sql.Tx) (*backfillState, error) {
	state := &backfillState{
		rosterByAthleteID: make(map[string]RosterPlayer),
		rosterByNorm:      make(map[string][]RosterPlayer),
		matchesByNorm:     make(map[string][]PlayerMatch),
		existingMatches:   make(map[playerTeamKey]struct{}),
		newMatches:        make(map[playerTeamKey]PlayerMatch),
		gamesByID:         make(map[string]gameMatchup),
		unmatchedSpelling: make(map[gameNormKey]string),
	}
	if err := state.loadAndRefreshRosters(ctx, tx); err != nil {
		return nil, err
	}
	if err := state.loadPlayerMatches(ctx, tx); err != nil {
		return nil, err
	}
	if err := state.loadGames(ctx, tx); err != nil {
		return nil, err
	}
	return state, nil
}

func (s *backfillState) loadAndRefreshRosters(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `SELECT athlete_id, team, full_name, norm_name FROM roster_players`)
	if err != nil {
		return fmt.Errorf("load roster_players for backfill: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type normUpdate struct {
		athleteID string
		normName  string
	}
	var updates []normUpdate

	for rows.Next() {
		var p RosterPlayer
		if err := rows.Scan(&p.AthleteID, &p.Team, &p.FullName, &p.NormName); err != nil {
			return fmt.Errorf("scan roster_players for backfill: %w", err)
		}
		wantNorm := NormalizePlayerName(p.FullName)
		if wantNorm != "" && p.NormName != wantNorm {
			updates = append(updates, normUpdate{athleteID: p.AthleteID, normName: wantNorm})
			p.NormName = wantNorm
		}
		s.rosterByAthleteID[p.AthleteID] = p
		if p.NormName != "" {
			s.rosterByNorm[p.NormName] = append(s.rosterByNorm[p.NormName], p)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate roster_players for backfill: %w", err)
	}

	for _, u := range updates {
		if _, err := tx.ExecContext(
			ctx,
			`UPDATE roster_players SET norm_name = ? WHERE athlete_id = ?`,
			u.normName,
			u.athleteID,
		); err != nil {
			return fmt.Errorf("update roster_players norm_name %s: %w", u.athleteID, err)
		}
	}
	return nil
}

func (s *backfillState) loadPlayerMatches(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `SELECT odds_name, team, espn_athlete_id, manual FROM player_matches`)
	if err != nil {
		return fmt.Errorf("load player_matches for backfill: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			m      PlayerMatch
			manual int
		)
		if err := rows.Scan(&m.OddsName, &m.Team, &m.ESPNAthleteID, &manual); err != nil {
			return fmt.Errorf("scan player_matches for backfill: %w", err)
		}
		m.Manual = manual == 1
		norm := NormalizePlayerName(m.OddsName)
		if norm != "" {
			s.matchesByNorm[norm] = append(s.matchesByNorm[norm], m)
			s.existingMatches[playerTeamKey{normName: norm, team: m.Team}] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate player_matches for backfill: %w", err)
	}
	return nil
}

func (s *backfillState) loadGames(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `SELECT id, home_team, away_team FROM games`)
	if err != nil {
		return fmt.Errorf("load games for backfill: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var id string
		var g gameMatchup
		if err := rows.Scan(&id, &g.homeTeam, &g.awayTeam); err != nil {
			return fmt.Errorf("scan games for backfill: %w", err)
		}
		s.gamesByID[id] = g
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate games for backfill: %w", err)
	}
	return nil
}

func (s *backfillState) resolve(
	gameID, rawName, existingTeam string,
	game gameMatchup,
	hasGame bool,
) (canonName, canonTeam, athleteID string, linked bool) {
	cleaned, teamHint := StripPlayerTag(rawName)
	norm := NormalizePlayerName(cleaned)
	if norm == "" {
		return cleaned, existingTeam, "", false
	}

	preferTeam := preferredTeam(existingTeam, teamHint, game, hasGame)

	if m, ok := s.matchFromCache(norm, preferTeam, game, hasGame); ok {
		if rp, found := s.rosterByAthleteID[m.ESPNAthleteID]; found && rp.FullName != "" {
			return rp.FullName, m.Team, m.ESPNAthleteID, true
		}
		return s.unifySpelling(gameID, norm, cleaned), m.Team, m.ESPNAthleteID, true
	}

	if rp, ok := s.matchFromRoster(norm, preferTeam, game, hasGame); ok {
		key := playerTeamKey{normName: norm, team: rp.Team}
		if _, exists := s.existingMatches[key]; !exists {
			s.newMatches[key] = PlayerMatch{
				OddsName:      norm,
				Team:          rp.Team,
				ESPNAthleteID: rp.AthleteID,
				Manual:        false,
			}
		}
		return rp.FullName, rp.Team, rp.AthleteID, true
	}

	team := existingTeam
	if team == "" && teamHint != "" {
		if !hasGame || teamHint == game.homeTeam || teamHint == game.awayTeam {
			team = teamHint
		}
	}
	return s.unifySpelling(gameID, norm, cleaned), team, "", false
}

func preferredTeam(existingTeam, teamHint string, game gameMatchup, hasGame bool) string {
	if teamHint != "" && (!hasGame || teamHint == game.homeTeam || teamHint == game.awayTeam) {
		return teamHint
	}
	if existingTeam != "" && (!hasGame || existingTeam == game.homeTeam || existingTeam == game.awayTeam) {
		return existingTeam
	}
	return ""
}

func (s *backfillState) matchFromCache(
	norm, preferTeam string,
	game gameMatchup,
	hasGame bool,
) (PlayerMatch, bool) {
	all := s.matchesByNorm[norm]
	if len(all) == 0 {
		return PlayerMatch{}, false
	}

	var candidates []PlayerMatch
	for _, m := range all {
		if hasGame {
			if m.Team == game.homeTeam || m.Team == game.awayTeam {
				candidates = append(candidates, m)
			}
		} else if preferTeam == "" || m.Team == preferTeam {
			candidates = append(candidates, m)
		}
	}
	if len(candidates) == 0 {
		return PlayerMatch{}, false
	}

	var manuals []PlayerMatch
	for _, m := range candidates {
		if m.Manual {
			manuals = append(manuals, m)
		}
	}
	if len(manuals) > 0 {
		return pickUniqueMatch(manuals, preferTeam)
	}
	return pickUniqueMatch(candidates, preferTeam)
}

func pickUniqueMatch(matches []PlayerMatch, preferTeam string) (PlayerMatch, bool) {
	if len(matches) == 1 {
		return matches[0], true
	}
	if preferTeam != "" {
		var narrowed []PlayerMatch
		for _, m := range matches {
			if m.Team == preferTeam {
				narrowed = append(narrowed, m)
			}
		}
		if len(narrowed) == 1 {
			return narrowed[0], true
		}
	}
	return PlayerMatch{}, false
}

func (s *backfillState) matchFromRoster(
	norm, preferTeam string,
	game gameMatchup,
	hasGame bool,
) (RosterPlayer, bool) {
	all := s.rosterByNorm[norm]
	if len(all) == 0 {
		return RosterPlayer{}, false
	}

	var candidates []RosterPlayer
	for _, p := range all {
		if hasGame {
			if p.Team == game.homeTeam || p.Team == game.awayTeam {
				candidates = append(candidates, p)
			}
		} else if preferTeam == "" || p.Team == preferTeam {
			candidates = append(candidates, p)
		}
	}
	if len(candidates) == 1 {
		return candidates[0], true
	}
	if len(candidates) > 1 && preferTeam != "" {
		var narrowed []RosterPlayer
		for _, p := range candidates {
			if p.Team == preferTeam {
				narrowed = append(narrowed, p)
			}
		}
		if len(narrowed) == 1 {
			return narrowed[0], true
		}
	}
	return RosterPlayer{}, false
}

func (s *backfillState) unifySpelling(gameID, norm, cleaned string) string {
	key := gameNormKey{gameID: gameID, normName: norm}
	if existing, ok := s.unmatchedSpelling[key]; ok {
		return existing
	}
	s.unmatchedSpelling[key] = cleaned
	return cleaned
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
		)
		if err := rows.Scan(
			&c.rowID, &c.gameID, &c.book, &c.marketType, &c.origTeam, &c.origName, &c.side,
			&link, &sid, &es, &c.fetchedAt,
		); err != nil {
			return fmt.Errorf("scan odds for backfill: %w", err)
		}
		if link.Valid {
			c.richness++
		}
		if sid.Valid {
			c.richness++
		}
		if es.Valid {
			c.richness++
		}

		game, hasGame := s.gamesByID[c.gameID]
		c.canonName, c.canonTeam, _, c.linked = s.resolve(c.gameID, c.origName, c.origTeam, game, hasGame)
		if c.canonName == "" {
			continue
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
			if preferBackfillOdds(c, winner) {
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

func preferBackfillOdds(cand, existing oddsBackfillCandidate) bool {
	if cand.linked != existing.linked {
		return cand.linked
	}
	if cand.richness != existing.richness {
		return cand.richness > existing.richness
	}
	candExact := cand.origTeam == cand.canonTeam && cand.origName == cand.canonName
	existingExact := existing.origTeam == existing.canonTeam && existing.origName == existing.canonName
	if candExact != existingExact {
		return candExact
	}
	if cand.fetchedAt != existing.fetchedAt {
		return cand.fetchedAt > existing.fetchedAt
	}
	return cand.rowID < existing.rowID
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
		if rp, ok := s.rosterByAthleteID[playerID]; ok {
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
	canonName, resolvedTeam, resolvedID, linked := s.resolve(l.gameID, l.playerName, l.team, game, hasGame)
	if team == "" && resolvedTeam != "" {
		team = resolvedTeam
	}
	if resolvedID != "" {
		playerID = resolvedID
	}
	if linked && canonName != "" {
		name = canonName
	} else if cleaned, _ := StripPlayerTag(l.playerName); cleaned != "" && cleaned != l.playerName {
		name = cleaned
	}
	return team, name, playerID
}

func (s *backfillState) flushNewMatches(ctx context.Context, tx *sql.Tx) error {
	if len(s.newMatches) == 0 {
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

	for _, m := range s.newMatches {
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
