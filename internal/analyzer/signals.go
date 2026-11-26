package analyzer

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/espn"
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

// NFLverseSeasonTypes returns the nflverse season_type values to query for a
// week's seasonType, including regular-season rows during the postseason.
func NFLverseSeasonTypes(seasonType int) []int {
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
		ctx, week.Season, NFLverseSeasonTypes(week.SeasonType)...,
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
		ctx, athleteIDs, season, NFLverseSeasonTypes(seasonType)...,
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
		prof.DefOverallEPAAllowed = prof.DefPassEPAAllowed + prof.DefRushEPAAllowed
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
	rankTeams(teams, func(t string) float64 { return profiles[t].DefOverallEPAAllowed }, false, func(t string, r int) {
		profiles[t].DefOverallEPARank = r
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

// EWMAHalfLifeWeeks is the default half-life in games for exponential moving
// average usage and opportunity features.
const EWMAHalfLifeWeeks = 3.0

// ComputeEWMA computes the normalized (adjust=True) exponentially weighted
// moving average of values ordered most recent first (values[0] has weight 1.0,
// values[i] has weight 0.5^(i/halfLife)). Returns 0 when values is empty.
func ComputeEWMA(values []float64, halfLife float64) float64 {
	if len(values) == 0 {
		return 0
	}
	if halfLife <= 0 {
		halfLife = EWMAHalfLifeWeeks
	}
	lambda := math.Pow(0.5, 1.0/halfLife)
	weight := 1.0
	var weightedSum, weightTotal float64
	for _, v := range values {
		weightedSum += weight * v
		weightTotal += weight
		weight *= lambda
	}
	if weightTotal == 0 {
		return 0
	}
	return weightedSum / weightTotal
}

// BuildPlayerEWMAUsage computes EWMA usage, production, efficiency, and
// expected-opportunity features from nflverse player-week rows using
// EWMAHalfLifeWeeks. Unlike BuildPlayerUsage, it returns a profile whenever at
// least 1 prior week exists. Returns nil when rows is empty.
func BuildPlayerEWMAUsage(rows []db.NFLversePlayerWeek) *PlayerEWMAUsage {
	return BuildPlayerEWMAUsageWithHalfLife(rows, EWMAHalfLifeWeeks)
}

// BuildPlayerEWMAUsageWithHalfLife computes EWMA usage, production,
// efficiency, and expected-opportunity features using the supplied half-life in
// games. Returns nil when rows is empty.
func BuildPlayerEWMAUsageWithHalfLife(rows []db.NFLversePlayerWeek, halfLife float64) *PlayerEWMAUsage {
	if len(rows) == 0 {
		return nil
	}
	if halfLife <= 0 {
		halfLife = EWMAHalfLifeWeeks
	}
	ordered := sortPlayerWeeksDesc(rows)
	lambda := math.Pow(0.5, 1.0/halfLife)
	weight := 1.0
	var weightTotal float64

	out := &PlayerEWMAUsage{
		GamesPlayed:   len(ordered),
		HalfLifeWeeks: halfLife,
	}
	for _, r := range ordered {
		weightTotal += weight
		out.OffensePct += weight * r.OffensePct
		out.TargetShare += weight * r.TargetShare
		out.AirYardsShare += weight * r.AirYardsShare
		out.WOPR += weight * r.WOPR
		out.CarryShare += weight * r.CarryShare
		out.PassAtt += weight * float64(r.PassAtt)
		out.Carries += weight * float64(r.Carries)
		out.Targets += weight * float64(r.Targets)
		out.Receptions += weight * float64(r.Receptions)
		out.PassYds += weight * float64(r.PassYds)
		out.RushYds += weight * float64(r.RushYds)
		out.RecYds += weight * float64(r.RecYds)
		out.PassTDs += weight * float64(r.PassTDs)
		out.RushTDs += weight * float64(r.RushTDs)
		out.RecTDs += weight * float64(r.RecTDs)
		out.RZOpps += weight * float64(r.RZOpps)
		out.TotalEPA += weight * r.TotalEPA
		out.CPOE += weight * r.CPOE
		out.ExpPassYds += weight * r.ExpPassYds
		out.ExpRushYds += weight * r.ExpRushYds
		out.ExpRecYds += weight * r.ExpRecYds
		out.ExpReceptions += weight * r.ExpReceptions
		out.ExpPassTDs += weight * r.ExpPassTDs
		out.ExpRushTDs += weight * r.ExpRushTDs
		out.ExpRecTDs += weight * r.ExpRecTDs
		weight *= lambda
	}
	if weightTotal == 0 {
		return nil
	}
	inv := 1.0 / weightTotal
	out.OffensePct *= inv
	out.TargetShare *= inv
	out.AirYardsShare *= inv
	out.WOPR *= inv
	out.CarryShare *= inv
	out.PassAtt *= inv
	out.Carries *= inv
	out.Targets *= inv
	out.Receptions *= inv
	out.PassYds *= inv
	out.RushYds *= inv
	out.RecYds *= inv
	out.PassTDs *= inv
	out.RushTDs *= inv
	out.RecTDs *= inv
	out.RZOpps *= inv
	out.TotalEPA *= inv
	out.CPOE *= inv
	out.ExpPassYds *= inv
	out.ExpRushYds *= inv
	out.ExpRecYds *= inv
	out.ExpReceptions *= inv
	out.ExpPassTDs *= inv
	out.ExpRushTDs *= inv
	out.ExpRecTDs *= inv
	return out
}

// BuildPlayerRoleProfiles deterministically derives each player's EWMA usage
// and RoleArchetype (QB, WR1, WR2, TE1, RB1, RB2) from trailing nflverse
// player-week rows by ranking teammates within (team, position).
func BuildPlayerRoleProfiles(rows []db.NFLversePlayerWeek) map[string]PlayerRoleProfile {
	if len(rows) == 0 {
		return nil
	}
	byPlayer := groupNFLverseRowsByPlayer(rows)
	if len(byPlayer) == 0 {
		return nil
	}

	profiles := make(map[string]PlayerRoleProfile, len(byPlayer))
	type teamPosKey struct {
		team string
		pos  string
	}
	byTeamPos := make(map[teamPosKey][]PlayerRoleProfile)

	for espnID, pRows := range byPlayer {
		ordered := sortPlayerWeeksDesc(pRows)
		ewma := BuildPlayerEWMAUsage(ordered)
		team := latestNonEmptyTeam(ordered)
		pos := latestNormalizedPosition(ordered)
		prof := PlayerRoleProfile{
			ESPNID:   espnID,
			Team:     team,
			Position: pos,
			EWMA:     ewma,
		}
		if pos == "QB" {
			prof.Role = RoleQB
		}
		profiles[espnID] = prof

		if team == "" || ewma == nil || !hasOffensiveParticipation(ewma) {
			continue
		}
		if pos == "WR" || pos == "TE" || pos == "RB" {
			key := teamPosKey{team: team, pos: pos}
			byTeamPos[key] = append(byTeamPos[key], prof)
		}
	}

	for key, group := range byTeamPos {
		switch key.pos {
		case "WR":
			sortPassCatchersDesc(group)
			for i, p := range group {
				switch i {
				case 0:
					p.Role = RoleWR1
				case 1:
					p.Role = RoleWR2
				default:
					continue
				}
				profiles[p.ESPNID] = p
			}
		case "TE":
			sortPassCatchersDesc(group)
			if len(group) > 0 {
				p := group[0]
				p.Role = RoleTE1
				profiles[p.ESPNID] = p
			}
		case "RB":
			sortRBsDesc(group)
			for i, p := range group {
				switch i {
				case 0:
					p.Role = RoleRB1
				case 1:
					p.Role = RoleRB2
				default:
					continue
				}
				profiles[p.ESPNID] = p
			}
		}
	}
	return profiles
}

var allRoleArchetypes = []RoleArchetype{
	RoleQB,
	RoleWR1,
	RoleWR2,
	RoleTE1,
	RoleRB1,
	RoleRB2,
}

// BuildOppRoleDefenseProfiles aggregates opposing player-week rows into
// per-game stats allowed and league ranks for each (defense, RoleArchetype).
// When roles is nil, player role profiles are derived from rows first.
func BuildOppRoleDefenseProfiles(
	rows []db.NFLversePlayerWeek,
	roles map[string]PlayerRoleProfile,
) map[string]map[RoleArchetype]*RoleDefenseProfile {
	if len(rows) == 0 {
		return nil
	}
	if roles == nil {
		roles = BuildPlayerRoleProfiles(rows)
	}
	if len(roles) == 0 {
		return nil
	}

	type oppRoleGameKey struct {
		opponent   string
		role       RoleArchetype
		seasonType int
		weekNum    int
	}
	type gameSum struct {
		passYds    float64
		passTDs    float64
		rushYds    float64
		rushTDs    float64
		recYds     float64
		receptions float64
		recTDs     float64
		targets    float64
		carries    float64
		totalEPA   float64
		expYds     float64
	}

	byGame := make(map[oppRoleGameKey]gameSum)
	for _, row := range rows {
		opp := strings.ToUpper(strings.TrimSpace(row.Opponent))
		if opp == "" {
			continue
		}
		prof, ok := roles[row.ESPNID]
		if !ok || prof.Role == RoleUnknown {
			continue
		}
		key := oppRoleGameKey{
			opponent:   opp,
			role:       prof.Role,
			seasonType: row.SeasonType,
			weekNum:    row.WeekNum,
		}
		cur := byGame[key]
		cur.passYds += float64(row.PassYds)
		cur.passTDs += float64(row.PassTDs)
		cur.rushYds += float64(row.RushYds)
		cur.rushTDs += float64(row.RushTDs)
		cur.recYds += float64(row.RecYds)
		cur.receptions += float64(row.Receptions)
		cur.recTDs += float64(row.RecTDs)
		cur.targets += float64(row.Targets)
		cur.carries += float64(row.Carries)
		cur.totalEPA += row.TotalEPA
		cur.expYds += roleExpYds(prof.Role, row)
		byGame[key] = cur
	}
	if len(byGame) == 0 {
		return nil
	}

	out := make(map[string]map[RoleArchetype]*RoleDefenseProfile)
	for key, sum := range byGame {
		byRole := out[key.opponent]
		if byRole == nil {
			byRole = make(map[RoleArchetype]*RoleDefenseProfile)
			out[key.opponent] = byRole
		}
		prof := byRole[key.role]
		if prof == nil {
			prof = &RoleDefenseProfile{Role: key.role}
			byRole[key.role] = prof
		}
		prof.Games++
		prof.PassYdsAllowed += sum.passYds
		prof.PassTDsAllowed += sum.passTDs
		prof.RushYdsAllowed += sum.rushYds
		prof.RushTDsAllowed += sum.rushTDs
		prof.RecYdsAllowed += sum.recYds
		prof.ReceptionsAllowed += sum.receptions
		prof.RecTDsAllowed += sum.recTDs
		prof.TargetsAllowed += sum.targets
		prof.CarriesAllowed += sum.carries
		prof.TotalEPAAllowed += sum.totalEPA
		prof.ExpYdsAllowed += sum.expYds
	}

	for _, byRole := range out {
		for _, prof := range byRole {
			if prof.Games == 0 {
				continue
			}
			inv := 1.0 / float64(prof.Games)
			prof.PassYdsAllowed *= inv
			prof.PassTDsAllowed *= inv
			prof.RushYdsAllowed *= inv
			prof.RushTDsAllowed *= inv
			prof.RecYdsAllowed *= inv
			prof.ReceptionsAllowed *= inv
			prof.RecTDsAllowed *= inv
			prof.TargetsAllowed *= inv
			prof.CarriesAllowed *= inv
			prof.TotalEPAAllowed *= inv
			prof.ExpYdsAllowed *= inv
		}
	}

	for _, role := range allRoleArchetypes {
		var teams []string
		for team, byRole := range out {
			if byRole[role] != nil {
				teams = append(teams, team)
			}
		}
		if len(teams) == 0 {
			continue
		}
		rankTeams(teams, func(t string) float64 {
			return rolePrimaryYdsAllowed(role, out[t][role])
		}, false, func(t string, r int) {
			out[t][role].YdsAllowedRank = r
		})
		rankTeams(teams, func(t string) float64 {
			return out[t][role].TotalEPAAllowed
		}, false, func(t string, r int) {
			out[t][role].EPAAllowedRank = r
		})
	}
	return out
}

func groupNFLverseRowsByPlayer(rows []db.NFLversePlayerWeek) map[string][]db.NFLversePlayerWeek {
	grouped := make(map[string][]db.NFLversePlayerWeek)
	for _, r := range rows {
		if r.ESPNID == "" {
			continue
		}
		grouped[r.ESPNID] = append(grouped[r.ESPNID], r)
	}
	return grouped
}

func sortPlayerWeeksDesc(rows []db.NFLversePlayerWeek) []db.NFLversePlayerWeek {
	if len(rows) <= 1 {
		return rows
	}
	alreadySorted := true
	for i := 1; i < len(rows); i++ {
		if comparePlayerWeeksDesc(rows[i-1], rows[i]) > 0 {
			alreadySorted = false
			break
		}
	}
	if alreadySorted {
		return rows
	}
	ordered := slices.Clone(rows)
	slices.SortStableFunc(ordered, comparePlayerWeeksDesc)
	return ordered
}

func comparePlayerWeeksDesc(a, b db.NFLversePlayerWeek) int {
	if c := cmp.Compare(b.Season, a.Season); c != 0 {
		return c
	}
	if c := cmp.Compare(b.SeasonType, a.SeasonType); c != 0 {
		return c
	}
	if c := cmp.Compare(b.WeekNum, a.WeekNum); c != 0 {
		return c
	}
	return cmp.Compare(a.ESPNID, b.ESPNID)
}

func latestNonEmptyTeam(ordered []db.NFLversePlayerWeek) string {
	for _, r := range ordered {
		if team := strings.ToUpper(strings.TrimSpace(r.Team)); team != "" {
			return team
		}
	}
	return ""
}

func latestNormalizedPosition(ordered []db.NFLversePlayerWeek) string {
	for _, r := range ordered {
		if pos := db.NormalizePosition(r.Position); pos != "" {
			return pos
		}
	}
	return ""
}

func hasOffensiveParticipation(e *PlayerEWMAUsage) bool {
	return e.OffensePct > 0 ||
		e.TargetShare > 0 ||
		e.CarryShare > 0 ||
		e.WOPR > 0 ||
		e.Targets > 0 ||
		e.Carries > 0 ||
		e.PassAtt > 0 ||
		e.ExpPassYds > 0 ||
		e.ExpRushYds > 0 ||
		e.ExpRecYds > 0
}

func sortPassCatchersDesc(group []PlayerRoleProfile) {
	slices.SortFunc(group, func(a, b PlayerRoleProfile) int {
		if c := cmp.Compare(b.EWMA.TargetShare, a.EWMA.TargetShare); c != 0 {
			return c
		}
		if c := cmp.Compare(b.EWMA.WOPR, a.EWMA.WOPR); c != 0 {
			return c
		}
		if c := cmp.Compare(b.EWMA.OffensePct, a.EWMA.OffensePct); c != 0 {
			return c
		}
		if c := cmp.Compare(b.EWMA.Targets, a.EWMA.Targets); c != 0 {
			return c
		}
		if c := cmp.Compare(b.EWMA.GamesPlayed, a.EWMA.GamesPlayed); c != 0 {
			return c
		}
		return cmp.Compare(a.ESPNID, b.ESPNID)
	})
}

func rbWorkloadScore(e *PlayerEWMAUsage) float64 {
	if e == nil {
		return 0
	}
	return 0.60*e.CarryShare + 0.40*e.OffensePct
}

func sortRBsDesc(group []PlayerRoleProfile) {
	slices.SortFunc(group, func(a, b PlayerRoleProfile) int {
		if c := cmp.Compare(rbWorkloadScore(b.EWMA), rbWorkloadScore(a.EWMA)); c != 0 {
			return c
		}
		if c := cmp.Compare(b.EWMA.CarryShare, a.EWMA.CarryShare); c != 0 {
			return c
		}
		if c := cmp.Compare(b.EWMA.Carries, a.EWMA.Carries); c != 0 {
			return c
		}
		if c := cmp.Compare(b.EWMA.OffensePct, a.EWMA.OffensePct); c != 0 {
			return c
		}
		if c := cmp.Compare(b.EWMA.TargetShare, a.EWMA.TargetShare); c != 0 {
			return c
		}
		if c := cmp.Compare(b.EWMA.GamesPlayed, a.EWMA.GamesPlayed); c != 0 {
			return c
		}
		return cmp.Compare(a.ESPNID, b.ESPNID)
	})
}

func roleExpYds(role RoleArchetype, row db.NFLversePlayerWeek) float64 {
	switch role {
	case RoleQB:
		return row.ExpPassYds
	case RoleRB1, RoleRB2:
		return row.ExpRushYds
	case RoleWR1, RoleWR2, RoleTE1:
		return row.ExpRecYds
	default:
		return 0
	}
}

func rolePrimaryYdsAllowed(role RoleArchetype, prof *RoleDefenseProfile) float64 {
	if prof == nil {
		return 0
	}
	switch role {
	case RoleQB:
		return prof.PassYdsAllowed
	case RoleRB1, RoleRB2:
		return prof.RushYdsAllowed
	case RoleWR1, RoleWR2, RoleTE1:
		return prof.RecYdsAllowed
	default:
		return 0
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

// OppDefenseGame holds the leading opposing player's stat line against a
// defense in one week for the opposing defense L5 chart.
type OppDefenseGame struct {
	SeasonType int
	WeekNum    int
	PlayerName string
	Team       string
	Value      float64
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

	CarriesPerGame float64
	TargetsPerGame float64
	PassAttPerGame float64
	AirYardsShare  float64
	RZOppsPerGame  float64
	EPAPerGame     float64

	TeamDefProfiles   map[string]*TeamDefenseProfile
	PosDefRanks       map[string]int
	OppOverallDefRank int
	OppPosDefRank     int
	OppRoleLabel      string
	OppDefenseL5      []OppDefenseGame
}

// MergeGameInjuries combines a game's stored summary injuries with key-position
// roster injuries for its home and away teams, keeping summary entries primary.
func MergeGameInjuries(game db.Game, rosters []db.RosterPlayer) map[string][]espn.Injury {
	summaryByTeam := make(map[string][]espn.Injury)
	if game.InjuriesJSON != nil && *game.InjuriesJSON != "" {
		if teamInjuries, err := espn.ParseInjuries(*game.InjuriesJSON); err == nil {
			for _, ti := range teamInjuries {
				if ti.Abbreviation != "" && len(ti.Injuries) > 0 {
					summaryByTeam[ti.Abbreviation] = ti.Injuries
				}
			}
		}
	}

	rosterByTeam := make(map[string][]espn.Injury, 2)
	for _, p := range rosters {
		if p.Team != game.HomeTeam && p.Team != game.AwayTeam {
			continue
		}
		if strings.TrimSpace(p.InjuryStatus) == "" || !espn.IsKeyInjuryPosition(p.Position) {
			continue
		}
		rosterByTeam[p.Team] = append(rosterByTeam[p.Team], espn.Injury{
			AthleteID:   p.AthleteID,
			FullName:    p.FullName,
			Position:    p.Position,
			HeadshotURL: p.HeadshotURL,
			Status:      p.InjuryStatus,
		})
	}

	merged := mergeTeamInjuryFeeds(summaryByTeam, rosterByTeam, game.AwayTeam, game.HomeTeam)
	return merged
}

// MergeInjuryFeeds combines per-team summary injuries with per-team roster
// injuries and converts them to analyzer.Injury slices, keeping summary entries
// primary.
func MergeInjuryFeeds(
	summaryByTeam map[string][]espn.Injury,
	rosterByTeam map[string][]espn.Injury,
) map[string][]Injury {
	return toAnalyzerInjuries(mergeTeamInjuryFeeds(summaryByTeam, rosterByTeam))
}

func mergeTeamInjuryFeeds(
	summaryByTeam map[string][]espn.Injury,
	rosterByTeam map[string][]espn.Injury,
	preferredTeams ...string,
) map[string][]espn.Injury {
	seen := make(map[string]struct{}, len(preferredTeams)+len(summaryByTeam)+len(rosterByTeam))
	teams := make([]string, 0, len(preferredTeams)+len(summaryByTeam)+len(rosterByTeam))
	addTeam := func(team string) {
		if team == "" {
			return
		}
		if _, ok := seen[team]; ok {
			return
		}
		seen[team] = struct{}{}
		teams = append(teams, team)
	}
	for _, team := range preferredTeams {
		addTeam(team)
	}
	for team := range summaryByTeam {
		addTeam(team)
	}
	for team := range rosterByTeam {
		addTeam(team)
	}

	merged := make(map[string][]espn.Injury, len(teams))
	for _, team := range teams {
		if combined := espn.MergeTeamInjuries(summaryByTeam[team], rosterByTeam[team]); len(combined) > 0 {
			merged[team] = combined
		}
	}
	if len(merged) == 0 {
		return nil
	}
	return merged
}

// LoadSlipSignals loads fresh nfelo projections, fresh nflverse team EPA/pace
// profiles, nflverse player usage, merged team injuries, roster positions,
// player_gamelogs, and sharp bookmaker benchmarks from stored odds for a slip's
// games and legs. Missing or stale sources fail soft.
func LoadSlipSignals(
	ctx context.Context,
	store *db.Store,
	now time.Time,
	weekID int64,
	games []db.Game,
	legs []db.Leg,
) (map[string]Game, map[string]PlayerStats) {
	return loadSignals(ctx, store, now, weekID, games, DistinctLegPlayerIDs(legs))
}

// LoadBoardSignals loads fresh nfelo projections, fresh nflverse team EPA/pace
// profiles, nflverse player usage, merged team injuries, roster positions,
// player_gamelogs, and sharp bookmaker benchmarks from stored odds for a
// board's games and playerIDs. Missing or stale sources fail soft.
func LoadBoardSignals(
	ctx context.Context,
	store *db.Store,
	now time.Time,
	weekID int64,
	games []db.Game,
	playerIDs []string,
) (map[string]Game, map[string]PlayerStats) {
	return loadSignals(ctx, store, now, weekID, games, distinctNonEmptyIDs(playerIDs))
}

func loadSignals(
	ctx context.Context,
	store *db.Store,
	now time.Time,
	weekID int64,
	games []db.Game,
	athleteIDs []string,
) (map[string]Game, map[string]PlayerStats) {
	nfelo := loadFreshNfelo(ctx, store, now, games)

	var teamEPA map[string]*TeamDefenseProfile
	var usageByPlayer map[string]*PlayerUsage
	var roleProfiles map[string]PlayerRoleProfile
	var roleDefByTeam map[string]map[RoleArchetype]*RoleDefenseProfile

	if week, err := store.WeekByID(ctx, weekID); err == nil {
		if profiles, fresh := LoadFreshTeamProfiles(ctx, store, now, week); fresh {
			teamEPA = profiles
			if len(athleteIDs) > 0 {
				seasonTypes := NFLverseSeasonTypes(week.SeasonType)
				if allRows, err := store.NFLversePlayerWeeksForSeasonTypes(
					ctx, week.Season, seasonTypes...,
				); err == nil && len(allRows) > 0 {
					byPlayer := groupNFLverseRowsByPlayer(allRows)
					usageByPlayer = make(map[string]*PlayerUsage, len(athleteIDs))
					for _, id := range athleteIDs {
						if usage := BuildPlayerUsage(sortPlayerWeeksDesc(byPlayer[id])); usage != nil {
							usageByPlayer[id] = usage
						}
					}
					roleProfiles = BuildPlayerRoleProfiles(allRows)
					roleDefByTeam = BuildOppRoleDefenseProfiles(allRows, roleProfiles)
				}
			}
		}
	}

	var rosters []db.RosterPlayer
	if teams := distinctGameTeams(games); len(teams) > 0 {
		if loaded, err := store.RosterForTeams(ctx, teams); err == nil {
			rosters = loaded
		}
	}
	positionByID := make(map[string]string, len(rosters))
	for _, p := range rosters {
		if p.Position != "" {
			positionByID[p.AthleteID] = p.Position
		}
	}

	var oddsRows []db.Odds
	var sharpByGame map[string]map[SharpKey]SharpQuote
	if len(games) > 0 {
		if loaded, err := store.OddsForWeek(ctx, weekID); err == nil {
			oddsRows = loaded
			sharpByGame = BuildSharpBenchmarks(oddsRows)
		}
	}
	scriptByGame := BuildMarketScripts(games, oddsRows)

	gamesByID := make(map[string]Game, len(games))
	for _, game := range games {
		var proj *db.NfeloGame
		if p, ok := nfelo[game.ID]; ok {
			proj = &p
		}
		converted := GameFromDB(game, proj, teamEPA)
		if len(roleDefByTeam) > 0 {
			converted.HomeRoleDefense = roleDefByTeam[game.HomeTeam]
			converted.AwayRoleDefense = roleDefByTeam[game.AwayTeam]
		}
		converted.Injuries = toAnalyzerInjuries(MergeGameInjuries(game, rosters))
		converted.Sharp = sharpByGame[game.ID]
		converted.MarketScript = scriptByGame[game.ID]
		gamesByID[game.ID] = converted
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
		var ewma *PlayerEWMAUsage
		var role RoleArchetype
		var nflvPos string
		if prof, ok := roleProfiles[id]; ok {
			ewma = prof.EWMA
			role = prof.Role
			nflvPos = prof.Position
		}
		pos := positionByID[id]
		if pos == "" {
			pos = nflvPos
		}
		if role == RoleUnknown && db.NormalizePosition(pos) == "QB" {
			role = RoleQB
		}
		if len(markets) == 0 && usage == nil && ewma == nil && pos == "" && role == RoleUnknown {
			continue
		}
		stats[id] = PlayerStats{
			Position: pos,
			Role:     role,
			Games:    markets,
			Usage:    usage,
			EWMA:     ewma,
		}
	}
	return gamesByID, stats
}

// BuildMarketScripts derives consensus spread, game total, and implied team
// totals per game from cached SPREAD and TOTAL odds rows without extra DB
// queries or API calls.
func BuildMarketScripts(games []db.Game, rows []db.Odds) map[string]MarketScript {
	if len(games) == 0 || len(rows) == 0 {
		return nil
	}

	byGame := make(map[string][]db.Odds, len(games))
	for _, row := range rows {
		if row.GameID == "" || row.Line == nil {
			continue
		}
		market := strings.ToUpper(strings.TrimSpace(row.MarketType))
		if market != marketSpread && market != marketTotal {
			continue
		}
		byGame[row.GameID] = append(byGame[row.GameID], row)
	}
	if len(byGame) == 0 {
		return nil
	}

	out := make(map[string]MarketScript, len(games))
	for _, g := range games {
		ms := BuildMarketScript(g.HomeTeam, g.AwayTeam, byGame[g.ID])
		if ms.HasSpread || ms.HasTotal {
			out[g.ID] = ms
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// BuildMarketScript derives consensus spread, game total, and implied team
// totals for one game from cached SPREAD and TOTAL odds rows.
func BuildMarketScript(homeTeam, awayTeam string, rows []db.Odds) MarketScript {
	if len(rows) == 0 {
		return MarketScript{}
	}

	home := strings.ToUpper(strings.TrimSpace(homeTeam))
	away := strings.ToUpper(strings.TrimSpace(awayTeam))
	spreads, totals := collectScriptLines(home, away, rows)

	var ms MarketScript
	if len(spreads) > 0 {
		ms.HomeSpread = Median(spreads)
		ms.HasSpread = true
	}
	if len(totals) > 0 {
		ms.GameTotal = Median(totals)
		ms.HasTotal = true
	}
	if ms.HasSpread && ms.HasTotal {
		ms.HomeImpliedTotal = (ms.GameTotal - ms.HomeSpread) / 2
		ms.AwayImpliedTotal = (ms.GameTotal + ms.HomeSpread) / 2
	}
	return ms
}

type twoSideLine struct {
	hasA bool
	valA float64
	hasB bool
	valB float64
}

func collectScriptLines(home, away string, rows []db.Odds) (spreads, totals []float64) {
	spreadByBook := make(map[string]*twoSideLine)
	totalByBook := make(map[string]*twoSideLine)

	for _, row := range rows {
		if row.Line == nil {
			continue
		}
		book := db.NormalizeBook(row.Book)
		if book == "" || excludedTier3Books[book] {
			continue
		}
		market := strings.ToUpper(strings.TrimSpace(row.MarketType))
		side := strings.ToUpper(strings.TrimSpace(row.Side))
		switch market {
		case marketSpread:
			if side != "TEAM" {
				continue
			}
			team := strings.ToUpper(strings.TrimSpace(row.Team))
			isHome := home != "" && team == home
			isAway := away != "" && team == away
			if !isHome && !isAway {
				continue
			}
			recordBookLine(spreadByBook, book, isHome, *row.Line)
		case marketTotal:
			if *row.Line <= 0 || (side != sideOver && side != sideUnder) {
				continue
			}
			recordBookLine(totalByBook, book, side == sideOver, *row.Line)
		}
	}

	spreads = make([]float64, 0, len(spreadByBook))
	for _, entry := range spreadByBook {
		switch {
		case entry.hasA && entry.hasB:
			if math.Abs(entry.valA+entry.valB) <= lineMatchEpsilon {
				spreads = append(spreads, entry.valA)
			} else {
				spreads = append(spreads, (entry.valA-entry.valB)/2)
			}
		case entry.hasA:
			spreads = append(spreads, entry.valA)
		case entry.hasB:
			spreads = append(spreads, -entry.valB)
		}
	}

	totals = make([]float64, 0, len(totalByBook))
	for _, entry := range totalByBook {
		switch {
		case entry.hasA && entry.hasB:
			if math.Abs(entry.valA-entry.valB) <= lineMatchEpsilon {
				totals = append(totals, entry.valA)
			} else {
				totals = append(totals, (entry.valA+entry.valB)/2)
			}
		case entry.hasA:
			totals = append(totals, entry.valA)
		case entry.hasB:
			totals = append(totals, entry.valB)
		}
	}
	return spreads, totals
}

func recordBookLine(byBook map[string]*twoSideLine, book string, isSideA bool, val float64) {
	entry := byBook[book]
	if entry == nil {
		entry = &twoSideLine{}
		byBook[book] = entry
	}
	if isSideA {
		entry.hasA = true
		entry.valA = val
	} else {
		entry.hasB = true
		entry.valB = val
	}
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

// BuildPositionDefenseRanks groups nflverse player-week rows for position by
// (opponent, season_type, week_num), sums the active market stat allowed per
// game, averages across games played by each defense, and ranks ascending
// (1 = fewest allowed, ties broken alphabetically).
func BuildPositionDefenseRanks(rows []db.NFLversePlayerWeek, position, market string) map[string]int {
	pos := db.NormalizePosition(position)
	if pos == "" || len(rows) == 0 {
		return nil
	}
	resolvedMarket := resolveResearchMarket(pos, market)
	if _, ok := marketStatValue(db.NFLversePlayerWeek{}, resolvedMarket); !ok {
		return nil
	}

	type oppGameKey struct {
		opponent   string
		seasonType int
		weekNum    int
	}
	gameTotals := make(map[oppGameKey]float64)
	for _, row := range rows {
		if db.NormalizePosition(row.Position) != pos {
			continue
		}
		opp := strings.ToUpper(strings.TrimSpace(row.Opponent))
		if opp == "" {
			continue
		}
		val, ok := marketStatValue(row, resolvedMarket)
		if !ok {
			continue
		}
		key := oppGameKey{
			opponent:   opp,
			seasonType: row.SeasonType,
			weekNum:    row.WeekNum,
		}
		gameTotals[key] += val
	}
	if len(gameTotals) == 0 {
		return nil
	}

	sumByOpp := make(map[string]float64)
	gamesByOpp := make(map[string]int)
	for key, total := range gameTotals {
		sumByOpp[key.opponent] += total
		gamesByOpp[key.opponent]++
	}

	teams := make([]string, 0, len(sumByOpp))
	avgByOpp := make(map[string]float64, len(sumByOpp))
	for opp, sum := range sumByOpp {
		teams = append(teams, opp)
		avgByOpp[opp] = sum / float64(gamesByOpp[opp])
	}

	ranks := make(map[string]int, len(teams))
	rankTeams(teams, func(t string) float64 { return avgByOpp[t] }, false, func(t string, r int) {
		ranks[t] = r
	})
	return ranks
}

// BuildOppDefenseLast5 groups opposing player-week rows by (season_type,
// week_num), selects the leading opposing player per game for market (highest
// value, tie-broken by usage then player name), and returns up to the 5 most
// recent games in chronological order.
func BuildOppDefenseLast5(rows []db.NFLversePlayerWeek, market string) []OppDefenseGame {
	if len(rows) == 0 {
		return nil
	}
	resolvedMarket := strings.ToUpper(strings.TrimSpace(market))
	if resolvedMarket == "" {
		resolvedMarket = defaultMarketForPosition(rows[0].Position)
	}
	if _, ok := marketStatValue(db.NFLversePlayerWeek{}, resolvedMarket); !ok {
		return nil
	}

	type weekKey struct {
		seasonType int
		weekNum    int
	}
	type candidate struct {
		row db.NFLversePlayerWeek
		val float64
	}
	bestByWeek := make(map[weekKey]candidate)
	for _, row := range rows {
		val, ok := marketStatValue(row, resolvedMarket)
		if !ok {
			continue
		}
		key := weekKey{seasonType: row.SeasonType, weekNum: row.WeekNum}
		cur, exists := bestByWeek[key]
		if !exists || oppCandidateBetter(row, val, cur.row, cur.val) {
			bestByWeek[key] = candidate{row: row, val: val}
		}
	}
	if len(bestByWeek) == 0 {
		return nil
	}

	out := make([]OppDefenseGame, 0, len(bestByWeek))
	for _, best := range bestByWeek {
		out = append(out, OppDefenseGame{
			SeasonType: best.row.SeasonType,
			WeekNum:    best.row.WeekNum,
			PlayerName: best.row.PlayerName,
			Team:       best.row.Team,
			Value:      best.val,
		})
	}
	slices.SortFunc(out, func(a, b OppDefenseGame) int {
		if a.SeasonType != b.SeasonType {
			return cmp.Compare(a.SeasonType, b.SeasonType)
		}
		return cmp.Compare(a.WeekNum, b.WeekNum)
	})
	if len(out) > 5 {
		out = out[len(out)-5:]
	}
	return out
}

// BuildOppDefenseLast5Comparable groups opposing player-week rows by
// (season_type, week_num) and selects the most comparable opposing player per
// game for targetAthleteID (preferring the same player in a rematch, then the
// same depth-chart role tier such as WR1/WR2/WR3+ or RB1/RB2/RB3+, then closest
// trailing usage profile). When no target role profile can be derived, it falls
// back to BuildOppDefenseLast5.
func BuildOppDefenseLast5Comparable(
	oppRows []db.NFLversePlayerWeek,
	leaguePosRows []db.NFLversePlayerWeek,
	targetAthleteID, position, market string,
) ([]OppDefenseGame, string) {
	if len(oppRows) == 0 {
		return nil, ""
	}
	normPos := db.NormalizePosition(position)
	if normPos == "" {
		normPos = db.NormalizePosition(oppRows[0].Position)
	}
	resolvedMarket := strings.ToUpper(strings.TrimSpace(market))
	if resolvedMarket == "" {
		resolvedMarket = defaultMarketForPosition(normPos)
	}
	if _, ok := marketStatValue(db.NFLversePlayerWeek{}, resolvedMarket); !ok {
		return nil, ""
	}

	if normPos != "WR" && normPos != "RB" && normPos != "TE" {
		return BuildOppDefenseLast5(oppRows, resolvedMarket), ""
	}

	profileSource := leaguePosRows
	if len(profileSource) == 0 {
		profileSource = oppRows
	}
	tiers, profiles := buildPlayerDepthTiers(profileSource, normPos)
	targetTier, hasTarget := tiers[targetAthleteID]
	targetProf := profiles[targetAthleteID]
	if !hasTarget {
		return BuildOppDefenseLast5(oppRows, resolvedMarket), ""
	}

	type weekKey struct {
		seasonType int
		weekNum    int
	}
	type candidate struct {
		row  db.NFLversePlayerWeek
		val  float64
		tier int
		prof PlayerRoleProfile
	}
	bestByWeek := make(map[weekKey]candidate)
	for _, row := range oppRows {
		val, ok := marketStatValue(row, resolvedMarket)
		if !ok {
			continue
		}
		candTier := tiers[row.ESPNID]
		if candTier == 0 {
			candTier = maxDepthTier(normPos)
		}
		cand := candidate{
			row:  row,
			val:  val,
			tier: candTier,
			prof: profiles[row.ESPNID],
		}
		key := weekKey{seasonType: row.SeasonType, weekNum: row.WeekNum}
		cur, exists := bestByWeek[key]
		if !exists || oppComparableBetter(
			cand.row, cand.val, cand.tier, cand.prof,
			cur.row, cur.val, cur.tier, cur.prof,
			targetAthleteID, targetTier, targetProf, normPos,
		) {
			bestByWeek[key] = cand
		}
	}
	if len(bestByWeek) == 0 {
		return nil, ""
	}

	out := make([]OppDefenseGame, 0, len(bestByWeek))
	for _, best := range bestByWeek {
		out = append(out, OppDefenseGame{
			SeasonType: best.row.SeasonType,
			WeekNum:    best.row.WeekNum,
			PlayerName: best.row.PlayerName,
			Team:       best.row.Team,
			Value:      best.val,
		})
	}
	slices.SortFunc(out, func(a, b OppDefenseGame) int {
		if a.SeasonType != b.SeasonType {
			return cmp.Compare(a.SeasonType, b.SeasonType)
		}
		return cmp.Compare(a.WeekNum, b.WeekNum)
	})
	if len(out) > 5 {
		out = out[len(out)-5:]
	}
	return out, depthTierLabel(normPos, targetTier)
}

func maxDepthTier(normPos string) int {
	switch normPos {
	case "WR", "RB":
		return 3
	case "TE":
		return 2
	default:
		return 1
	}
}

func depthTierLabel(normPos string, tier int) string {
	switch normPos {
	case "WR":
		switch tier {
		case 1:
			return "WR1"
		case 2:
			return "WR2"
		default:
			return "WR3+"
		}
	case "RB":
		switch tier {
		case 1:
			return "RB1"
		case 2:
			return "RB2"
		default:
			return "RB3+"
		}
	case "TE":
		if tier == 1 {
			return "TE1"
		}
		return "TE2+"
	default:
		return ""
	}
}

func buildPlayerDepthTiers(
	rows []db.NFLversePlayerWeek,
	normPos string,
) (map[string]int, map[string]PlayerRoleProfile) {
	byPlayer := groupNFLverseRowsByPlayer(rows)
	if len(byPlayer) == 0 {
		return nil, nil
	}

	profiles := make(map[string]PlayerRoleProfile, len(byPlayer))
	byTeam := make(map[string][]PlayerRoleProfile)

	for espnID, pRows := range byPlayer {
		ordered := sortPlayerWeeksDesc(pRows)
		pos := latestNormalizedPosition(ordered)
		if pos != normPos {
			continue
		}
		ewma := BuildPlayerEWMAUsage(ordered)
		team := latestNonEmptyTeam(ordered)
		prof := PlayerRoleProfile{
			ESPNID:   espnID,
			Team:     team,
			Position: pos,
			EWMA:     ewma,
		}
		profiles[espnID] = prof
		if team != "" && ewma != nil {
			byTeam[team] = append(byTeam[team], prof)
		}
	}

	capTier := maxDepthTier(normPos)
	tiers := make(map[string]int, len(profiles))
	for _, group := range byTeam {
		switch normPos {
		case "RB":
			sortRBsDesc(group)
		default:
			sortPassCatchersDesc(group)
		}
		for i, p := range group {
			tier := min(i+1, capTier)
			tiers[p.ESPNID] = tier
		}
	}
	return tiers, profiles
}

func oppComparableBetter(
	a db.NFLversePlayerWeek, aVal float64, aTier int, aProf PlayerRoleProfile,
	b db.NFLversePlayerWeek, bVal float64, bTier int, bProf PlayerRoleProfile,
	targetID string, targetTier int, targetProf PlayerRoleProfile, normPos string,
) bool {
	if targetID != "" {
		aSelf := a.ESPNID == targetID
		bSelf := b.ESPNID == targetID
		if aSelf != bSelf {
			return aSelf
		}
	}

	aTierDist := absInt(aTier - targetTier)
	bTierDist := absInt(bTier - targetTier)
	if aTierDist != bTierDist {
		return aTierDist < bTierDist
	}

	aDist := oppUsageDistance(a, aProf, targetProf, normPos)
	bDist := oppUsageDistance(b, bProf, targetProf, normPos)
	if math.Abs(aDist-bDist) > 1e-6 {
		return aDist < bDist
	}

	if a.OffensePct != b.OffensePct {
		return a.OffensePct > b.OffensePct
	}
	if a.OffenseSnaps != b.OffenseSnaps {
		return a.OffenseSnaps > b.OffenseSnaps
	}
	if aVal != bVal {
		return aVal > bVal
	}
	if a.PlayerName != b.PlayerName {
		return a.PlayerName < b.PlayerName
	}
	return a.ESPNID < b.ESPNID
}

func oppUsageDistance(
	row db.NFLversePlayerWeek,
	candProf, targetProf PlayerRoleProfile,
	normPos string,
) float64 {
	if targetProf.EWMA == nil {
		return 0
	}
	var candSnap, candTargetShare, candCarries float64
	if candProf.EWMA != nil {
		candSnap = candProf.EWMA.OffensePct
		candTargetShare = candProf.EWMA.TargetShare
		candCarries = candProf.EWMA.Carries
	} else {
		candSnap = row.OffensePct
		candTargetShare = row.TargetShare
		candCarries = float64(row.Carries)
	}
	switch normPos {
	case "RB":
		carryDiff := math.Abs(candCarries-targetProf.EWMA.Carries) / 20.0
		snapDiff := math.Abs(candSnap - targetProf.EWMA.OffensePct)
		return carryDiff*2.0 + snapDiff
	default:
		tsDiff := math.Abs(candTargetShare - targetProf.EWMA.TargetShare)
		snapDiff := math.Abs(candSnap - targetProf.EWMA.OffensePct)
		return tsDiff*3.0 + snapDiff
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func oppCandidateBetter(a db.NFLversePlayerWeek, aVal float64, b db.NFLversePlayerWeek, bVal float64) bool {
	if aVal != bVal {
		return aVal > bVal
	}
	if a.OffensePct != b.OffensePct {
		return a.OffensePct > b.OffensePct
	}
	if a.TargetShare != b.TargetShare {
		return a.TargetShare > b.TargetShare
	}
	if a.OffenseSnaps != b.OffenseSnaps {
		return a.OffenseSnaps > b.OffenseSnaps
	}
	if a.PlayerName != b.PlayerName {
		return a.PlayerName < b.PlayerName
	}
	return a.ESPNID < b.ESPNID
}

func defaultMarketForPosition(pos string) string {
	switch db.NormalizePosition(pos) {
	case "QB":
		return marketPassYds
	case "RB":
		return marketRushYds
	case "WR", "TE":
		return marketRecYds
	case "K":
		return marketKickerPts
	default:
		return ""
	}
}

func resolveResearchMarket(position, market string) string {
	resolved := strings.ToUpper(strings.TrimSpace(market))
	if resolved != "" {
		return resolved
	}
	return defaultMarketForPosition(position)
}

func marketStatValue(row db.NFLversePlayerWeek, market string) (float64, bool) {
	switch strings.ToUpper(strings.TrimSpace(market)) {
	case marketPassYds:
		return float64(row.PassYds), true
	case marketPassTDs:
		return float64(row.PassTDs), true
	case marketRushYds:
		return float64(row.RushYds), true
	case marketRushAtt:
		return float64(row.Carries), true
	case marketRecYds:
		return float64(row.RecYds), true
	case marketReceptions:
		return float64(row.Receptions), true
	case marketAnytimeTD:
		return float64(row.RushTDs + row.RecTDs), true
	case marketKickerPts:
		return float64(row.KickerPts), true
	default:
		return 0, false
	}
}

// LoadPlayerResearchSignals loads L3 nflverse usage, overall and position
// defense rank maps, and opposing defense L5 games for the latest week when
// nflverse data is fresh.
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

	res := PlayerResearchSignals{
		TeamDefProfiles: teamEPA,
	}
	normPos := db.NormalizePosition(position)

	seasonTypes := NFLverseSeasonTypes(week.SeasonType)
	if pWeeks, err := store.NFLversePlayerWeeksForPlayerTypes(
		ctx, athleteID, week.Season, seasonTypes...,
	); err == nil && len(pWeeks) > 0 {
		if normPos == "" {
			normPos = db.NormalizePosition(pWeeks[0].Position)
		}
		populateL3ResearchUsage(&res, pWeeks)
	}

	market := resolveResearchMarket(normPos, activeMarket)
	var posWeeks []db.NFLversePlayerWeek
	if normPos != "" && market != "" {
		if loaded, err := store.NFLversePlayerWeeksForPositionTypes(
			ctx, normPos, week.Season, seasonTypes...,
		); err == nil && len(loaded) > 0 {
			posWeeks = loaded
			res.PosDefRanks = BuildPositionDefenseRanks(posWeeks, normPos, market)
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
	if opp == "" {
		return res
	}
	res.Opponent = opp

	if oppProf := teamEPA[opp]; oppProf != nil {
		res.OppOverallDefRank = oppProf.DefOverallEPARank
		matchupMarket := market
		if matchupMarket == "" {
			matchupMarket = marketPassYds
		}
		if split, rank, ok := MatchupDefenseRank(matchupMarket, oppProf); ok {
			res.OppDefSplit = split
			res.OppDefRank = rank
		}
	}
	if rank, ok := res.PosDefRanks[opp]; ok {
		res.OppPosDefRank = rank
	}
	if normPos != "" && market != "" {
		if oppWeeks, err := store.NFLversePlayerWeeksAgainstOpponent(
			ctx, opp, normPos, week.Season, seasonTypes...,
		); err == nil && len(oppWeeks) > 0 {
			res.OppDefenseL5, res.OppRoleLabel = BuildOppDefenseLast5Comparable(
				oppWeeks, posWeeks, athleteID, normPos, market,
			)
		}
	}
	return res
}

func populateL3ResearchUsage(res *PlayerResearchSignals, rows []db.NFLversePlayerWeek) {
	l3Count := min(3, len(rows))
	if l3Count == 0 {
		return
	}
	var (
		snapSum, targetShareSum, woprSum   float64
		carriesSum, targetsSum, passAttSum float64
		airYardsSum, rzOppsSum, epaSum     float64
	)
	for _, w := range rows[:l3Count] {
		snapSum += w.OffensePct
		targetShareSum += w.TargetShare
		woprSum += w.WOPR
		carriesSum += float64(w.Carries)
		targetsSum += float64(w.Targets)
		passAttSum += float64(w.PassAtt)
		airYardsSum += w.AirYardsShare
		rzOppsSum += float64(w.RZOpps)
		epaSum += w.TotalEPA
	}
	n := float64(l3Count)
	res.HasUsage = true
	res.SnapPct = snapSum / n
	res.TargetShare = targetShareSum / n
	res.WOPR = woprSum / n
	res.CarriesPerGame = carriesSum / n
	res.TargetsPerGame = targetsSum / n
	res.PassAttPerGame = passAttSum / n
	res.AirYardsShare = airYardsSum / n
	res.RZOppsPerGame = rzOppsSum / n
	res.EPAPerGame = epaSum / n
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

// DistinctLegPlayerIDs returns the unique non-empty player IDs referenced by
// legs in first-seen order.
func DistinctLegPlayerIDs(legs []db.Leg) []string {
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

func distinctNonEmptyIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	var out []string
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func distinctGameTeams(games []db.Game) []string {
	seen := make(map[string]struct{}, len(games)*2)
	teams := make([]string, 0, len(games)*2)
	for _, g := range games {
		for _, team := range []string{g.HomeTeam, g.AwayTeam} {
			if team == "" {
				continue
			}
			if _, ok := seen[team]; ok {
				continue
			}
			seen[team] = struct{}{}
			teams = append(teams, team)
		}
	}
	return teams
}

func toAnalyzerInjuries(byTeam map[string][]espn.Injury) map[string][]Injury {
	if len(byTeam) == 0 {
		return nil
	}
	out := make(map[string][]Injury, len(byTeam))
	for team, list := range byTeam {
		if len(list) == 0 {
			continue
		}
		converted := make([]Injury, len(list))
		for i, inj := range list {
			converted[i] = Injury{
				AthleteID: inj.AthleteID,
				FullName:  inj.FullName,
				Position:  inj.Position,
				Status:    inj.Status,
			}
		}
		out[team] = converted
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
