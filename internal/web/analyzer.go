package web

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/db"
)

// nfeloSource is the sync_state row that records the last nfelo fetch.
const nfeloSource = "nfelo:projections"

// nfeloFreshFor is how long a successful nfelo fetch may shape a warning. The
// file changes about every two days, and a stale model is worse than none.
const nfeloFreshFor = 72 * time.Hour

// nflverseFreshFor is how long a successful nflverse sync may shape analyzer
// rules and grades.
const nflverseFreshFor = 7 * 24 * time.Hour

const (
	seasonTypeRegular    = 2
	seasonTypePostseason = 3
)

func nflverseSource(season int) string {
	return fmt.Sprintf("nflverse:season:%d", season)
}

// AnalyzerView is the sharp-friend banner under the slip's legs.
type AnalyzerView struct {
	Clean    bool
	Grade    GradeView
	Warnings []AnalyzerWarningView
}

// GradeView is a clickable A-F grade pill with an inline factor breakdown.
type GradeView struct {
	Letter  string
	Score   int
	Class   string
	Summary string
	Factors []GradeFactorView
}

// GradeFactorView is one scoring adjustment in a grade breakdown.
type GradeFactorView struct {
	Rule      string
	Label     string
	DeltaText string
	Positive  bool
	Negative  bool
	Detail    string
}

// AnalyzerWarningView is one banner card.
type AnalyzerWarningView struct {
	Rule     string
	Severity string
	Class    string
	Message  string
	Legs     []AnalyzerLegView
}

// AnalyzerLegView is one leg a warning names, with the person who owns it.
type AnalyzerLegView struct {
	OwnerID   int64
	Owner     string
	Selection string
}

// AnalyzerGuidePage is the in-app grading rubric and analyzer rule reference.
type AnalyzerGuidePage struct {
	Page
	BackURL    string
	Rubric     analyzer.RubricDoc
	Bands      []GuideBandView
	Categories []GuideRuleCategoryView
}

// GuideBandView is one letter-grade score band on the guide page.
type GuideBandView struct {
	Letter      string
	Range       string
	Description string
	Class       string
}

// GuideRuleCategoryView groups analyzer rules under a category heading.
type GuideRuleCategoryView struct {
	Name  string
	Rules []GuideRuleView
}

// GuideRuleView is one analyzer rule card on the guide page.
type GuideRuleView struct {
	ID            string
	Title         string
	SeverityLabel string
	SeverityClass string
	Threshold     string
	Rationale     string
	Example       string
}

// analyzerGuide serves GET /analyzer/guide, rendering the live analyzer rules
// and A-F grading rubric.
func (h *slipHandlers) analyzerGuide(w http.ResponseWriter, r *http.Request) {
	backURL := safeReturnPath(r.URL.Query().Get("from"))
	rubric := analyzer.GradeRubric()

	bands := make([]GuideBandView, 0, len(rubric.Bands))
	for _, band := range rubric.Bands {
		bands = append(bands, GuideBandView{
			Letter:      band.Letter,
			Range:       band.Range,
			Description: band.Description,
			Class:       gradeClass(band.Letter),
		})
	}

	docs := analyzer.RuleDocs()
	var categories []GuideRuleCategoryView
	catIndex := make(map[string]int)
	for _, doc := range docs {
		sevLabel, sevClass := analyzerSeverity(doc.Severity)
		ruleView := GuideRuleView{
			ID:            doc.ID,
			Title:         doc.Title,
			SeverityLabel: sevLabel,
			SeverityClass: sevClass,
			Threshold:     doc.Threshold,
			Rationale:     doc.Rationale,
			Example:       doc.Example,
		}
		idx, ok := catIndex[doc.Category]
		if !ok {
			idx = len(categories)
			catIndex[doc.Category] = idx
			categories = append(categories, GuideRuleCategoryView{Name: doc.Category})
		}
		categories[idx].Rules = append(categories[idx].Rules, ruleView)
	}

	page := h.page(r, "Grading & Analyzer Guide")
	page.Tab = ""
	h.renderPage(w, "analyzer-guide", AnalyzerGuidePage{
		Page:       page,
		BackURL:    backURL,
		Rubric:     rubric,
		Bands:      bands,
		Categories: categories,
	})
}

func safeReturnPath(raw string) string {
	if raw == "" || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") {
		return ""
	}
	return raw
}

// analyzerGameMap maps the week's games into analyzer inputs. The map is keyed
// by the same ID the legs carry so no leg is dropped. nfelo and nflverse rows
// are only present when the caller has already checked that they are fresh.
func analyzerGameMap(
	games []db.Game,
	nfelo map[string]db.NfeloGame,
	teamEPA map[string]*analyzer.TeamDefenseProfile,
) map[string]analyzer.Game {
	gamesByID := make(map[string]analyzer.Game, len(games))
	for _, game := range games {
		var proj *db.NfeloGame
		if p, ok := nfelo[game.ID]; ok {
			proj = &p
		}
		converted := analyzerGame(game, proj)
		converted.HomeEPA = teamEPA[game.HomeTeam]
		converted.AwayEPA = teamEPA[game.AwayTeam]
		gamesByID[game.ID] = converted
	}
	return gamesByID
}

// analyzerView shapes the analyzer's warnings and deterministic A-F grades for
// the slip banner and each leg row.
func analyzerView(
	legs []db.Leg,
	gamesByID map[string]analyzer.Game,
	stats map[string]analyzer.PlayerStats,
	names map[int64]string,
) (AnalyzerView, map[int64]GradeView) {
	input := make([]analyzer.Leg, 0, len(legs))
	legsByID := make(map[int64]db.Leg, len(legs))
	for _, leg := range legs {
		legsByID[leg.ID] = leg
		input = append(input, analyzerLeg(leg, names))
	}

	warnings := analyzer.AnalyzeSlip(input, gamesByID, stats)
	slipGrade := analyzer.ScoreSlip(input, gamesByID, stats)

	legGrades := make(map[int64]GradeView, len(slipGrade.LegGrades))
	for _, lg := range slipGrade.LegGrades {
		if lg.Letter == "" {
			continue
		}
		legGrades[lg.LegID] = gradeView(lg.Letter, lg.Score, "", lg.Factors)
	}

	view := AnalyzerView{
		Clean: len(warnings) == 0,
		Grade: gradeView(slipGrade.Letter, slipGrade.Score, slipGrade.Summary, slipGrade.Factors),
	}
	for _, warning := range warnings {
		label, class := analyzerSeverity(warning.Severity)
		card := AnalyzerWarningView{
			Rule:     warning.Rule,
			Severity: label,
			Class:    class,
			Message:  warning.Message,
		}
		for _, id := range warning.LegIDs {
			leg, ok := legsByID[id]
			if !ok {
				continue
			}
			card.Legs = append(card.Legs, AnalyzerLegView{
				OwnerID:   leg.UserID,
				Owner:     names[leg.UserID],
				Selection: leg.Selection,
			})
		}
		view.Warnings = append(view.Warnings, card)
	}
	return view, legGrades
}

func gradeView(letter string, score int, summary string, factors []analyzer.GradeFactor) GradeView {
	if letter == "" {
		return GradeView{}
	}
	factorViews := make([]GradeFactorView, 0, len(factors))
	for _, f := range factors {
		factorViews = append(factorViews, GradeFactorView{
			Rule:      f.Rule,
			Label:     f.Label,
			DeltaText: fmt.Sprintf("%+d", f.Delta),
			Positive:  f.Delta > 0,
			Negative:  f.Delta < 0,
			Detail:    f.Detail,
		})
	}
	return GradeView{
		Letter:  letter,
		Score:   score,
		Class:   gradeClass(letter),
		Summary: summary,
		Factors: factorViews,
	}
}

func gradeClass(letter string) string {
	switch letter {
	case "A":
		return "border-emerald-700 bg-emerald-950 text-emerald-300"
	case "B":
		return "border-sky-700 bg-sky-950 text-sky-300"
	case "C":
		return "border-amber-700 bg-amber-950 text-amber-300"
	case "D":
		return "border-orange-700 bg-orange-950 text-orange-300"
	case "F":
		return "border-rose-800 bg-rose-950 text-rose-300"
	default:
		return "border-slate-700 bg-slate-950 text-slate-300"
	}
}

// legModel is the nfelo chip beside a leg's odds. A spread shows the model's
// number from the leg team's side; a moneyline shows the model's win
// probability. Anything else, or a game without model data, has no chip.
func legModel(leg db.Leg, games map[string]analyzer.Game) string {
	if leg.Team == nil || leg.GameID == nil {
		return ""
	}
	game, ok := games[*leg.GameID]
	if !ok || game.Nfelo == nil {
		return ""
	}
	switch leg.MarketType {
	case "SPREAD":
		spread, ok := game.ModelSpread(*leg.Team)
		if !ok {
			return ""
		}
		return "nfelo " + formatModelSpread(spread)
	case "ML":
		probability, ok := game.ModelWinProb(*leg.Team)
		if !ok {
			return ""
		}
		return fmt.Sprintf("nfelo %d%%", int(math.Round(probability*100)))
	default:
		return ""
	}
}

// formatModelSpread prints a model spread with one decimal and a plus on a
// positive number.
func formatModelSpread(spread float64) string {
	text := strconv.FormatFloat(spread, 'f', 1, 64)
	if spread > 0 {
		return "+" + text
	}
	return text
}

// analyzerGame carries the slice of a stored game a rule reads. nfelo data is
// included only for a scheduled game, because a live or final game's model no
// longer describes the game in front of the slip.
func analyzerGame(game db.Game, projection *db.NfeloGame) analyzer.Game {
	converted := analyzer.Game{
		ID:     game.ID,
		Home:   game.HomeTeam,
		Away:   game.AwayTeam,
		Indoor: game.Indoor,
	}
	if projection != nil && game.Status == db.GameScheduled {
		converted.Nfelo = &analyzer.Nfelo{
			HomeSpread:  projection.HomeSpread,
			HomeWinProb: projection.HomeWinProb,
			HomeQB:      projection.HomeQB,
			AwayQB:      projection.AwayQB,
			HomeQBAdj:   projection.HomeQBAdj,
			AwayQBAdj:   projection.AwayQBAdj,
		}
	}
	if game.WindMPH != nil {
		weather := analyzer.Weather{WindMPH: *game.WindMPH}
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

// analyzerLeg carries the pick fields a rule reads.
func analyzerLeg(leg db.Leg, names map[int64]string) analyzer.Leg {
	converted := analyzer.Leg{
		ID:     leg.ID,
		Owner:  names[leg.UserID],
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

// nfeloForGames reads the model rows the analyzer may use. It returns none
// when nfelo has never synced or its last good fetch is older than 72 hours, so
// stale data cannot shape a sentence. A read error on one game just leaves that
// game without a model.
func (h *slipHandlers) nfeloForGames(
	ctx context.Context,
	games []db.Game,
) map[string]db.NfeloGame {
	state, err := h.deps.Store.SyncState(ctx, nfeloSource)
	if err != nil || h.deps.now().Sub(state.FetchedAt) > nfeloFreshFor {
		return nil
	}
	rows := make(map[string]db.NfeloGame, len(games))
	for _, game := range games {
		projection, ok, err := h.deps.Store.NfeloForGame(ctx, game.ID)
		if err != nil || !ok {
			continue
		}
		rows[game.ID] = projection
	}
	return rows
}

// nflverseWeekForSlip loads fresh nflverse team EPA/pace profiles and player
// usage for the slip's week. Missing or stale (> 7 days) sync state fails soft
// by returning nil maps.
func (h *slipHandlers) nflverseForSlip(
	ctx context.Context,
	weekID int64,
	legs []db.Leg,
) (map[string]*analyzer.TeamDefenseProfile, map[string]*analyzer.PlayerUsage) {
	week, err := h.deps.Store.WeekByID(ctx, weekID)
	if err != nil {
		return nil, nil
	}
	state, err := h.deps.Store.SyncState(ctx, nflverseSource(week.Season))
	if err != nil || h.deps.now().Sub(state.FetchedAt) > nflverseFreshFor {
		return nil, nil
	}

	seasonType := week.SeasonType
	teamWeeks, err := h.deps.Store.NFLverseTeamWeeksForSeason(ctx, week.Season, seasonType)
	if err == nil && len(teamWeeks) == 0 && seasonType == seasonTypePostseason {
		seasonType = seasonTypeRegular
		teamWeeks, err = h.deps.Store.NFLverseTeamWeeksForSeason(ctx, week.Season, seasonType)
	}
	var teamProfiles map[string]*analyzer.TeamDefenseProfile
	if err == nil && len(teamWeeks) > 0 {
		teamProfiles = buildTeamDefenseProfiles(teamWeeks)
	}

	usageByPlayer := h.nflverseUsageForLegs(ctx, legs, week.Season, week.SeasonType)
	return teamProfiles, usageByPlayer
}

func (h *slipHandlers) nflverseUsageForLegs(
	ctx context.Context,
	legs []db.Leg,
	season, seasonType int,
) map[string]*analyzer.PlayerUsage {
	seen := make(map[string]struct{}, len(legs))
	usageByPlayer := make(map[string]*analyzer.PlayerUsage)
	for _, leg := range legs {
		if leg.PlayerID == nil || *leg.PlayerID == "" {
			continue
		}
		id := *leg.PlayerID
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}

		rows, err := h.deps.Store.NFLversePlayerWeeksForPlayer(ctx, id, season, seasonType)
		if err == nil && len(rows) < 3 && seasonType == seasonTypePostseason {
			rows, err = h.deps.Store.NFLversePlayerWeeksForPlayer(ctx, id, season, seasonTypeRegular)
		}
		if err != nil {
			continue
		}
		if usage := buildPlayerUsage(rows); usage != nil {
			usageByPlayer[id] = usage
		}
	}
	return usageByPlayer
}

func buildTeamDefenseProfiles(rows []db.NFLverseTeamWeek) map[string]*analyzer.TeamDefenseProfile {
	byTeam := make(map[string][]db.NFLverseTeamWeek)
	for _, row := range rows {
		byTeam[row.Team] = append(byTeam[row.Team], row)
	}
	if len(byTeam) == 0 {
		return nil
	}

	profiles := make(map[string]*analyzer.TeamDefenseProfile, len(byTeam))
	teams := make([]string, 0, len(byTeam))
	for team, weeks := range byTeam {
		teams = append(teams, team)
		prof := &analyzer.TeamDefenseProfile{}
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

func buildPlayerUsage(rows []db.NFLversePlayerWeek) *analyzer.PlayerUsage {
	if len(rows) < 3 {
		return nil
	}
	weeks := make([]analyzer.PlayerWeekUsage, 0, len(rows))
	var seasonSnap, seasonTarget float64
	var l3Snap, l3Target float64
	for i, row := range rows {
		weeks = append(weeks, analyzer.PlayerWeekUsage{
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
		if i < 3 {
			l3Snap += row.OffensePct
			l3Target += row.TargetShare
		}
	}
	n := float64(len(rows))
	return &analyzer.PlayerUsage{
		Weeks:             weeks,
		Last3SnapAvg:      l3Snap / 3,
		SeasonSnapAvg:     seasonSnap / n,
		Last3TargetShare:  l3Target / 3,
		SeasonTargetShare: seasonTarget / n,
	}
}

// playerStatsForLegs loads player_gamelogs and optional nflverse usage for
// every distinct matched athlete on the slip. A missing row or read error leaves
// that player without stats.
func (h *slipHandlers) playerStatsForLegs(
	ctx context.Context,
	legs []db.Leg,
	usageByPlayer map[string]*analyzer.PlayerUsage,
) map[string]analyzer.PlayerStats {
	seen := make(map[string]struct{}, len(legs))
	var athleteIDs []string
	for _, leg := range legs {
		if leg.PlayerID == nil || *leg.PlayerID == "" {
			continue
		}
		id := *leg.PlayerID
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			athleteIDs = append(athleteIDs, id)
		}
	}
	if len(athleteIDs) == 0 {
		return nil
	}

	logs, err := h.deps.Store.PlayerGameLogs(ctx, athleteIDs)
	if err != nil {
		logs = nil
	}

	stats := make(map[string]analyzer.PlayerStats, len(athleteIDs))
	for _, id := range athleteIDs {
		var markets map[string][]float64
		if log, ok := logs[id]; ok {
			_ = json.Unmarshal([]byte(log.StatsJSON), &markets)
		}
		usage := usageByPlayer[id]
		if len(markets) == 0 && usage == nil {
			continue
		}
		stats[id] = analyzer.PlayerStats{
			Games: markets,
			Usage: usage,
		}
	}
	return stats
}

// analyzerSeverity names the card's heading and the classes that color it.
func analyzerSeverity(severity analyzer.Severity) (string, string) {
	switch severity {
	case analyzer.Danger:
		return "Danger", "border-rose-800 bg-rose-950 text-rose-200"
	case analyzer.Warn:
		return "Warn", "border-amber-700 bg-amber-950 text-amber-300"
	default:
		return "Info", "border-sky-700 bg-sky-950 text-sky-300"
	}
}
