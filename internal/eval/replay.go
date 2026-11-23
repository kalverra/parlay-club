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
		teamEPA, usageByPlayer := loadPriorNFLverse(ctx, store, week, athleteIDs)

		nfeloByGame, err := store.NfeloForGames(ctx, gameIDs)
		if err != nil {
			nfeloByGame = nil
		}

		gamesMap := make(map[string]analyzer.Game, len(weekGames))
		positionByID := make(map[string]string)
		gameHasInjuries := make(map[string]bool, len(weekGames))
		gameHasArchivedOdds := make(map[string]bool, len(weekGames))
		referencedGames, gameNeedsOddsCheck := referencedGamesFromSlips(slips)
		linesByKickoff := make(map[int64]bool)

		for _, g := range weekGames {
			var proj *db.NfeloGame
			if p, ok := nfeloByGame[g.ID]; ok {
				proj = &p
			}
			// Reconstruct pre-kickoff game state as scheduled so GameFromDB attaches nfelo.
			preKickoffGame := g
			preKickoffGame.Status = db.GameScheduled
			converted := analyzer.GameFromDB(preKickoffGame, proj, teamEPA)

			if referencedGames[g.ID] {
				injuries, rosterPositions, hasInj := loadArchivedGameInjuries(ctx, store, g)
				if len(injuries) > 0 {
					converted.Injuries = injuries
				}
				gameHasInjuries[g.ID] = hasInj
				for id, pos := range rosterPositions {
					if _, exists := positionByID[id]; !exists {
						positionByID[id] = pos
					}
				}

				if gameNeedsOddsCheck[g.ID] {
					gameHasArchivedOdds[g.ID] = hasArchivedOddsBefore(ctx, store, g.ID, g.Kickoff, linesByKickoff)
				}
			}
			gamesMap[g.ID] = converted
		}

		statsMap, playerHasGameLog := loadArchivedPlayerStats(
			ctx,
			store,
			rawLegs,
			gameByDBID,
			usageByPlayer,
			positionByID,
		)

		accumulateWeekCoverage(
			&dataset.Coverage,
			slips,
			gamesMap,
			statsMap,
			playerHasGameLog,
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
	gameHasInjuries map[string]bool,
	gameHasArchivedOdds map[string]bool,
) {
	for _, s := range slips {
		for _, rl := range s.Legs {
			cov.TotalLegs++
			if rl.Leg.PlayerID != "" && playerHasGameLog[rl.Leg.PlayerID] {
				cov.WithGameLogs++
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
			if !isSettledResult(dl.Result) {
				continue
			}
			dec := 0.0
			if dl.Odds != 0 {
				dec = parlay.AmericanToDecimal(dl.Odds)
			}
			if dl.Result.Counts() {
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

func loadPriorNFLverse(
	ctx context.Context,
	store *db.Store,
	week db.Week,
	athleteIDs []string,
) (map[string]*analyzer.TeamDefenseProfile, map[string]*analyzer.PlayerUsage) {
	seasonTypes := analyzer.NFLverseSeasonTypes(week.SeasonType)

	var teamEPA map[string]*analyzer.TeamDefenseProfile
	if rows, err := store.NFLverseTeamWeeksForSeasonTypes(ctx, week.Season, seasonTypes...); err == nil {
		filtered := make([]db.NFLverseTeamWeek, 0, len(rows))
		for _, r := range rows {
			if isPriorNFLverseWeek(r.SeasonType, r.WeekNum, week) {
				filtered = append(filtered, r)
			}
		}
		teamEPA = analyzer.BuildTeamDefenseProfiles(filtered)
	}

	var usageByPlayer map[string]*analyzer.PlayerUsage
	if len(athleteIDs) > 0 {
		if byPlayer, err := store.NFLversePlayerWeeksForPlayersTypes(
			ctx,
			athleteIDs,
			week.Season,
			seasonTypes...); err == nil {
			usageByPlayer = make(map[string]*analyzer.PlayerUsage, len(athleteIDs))
			for _, id := range athleteIDs {
				rows := byPlayer[id]
				filtered := make([]db.NFLversePlayerWeek, 0, len(rows))
				for _, r := range rows {
					if isPriorNFLverseWeek(r.SeasonType, r.WeekNum, week) {
						filtered = append(filtered, r)
					}
				}
				if u := analyzer.BuildPlayerUsage(filtered); u != nil {
					usageByPlayer[id] = u
				}
			}
		}
	}

	return teamEPA, usageByPlayer
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
	usageByPlayer map[string]*analyzer.PlayerUsage,
	positionByID map[string]string,
) (map[string]analyzer.PlayerStats, map[string]bool) {
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
		usage := usageByPlayer[id]
		pos := positionByID[id]
		if len(markets) == 0 && usage == nil && pos == "" {
			continue
		}
		stats[id] = analyzer.PlayerStats{
			Position: pos,
			Games:    markets,
			Usage:    usage,
		}
	}

	return stats, hasGameLog
}

func loadArchivedGameInjuries(
	ctx context.Context,
	store *db.Store,
	game db.Game,
) (map[string][]analyzer.Injury, map[string]string, bool) {
	positions := make(map[string]string)
	if game.Kickoff.IsZero() {
		return nil, positions, false
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
	return merged, positions, hasArchive
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

func hasArchivedOddsBefore(
	ctx context.Context,
	store *db.Store,
	gameID string,
	kickoff time.Time,
	linesByKickoff map[int64]bool,
) bool {
	if kickoff.IsZero() {
		return false
	}
	if hasArchivedResponseBefore(ctx, store, db.ArchiveOddsPropsKey(gameID), kickoff) {
		return true
	}
	if linesByKickoff != nil {
		key := kickoff.UTC().UnixNano()
		if cached, ok := linesByKickoff[key]; ok {
			return cached
		}
		found := hasArchivedResponseBefore(ctx, store, db.ArchiveOddsLinesKey, kickoff)
		linesByKickoff[key] = found
		return found
	}
	return hasArchivedResponseBefore(ctx, store, db.ArchiveOddsLinesKey, kickoff)
}

func hasArchivedResponseBefore(ctx context.Context, store *db.Store, key string, kickoff time.Time) bool {
	resp, ok, err := store.LatestAPIResponseBefore(ctx, key, beforeKickoff(kickoff))
	return err == nil && ok && resp.FetchedAt.Before(kickoff)
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
