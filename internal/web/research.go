package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/espn"
)

// researchHandlers serves the Research tab and partials.
type researchHandlers struct {
	deps Dependencies
}

// Routes mounts the Research tab and endpoints.
func (h *researchHandlers) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /research", h.research)
	mux.HandleFunc("GET /research/games/{id}", h.researchGame)
	mux.HandleFunc("GET /research/players/{espnID}", h.researchPlayer)
	mux.HandleFunc("POST /research/matches", h.saveMatch)
}

// ResearchPage is the full Research tab.
type ResearchPage struct {
	Page
	WeekLabel        string
	Games            []ResearchGameOption
	Players          []ResearchPlayerOption
	SelectedGameID   string
	SelectedPlayerID string
	GameResearch     *GameResearchView
	PlayerResearch   *PlayerResearchView
}

// ResearchGameOption is one game in the selector.
type ResearchGameOption struct {
	ID       string
	Label    string
	Selected bool
}

// ResearchPlayerOption is one athlete in the player picker.
type ResearchPlayerOption struct {
	ID       string
	Name     string
	Team     string
	Position string
	Selected bool
}

// GameResearchView is the view for game injuries and player matching.
type GameResearchView struct {
	GameID        string
	GameTitle     string
	HomeTeam      string
	AwayTeam      string
	HomeLogo      string
	AwayLogo      string
	Injuries      []TeamInjuriesView
	Matches       []PropMatchView
	RosterOptions []RosterOptionView
	HasProps      bool
}

// TeamInjuriesView is one team's filtered injuries in a game.
type TeamInjuriesView struct {
	Team         string
	Abbreviation string
	LogoURL      string
	Injuries     []InjuryView
}

// InjuryView is one athlete's injury report.
type InjuryView struct {
	AthleteID   string
	FullName    string
	Position    string
	HeadshotURL string
	Initials    string
	Status      string
	Detail      string
}

// PropMatchView is one sportsbook prop player and their match state.
type PropMatchView struct {
	OddsName      string
	Team          string
	ESPNAthleteID string
	AthleteName   string
	Position      string
	HeadshotURL   string
	Initials      string
	Manual        bool
	Matched       bool
}

// RosterOptionView is one candidate athlete for manual matching.
type RosterOptionView struct {
	AthleteID string
	FullName  string
	Team      string
	Position  string
}

// PlayerResearchView is the view for athlete hit bars and market stats.
type PlayerResearchView struct {
	AthleteID        string
	Name             string
	Team             string
	Position         string
	HeadshotURL      string
	Initials         string
	TeamLogo         string
	InjuryStatus     string
	GamesPlayed      int
	ActiveMarket     string
	ActiveMarketName string
	ComparisonLine   *float64
	HasLine          bool
	LineStr          string
	L5Median         string
	SeasonMean       string
	L5HitRate        string
	SeasonHitRate    string
	L5MinMax         string
	SnapPct          string
	TargetShare      string
	WOPR             string
	OppDefLabel      string
	OppDefRank       string
	Bars             []HitBarView
	PlayerChart      BarChartView
	OppDefenseTitle  string
	OppOverallBadge  *DefRankBadge
	OppPosBadge      *DefRankBadge
	OppDefenseChart  *BarChartView
	Matchup          *MatchupContextView
	UsageTiles       []UsageTileView
	AvailableMarkets []MarketOptionView
}

// BarChartView holds a shared Last-5 bar chart with an optional dotted O/U line.
type BarChartView struct {
	HasLine bool
	LineStr string
	LinePct int
	Bars    []HitBarView
}

// DefRankBadge is a color-tinted defense rank pill (#1-#10 tough, #11-#22 neutral, #23-#32 soft).
type DefRankBadge struct {
	Rank       int
	Label      string
	BadgeClass string
}

// MatchupContextView holds this week's game context for the player's team.
type MatchupContextView struct {
	GameID        string
	Opponent      string
	OpponentLabel string
	OpponentLogo  string
	Spread        string
	GameTotal     string
	Weather       string
}

// UsageTileView is one position-tailored stat tile in the research card.
type UsageTileView struct {
	Label string
	Value string
}

// HitBarView is one game's bar in a last-5 chart.
type HitBarView struct {
	Index         int
	Label         string
	SubLabel      string
	Value         float64
	ValueStr      string
	HeightPct     int
	ResultClass   string
	ResultText    string
	OpponentLabel string
	OverallDef    *DefRankBadge
	PosDef        *DefRankBadge
}

// MarketOptionView is one selectable market for the athlete.
type MarketOptionView struct {
	Market string
	Label  string
	Active bool
	Href   string
}

// research renders the full research page or HTMX partials.
func (h *researchHandlers) research(w http.ResponseWriter, r *http.Request) {
	page := ResearchPage{Page: h.page(r)}

	if h.deps.Store == nil {
		h.render(w, page)
		return
	}

	week, err := h.deps.Store.LatestWeek(r.Context())
	if err == nil {
		page.WeekLabel = weekLabel(week)
		games, err := h.deps.Store.GamesForWeek(r.Context(), week.ID)
		if err == nil {
			teams := make([]string, 0, len(games)*2)
			for _, g := range games {
				label := fmt.Sprintf("%s @ %s", g.AwayTeam, g.HomeTeam)
				if g.AwayTeam == "" || g.HomeTeam == "" {
					label = g.ID
				}
				page.Games = append(page.Games, ResearchGameOption{
					ID:    g.ID,
					Label: label,
				})
				if g.HomeTeam != "" {
					teams = append(teams, g.HomeTeam)
				}
				if g.AwayTeam != "" {
					teams = append(teams, g.AwayTeam)
				}
			}

			if len(teams) > 0 {
				rosterPlayers, err := h.deps.Store.RosterForTeams(r.Context(), teams)
				if err == nil {
					for _, p := range rosterPlayers {
						page.Players = append(page.Players, ResearchPlayerOption{
							ID:       p.AthleteID,
							Name:     p.FullName,
							Team:     p.Team,
							Position: p.Position,
						})
					}
					slices.SortFunc(page.Players, func(a, b ResearchPlayerOption) int {
						return strings.Compare(a.Name, b.Name)
					})
				}
			}
		}
	}

	gameID := strings.TrimSpace(r.URL.Query().Get("game_id"))
	playerID := strings.TrimSpace(r.URL.Query().Get("player_id"))
	if playerID == "" {
		if playerName := strings.TrimSpace(r.URL.Query().Get("player")); playerName != "" {
			norm := db.NormalizePlayerName(playerName)
			for _, p := range page.Players {
				if db.NormalizePlayerName(p.Name) == norm {
					playerID = p.ID
					break
				}
			}
		}
	}

	legIDStr := strings.TrimSpace(r.URL.Query().Get("leg_id"))
	market := strings.TrimSpace(r.URL.Query().Get("market"))
	lineStr := strings.TrimSpace(r.URL.Query().Get("line"))

	isHTMX := r.Header.Get("Hx-Request") != ""

	if playerID != "" {
		view, err := h.loadPlayerResearch(r.Context(), playerID, legIDStr, market, lineStr)
		if err == nil {
			if isHTMX && gameID == "" {
				h.renderPartial(w, "research-player", view)
				return
			}
			page.PlayerResearch = view
			page.SelectedPlayerID = playerID
			for i := range page.Players {
				if page.Players[i].ID == playerID {
					page.Players[i].Selected = true
				}
			}
		}
	} else if gameID != "" {
		view, err := h.loadGameResearch(r.Context(), gameID)
		if err == nil {
			if isHTMX {
				h.renderPartial(w, "research-game", view)
				return
			}
			page.GameResearch = view
			page.SelectedGameID = gameID
			for i := range page.Games {
				if page.Games[i].ID == gameID {
					page.Games[i].Selected = true
				}
			}
		}
	}

	h.render(w, page)
}

// researchGame serves the game research partial.
func (h *researchHandlers) researchGame(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("id")
	if gameID == "" || h.deps.Store == nil {
		http.NotFound(w, r)
		return
	}

	view, err := h.loadGameResearch(r.Context(), gameID)
	if errors.Is(err, db.ErrGameNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		h.deps.Logger.Error().Err(err).Str("game_id", gameID).Msg("load game research")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	h.renderPartial(w, "research-game", view)
}

// researchPlayer serves the player research partial.
func (h *researchHandlers) researchPlayer(w http.ResponseWriter, r *http.Request) {
	espnID := r.PathValue("espnID")
	if espnID == "" || h.deps.Store == nil {
		http.NotFound(w, r)
		return
	}

	legIDStr := strings.TrimSpace(r.URL.Query().Get("leg_id"))
	market := strings.TrimSpace(r.URL.Query().Get("market"))
	lineStr := strings.TrimSpace(r.URL.Query().Get("line"))

	view, err := h.loadPlayerResearch(r.Context(), espnID, legIDStr, market, lineStr)
	if errors.Is(err, db.ErrPlayerNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		h.deps.Logger.Error().Err(err).Str("athlete_id", espnID).Msg("load player research")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	h.renderPartial(w, "research-player", view)
}

// saveMatch saves a manual player match fix.
func (h *researchHandlers) saveMatch(w http.ResponseWriter, r *http.Request) {
	if h.deps.Store == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	oddsName := strings.TrimSpace(r.PostForm.Get("odds_name"))
	if oddsName == "" {
		oddsName = strings.TrimSpace(r.PostForm.Get("player_name"))
	}
	team := strings.TrimSpace(r.PostForm.Get("team"))
	athleteID := strings.TrimSpace(r.PostForm.Get("athlete_id"))
	gameID := strings.TrimSpace(r.PostForm.Get("game_id"))

	if oddsName == "" || team == "" || athleteID == "" {
		http.Error(w, "missing required match fields", http.StatusBadRequest)
		return
	}

	if err := h.deps.Store.SaveManualPlayerMatch(r.Context(), oddsName, team, athleteID); err != nil {
		h.deps.Logger.Error().Err(err).Msg("save manual match")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if r.Header.Get("Hx-Request") != "" && gameID != "" {
		view, err := h.loadGameResearch(r.Context(), gameID)
		if err == nil {
			h.renderPartial(w, "research-game", view)
			return
		}
	}

	if gameID != "" {
		http.Redirect(w, r, "/research?game_id="+url.QueryEscape(gameID), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/research", http.StatusSeeOther)
}

// loadGameResearch builds the GameResearchView for one game.
func (h *researchHandlers) loadGameResearch(ctx context.Context, gameID string) (*GameResearchView, error) {
	if h.deps.Refresh != nil {
		if err := h.deps.Refresh.SyncGameResearch(ctx, gameID); err != nil {
			h.deps.Logger.Warn().Err(err).Str("game_id", gameID).Msg("sync game research")
		}
	}

	game, err := h.deps.Store.GameByID(ctx, gameID)
	if err != nil {
		return nil, err
	}

	view := &GameResearchView{
		GameID:    game.ID,
		GameTitle: fmt.Sprintf("%s @ %s", game.AwayTeam, game.HomeTeam),
		HomeTeam:  game.HomeTeam,
		AwayTeam:  game.AwayTeam,
		HomeLogo:  db.StringValue(game.HomeLogo),
		AwayLogo:  db.StringValue(game.AwayLogo),
	}
	if game.AwayTeam == "" || game.HomeTeam == "" {
		view.GameTitle = game.ID
	}

	// Roster options
	teams := []string{game.HomeTeam, game.AwayTeam}
	rosterPlayers, err := h.deps.Store.RosterForTeams(ctx, teams)
	rosterMap := make(map[string]db.RosterPlayer, len(rosterPlayers))
	if err == nil {
		for _, p := range rosterPlayers {
			rosterMap[p.AthleteID] = p
			view.RosterOptions = append(view.RosterOptions, RosterOptionView{
				AthleteID: p.AthleteID,
				FullName:  p.FullName,
				Team:      p.Team,
				Position:  p.Position,
			})
		}
		slices.SortFunc(view.RosterOptions, func(a, b RosterOptionView) int {
			return strings.Compare(a.FullName, b.FullName)
		})
	}

	// Parse injuries
	view.Injuries = buildInjuryViews(game, rosterPlayers, rosterMap)

	playerMatches, err := h.deps.Store.PlayerMatchesForTeams(ctx, teams)
	matchesByNormTeam := make(map[string]db.PlayerMatch)
	if err == nil {
		for _, m := range playerMatches {
			matchesByNormTeam[m.OddsName+"|"+m.Team] = m
		}
	}

	// Odds and player matches
	oddsRows, err := h.deps.Store.OddsForGame(ctx, game.ID)
	if err == nil {
		resolver := db.NewBatchPlayerResolver(rosterPlayers, playerMatches)
		seenPlayers := make(map[string]bool)
		for _, o := range oddsRows {
			name := strings.TrimSpace(o.PlayerName)
			if name == "" || seenPlayers[name] {
				continue
			}
			seenPlayers[name] = true
			view.HasProps = true

			resolved := resolver.ResolveGamePlayer(game.ID, name, o.Team, game.HomeTeam, game.AwayTeam)
			mv := PropMatchView{
				OddsName: name,
				Team:     resolved.Team,
			}
			if mv.Team == "" {
				mv.Team = o.Team
			}
			if resolved.Linked {
				mv.Matched = true
				mv.ESPNAthleteID = resolved.AthleteID
				mv.AthleteName = resolved.Name
				mv.Position = resolved.Position
				mv.HeadshotURL = resolved.HeadshotURL
				mv.Initials = Initials(name)
				if resolved.Name != "" {
					mv.Initials = Initials(resolved.Name)
				}
			}
			norm := db.NormalizePlayerName(name)
			if m, ok := matchesByNormTeam[norm+"|"+mv.Team]; ok {
				mv.Manual = m.Manual
			} else {
				for _, m := range playerMatches {
					if m.OddsName == norm && (m.Team == mv.Team || m.ESPNAthleteID == resolved.AthleteID) {
						mv.Manual = m.Manual
						break
					}
				}
			}
			view.Matches = append(view.Matches, mv)
		}
		if newMatches := resolver.NewMatches(); len(newMatches) > 0 {
			if err := h.deps.Store.UpsertPlayerMatches(ctx, newMatches); err != nil {
				h.deps.Logger.Warn().Err(err).Str("game_id", gameID).Msg("upsert research player matches")
			}
		}
		slices.SortFunc(view.Matches, func(a, b PropMatchView) int {
			return strings.Compare(a.OddsName, b.OddsName)
		})
	}

	return view, nil
}

// loadPlayerResearch builds the PlayerResearchView for one athlete.
func (h *researchHandlers) loadPlayerResearch(
	ctx context.Context,
	athleteID, legIDStr, marketParam, lineParam string,
) (*PlayerResearchView, error) {
	if h.deps.Refresh != nil {
		if err := h.deps.Refresh.SyncPlayerResearch(ctx, athleteID); err != nil {
			h.deps.Logger.Warn().Err(err).Str("athlete_id", athleteID).Msg("sync player research")
		}
	}

	player, playerErr := h.deps.Store.RosterPlayerByID(ctx, athleteID)
	gamelog, logErr := h.deps.Store.PlayerGameLog(ctx, athleteID)

	if playerErr != nil && logErr != nil {
		return nil, db.ErrPlayerNotFound
	}

	view := &PlayerResearchView{
		AthleteID:    athleteID,
		Name:         player.FullName,
		Team:         player.Team,
		Position:     player.Position,
		HeadshotURL:  player.HeadshotURL,
		InjuryStatus: strings.TrimSpace(player.InjuryStatus),
	}
	if view.Name == "" {
		view.Name = "Athlete #" + athleteID
	}
	view.Initials = Initials(view.Name)

	var logos map[string]string
	if loadedLogos, err := h.deps.Store.TeamLogos(ctx); err == nil {
		logos = loadedLogos
		if view.Team != "" {
			view.TeamLogo = logos[view.Team]
		}
	}

	var markets map[string][]float64
	var entries []espn.GameLogEntry
	if logErr == nil {
		view.GamesPlayed = gamelog.GamesPlayed
		_ = json.Unmarshal([]byte(gamelog.StatsJSON), &markets)
		if gamelog.GamesJSON != "" {
			_ = json.Unmarshal([]byte(gamelog.GamesJSON), &entries)
		}
	}

	activeMarket, comparisonLine := h.resolveMarketAndLine(
		ctx, athleteID, player, markets, legIDStr, marketParam, lineParam,
	)

	view.ActiveMarket = activeMarket
	view.ActiveMarketName = db.MarketLabel(activeMarket)
	if view.ActiveMarketName == "" {
		view.ActiveMarketName = activeMarket
	}
	if comparisonLine != nil {
		view.ComparisonLine = comparisonLine
		view.HasLine = true
		view.LineStr = formatNumber(*comparisonLine)
	}

	view.AvailableMarkets = buildMarketOptions(athleteID, markets, activeMarket)

	h.populateMatchupContext(ctx, view, logos)
	sig := h.populateNFLverseResearch(ctx, view, activeMarket, comparisonLine)

	games := markets[activeMarket]
	if len(games) > 0 {
		buildPlayerChart(view, games, entries, comparisonLine, sig)
	}

	return view, nil
}

// populateMatchupContext loads this week's opponent, cached spread/total, and
// weather/indoor status for the player's team.
func (h *researchHandlers) populateMatchupContext(
	ctx context.Context,
	view *PlayerResearchView,
	logos map[string]string,
) {
	if view.Team == "" {
		return
	}
	week, err := h.deps.Store.LatestWeek(ctx)
	if err != nil {
		return
	}
	weekGames, err := h.deps.Store.GamesForWeek(ctx, week.ID)
	if err != nil {
		return
	}

	var game *db.Game
	var opp string
	var isHome bool
	for i := range weekGames {
		g := &weekGames[i]
		switch view.Team {
		case g.HomeTeam:
			game = g
			opp = g.AwayTeam
			isHome = true
		case g.AwayTeam:
			game = g
			opp = g.HomeTeam
			isHome = false
		}
		if game != nil {
			break
		}
	}
	if game == nil || opp == "" {
		return
	}

	oppLabel := "@ " + opp
	if isHome {
		oppLabel = "vs " + opp
	}
	mc := &MatchupContextView{
		GameID:        game.ID,
		Opponent:      opp,
		OpponentLabel: oppLabel,
		OpponentLogo:  logos[opp],
		Weather:       formatGameWeather(*game),
	}

	if oddsRows, err := h.deps.Store.OddsForGame(ctx, game.ID); err == nil {
		for _, o := range oddsRows {
			if o.Line == nil {
				continue
			}
			if mc.Spread == "" && o.MarketType == "SPREAD" && o.Team == view.Team {
				mc.Spread = formatSignedNumber(*o.Line)
			}
			if mc.GameTotal == "" && o.MarketType == "TOTAL" {
				mc.GameTotal = formatNumber(*o.Line)
			}
			if mc.Spread != "" && mc.GameTotal != "" {
				break
			}
		}
	}

	view.Matchup = mc
}

// populateNFLverseResearch attaches position-tailored usage tiles, opponent
// defense ranks, and the opposing defense L5 chart when fresh nflverse data exists.
func (h *researchHandlers) populateNFLverseResearch(
	ctx context.Context,
	view *PlayerResearchView,
	activeMarket string,
	comparisonLine *float64,
) analyzer.PlayerResearchSignals {
	sig := analyzer.LoadPlayerResearchSignals(
		ctx, h.deps.Store, h.deps.now(), view.AthleteID, view.Team, view.Position, activeMarket,
	)
	if sig.HasUsage {
		view.SnapPct = fmt.Sprintf("%.0f%%", sig.SnapPct*100)
		view.TargetShare = fmt.Sprintf("%.1f%%", sig.TargetShare*100)
		view.WOPR = fmt.Sprintf("%.2f", sig.WOPR)
	}
	if sig.OppDefRank > 0 {
		if sig.OppDefSplit == "rush" {
			view.OppDefLabel = "Opp Rush Def"
		} else {
			view.OppDefLabel = "Opp Pass Def"
		}
		view.OppDefRank = fmt.Sprintf("#%d vs %s", sig.OppDefRank, sig.Opponent)
	}

	view.UsageTiles = buildUsageTiles(view.Position, sig, view.OppDefLabel, view.OppDefRank)

	normPos := normalizeResearchPosition(view.Position)
	if sig.Opponent != "" && normPos != "" && view.ActiveMarketName != "" {
		view.OppDefenseTitle = fmt.Sprintf(
			"%s Defense · Last 5 vs %s · %s",
			sig.Opponent,
			normPos,
			view.ActiveMarketName,
		)
	}
	if sig.OppOverallDefRank > 0 {
		view.OppOverallBadge = newDefRankBadge(sig.OppOverallDefRank, fmt.Sprintf("Def #%d", sig.OppOverallDefRank))
	}
	if sig.OppPosDefRank > 0 && normPos != "" {
		view.OppPosBadge = newDefRankBadge(sig.OppPosDefRank, fmt.Sprintf("vs %s #%d", normPos, sig.OppPosDefRank))
	}
	if len(sig.OppDefenseL5) > 0 {
		chart := buildOppDefenseChart(sig.OppDefenseL5, comparisonLine)
		view.OppDefenseChart = &chart
	}

	return sig
}

func (h *researchHandlers) resolveMarketAndLine(
	ctx context.Context,
	athleteID string,
	player db.RosterPlayer,
	markets map[string][]float64,
	legIDStr, marketParam, lineParam string,
) (string, *float64) {
	activeMarket := marketParam
	var comparisonLine *float64
	if lineParam != "" {
		if parsed, err := strconv.ParseFloat(lineParam, 64); err == nil {
			comparisonLine = &parsed
		}
	}

	var legMarket string
	var legLine *float64
	if legIDStr != "" {
		if legID, err := strconv.ParseInt(legIDStr, 10, 64); err == nil {
			if leg, err := h.deps.Store.LegByID(ctx, legID); err == nil {
				legMarket = leg.MarketType
				legLine = leg.Line
				if activeMarket == "" && legMarket != "" {
					activeMarket = legMarket
				}
			}
		}
	}

	if activeMarket == "" && (player.FullName != "" || athleteID != "") {
		prop, ok, err := h.deps.Store.LatestPropForPlayer(ctx, athleteID, player.FullName, player.Team)
		if err == nil && ok {
			if prop.MarketType != "" {
				activeMarket = prop.MarketType
			}
			if comparisonLine == nil && prop.Line != nil {
				comparisonLine = prop.Line
			}
		}
	}

	if activeMarket == "" && len(markets) > 0 {
		activeMarket = defaultResearchMarket(markets)
	}

	if comparisonLine == nil && legLine != nil && (legMarket == "" || legMarket == activeMarket) {
		comparisonLine = legLine
	}

	if comparisonLine == nil && activeMarket != "" && (player.FullName != "" || athleteID != "") {
		prop, ok, err := h.deps.Store.LatestPropForPlayerMarket(
			ctx, athleteID, player.FullName, player.Team, activeMarket,
		)
		if err == nil && ok && prop.Line != nil {
			comparisonLine = prop.Line
		}
	}

	return activeMarket, comparisonLine
}

func defaultResearchMarket(markets map[string][]float64) string {
	standardOrder := []string{
		"PASS_YDS", "RUSH_YDS", "REC_YDS", "RECEPTIONS",
		"PASS_TDS", "RUSH_ATT", "ANYTIME_TD", "KICKER_PTS",
	}
	for _, m := range standardOrder {
		if len(markets[m]) > 0 {
			return m
		}
	}
	for m, vals := range markets {
		if len(vals) > 0 {
			return m
		}
	}
	return ""
}

func buildMarketOptions(
	athleteID string,
	markets map[string][]float64,
	activeMarket string,
) []MarketOptionView {
	standardMarkets := []string{
		"PASS_YDS", "PASS_TDS", "RUSH_YDS", "RUSH_ATT",
		"REC_YDS", "RECEPTIONS", "ANYTIME_TD", "KICKER_PTS",
	}
	var options []MarketOptionView
	for _, m := range standardMarkets {
		if len(markets[m]) == 0 {
			continue
		}
		href := fmt.Sprintf("/research/players/%s?market=%s", athleteID, m)
		label := db.MarketLabel(m)
		if label == "" {
			label = m
		}
		options = append(options, MarketOptionView{
			Market: m,
			Label:  label,
			Active: m == activeMarket,
			Href:   href,
		})
	}
	return options
}

func buildPlayerChart(
	view *PlayerResearchView,
	games []float64,
	entries []espn.GameLogEntry,
	comparisonLine *float64,
	sig analyzer.PlayerResearchSignals,
) {
	l5Len := min(5, len(games))
	l5 := games[:l5Len]
	view.L5Median = formatNumber(analyzer.Median(l5))
	view.SeasonMean = fmt.Sprintf("%.1f", analyzer.Mean(games))

	minVal, maxVal := l5[0], l5[0]
	for _, v := range l5[1:] {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}
	view.L5MinMax = fmt.Sprintf("%s–%s", formatNumber(minVal), formatNumber(maxVal))

	if comparisonLine != nil {
		l5Hits := 0
		for _, v := range l5 {
			if v > *comparisonLine {
				l5Hits++
			}
		}
		view.L5HitRate = fmt.Sprintf("%d/%d", l5Hits, l5Len)

		seasonHits := 0
		for _, v := range games {
			if v > *comparisonLine {
				seasonHits++
			}
		}
		view.SeasonHitRate = fmt.Sprintf("%d/%d", seasonHits, len(games))
	}

	fallbackLabels := []string{"L5", "L4", "L3", "L2", "Last"}
	if l5Len < 5 {
		fallbackLabels = fallbackLabels[5-l5Len:]
	}

	normPos := normalizeResearchPosition(view.Position)
	rawBars := make([]HitBarView, 0, l5Len)
	for i := range l5Len {
		srcIdx := l5Len - 1 - i
		val := l5[srcIdx]
		bar := HitBarView{
			Index:    i + 1,
			Label:    fallbackLabels[i],
			Value:    val,
			ValueStr: formatNumber(val),
		}
		if srcIdx < len(entries) {
			entry := entries[srcIdx]
			if oppLabel := formatEntryOpponent(entry); oppLabel != "" {
				bar.OpponentLabel = oppLabel
				bar.Label = oppLabel
			}
			if entry.Opponent != "" {
				if prof := sig.TeamDefProfiles[entry.Opponent]; prof != nil && prof.DefOverallEPARank > 0 {
					bar.OverallDef = newDefRankBadge(
						prof.DefOverallEPARank,
						fmt.Sprintf("Def #%d", prof.DefOverallEPARank),
					)
				}
				if posRank, ok := sig.PosDefRanks[entry.Opponent]; ok && posRank > 0 && normPos != "" {
					bar.PosDef = newDefRankBadge(posRank, fmt.Sprintf("vs %s #%d", normPos, posRank))
				}
			}
		}
		rawBars = append(rawBars, bar)
	}

	chart := buildBarChart(rawBars, comparisonLine)
	view.PlayerChart = chart
	view.Bars = chart.Bars
}

func buildOppDefenseChart(games []analyzer.OppDefenseGame, comparisonLine *float64) BarChartView {
	rawBars := make([]HitBarView, 0, len(games))
	for i, g := range games {
		subLabel := fmt.Sprintf("W%d", g.WeekNum)
		if g.Team != "" {
			subLabel = fmt.Sprintf("%s · W%d", g.Team, g.WeekNum)
		}
		rawBars = append(rawBars, HitBarView{
			Index:    i + 1,
			Label:    abbreviatePlayerName(g.PlayerName),
			SubLabel: subLabel,
			Value:    g.Value,
			ValueStr: formatNumber(g.Value),
		})
	}
	return buildBarChart(rawBars, comparisonLine)
}

func buildBarChart(bars []HitBarView, comparisonLine *float64) BarChartView {
	maxVal := 0.0
	for _, b := range bars {
		if b.Value > maxVal {
			maxVal = b.Value
		}
	}
	if comparisonLine != nil && *comparisonLine > maxVal {
		maxVal = *comparisonLine
	}
	if maxVal <= 0 {
		maxVal = 1
	}

	out := make([]HitBarView, len(bars))
	for i, bar := range bars {
		pct := int(math.Round((bar.Value / maxVal) * 100))
		bar.HeightPct = max(0, min(100, pct))

		if comparisonLine != nil {
			switch {
			case bar.Value > *comparisonLine:
				bar.ResultClass = "bg-emerald-500 border-emerald-400 text-emerald-300"
				bar.ResultText = "OVER"
			case bar.Value < *comparisonLine:
				bar.ResultClass = "bg-rose-500 border-rose-400 text-rose-300"
				bar.ResultText = "UNDER"
			default:
				bar.ResultClass = "bg-amber-500 border-amber-400 text-amber-300"
				bar.ResultText = "PUSH"
			}
		} else {
			bar.ResultClass = "bg-sky-500 border-sky-400 text-sky-300"
		}
		out[i] = bar
	}

	chart := BarChartView{Bars: out}
	if comparisonLine != nil {
		linePct := int(math.Round((*comparisonLine / maxVal) * 100))
		chart.HasLine = true
		chart.LineStr = formatNumber(*comparisonLine)
		chart.LinePct = max(0, min(100, linePct))
	}
	return chart
}

func buildUsageTiles(
	position string,
	sig analyzer.PlayerResearchSignals,
	oppDefLabel, oppDefRank string,
) []UsageTileView {
	if !sig.HasUsage && sig.OppDefRank <= 0 {
		return nil
	}

	normPos := normalizeResearchPosition(position)
	if sig.HasUsage {
		switch normPos {
		case "QB":
			label := oppDefLabel
			if label == "" {
				label = "Opp Def"
			}
			rank := oppDefRank
			if rank == "" {
				rank = "--"
			}
			return []UsageTileView{
				{Label: "Pass Att/g (L3)", Value: fmt.Sprintf("%.1f", sig.PassAttPerGame)},
				{Label: "EPA/g (L3)", Value: fmt.Sprintf("%.1f", sig.EPAPerGame)},
				{Label: "Rush Att/g (L3)", Value: fmt.Sprintf("%.1f", sig.CarriesPerGame)},
				{Label: label, Value: rank},
			}
		case "RB":
			return []UsageTileView{
				{Label: "Snap % (L3)", Value: fmt.Sprintf("%.0f%%", sig.SnapPct*100)},
				{Label: "Carries/g (L3)", Value: fmt.Sprintf("%.1f", sig.CarriesPerGame)},
				{Label: "Targets/g (L3)", Value: fmt.Sprintf("%.1f", sig.TargetsPerGame)},
				{Label: "RZ Opps/g", Value: fmt.Sprintf("%.1f", sig.RZOppsPerGame)},
			}
		case "WR", "TE":
			return []UsageTileView{
				{Label: "Snap % (L3)", Value: fmt.Sprintf("%.0f%%", sig.SnapPct*100)},
				{Label: "Target share", Value: fmt.Sprintf("%.1f%%", sig.TargetShare*100)},
				{Label: "Air Yards %", Value: fmt.Sprintf("%.1f%%", sig.AirYardsShare*100)},
				{Label: "WOPR", Value: fmt.Sprintf("%.2f", sig.WOPR)},
			}
		}
	}

	var tiles []UsageTileView
	if sig.HasUsage && sig.SnapPct > 0 {
		tiles = append(tiles, UsageTileView{Label: "Snap % (L3)", Value: fmt.Sprintf("%.0f%%", sig.SnapPct*100)})
	}
	if sig.OppDefRank > 0 {
		label := oppDefLabel
		if label == "" {
			label = "Opp Def"
		}
		tiles = append(tiles, UsageTileView{Label: label, Value: oppDefRank})
	}
	return tiles
}

func newDefRankBadge(rank int, label string) *DefRankBadge {
	if rank <= 0 {
		return nil
	}
	badgeClass := "border-slate-700 bg-slate-800/80 text-slate-300"
	switch {
	case rank <= 10:
		badgeClass = "border-rose-700/60 bg-rose-950/50 text-rose-300"
	case rank >= 23:
		badgeClass = "border-emerald-700/60 bg-emerald-950/50 text-emerald-300"
	}
	return &DefRankBadge{
		Rank:       rank,
		Label:      label,
		BadgeClass: badgeClass,
	}
}

func formatEntryOpponent(entry espn.GameLogEntry) string {
	opp := strings.TrimSpace(entry.Opponent)
	if opp == "" {
		return ""
	}
	atVs := strings.TrimSpace(entry.AtVs)
	switch atVs {
	case "":
		return opp
	case "@":
		return "@" + opp
	default:
		return atVs + " " + opp
	}
}

func formatGameWeather(game db.Game) string {
	if game.Indoor {
		return "Indoor"
	}
	var parts []string
	if game.TempF != nil {
		parts = append(parts, fmt.Sprintf("%.0f°F", *game.TempF))
	}
	if game.WindMPH != nil {
		parts = append(parts, fmt.Sprintf("%.0f mph", *game.WindMPH))
	}
	return strings.Join(parts, " · ")
}

func normalizeResearchPosition(position string) string {
	pos := strings.ToUpper(strings.TrimSpace(position))
	if pos == "FB" || pos == "HB" {
		return "RB"
	}
	return pos
}

func abbreviatePlayerName(name string) string {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	r, _ := utf8.DecodeRuneInString(parts[0])
	return fmt.Sprintf("%c. %s", r, strings.Join(parts[1:], " "))
}

func formatSignedNumber(val float64) string {
	if val > 0 {
		return "+" + formatNumber(val)
	}
	return formatNumber(val)
}

func (h *researchHandlers) page(r *http.Request) Page {
	p := Page{Title: "Research"}
	if h.deps.UserFrom != nil {
		if user, ok := h.deps.UserFrom(r.Context()); ok {
			view := NewUser(user)
			p.User = &view
		}
	}
	if h.deps.GroupsFrom != nil {
		p.Groups = h.deps.GroupsFrom(r.Context())
	}
	return p
}

func (h *researchHandlers) render(w http.ResponseWriter, data any) {
	html, err := h.deps.Renderer.Render("research", data)
	if err != nil {
		h.deps.Logger.Error().Err(err).Msg("render research page")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(html)
}

func (h *researchHandlers) renderPartial(w http.ResponseWriter, name string, data any) {
	html, err := h.deps.Renderer.RenderPartial(name, data)
	if err != nil {
		h.deps.Logger.Error().Err(err).Str("partial", name).Msg("render partial")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(html)
}

func buildInjuryViews(
	game db.Game,
	rosterPlayers []db.RosterPlayer,
	rosterMap map[string]db.RosterPlayer,
) []TeamInjuriesView {
	mergedByTeam := analyzer.MergeGameInjuries(game, rosterPlayers)
	rawInjuries := ""
	if game.InjuriesJSON != nil {
		rawInjuries = *game.InjuriesJSON
	}
	teamInjuries, err := espn.ParseInjuries(rawInjuries)
	if err != nil {
		teamInjuries = nil
	}

	toInjuryViews := func(injuries []espn.Injury) []InjuryView {
		out := make([]InjuryView, 0, len(injuries))
		for _, inj := range injuries {
			headshotURL := inj.HeadshotURL
			if headshotURL == "" {
				if rp, ok := rosterMap[inj.AthleteID]; ok {
					headshotURL = rp.HeadshotURL
				}
			}
			out = append(out, InjuryView{
				AthleteID:   inj.AthleteID,
				FullName:    inj.FullName,
				Position:    inj.Position,
				HeadshotURL: headshotURL,
				Initials:    Initials(inj.FullName),
				Status:      inj.Status,
				Detail:      inj.Detail,
			})
		}
		return out
	}

	views := make([]TeamInjuriesView, 0, len(teamInjuries)+len(mergedByTeam))
	seenTeams := make(map[string]struct{}, len(teamInjuries)+len(mergedByTeam))
	for _, ti := range teamInjuries {
		if ti.Abbreviation != "" {
			seenTeams[ti.Abbreviation] = struct{}{}
		}
		logoURL := ti.LogoURL
		if logoURL == "" {
			logoURL = gameTeamLogo(game, ti.Abbreviation, ti.Team)
		}
		injuries := ti.Injuries
		if merged, ok := mergedByTeam[ti.Abbreviation]; ok {
			injuries = merged
		}
		views = append(views, TeamInjuriesView{
			Team:         ti.Team,
			Abbreviation: ti.Abbreviation,
			LogoURL:      logoURL,
			Injuries:     toInjuryViews(injuries),
		})
	}

	for _, abbr := range []string{game.AwayTeam, game.HomeTeam} {
		if abbr == "" {
			continue
		}
		if _, ok := seenTeams[abbr]; ok {
			continue
		}
		injuries := mergedByTeam[abbr]
		if len(injuries) == 0 {
			continue
		}
		seenTeams[abbr] = struct{}{}
		teamName := gameTeamName(game, abbr)
		views = append(views, TeamInjuriesView{
			Team:         teamName,
			Abbreviation: abbr,
			LogoURL:      gameTeamLogo(game, abbr, teamName),
			Injuries:     toInjuryViews(injuries),
		})
	}

	return views
}

func gameTeamName(game db.Game, abbr string) string {
	switch {
	case abbr != "" && abbr == game.AwayTeam && game.AwayName != "":
		return game.AwayName
	case abbr != "" && abbr == game.HomeTeam && game.HomeName != "":
		return game.HomeName
	default:
		return abbr
	}
}

func gameTeamLogo(game db.Game, abbr, teamName string) string {
	switch {
	case abbr != "" && abbr == game.AwayTeam:
		return db.StringValue(game.AwayLogo)
	case abbr != "" && abbr == game.HomeTeam:
		return db.StringValue(game.HomeLogo)
	case teamName != "" && teamName == game.AwayName:
		return db.StringValue(game.AwayLogo)
	case teamName != "" && teamName == game.HomeName:
		return db.StringValue(game.HomeLogo)
	default:
		return ""
	}
}

func formatNumber(val float64) string {
	if val == math.Trunc(val) {
		return fmt.Sprintf("%.0f", val)
	}
	return fmt.Sprintf("%.1f", val)
}
