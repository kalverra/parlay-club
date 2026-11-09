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
	Injuries      []TeamInjuriesView
	Matches       []PropMatchView
	RosterOptions []RosterOptionView
	HasProps      bool
}

// TeamInjuriesView is one team's filtered injuries in a game.
type TeamInjuriesView struct {
	Team         string
	Abbreviation string
	Injuries     []InjuryView
}

// InjuryView is one athlete's injury report.
type InjuryView struct {
	AthleteID string
	FullName  string
	Position  string
	Status    string
	Detail    string
}

// PropMatchView is one sportsbook prop player and their match state.
type PropMatchView struct {
	OddsName      string
	Team          string
	ESPNAthleteID string
	AthleteName   string
	Position      string
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
	GamesPlayed      int
	ActiveMarket     string
	ActiveMarketName string
	ComparisonLine   *float64
	HasLine          bool
	LineStr          string
	L5Median         string
	SeasonMean       string
	Bars             []HitBarView
	AvailableMarkets []MarketOptionView
}

// HitBarView is one game's bar in the last-5 chart.
type HitBarView struct {
	Index       int
	Label       string
	Value       float64
	ValueStr    string
	HeightPct   int
	ResultClass string
	ResultText  string
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
	}
	if game.AwayTeam == "" || game.HomeTeam == "" {
		view.GameTitle = game.ID
	}

	// Parse injuries
	rawInjuries := ""
	if game.InjuriesJSON != nil {
		rawInjuries = *game.InjuriesJSON
	}
	teamInjuries, err := espn.ParseInjuries(rawInjuries)
	if err == nil {
		for _, ti := range teamInjuries {
			tView := TeamInjuriesView{
				Team:         ti.Team,
				Abbreviation: ti.Abbreviation,
			}
			for _, inj := range ti.Injuries {
				tView.Injuries = append(tView.Injuries, InjuryView{
					AthleteID: inj.AthleteID,
					FullName:  inj.FullName,
					Position:  inj.Position,
					Status:    inj.Status,
					Detail:    inj.Detail,
				})
			}
			view.Injuries = append(view.Injuries, tView)
		}
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

	// Odds and player matches
	oddsRows, err := h.deps.Store.OddsForGame(ctx, game.ID)
	if err == nil {
		seenPlayers := make(map[string]bool)
		for _, o := range oddsRows {
			name := strings.TrimSpace(o.PlayerName)
			if name == "" || seenPlayers[name] {
				continue
			}
			seenPlayers[name] = true
			view.HasProps = true

			aid, team, _ := h.deps.Store.ResolvePlayerMatch(ctx, name, game.HomeTeam, game.AwayTeam)
			mv := PropMatchView{
				OddsName: name,
				Team:     team,
			}
			if team == "" {
				mv.Team = o.Team
			}
			if aid != "" {
				mv.Matched = true
				mv.ESPNAthleteID = aid
				if rp, ok := rosterMap[aid]; ok {
					mv.AthleteName = rp.FullName
					mv.Position = rp.Position
					if mv.Team == "" {
						mv.Team = rp.Team
					}
				}
			}
			view.Matches = append(view.Matches, mv)
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
		AthleteID: athleteID,
		Name:      player.FullName,
		Team:      player.Team,
		Position:  player.Position,
	}
	if view.Name == "" {
		view.Name = "Athlete #" + athleteID
	}

	var markets map[string][]float64
	if logErr == nil {
		view.GamesPlayed = gamelog.GamesPlayed
		_ = json.Unmarshal([]byte(gamelog.StatsJSON), &markets)
	}

	activeMarket, comparisonLine := h.resolveMarketAndLine(
		ctx, athleteID, player, markets, legIDStr, marketParam, lineParam,
	)

	view.ActiveMarket = activeMarket
	view.ActiveMarketName = marketLabel(activeMarket)
	if comparisonLine != nil {
		view.ComparisonLine = comparisonLine
		view.HasLine = true
		view.LineStr = formatNumber(*comparisonLine)
	}

	view.AvailableMarkets = buildMarketOptions(athleteID, markets, activeMarket, comparisonLine)

	games := markets[activeMarket]
	if len(games) > 0 {
		view.Bars, view.L5Median, view.SeasonMean = buildHitBars(games, comparisonLine)
	}

	return view, nil
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

	if legIDStr != "" {
		if legID, err := strconv.ParseInt(legIDStr, 10, 64); err == nil {
			if leg, err := h.deps.Store.LegByID(ctx, legID); err == nil {
				if activeMarket == "" && leg.MarketType != "" {
					activeMarket = leg.MarketType
				}
				if comparisonLine == nil && leg.Line != nil {
					comparisonLine = leg.Line
				}
			}
		}
	}

	if (activeMarket == "" || comparisonLine == nil) && (player.FullName != "" || athleteID != "") {
		prop, ok, err := h.deps.Store.LatestPropForPlayer(ctx, athleteID, player.FullName, player.Team)
		if err == nil && ok {
			if activeMarket == "" && prop.MarketType != "" {
				activeMarket = prop.MarketType
			}
			if comparisonLine == nil && prop.Line != nil {
				comparisonLine = prop.Line
			}
		}
	}

	if activeMarket == "" && len(markets) > 0 {
		standardOrder := []string{
			"PASS_YDS", "RUSH_YDS", "REC_YDS", "RECEPTIONS",
			"PASS_TDS", "RUSH_ATT", "ANYTIME_TD", "KICKER_PTS",
		}
		for _, m := range standardOrder {
			if len(markets[m]) > 0 {
				activeMarket = m
				break
			}
		}
		if activeMarket == "" {
			for m := range markets {
				if len(markets[m]) > 0 {
					activeMarket = m
					break
				}
			}
		}
	}

	return activeMarket, comparisonLine
}

func buildMarketOptions(
	athleteID string,
	markets map[string][]float64,
	activeMarket string,
	comparisonLine *float64,
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
		if comparisonLine != nil {
			href += fmt.Sprintf("&line=%g", *comparisonLine)
		}
		options = append(options, MarketOptionView{
			Market: m,
			Label:  marketLabel(m),
			Active: m == activeMarket,
			Href:   href,
		})
	}
	return options
}

func buildHitBars(games []float64, comparisonLine *float64) ([]HitBarView, string, string) {
	l5Len := min(5, len(games))
	l5 := games[:l5Len]
	median := computeMedian(l5)
	mean := computeMean(games)
	l5Median := formatNumber(median)
	seasonMean := fmt.Sprintf("%.1f", mean)

	chronological := make([]float64, len(l5))
	for i := range l5 {
		chronological[i] = l5[len(l5)-1-i]
	}

	maxVal := 0.0
	for _, v := range chronological {
		if v > maxVal {
			maxVal = v
		}
	}
	if comparisonLine != nil && *comparisonLine > maxVal {
		maxVal = *comparisonLine
	}
	if maxVal <= 0 {
		maxVal = 1
	}

	labels := []string{"L5", "L4", "L3", "L2", "Last"}
	if len(chronological) < 5 {
		labels = labels[5-len(chronological):]
	}

	bars := make([]HitBarView, 0, len(chronological))
	for i, val := range chronological {
		pct := int(math.Round((val / maxVal) * 100))
		pct = max(10, min(100, pct))

		bar := HitBarView{
			Index:     i + 1,
			Label:     labels[i],
			Value:     val,
			ValueStr:  formatNumber(val),
			HeightPct: pct,
		}

		if comparisonLine != nil {
			switch {
			case val > *comparisonLine:
				bar.ResultClass = "bg-emerald-500 border-emerald-400 text-emerald-300"
				bar.ResultText = "OVER"
			case val < *comparisonLine:
				bar.ResultClass = "bg-rose-500 border-rose-400 text-rose-300"
				bar.ResultText = "UNDER"
			default:
				bar.ResultClass = "bg-amber-500 border-amber-400 text-amber-300"
				bar.ResultText = "PUSH"
			}
		} else {
			bar.ResultClass = "bg-sky-500 border-sky-400 text-sky-300"
		}

		bars = append(bars, bar)
	}

	return bars, l5Median, seasonMean
}

func (h *researchHandlers) page(r *http.Request) Page {
	p := Page{Title: "Research", Tab: TabResearch}
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

func marketLabel(market string) string {
	switch market {
	case "PASS_YDS":
		return "Passing Yards"
	case "PASS_TDS":
		return "Passing Touchdowns"
	case "RUSH_YDS":
		return "Rushing Yards"
	case "RUSH_ATT":
		return "Rushing Attempts"
	case "REC_YDS":
		return "Receiving Yards"
	case "RECEPTIONS":
		return "Receptions"
	case "ANYTIME_TD":
		return "Anytime Touchdown"
	case "KICKER_PTS":
		return "Kicker Points"
	default:
		return market
	}
}

func computeMedian(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2.0
}

func computeMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func formatNumber(val float64) string {
	if val == math.Trunc(val) {
		return fmt.Sprintf("%.0f", val)
	}
	return fmt.Sprintf("%.1f", val)
}
