package db

// ResolvedPlayer is the canonicalized player identity for a prop or leg row.
type ResolvedPlayer struct {
	Name      string
	Team      string
	AthleteID string
	Linked    bool
}

type playerTeamKey struct {
	normName string
	team     string
}

type gameNormKey struct {
	gameID   string
	normName string
}

// BatchPlayerResolver resolves sportsbook player names against preloaded
// roster_players and player_matches rows without per-row database queries.
type BatchPlayerResolver struct {
	rosterByAthleteID map[string]RosterPlayer
	rosterByNorm      map[string][]RosterPlayer
	matchesByNorm     map[string][]PlayerMatch
	existingMatches   map[playerTeamKey]struct{}
	newMatches        map[playerTeamKey]PlayerMatch
	unmatchedSpelling map[gameNormKey]string
}

// NewBatchPlayerResolver constructs a BatchPlayerResolver from preloaded
// roster players and cached player matches.
func NewBatchPlayerResolver(rosters []RosterPlayer, matches []PlayerMatch) *BatchPlayerResolver {
	r := &BatchPlayerResolver{
		rosterByAthleteID: make(map[string]RosterPlayer, len(rosters)),
		rosterByNorm:      make(map[string][]RosterPlayer, len(rosters)),
		matchesByNorm:     make(map[string][]PlayerMatch, len(matches)),
		existingMatches:   make(map[playerTeamKey]struct{}, len(matches)),
		newMatches:        make(map[playerTeamKey]PlayerMatch),
		unmatchedSpelling: make(map[gameNormKey]string),
	}

	for _, p := range rosters {
		r.rosterByAthleteID[p.AthleteID] = p
		norm := p.NormName
		if norm == "" {
			norm = NormalizePlayerName(p.FullName)
		}
		if norm != "" {
			r.rosterByNorm[norm] = append(r.rosterByNorm[norm], p)
		}
	}
	for _, m := range matches {
		norm := NormalizePlayerName(m.OddsName)
		if norm != "" {
			r.matchesByNorm[norm] = append(r.matchesByNorm[norm], m)
			r.existingMatches[playerTeamKey{normName: norm, team: m.Team}] = struct{}{}
		}
	}
	return r
}

// RosterPlayerByID returns a preloaded roster player by ESPN athlete ID.
func (r *BatchPlayerResolver) RosterPlayerByID(athleteID string) (RosterPlayer, bool) {
	rp, ok := r.rosterByAthleteID[athleteID]
	return rp, ok
}

// ResolveGamePlayer resolves a raw sportsbook player name within a known game
// matchup.
func (r *BatchPlayerResolver) ResolveGamePlayer(
	gameID, rawName, existingTeam, homeTeam, awayTeam string,
) ResolvedPlayer {
	return r.ResolvePlayer(gameID, rawName, existingTeam, homeTeam, awayTeam, true)
}

// ResolvePlayer resolves a raw sportsbook player name with an optional game
// matchup.
func (r *BatchPlayerResolver) ResolvePlayer(
	gameID, rawName, existingTeam, homeTeam, awayTeam string,
	hasGame bool,
) ResolvedPlayer {
	cleaned, teamHint := StripPlayerTag(rawName)
	norm := NormalizePlayerName(cleaned)
	if norm == "" {
		return ResolvedPlayer{Name: cleaned, Team: existingTeam}
	}

	preferTeam := preferredTeam(existingTeam, teamHint, homeTeam, awayTeam, hasGame)

	if m, ok := r.matchFromCache(norm, preferTeam, homeTeam, awayTeam, hasGame); ok {
		name := ""
		if rp, found := r.rosterByAthleteID[m.ESPNAthleteID]; found && rp.FullName != "" {
			name = rp.FullName
		} else {
			name = r.unifySpelling(gameID, norm, cleaned)
		}
		return ResolvedPlayer{
			Name:      name,
			Team:      m.Team,
			AthleteID: m.ESPNAthleteID,
			Linked:    true,
		}
	}

	if rp, ok := r.matchFromRoster(norm, preferTeam, homeTeam, awayTeam, hasGame); ok {
		// Avoid caching manual=0 rows when multiple roster players share norm,
		// so a hinted match on one team cannot auto-match untagged rows later.
		if len(r.rosterByNorm[norm]) <= 1 {
			key := playerTeamKey{normName: norm, team: rp.Team}
			if _, exists := r.existingMatches[key]; !exists {
				r.newMatches[key] = PlayerMatch{
					OddsName:      norm,
					Team:          rp.Team,
					ESPNAthleteID: rp.AthleteID,
					Manual:        false,
				}
			}
		}
		return ResolvedPlayer{
			Name:      rp.FullName,
			Team:      rp.Team,
			AthleteID: rp.AthleteID,
			Linked:    true,
		}
	}

	team := existingTeam
	if team == "" && teamHint != "" {
		if !hasGame || teamHint == homeTeam || teamHint == awayTeam {
			team = teamHint
		}
	}
	return ResolvedPlayer{
		Name:   r.unifySpelling(gameID, norm, cleaned),
		Team:   team,
		Linked: false,
	}
}

// NewMatches returns any newly discovered automatic player_matches rows that
// were not already present in the preloaded cache.
func (r *BatchPlayerResolver) NewMatches() []PlayerMatch {
	if len(r.newMatches) == 0 {
		return nil
	}
	matches := make([]PlayerMatch, 0, len(r.newMatches))
	for _, m := range r.newMatches {
		matches = append(matches, m)
	}
	return matches
}

func preferredTeam(existingTeam, teamHint, homeTeam, awayTeam string, hasGame bool) string {
	if teamHint != "" && (!hasGame || teamHint == homeTeam || teamHint == awayTeam) {
		return teamHint
	}
	if existingTeam != "" && (!hasGame || existingTeam == homeTeam || existingTeam == awayTeam) {
		return existingTeam
	}
	return ""
}

func (r *BatchPlayerResolver) rosterCandidates(
	norm, preferTeam, homeTeam, awayTeam string,
	hasGame bool,
) []RosterPlayer {
	all := r.rosterByNorm[norm]
	if len(all) == 0 {
		return nil
	}
	var candidates []RosterPlayer
	for _, p := range all {
		if hasGame {
			if p.Team == homeTeam || p.Team == awayTeam {
				candidates = append(candidates, p)
			}
		} else if preferTeam == "" || p.Team == preferTeam {
			candidates = append(candidates, p)
		}
	}
	return candidates
}

func (r *BatchPlayerResolver) matchFromCache(
	norm, preferTeam, homeTeam, awayTeam string,
	hasGame bool,
) (PlayerMatch, bool) {
	all := r.matchesByNorm[norm]
	if len(all) == 0 {
		return PlayerMatch{}, false
	}

	var candidates []PlayerMatch
	for _, m := range all {
		if hasGame {
			if m.Team == homeTeam || m.Team == awayTeam {
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
		return pickPlayerMatch(manuals, preferTeam, true)
	}

	// Refuse non-manual cache matches when no valid team hint is present and
	// multiple roster players in the matchup share norm.
	if preferTeam == "" && len(r.rosterCandidates(norm, "", homeTeam, awayTeam, hasGame)) > 1 {
		return PlayerMatch{}, false
	}
	return pickPlayerMatch(candidates, preferTeam, false)
}

func pickPlayerMatch(candidates []PlayerMatch, preferTeam string, allowSingleMismatch bool) (PlayerMatch, bool) {
	if preferTeam != "" {
		var hinted []PlayerMatch
		for _, c := range candidates {
			if c.Team == preferTeam {
				hinted = append(hinted, c)
			}
		}
		if len(hinted) == 1 {
			return hinted[0], true
		}
		if len(hinted) > 1 || !allowSingleMismatch {
			return PlayerMatch{}, false
		}
	}
	if len(candidates) == 1 {
		return candidates[0], true
	}
	return PlayerMatch{}, false
}

func (r *BatchPlayerResolver) matchFromRoster(
	norm, preferTeam, homeTeam, awayTeam string,
	hasGame bool,
) (RosterPlayer, bool) {
	candidates := r.rosterCandidates(norm, preferTeam, homeTeam, awayTeam, hasGame)
	if len(candidates) == 0 {
		return RosterPlayer{}, false
	}
	if preferTeam != "" {
		var hinted []RosterPlayer
		for _, c := range candidates {
			if c.Team == preferTeam {
				hinted = append(hinted, c)
			}
		}
		if len(hinted) == 1 {
			return hinted[0], true
		}
		return RosterPlayer{}, false
	}
	if len(candidates) == 1 {
		return candidates[0], true
	}
	return RosterPlayer{}, false
}

func (r *BatchPlayerResolver) unifySpelling(gameID, norm, cleaned string) string {
	key := gameNormKey{gameID: gameID, normName: norm}
	if existing, ok := r.unmatchedSpelling[key]; ok {
		return existing
	}
	r.unmatchedSpelling[key] = cleaned
	return cleaned
}

// OddsCandidate describes one row competing for a canonical odds key.
type OddsCandidate struct {
	Linked        bool
	Richness      int
	AlreadyCanon  bool
	FetchedAt     string
	TieBreakRowID int64
}

// OddsRichness counts how many optional link/sid fields are populated on row.
func OddsRichness(row Odds) int {
	score := 0
	if row.Link != nil {
		score++
	}
	if row.Sid != nil {
		score++
	}
	if row.EventSid != nil {
		score++
	}
	return score
}

// PreferOddsCandidate reports whether cand should replace existing when two
// odds rows collapse onto the same canonical key.
func PreferOddsCandidate(cand, existing OddsCandidate) bool {
	if cand.Linked != existing.Linked {
		return cand.Linked
	}
	if cand.Richness != existing.Richness {
		return cand.Richness > existing.Richness
	}
	if cand.AlreadyCanon != existing.AlreadyCanon {
		return cand.AlreadyCanon
	}
	if cand.FetchedAt != existing.FetchedAt {
		return cand.FetchedAt > existing.FetchedAt
	}
	return cand.TieBreakRowID < existing.TieBreakRowID
}
