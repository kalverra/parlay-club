package analyzer

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/nflverse"
)

// NfeloSource is the sync_state row that records the last nfelo fetch.
const NfeloSource = "nfelo:projections"

// NfeloFreshFor is how long a successful nfelo fetch may shape a warning. The
// file changes about every two days, and a stale model is worse than none.
const NfeloFreshFor = 72 * time.Hour

// NFLverseFreshFor is how long a successful nflverse sync may shape analyzer
// rules, grades, and board/research signals.
const NFLverseFreshFor = 7 * 24 * time.Hour

// NFLverseSource returns the sync_state source key for a season's nflverse sync.
func NFLverseSource(season int) string {
	return fmt.Sprintf("nflverse:season:%d", season)
}

func nflverseSeasonTypes(seasonType int) []int {
	if seasonType == nflverse.SeasonTypePostseason {
		return []int{nflverse.SeasonTypeRegular, nflverse.SeasonTypePostseason}
	}
	return []int{seasonType}
}

// LoadFreshTeamProfiles returns season-to-date and L4 team EPA and pace
// profiles when nflverse:season:{year} is present and under NFLverseFreshFor.
func LoadFreshTeamProfiles(
	ctx context.Context,
	store *db.Store,
	now time.Time,
	week db.Week,
) (map[string]*TeamDefenseProfile, bool) {
	state, err := store.SyncState(ctx, NFLverseSource(week.Season))
	if err != nil || now.Sub(state.FetchedAt) > NFLverseFreshFor {
		return nil, false
	}

	teamWeeks, err := store.NFLverseTeamWeeksForSeasonTypes(
		ctx, week.Season, nflverseSeasonTypes(week.SeasonType)...,
	)
	if err != nil || len(teamWeeks) == 0 {
		return nil, true
	}
	return BuildTeamDefenseProfiles(teamWeeks), true
}

// LoadPlayerUsageForIDs loads nflverse player usage for athleteIDs, merging
// regular-season and postseason rows during the postseason.
func LoadPlayerUsageForIDs(
	ctx context.Context,
	store *db.Store,
	athleteIDs []string,
	season, seasonType int,
) map[string]*PlayerUsage {
	if len(athleteIDs) == 0 {
		return nil
	}
	byPlayer, err := store.NFLversePlayerWeeksForPlayersTypes(
		ctx, athleteIDs, season, nflverseSeasonTypes(seasonType)...,
	)
	if err != nil {
		return nil
	}

	usageByPlayer := make(map[string]*PlayerUsage)
	for _, id := range athleteIDs {
		if usage := BuildPlayerUsage(byPlayer[id]); usage != nil {
			usageByPlayer[id] = usage
		}
	}
	return usageByPlayer
}

// BuildTeamDefenseProfiles aggregates weekly nflverse team efficiency rows into
// season-to-date and last-4-week profiles with 1..32 league rankings.
func BuildTeamDefenseProfiles(rows []db.NFLverseTeamWeek) map[string]*TeamDefenseProfile {
	byTeam := make(map[string][]db.NFLverseTeamWeek)
	for _, row := range rows {
		byTeam[row.Team] = append(byTeam[row.Team], row)
	}
	if len(byTeam) == 0 {
		return nil
	}

	profiles := make(map[string]*TeamDefenseProfile, len(byTeam))
	teams := make([]string, 0, len(byTeam))
	for team, weeks := range byTeam {
		teams = append(teams, team)
		prof := &TeamDefenseProfile{}
		n := float64(len(weeks))
		for _, w := range weeks {
			prof.OffPassEPA += w.OffPassEPA
			prof.OffRushEPA += w.OffRushEPA
			prof.DefPassEPAAllowed += w.DefPassEPAAllowed
			prof.DefRushEPAAllowed += w.DefRushEPAAllowed
			prof.PlaysPerGame += w.PlaysPerGame
			prof.PassRate += w.PassRate
		}
		prof.OffPassEPA /= n
		prof.OffRushEPA /= n
		prof.DefPassEPAAllowed /= n
		prof.DefRushEPAAllowed /= n
		prof.PlaysPerGame /= n
		prof.PassRate /= n

		l4Start := max(0, len(weeks)-4)
		l4 := weeks[l4Start:]
		l4N := float64(len(l4))
		for _, w := range l4 {
			prof.L4OffPassEPA += w.OffPassEPA
			prof.L4OffRushEPA += w.OffRushEPA
			prof.L4DefPassEPAAllowed += w.DefPassEPAAllowed
			prof.L4DefRushEPAAllowed += w.DefRushEPAAllowed
			prof.L4PlaysPerGame += w.PlaysPerGame
			prof.L4PassRate += w.PassRate
		}
		prof.L4OffPassEPA /= l4N
		prof.L4OffRushEPA /= l4N
		prof.L4DefPassEPAAllowed /= l4N
		prof.L4DefRushEPAAllowed /= l4N
		prof.L4PlaysPerGame /= l4N
		prof.L4PassRate /= l4N

		profiles[team] = prof
	}

	rankTeams(teams, func(t string) float64 { return profiles[t].OffPassEPA }, true, func(t string, r int) {
		profiles[t].OffPassEPARank = r
	})
	rankTeams(teams, func(t string) float64 { return profiles[t].OffRushEPA }, true, func(t string, r int) {
		profiles[t].OffRushEPARank = r
	})
	rankTeams(teams, func(t string) float64 { return profiles[t].DefPassEPAAllowed }, false, func(t string, r int) {
		profiles[t].DefPassEPAAllowedRank = r
	})
	rankTeams(teams, func(t string) float64 { return profiles[t].DefRushEPAAllowed }, false, func(t string, r int) {
		profiles[t].DefRushEPAAllowedRank = r
	})
	rankTeams(teams, func(t string) float64 { return profiles[t].PlaysPerGame }, true, func(t string, r int) {
		profiles[t].PlaysPerGameRank = r
	})
	rankTeams(teams, func(t string) float64 { return profiles[t].PassRate }, true, func(t string, r int) {
		profiles[t].PassRateRank = r
	})

	return profiles
}

func rankTeams(
	teams []string,
	metric func(string) float64,
	desc bool,
	assign func(string, int),
) {
	ordered := slices.Clone(teams)
	slices.SortFunc(ordered, func(a, b string) int {
		va, vb := metric(a), metric(b)
		if va != vb {
			if desc {
				return cmp.Compare(vb, va)
			}
			return cmp.Compare(va, vb)
		}
		return cmp.Compare(a, b)
	})
	for i, team := range ordered {
		assign(team, i+1)
	}
}

// BuildPlayerUsage converts most-recent-first weekly nflverse rows into a
// PlayerUsage with precomputed rolling averages. Returns nil when fewer than
// 3 weeks exist.
func BuildPlayerUsage(rows []db.NFLversePlayerWeek) *PlayerUsage {
	if len(rows) < 3 {
		return nil
	}
	weeks := make([]PlayerWeekUsage, 0, len(rows))
	var seasonSnap, seasonTarget float64
	var seasonCarries int
	for _, row := range rows {
		weeks = append(weeks, PlayerWeekUsage{
			WeekNum:       row.WeekNum,
			OffensePct:    row.OffensePct,
			TargetShare:   row.TargetShare,
			AirYardsShare: row.AirYardsShare,
			WOPR:          row.WOPR,
			Carries:       row.Carries,
			Targets:       row.Targets,
			TotalEPA:      row.TotalEPA,
			RZOpps:        row.RZOpps,
		})
		seasonSnap += row.OffensePct
		seasonTarget += row.TargetShare
		seasonCarries += row.Carries
	}
	l3Snap, l3Target, l3WOPR, _ := SummarizeRecentUsage(rows)
	n := float64(len(rows))
	return &PlayerUsage{
		Weeks:             weeks,
		Last3SnapAvg:      l3Snap,
		SeasonSnapAvg:     seasonSnap / n,
		Last3TargetShare:  l3Target,
		SeasonTargetShare: seasonTarget / n,
		Last3WOPR:         l3WOPR,
		AvgCarries:        float64(seasonCarries) / n,
	}
}

// SummarizeRecentUsage computes up-to-last-3-week average snap percentage,
// target share, and WOPR from most-recent-first nflverse player week rows.
// Returns ok=false when rows is empty.
func SummarizeRecentUsage(rows []db.NFLversePlayerWeek) (snapPct, targetShare, wopr float64, ok bool) {
	if len(rows) == 0 {
		return 0, 0, 0, false
	}
	l3Count := min(3, len(rows))
	var snapSum, targetSum, woprSum float64
	for _, w := range rows[:l3Count] {
		snapSum += w.OffensePct
		targetSum += w.TargetShare
		woprSum += w.WOPR
	}
	n := float64(l3Count)
	return snapSum / n, targetSum / n, woprSum / n, true
}

// GameFromDB converts a stored db.Game, optional nfelo projection, and team EPA
// profiles into an analyzer.Game. nfelo data is attached only for scheduled
// games.
func GameFromDB(game db.Game, proj *db.NfeloGame, teamEPA map[string]*TeamDefenseProfile) Game {
	converted := Game{
		ID:      game.ID,
		Home:    game.HomeTeam,
		Away:    game.AwayTeam,
		Indoor:  game.Indoor,
		HomeEPA: teamEPA[game.HomeTeam],
		AwayEPA: teamEPA[game.AwayTeam],
	}
	if proj != nil && game.Status == db.GameScheduled {
		converted.Nfelo = &Nfelo{
			HomeSpread:  proj.HomeSpread,
			HomeWinProb: proj.HomeWinProb,
			HomeQB:      proj.HomeQB,
			AwayQB:      proj.AwayQB,
			HomeQBAdj:   proj.HomeQBAdj,
			AwayQBAdj:   proj.AwayQBAdj,
		}
	}
	if game.WindMPH != nil {
		weather := Weather{WindMPH: *game.WindMPH}
		if game.WindGustMPH != nil {
			weather.GustMPH = *game.WindGustMPH
		}
		if game.PrecipIN != nil {
			weather.PrecipIn = *game.PrecipIN
		}
		if game.TempF != nil {
			weather.TempF = *game.TempF
		}
		converted.Weather = &weather
	}
	return converted
}

// LegFromDB converts a stored db.Leg and display owner name into an analyzer.Leg.
func LegFromDB(leg db.Leg, owner string) Leg {
	converted := Leg{
		ID:     leg.ID,
		Owner:  owner,
		Market: leg.MarketType,
		Line:   leg.Line,
		Odds:   leg.Odds,
	}
	if leg.GameID != nil {
		converted.GameID = *leg.GameID
	}
	if leg.Team != nil {
		converted.Team = *leg.Team
	}
	if leg.PlayerID != nil {
		converted.PlayerID = *leg.PlayerID
	}
	if leg.PlayerName != nil {
		converted.PlayerName = *leg.PlayerName
	}
	if leg.Side != nil {
		converted.Side = *leg.Side
	}
	return converted
}

// PlayerResearchSignals holds nflverse usage and opponent matchup rank for a
// player research view. Zero values indicate missing or stale data.
type PlayerResearchSignals struct {
	HasUsage    bool
	SnapPct     float64
	TargetShare float64
	WOPR        float64
	Opponent    string
	OppDefSplit string
	OppDefRank  int
}

// LoadSlipSignals loads fresh nfelo projections, fresh nflverse team EPA/pace
// profiles, nflverse player usage, and player_gamelogs for a slip's games and
// legs. Missing or stale sources fail soft.
func LoadSlipSignals(
	ctx context.Context,
	store *db.Store,
	now time.Time,
	weekID int64,
	games []db.Game,
	legs []db.Leg,
) (map[string]Game, map[string]PlayerStats) {
	nfelo := loadFreshNfelo(ctx, store, now, games)

	var teamEPA map[string]*TeamDefenseProfile
	var usageByPlayer map[string]*PlayerUsage
	athleteIDs := distinctLegPlayerIDs(legs)

	if week, err := store.WeekByID(ctx, weekID); err == nil {
		if profiles, fresh := LoadFreshTeamProfiles(ctx, store, now, week); fresh {
			teamEPA = profiles
			usageByPlayer = LoadPlayerUsageForIDs(ctx, store, athleteIDs, week.Season, week.SeasonType)
		}
	}

	gamesByID := make(map[string]Game, len(games))
	for _, game := range games {
		var proj *db.NfeloGame
		if p, ok := nfelo[game.ID]; ok {
			proj = &p
		}
		gamesByID[game.ID] = GameFromDB(game, proj, teamEPA)
	}

	if len(athleteIDs) == 0 {
		return gamesByID, nil
	}
	logs, err := store.PlayerGameLogs(ctx, athleteIDs)
	if err != nil {
		logs = nil
	}

	stats := make(map[string]PlayerStats, len(athleteIDs))
	for _, id := range athleteIDs {
		var markets map[string][]float64
		if log, ok := logs[id]; ok {
			_ = json.Unmarshal([]byte(log.StatsJSON), &markets)
		}
		usage := usageByPlayer[id]
		if len(markets) == 0 && usage == nil {
			continue
		}
		stats[id] = PlayerStats{
			Games: markets,
			Usage: usage,
		}
	}
	return gamesByID, stats
}

// LoadGameSignals loads fresh nflverse team defense profiles and player usage
// for a single week's pick board. Missing or stale data fails soft with nil maps.
func LoadGameSignals(
	ctx context.Context,
	store *db.Store,
	now time.Time,
	weekID int64,
	playerIDs []string,
) (map[string]*TeamDefenseProfile, map[string]*PlayerUsage) {
	week, err := store.WeekByID(ctx, weekID)
	if err != nil {
		return nil, nil
	}
	teamEPA, fresh := LoadFreshTeamProfiles(ctx, store, now, week)
	if !fresh {
		return nil, nil
	}
	playerUsage := LoadPlayerUsageForIDs(ctx, store, playerIDs, week.Season, week.SeasonType)
	return teamEPA, playerUsage
}

// LoadPlayerResearchSignals loads L3 nflverse usage and opponent defense rank
// for the latest week when nflverse data is fresh.
func LoadPlayerResearchSignals(
	ctx context.Context,
	store *db.Store,
	now time.Time,
	athleteID, team, position, activeMarket string,
) PlayerResearchSignals {
	week, err := store.LatestWeek(ctx)
	if err != nil {
		return PlayerResearchSignals{}
	}
	teamEPA, fresh := LoadFreshTeamProfiles(ctx, store, now, week)
	if !fresh {
		return PlayerResearchSignals{}
	}

	var res PlayerResearchSignals
	if pWeeks, err := store.NFLversePlayerWeeksForPlayerTypes(
		ctx, athleteID, week.Season, nflverseSeasonTypes(week.SeasonType)...,
	); err == nil {
		if snap, target, wopr, ok := SummarizeRecentUsage(pWeeks); ok {
			res.HasUsage = true
			res.SnapPct = snap
			res.TargetShare = target
			res.WOPR = wopr
		}
	}

	if team == "" || len(teamEPA) == 0 {
		return res
	}
	weekGames, err := store.GamesForWeek(ctx, week.ID)
	if err != nil {
		return res
	}
	opp := ""
	for _, g := range weekGames {
		switch team {
		case g.HomeTeam:
			opp = g.AwayTeam
		case g.AwayTeam:
			opp = g.HomeTeam
		}
		if opp != "" {
			break
		}
	}
	oppProf := teamEPA[opp]
	if oppProf == nil {
		return res
	}
	market := activeMarket
	if market == "" {
		if strings.EqualFold(position, "RB") {
			market = "RUSH_YDS"
		} else {
			market = "PASS_YDS"
		}
	}
	split, rank, ok := MatchupDefenseRank(market, oppProf)
	if !ok {
		return res
	}
	res.Opponent = opp
	res.OppDefSplit = split
	res.OppDefRank = rank
	return res
}

func loadFreshNfelo(ctx context.Context, store *db.Store, now time.Time, games []db.Game) map[string]db.NfeloGame {
	if len(games) == 0 {
		return nil
	}
	state, err := store.SyncState(ctx, NfeloSource)
	if err != nil || now.Sub(state.FetchedAt) > NfeloFreshFor {
		return nil
	}
	gameIDs := make([]string, 0, len(games))
	for _, game := range games {
		gameIDs = append(gameIDs, game.ID)
	}
	rows, err := store.NfeloForGames(ctx, gameIDs)
	if err != nil {
		return nil
	}
	return rows
}

func distinctLegPlayerIDs(legs []db.Leg) []string {
	seen := make(map[string]struct{}, len(legs))
	var athleteIDs []string
	for _, leg := range legs {
		if leg.PlayerID == nil || *leg.PlayerID == "" {
			continue
		}
		id := *leg.PlayerID
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		athleteIDs = append(athleteIDs, id)
	}
	return athleteIDs
}
