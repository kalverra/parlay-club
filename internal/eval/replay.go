package eval

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/espn"
	"github.com/kalverra/parlay-club/internal/ingest/nflverse"
	"github.com/kalverra/parlay-club/internal/ingest/odds"
	"github.com/kalverra/parlay-club/internal/parlay"
)

// LoadDatasetJSON reads a JSON-encoded ReplayDataset from path.
func LoadDatasetJSON(path string) (ReplayDataset, error) {
	// #nosec G304 -- caller-supplied replay dataset JSON path.
	raw, err := os.ReadFile(path)
	if err != nil {
		return ReplayDataset{}, fmt.Errorf("read dataset %s: %w", path, err)
	}
	var dataset ReplayDataset
	if err := json.Unmarshal(raw, &dataset); err != nil {
		return ReplayDataset{}, fmt.Errorf("decode dataset %s: %w", path, err)
	}
	if dataset.Coverage.TotalLegs == 0 {
		for _, w := range dataset.Weeks {
			for _, s := range w.Slips {
				dataset.Coverage.TotalLegs += len(s.Legs)
			}
		}
	}
	return dataset, nil
}

// ReplayFilter narrows which weeks and slips LoadDataset replays.
type ReplayFilter struct {
	Season   int   // 0 = all seasons
	FromWeek int   // 0 = unbounded
	ToWeek   int   // 0 = unbounded
	GroupID  int64 // 0 = all groups
}

// LoadDataset reconstructs leak-free pre-kickoff analyzer inputs and settled
// slips/legs from store. It reads prior-week nflverse rows (week_num < target)
// and pre-kickoff api_archive payloads (fetched_at < game.Kickoff), failing
// soft when archive entries are missing without ever falling back to
// overwritten live tables (player_gamelogs, roster_players, odds).
func LoadDataset(ctx context.Context, store *db.Store, filter ReplayFilter) (ReplayDataset, error) {
	allWeeks, err := store.Weeks(ctx)
	if err != nil {
		return ReplayDataset{}, fmt.Errorf("load weeks: %w", err)
	}
	weeks := filterWeeks(allWeeks, filter)

	var dataset ReplayDataset
	for _, week := range weeks {
		slips, rawLegs, err := loadWeekSlips(ctx, store, week.ID, filter.GroupID)
		if err != nil {
			return ReplayDataset{}, fmt.Errorf("load week %d slips: %w", week.ID, err)
		}
		if len(slips) == 0 {
			continue
		}

		weekGames, err := store.GamesForWeek(ctx, week.ID)
		if err != nil {
			return ReplayDataset{}, fmt.Errorf("load week %d games: %w", week.ID, err)
		}
		gameByDBID := make(map[string]db.Game, len(weekGames))
		gameIDs := make([]string, 0, len(weekGames))
		for _, g := range weekGames {
			gameByDBID[g.ID] = g
			gameIDs = append(gameIDs, g.ID)
		}

		athleteIDs := analyzer.DistinctLegPlayerIDs(rawLegs)
		nflverseData := loadPriorNFLverse(ctx, store, week, athleteIDs)

		nfeloByGame, err := store.NfeloForGames(ctx, gameIDs)
		if err != nil {
			nfeloByGame = nil
		}

		gamesMap := make(map[string]analyzer.Game, len(weekGames))
		positionByID := make(map[string]string)
		gameHasInjuries := make(map[string]bool, len(weekGames))
		gameHasArchivedOdds := make(map[string]bool, len(weekGames))
		referencedGames, gameNeedsOddsCheck := referencedGamesFromSlips(slips)
		matchGames := buildMatchGames(weekGames)
		linesCache := make(map[string]decodedLinesSnapshot)

		for _, g := range weekGames {
			var proj *db.NfeloGame
			if p, ok := nfeloByGame[g.ID]; ok {
				proj = &p
			}
			// Reconstruct pre-kickoff game state as scheduled so GameFromDB attaches nfelo.
			preKickoffGame := g
			preKickoffGame.Status = db.GameScheduled
			converted := analyzer.GameFromDB(preKickoffGame, proj, nflverseData.teamEPA)

			if referencedGames[g.ID] {
				injuries, rosterPositions, rosterPlayers, hasInj := loadArchivedGameInjuries(ctx, store, g)
				if len(injuries) > 0 {
					converted.Injuries = injuries
				}
				gameHasInjuries[g.ID] = hasInj
				for id, pos := range rosterPositions {
					if _, exists := positionByID[id]; !exists {
						positionByID[id] = pos
					}
				}

				preKickoffOdds, hasArchivedOdds := loadPreKickoffGameOdds(
					ctx,
					store,
					g,
					matchGames,
					rosterPlayers,
					nflverseData.playersByTeam,
					linesCache,
				)
				if len(preKickoffOdds) > 0 {
					if sharpByGame := analyzer.BuildSharpBenchmarks(preKickoffOdds); len(sharpByGame) > 0 {
						converted.Sharp = sharpByGame[g.ID]
					}
				}
				if gameNeedsOddsCheck[g.ID] {
					gameHasArchivedOdds[g.ID] = hasArchivedOdds
				}
			}
			gamesMap[g.ID] = converted
		}

		statsMap, playerHasGameLog, playerHasL5Fallback := loadArchivedPlayerStats(
			ctx,
			store,
			rawLegs,
			gameByDBID,
			nflverseData,
			positionByID,
		)

		accumulateWeekCoverage(
			&dataset.Coverage,
			slips,
			gamesMap,
			statsMap,
			playerHasGameLog,
			playerHasL5Fallback,
			gameHasInjuries,
			gameHasArchivedOdds,
		)

		dataset.Weeks = append(dataset.Weeks, ReplayedWeek{
			Season:   week.Season,
			WeekNum:  week.WeekNum,
			WeekType: weekTypeLabel(week.SeasonType),
			Games:    gamesMap,
			Stats:    statsMap,
			Slips:    slips,
		})
	}

	return dataset, nil
}

func referencedGamesFromSlips(slips []ReplayedSlip) (map[string]bool, map[string]bool) {
	referenced := make(map[string]bool, len(slips))
	needsOddsCheck := make(map[string]bool, len(slips))
	for _, s := range slips {
		for _, rl := range s.Legs {
			if rl.Leg.GameID == "" {
				continue
			}
			referenced[rl.Leg.GameID] = true
			if rl.DecimalOdds <= 1 {
				needsOddsCheck[rl.Leg.GameID] = true
			}
		}
	}
	return referenced, needsOddsCheck
}

func accumulateWeekCoverage(
	cov *DataCoverage,
	slips []ReplayedSlip,
	gamesMap map[string]analyzer.Game,
	statsMap map[string]analyzer.PlayerStats,
	playerHasGameLog map[string]bool,
	playerHasL5Fallback map[string]bool,
	gameHasInjuries map[string]bool,
	gameHasArchivedOdds map[string]bool,
) {
	for _, s := range slips {
		for _, rl := range s.Legs {
			cov.TotalLegs++
			if rl.Leg.PlayerID != "" {
				if playerHasGameLog[rl.Leg.PlayerID] {
					cov.WithGameLogs++
				} else if playerHasL5Fallback[rl.Leg.PlayerID] {
					cov.WithL5Fallback++
				}
			}
			if rl.Leg.GameID == "" {
				if rl.DecimalOdds > 1 {
					cov.WithOdds++
				}
				continue
			}
			if gameHasInjuries[rl.Leg.GameID] {
				cov.WithInjuries++
			}
			if g, ok := gamesMap[rl.Leg.GameID]; ok {
				if g.Nfelo != nil {
					cov.WithNfelo++
				}
				if analyzer.HasSharpBenchmark(g, rl.Leg) {
					cov.WithSharp++
				}
				hasNFLV := g.HomeEPA != nil || g.AwayEPA != nil
				if !hasNFLV && rl.Leg.PlayerID != "" {
					if ps, ok := statsMap[rl.Leg.PlayerID]; ok && ps.Usage != nil {
						hasNFLV = true
					}
				}
				if hasNFLV {
					cov.WithNFLVerse++
				}
			}
			if gameHasArchivedOdds[rl.Leg.GameID] || rl.DecimalOdds > 1 {
				cov.WithOdds++
			}
		}
	}
}

func filterWeeks(weeks []db.Week, filter ReplayFilter) []db.Week {
	out := make([]db.Week, 0, len(weeks))
	for _, w := range weeks {
		if filter.Season != 0 && w.Season != filter.Season {
			continue
		}
		if filter.FromWeek != 0 && w.WeekNum < filter.FromWeek {
			continue
		}
		if filter.ToWeek != 0 && w.WeekNum > filter.ToWeek {
			continue
		}
		out = append(out, w)
	}
	slices.SortFunc(out, func(a, b db.Week) int {
		if c := cmp.Compare(a.Season, b.Season); c != 0 {
			return c
		}
		if c := cmp.Compare(a.SeasonType, b.SeasonType); c != 0 {
			return c
		}
		return cmp.Compare(a.WeekNum, b.WeekNum)
	})
	return out
}

func weekTypeLabel(seasonType int) string {
	if seasonType == nflverse.SeasonTypePostseason {
		return "POST"
	}
	return "REG"
}

func loadWeekSlips(
	ctx context.Context,
	store *db.Store,
	weekID int64,
	groupID int64,
) ([]ReplayedSlip, []db.Leg, error) {
	rows, err := store.SlipsWithLegsForWeek(ctx, weekID, groupID, db.SlipGraded)
	if err != nil {
		return nil, nil, err
	}

	var slips []ReplayedSlip
	var allLegs []db.Leg
	for _, row := range rows {
		slip := row.Slip
		dbLegs := row.Legs

		replayedLegs := make([]ReplayedLeg, 0, len(dbLegs))
		parlayLegs := make([]parlay.Leg, 0, len(dbLegs))
		allLegsValidOdds := true

		for _, dl := range dbLegs {
			dec := 0.0
			if dl.Odds != 0 {
				dec = parlay.AmericanToDecimal(dl.Odds)
			}
			if isSettledResult(dl.Result) && dl.Result.Counts() {
				if dec > 1 {
					parlayLegs = append(parlayLegs, parlay.Leg{
						Decimal: dec,
						Result:  dl.Result,
					})
				} else {
					allLegsValidOdds = false
				}
			}
			replayedLegs = append(replayedLegs, ReplayedLeg{
				Leg:         analyzer.LegFromDB(dl, ""),
				Result:      dl.Result,
				DecimalOdds: dec,
			})
			allLegs = append(allLegs, dl)
		}

		if len(replayedLegs) == 0 {
			continue
		}
		combined := 0.0
		if allLegsValidOdds && len(parlayLegs) > 0 {
			combined = parlay.CombinedOdds(parlayLegs)
		}
		slips = append(slips, ReplayedSlip{
			SlipID:       slip.ID,
			Result:       slip.Result,
			CombinedOdds: combined,
			Legs:         replayedLegs,
		})
	}
	return slips, allLegs, nil
}

func isSettledResult(r parlay.Result) bool {
	return r == parlay.Won || r == parlay.Lost || r == parlay.Push || r == parlay.Void
}

type priorNFLverseData struct {
	teamEPA       map[string]*analyzer.TeamDefenseProfile
	usageByPlayer map[string]*analyzer.PlayerUsage
	gamesByPlayer map[string]map[string][]float64
	positionByID  map[string]string
	playersByTeam map[string][]db.RosterPlayer
}

func loadPriorNFLverse(
	ctx context.Context,
	store *db.Store,
	week db.Week,
	athleteIDs []string,
) priorNFLverseData {
	seasonTypes := analyzer.NFLverseSeasonTypes(week.SeasonType)
	out := priorNFLverseData{
		playersByTeam: make(map[string][]db.RosterPlayer),
	}

	if rows, err := store.NFLverseTeamWeeksForSeasonTypes(ctx, week.Season, seasonTypes...); err == nil {
		filtered := make([]db.NFLverseTeamWeek, 0, len(rows))
		for _, r := range rows {
			if isPriorNFLverseWeek(r.SeasonType, r.WeekNum, week) {
				filtered = append(filtered, r)
			}
		}
		out.teamEPA = analyzer.BuildTeamDefenseProfiles(filtered)
	}

	if len(athleteIDs) > 0 {
		if byPlayer, err := store.NFLversePlayerWeeksForPlayersTypes(
			ctx,
			athleteIDs,
			week.Season,
			seasonTypes...); err == nil {
			out.usageByPlayer = make(map[string]*analyzer.PlayerUsage, len(athleteIDs))
			out.gamesByPlayer = make(map[string]map[string][]float64, len(athleteIDs))
			out.positionByID = make(map[string]string, len(athleteIDs))
			for _, id := range athleteIDs {
				rows := byPlayer[id]
				filtered := make([]db.NFLversePlayerWeek, 0, len(rows))
				for _, r := range rows {
					if isPriorNFLverseWeek(r.SeasonType, r.WeekNum, week) {
						filtered = append(filtered, r)
					}
				}
				if len(filtered) == 0 {
					continue
				}
				if u := analyzer.BuildPlayerUsage(filtered); u != nil {
					out.usageByPlayer[id] = u
				}
				out.gamesByPlayer[id] = buildNFLversePlayerGames(filtered)
				for _, r := range filtered {
					if r.Position != "" {
						out.positionByID[id] = r.Position
						break
					}
				}
			}
		}
	}

	seenPlayers := make(map[string]bool)
	// Iterate most recent season type first so postseason rows precede regular season.
	for _, seasonType := range slices.Backward(seasonTypes) {
		rows, err := store.NFLversePlayerWeeksForSeason(ctx, week.Season, seasonType)
		if err != nil {
			continue
		}
		for _, r := range rows {
			if !isPriorNFLverseWeek(r.SeasonType, r.WeekNum, week) {
				continue
			}
			if r.ESPNID == "" || r.Team == "" || r.PlayerName == "" || seenPlayers[r.ESPNID] {
				continue
			}
			seenPlayers[r.ESPNID] = true
			out.playersByTeam[r.Team] = append(out.playersByTeam[r.Team], db.RosterPlayer{
				AthleteID: r.ESPNID,
				Team:      r.Team,
				FullName:  r.PlayerName,
				NormName:  db.NormalizePlayerName(r.PlayerName),
				Position:  r.Position,
			})
		}
	}

	return out
}

// buildNFLversePlayerGames constructs per-market stat slices (most recent game
// first) from strictly prior-week nflverse rows ordered season_type DESC,
// week_num DESC.
func buildNFLversePlayerGames(rows []db.NFLversePlayerWeek) map[string][]float64 {
	if len(rows) == 0 {
		return nil
	}
	n := len(rows)
	games := map[string][]float64{
		"PASS_YDS":   make([]float64, 0, n),
		"PASS_TDS":   make([]float64, 0, n),
		"RUSH_YDS":   make([]float64, 0, n),
		"RUSH_ATT":   make([]float64, 0, n),
		"REC_YDS":    make([]float64, 0, n),
		"RECEPTIONS": make([]float64, 0, n),
		"ANYTIME_TD": make([]float64, 0, n),
		"KICKER_PTS": make([]float64, 0, n),
	}
	for _, r := range rows {
		games["PASS_YDS"] = append(games["PASS_YDS"], float64(r.PassYds))
		games["PASS_TDS"] = append(games["PASS_TDS"], float64(r.PassTDs))
		games["RUSH_YDS"] = append(games["RUSH_YDS"], float64(r.RushYds))
		games["RUSH_ATT"] = append(games["RUSH_ATT"], float64(r.Carries))
		games["REC_YDS"] = append(games["REC_YDS"], float64(r.RecYds))
		games["RECEPTIONS"] = append(games["RECEPTIONS"], float64(r.Receptions))
		games["ANYTIME_TD"] = append(games["ANYTIME_TD"], float64(r.RushTDs+r.RecTDs))
		games["KICKER_PTS"] = append(games["KICKER_PTS"], float64(r.KickerPts))
	}
	return games
}

func isPriorNFLverseWeek(rowSeasonType, rowWeekNum int, target db.Week) bool {
	if target.SeasonType == nflverse.SeasonTypePostseason {
		if rowSeasonType == nflverse.SeasonTypeRegular {
			return true
		}
		return rowSeasonType == nflverse.SeasonTypePostseason && rowWeekNum < target.WeekNum
	}
	return rowSeasonType == target.SeasonType && rowWeekNum < target.WeekNum
}

func loadArchivedPlayerStats(
	ctx context.Context,
	store *db.Store,
	legs []db.Leg,
	games map[string]db.Game,
	nflverseData priorNFLverseData,
	positionByID map[string]string,
) (map[string]analyzer.PlayerStats, map[string]bool, map[string]bool) {
	kickoffByAthlete := make(map[string]time.Time)
	for _, l := range legs {
		if l.PlayerID == nil || *l.PlayerID == "" || l.GameID == nil || *l.GameID == "" {
			continue
		}
		g, ok := games[*l.GameID]
		if !ok || g.Kickoff.IsZero() {
			continue
		}
		id := *l.PlayerID
		if prev, exists := kickoffByAthlete[id]; !exists || g.Kickoff.Before(prev) {
			kickoffByAthlete[id] = g.Kickoff
		}
	}

	athleteIDs := analyzer.DistinctLegPlayerIDs(legs)
	stats := make(map[string]analyzer.PlayerStats, len(athleteIDs))
	hasGameLog := make(map[string]bool, len(athleteIDs))
	hasL5Fallback := make(map[string]bool, len(athleteIDs))

	for _, id := range athleteIDs {
		var markets map[string][]float64
		if kickoff, ok := kickoffByAthlete[id]; ok {
			key := db.ArchiveGameLogKey(id)
			if raw, found := latestArchivedBodyBefore(ctx, store, key, kickoff); found {
				if parsed, err := espn.ParseGameLogJSON(raw); err == nil && len(parsed) > 0 {
					markets = parsed
					hasGameLog[id] = true
				}
			}
		}
		if len(markets) == 0 {
			if fallback := nflverseData.gamesByPlayer[id]; len(fallback) > 0 {
				markets = fallback
				hasL5Fallback[id] = true
			}
		}
		usage := nflverseData.usageByPlayer[id]
		pos := positionByID[id]
		if pos == "" {
			pos = nflverseData.positionByID[id]
		}
		if len(markets) == 0 && usage == nil && pos == "" {
			continue
		}
		stats[id] = analyzer.PlayerStats{
			Position: pos,
			Games:    markets,
			Usage:    usage,
		}
	}

	return stats, hasGameLog, hasL5Fallback
}

func loadArchivedGameInjuries(
	ctx context.Context,
	store *db.Store,
	game db.Game,
) (map[string][]analyzer.Injury, map[string]string, []db.RosterPlayer, bool) {
	positions := make(map[string]string)
	if game.Kickoff.IsZero() {
		return nil, positions, nil, false
	}

	hasArchive := false
	summaryByTeam := make(map[string][]espn.Injury)
	sumKey := db.ArchiveSummaryKey(game.ID)
	if raw, ok := latestArchivedBodyBefore(ctx, store, sumKey, game.Kickoff); ok {
		hasArchive = true
		if injJSON, ok := extractSummaryInjuriesJSON(raw); ok {
			if teamInjuries, err := espn.ParseInjuries(injJSON); err == nil {
				for _, ti := range teamInjuries {
					if ti.Abbreviation != "" && len(ti.Injuries) > 0 {
						summaryByTeam[ti.Abbreviation] = ti.Injuries
					}
				}
			}
		}
	}

	rosterByTeam := make(map[string][]espn.Injury, 2)
	var rosterPlayers []db.RosterPlayer
	teamCandidates := []struct {
		abbr   string
		espnID string
	}{
		{abbr: game.AwayTeam, espnID: game.AwayESPNID},
		{abbr: game.HomeTeam, espnID: game.HomeESPNID},
	}
	for _, tc := range teamCandidates {
		if tc.abbr == "" {
			continue
		}
		var raw []byte
		var found bool
		if tc.espnID != "" {
			key := db.ArchiveRosterKey(tc.espnID)
			raw, found = latestArchivedBodyBefore(ctx, store, key, game.Kickoff)
		}
		if !found {
			key := db.ArchiveRosterKey(tc.abbr)
			raw, found = latestArchivedBodyBefore(ctx, store, key, game.Kickoff)
		}
		if !found {
			continue
		}
		hasArchive = true
		players, err := espn.ParseRosterJSON(raw)
		if err != nil {
			continue
		}
		for _, p := range players {
			if p.Position != "" {
				positions[p.ID] = p.Position
			}
			rosterPlayers = append(rosterPlayers, db.RosterPlayer{
				AthleteID:   p.ID,
				Team:        tc.abbr,
				ESPNTeamID:  tc.espnID,
				FullName:    p.FullName,
				NormName:    db.NormalizePlayerName(p.FullName),
				Position:    p.Position,
				HeadshotURL: p.HeadshotURL,
			})
			if strings.TrimSpace(p.InjuryStatus) == "" || !espn.IsKeyInjuryPosition(p.Position) {
				continue
			}
			rosterByTeam[tc.abbr] = append(rosterByTeam[tc.abbr], espn.Injury{
				AthleteID:   p.ID,
				FullName:    p.FullName,
				Position:    p.Position,
				HeadshotURL: p.HeadshotURL,
				Status:      p.InjuryStatus,
			})
		}
	}

	merged := analyzer.MergeInjuryFeeds(summaryByTeam, rosterByTeam)
	return merged, positions, rosterPlayers, hasArchive
}

func extractSummaryInjuriesJSON(raw []byte) (string, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return "", false
	}
	if trimmed[0] == '[' {
		return string(trimmed), true
	}
	var obj struct {
		Injuries json.RawMessage `json:"injuries"`
	}
	if err := json.Unmarshal(trimmed, &obj); err != nil {
		return "", false
	}
	if len(obj.Injuries) == 0 || string(obj.Injuries) == "null" {
		return "[]", true
	}
	return string(obj.Injuries), true
}

type decodedLinesSnapshot struct {
	events       []odds.Event
	linesByEvent map[string][]odds.Line
}

func buildMatchGames(games []db.Game) []odds.MatchGame {
	out := make([]odds.MatchGame, 0, len(games))
	for _, g := range games {
		out = append(out, odds.MatchGame{
			ID:       g.ID,
			HomeName: g.HomeName,
			AwayName: g.AwayName,
			Kickoff:  g.Kickoff,
		})
	}
	return out
}

func loadPreKickoffGameOdds(
	ctx context.Context,
	store *db.Store,
	game db.Game,
	matchGames []odds.MatchGame,
	rosterPlayers []db.RosterPlayer,
	priorNFLverseByTeam map[string][]db.RosterPlayer,
	linesCache map[string]decodedLinesSnapshot,
) ([]db.Odds, bool) {
	if game.Kickoff.IsZero() {
		return nil, false
	}

	var rows []db.Odds
	hasArchived := false

	if snap, ok := loadPreKickoffLinesSnapshot(ctx, store, game.Kickoff, linesCache); ok {
		hasArchived = true
		eventID := resolveOddsEventID(game, snap, matchGames)
		if eventID != "" {
			for _, line := range snap.linesByEvent[eventID] {
				if row, ok := mapLineToDBOdds(game, line); ok {
					rows = append(rows, row)
				}
			}
		}
	}

	if rawProps, ok := latestArchivedBodyBefore(ctx, store, db.ArchiveOddsPropsKey(game.ID), game.Kickoff); ok {
		hasArchived = true
		if props, err := odds.ParsePropsJSON(rawProps); err == nil && len(props) > 0 {
			rows = append(rows, mapPropsToDBOdds(game, props, rosterPlayers, priorNFLverseByTeam)...)
		}
	}

	return rows, hasArchived
}

func loadPreKickoffLinesSnapshot(
	ctx context.Context,
	store *db.Store,
	kickoff time.Time,
	cache map[string]decodedLinesSnapshot,
) (decodedLinesSnapshot, bool) {
	resp, ok, err := store.LatestAPIResponseBefore(ctx, db.ArchiveOddsLinesKey, beforeKickoff(kickoff))
	if err != nil || !ok || !resp.FetchedAt.Before(kickoff) {
		return decodedLinesSnapshot{}, false
	}
	if cached, hit := cache[resp.BodySHA256]; hit {
		return cached, true
	}

	var snap decodedLinesSnapshot
	body, err := store.APIResponseBody(ctx, resp.BodySHA256)
	if err == nil {
		if events, lines, parseErr := odds.ParseLinesSnapshotJSON(body); parseErr == nil {
			byEvent := make(map[string][]odds.Line)
			for _, line := range lines {
				byEvent[line.EventID] = append(byEvent[line.EventID], line)
			}
			snap = decodedLinesSnapshot{
				events:       events,
				linesByEvent: byEvent,
			}
		}
	}
	cache[resp.BodySHA256] = snap
	return snap, true
}

func resolveOddsEventID(game db.Game, snap decodedLinesSnapshot, matchGames []odds.MatchGame) string {
	if game.OddsEventID != nil && *game.OddsEventID != "" && len(snap.linesByEvent[*game.OddsEventID]) > 0 {
		return *game.OddsEventID
	}
	if len(snap.events) == 0 {
		return ""
	}
	matches, _ := odds.MatchEvents(snap.events, matchGames)
	return matches[game.ID]
}

func mapLineToDBOdds(game db.Game, line odds.Line) (db.Odds, bool) {
	team := ""
	switch line.MarketType {
	case odds.MarketML, odds.MarketSpread:
		switch line.Team {
		case game.HomeName, game.HomeTeam:
			team = game.HomeTeam
		case game.AwayName, game.AwayTeam:
			team = game.AwayTeam
		default:
			return db.Odds{}, false
		}
	case odds.MarketTotal:
		if line.Team != "" {
			return db.Odds{}, false
		}
	default:
		return db.Odds{}, false
	}

	return db.Odds{
		GameID:     game.ID,
		Book:       line.Book,
		MarketType: line.MarketType,
		Team:       team,
		Side:       line.Side,
		Line:       line.Line,
		Price:      line.Price,
	}, true
}

func mapPropsToDBOdds(
	game db.Game,
	props []odds.Prop,
	rosterPlayers []db.RosterPlayer,
	priorNFLverseByTeam map[string][]db.RosterPlayer,
) []db.Odds {
	combined := combinePreKickoffRoster(game, rosterPlayers, priorNFLverseByTeam)
	resolver := db.NewBatchPlayerResolver(combined, nil)

	type oddsKey struct {
		book       string
		marketType string
		team       string
		playerName string
		side       string
	}
	type mappedOdds struct {
		row  db.Odds
		pref db.OddsCandidate
	}

	mapped := make([]mappedOdds, 0, len(props))
	indexByKey := make(map[oddsKey]int, len(props))

	for _, prop := range props {
		row := db.Odds{
			GameID:     game.ID,
			Book:       prop.Book,
			MarketType: prop.MarketType,
			Side:       prop.Side,
			Line:       prop.Line,
			Price:      prop.Price,
			Link:       db.OptionalString(prop.Link),
			Sid:        db.OptionalString(prop.Sid),
			EventSid:   db.OptionalString(prop.EventSid),
		}
		linked := false
		if prop.MarketType == odds.MarketTeamTotal {
			switch prop.Team {
			case game.HomeName, game.HomeTeam:
				row.Team = game.HomeTeam
			case game.AwayName, game.AwayTeam:
				row.Team = game.AwayTeam
			default:
				continue
			}
		} else {
			resolved := resolver.ResolveGamePlayer(game.ID, prop.PlayerName, "", game.HomeTeam, game.AwayTeam)
			row.PlayerName = resolved.Name
			row.Team = resolved.Team
			linked = resolved.Linked
		}

		cand := mappedOdds{
			row: row,
			pref: db.OddsCandidate{
				Linked:   linked,
				Richness: db.OddsRichness(row),
			},
		}
		key := oddsKey{
			book:       row.Book,
			marketType: row.MarketType,
			team:       row.Team,
			playerName: row.PlayerName,
			side:       row.Side,
		}
		if idx, exists := indexByKey[key]; exists {
			if db.PreferOddsCandidate(cand.pref, mapped[idx].pref) {
				mapped[idx] = cand
			}
			continue
		}
		indexByKey[key] = len(mapped)
		mapped = append(mapped, cand)
	}

	out := make([]db.Odds, len(mapped))
	for i, m := range mapped {
		out[i] = m.row
	}
	return out
}

func combinePreKickoffRoster(
	game db.Game,
	rosterPlayers []db.RosterPlayer,
	priorNFLverseByTeam map[string][]db.RosterPlayer,
) []db.RosterPlayer {
	combined := make([]db.RosterPlayer, 0, len(rosterPlayers))
	seen := make(map[string]bool, len(rosterPlayers))
	for _, rp := range rosterPlayers {
		combined = append(combined, rp)
		if rp.AthleteID != "" {
			seen[rp.AthleteID] = true
		}
	}
	for _, team := range []string{game.AwayTeam, game.HomeTeam} {
		if team == "" {
			continue
		}
		for _, rp := range priorNFLverseByTeam[team] {
			if rp.AthleteID != "" && seen[rp.AthleteID] {
				continue
			}
			if rp.AthleteID != "" {
				seen[rp.AthleteID] = true
			}
			combined = append(combined, rp)
		}
	}
	return combined
}

func beforeKickoff(kickoff time.Time) time.Time {
	if kickoff.Nanosecond() > 0 {
		return kickoff.Truncate(time.Second)
	}
	return kickoff.Add(-time.Second)
}

func latestArchivedBodyBefore(
	ctx context.Context,
	store *db.Store,
	key string,
	kickoff time.Time,
) ([]byte, bool) {
	resp, ok, err := store.LatestAPIResponseBefore(ctx, key, beforeKickoff(kickoff))
	if err != nil || !ok || !resp.FetchedAt.Before(kickoff) {
		return nil, false
	}
	body, err := store.APIResponseBody(ctx, resp.BodySHA256)
	if err != nil {
		return nil, false
	}
	return body, true
}
