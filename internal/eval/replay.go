package eval

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
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

		athleteIDs := distinctAthleteIDs(rawLegs)
		teamEPA, usageByPlayer := loadPriorNFLverse(ctx, store, week, athleteIDs)

		nfeloByGame, err := store.NfeloForGames(ctx, gameIDs)
		if err != nil {
			nfeloByGame = nil
		}

		gamesMap := make(map[string]analyzer.Game, len(weekGames))
		positionByID := make(map[string]string)
		gameHasInjuries := make(map[string]bool, len(weekGames))
		gameHasArchivedOdds := make(map[string]bool, len(weekGames))

		for _, g := range weekGames {
			var proj *db.NfeloGame
			if p, ok := nfeloByGame[g.ID]; ok {
				proj = &p
			}
			// Reconstruct pre-kickoff game state as scheduled so GameFromDB attaches nfelo.
			preKickoffGame := g
			preKickoffGame.Status = db.GameScheduled
			converted := analyzer.GameFromDB(preKickoffGame, proj, teamEPA)

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

			gameHasArchivedOdds[g.ID] = hasArchivedOddsBefore(ctx, store, g.ID, g.Kickoff)
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

		for _, s := range slips {
			for _, rl := range s.Legs {
				dataset.Coverage.TotalLegs++
				if rl.Leg.PlayerID != "" && playerHasGameLog[rl.Leg.PlayerID] {
					dataset.Coverage.WithGameLogs++
				}
				if rl.Leg.GameID != "" {
					if gameHasInjuries[rl.Leg.GameID] {
						dataset.Coverage.WithInjuries++
					}
					if g, ok := gamesMap[rl.Leg.GameID]; ok {
						if g.Nfelo != nil {
							dataset.Coverage.WithNfelo++
						}
						hasNFLV := g.HomeEPA != nil || g.AwayEPA != nil
						if !hasNFLV && rl.Leg.PlayerID != "" {
							if ps, ok := statsMap[rl.Leg.PlayerID]; ok && ps.Usage != nil {
								hasNFLV = true
							}
						}
						if hasNFLV {
							dataset.Coverage.WithNFLVerse++
						}
					}
					if gameHasArchivedOdds[rl.Leg.GameID] || rl.DecimalOdds > 1 {
						dataset.Coverage.WithOdds++
					}
				} else if rl.DecimalOdds > 1 {
					dataset.Coverage.WithOdds++
				}
			}
		}

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
	slipIDs, err := store.SlipIDsForWeek(ctx, weekID)
	if err != nil {
		return nil, nil, err
	}

	var slips []ReplayedSlip
	var allLegs []db.Leg
	for _, id := range slipIDs {
		slip, err := store.SlipByID(ctx, id)
		if err != nil {
			return nil, nil, err
		}
		if groupID != 0 && slip.GroupID != groupID {
			continue
		}
		if slip.Status != db.SlipGraded {
			continue
		}
		dbLegs, err := store.LegsForSlip(ctx, slip.ID)
		if err != nil {
			return nil, nil, err
		}

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

func distinctAthleteIDs(legs []db.Leg) []string {
	seen := make(map[string]struct{}, len(legs))
	var ids []string
	for _, l := range legs {
		if l.PlayerID == nil || *l.PlayerID == "" {
			continue
		}
		id := *l.PlayerID
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func loadPriorNFLverse(
	ctx context.Context,
	store *db.Store,
	week db.Week,
	athleteIDs []string,
) (map[string]*analyzer.TeamDefenseProfile, map[string]*analyzer.PlayerUsage) {
	seasonTypes := []int{week.SeasonType}
	if week.SeasonType == nflverse.SeasonTypePostseason {
		seasonTypes = []int{nflverse.SeasonTypeRegular, nflverse.SeasonTypePostseason}
	}

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

	athleteIDs := distinctAthleteIDs(legs)
	stats := make(map[string]analyzer.PlayerStats, len(athleteIDs))
	hasGameLog := make(map[string]bool, len(athleteIDs))

	for _, id := range athleteIDs {
		var markets map[string][]float64
		if kickoff, ok := kickoffByAthlete[id]; ok {
			key := "espn:gamelog:" + id
			if raw, found := latestArchivedBodyBefore(ctx, store, key, key, kickoff); found {
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
	sumKey := "espn:summary:" + game.ID
	if raw, ok := latestArchivedBodyBefore(ctx, store, sumKey, sumKey, game.Kickoff); ok {
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
			key := "espn:roster:" + tc.espnID
			raw, found = latestArchivedBodyBefore(ctx, store, key, key, game.Kickoff)
		}
		if !found {
			key := "espn:roster:" + tc.abbr
			raw, found = latestArchivedBodyBefore(ctx, store, key, key, game.Kickoff)
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

	merged := make(map[string][]analyzer.Injury)
	for _, team := range []string{game.AwayTeam, game.HomeTeam} {
		if team == "" {
			continue
		}
		combined := espn.MergeTeamInjuries(summaryByTeam[team], rosterByTeam[team])
		if len(combined) == 0 {
			continue
		}
		converted := make([]analyzer.Injury, len(combined))
		for i, inj := range combined {
			converted[i] = analyzer.Injury{
				AthleteID: inj.AthleteID,
				FullName:  inj.FullName,
				Position:  inj.Position,
				Status:    inj.Status,
			}
		}
		merged[team] = converted
	}

	if len(merged) == 0 {
		return nil, positions, hasArchive
	}
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

func hasArchivedOddsBefore(ctx context.Context, store *db.Store, gameID string, kickoff time.Time) bool {
	if kickoff.IsZero() {
		return false
	}
	propsKey := "odds:props:" + gameID
	if raw, ok := latestArchivedBodyBefore(ctx, store, propsKey, propsKey, kickoff); ok {
		if _, err := odds.ParsePropsJSON(raw); err == nil {
			return true
		}
	}
	if raw, ok := latestArchivedBodyBefore(ctx, store, "odds:lines", "odds:lines", kickoff); ok {
		if _, err := odds.ParseLinesJSON(raw); err == nil {
			return true
		}
	}
	return false
}

func latestArchivedBodyBefore(
	ctx context.Context,
	store *db.Store,
	keyPrefix string,
	exactKey string,
	kickoff time.Time,
) ([]byte, bool) {
	responses, err := store.ListAPIResponses(ctx, db.APIResponseFilter{
		KeyPrefix: keyPrefix,
		Until:     kickoff,
	})
	if err != nil || len(responses) == 0 {
		return nil, false
	}
	for _, r := range slices.Backward(responses) {
		if exactKey != "" && r.Key != exactKey {
			continue
		}
		if !r.FetchedAt.Before(kickoff) {
			continue
		}
		body, err := store.APIResponseBody(ctx, r.BodySHA256)
		if err != nil {
			continue
		}
		return body, true
	}
	return nil, false
}
