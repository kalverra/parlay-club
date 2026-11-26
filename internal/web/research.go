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
	"time"
	"unicode/utf8"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/board"
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
	mux.HandleFunc("GET /research/explore", h.researchExplore)
	mux.HandleFunc("GET /research/games/{id}", h.researchGame)
	mux.HandleFunc("POST /research/games/{id}/props", h.researchGameProps)
	mux.HandleFunc("GET /research/players/{espnID}", h.researchPlayer)
	mux.HandleFunc("POST /research/matches", h.saveMatch)
}

// ResearchPage is the full Research tab: the explorer plus, for a targeted
// game or player, the deeper sheet.
type ResearchPage struct {
	Page
	Explore        ResearchExploreView
	GameResearch   *GameResearchView
	PlayerResearch *PlayerResearchView
}

// ResearchExploreView is the #research-explore partial: the targeted search
// form, the open-browse chip bars, and the candidate bet cards.
type ResearchExploreView struct {
	WeekLabel       string
	Search          string
	GameID          string
	PlayerID        string
	Category        string
	Grade           string
	Signal          string
	Sort            string
	Limit           int
	Games           []ResearchGameOption
	Players         []ResearchPlayerOption
	CategoryChips   []ResearchChip
	GradeChips      []ResearchChip
	SignalChips     []ResearchChip
	SortChips       []ResearchChip
	Cards           []ResearchCardView
	TotalMatched    int
	HasMore         bool
	MoreHref        string
	MorePushURL     string
	GameSheetHref   string
	PlayerSheetHref string
	BackHref        string
	PropsGameID     string
	PropsMatchup    string
	GameNoProps     bool
	NoGames         bool
	NoLines         bool
	AllStarted      bool
	Empty           bool
}

// ResearchChip is one filter or sort link in the explorer.
type ResearchChip struct {
	Label   string
	Count   int
	Active  bool
	Href    string
	PushURL string
}

// ResearchCardView is one candidate bet card. The identity fields after Market
// are what a per-bet research link needs, so the next phase can key on the same
// selection without re-reading the row.
type ResearchCardView struct {
	GameID          string
	Market          string
	MarketType      string
	Side            string
	Team            string
	Line            string
	Selection       string
	PlayerName      string
	PlayerID        string
	Position        string
	HeadshotURL     string
	Initials        string
	TeamLogo        string
	AwayLogo        string
	HomeLogo        string
	Matchup         string
	KickoffISO      string
	KickoffFallback string
	Odds            string
	OddsValue       int
	Implied         string
	GradeLetter     string
	GradeScore      int
	GradeClass      string
	Reason          string
	Badge           string
	GameSheetHref   string
	PlayerSheetHref string
}

// researchQuery is one explorer request. GameID and PlayerID target the slate;
// the rest are the open-browse chips.
type researchQuery struct {
	Q        string
	GameID   string
	PlayerID string
	Category board.Category
	Grade    string
	Signal   string
	Sort     board.ExploreSort
	Limit    int
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

// research renders the full research page or an HTMX sheet partial.
func (h *researchHandlers) research(w http.ResponseWriter, r *http.Request) {
	query := parseResearchQuery(r)
	playerName := strings.TrimSpace(r.URL.Query().Get("player"))
	explore := h.buildExplore(r.Context(), query, playerName)

	page := ResearchPage{Page: h.page(r), Explore: explore}
	page.WeekLabel = explore.WeekLabel

	if h.deps.Store == nil {
		h.render(w, page)
		return
	}

	gameID := explore.GameID
	playerID := explore.PlayerID
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
		}
	} else if gameID != "" {
		view, err := h.loadGameResearch(r.Context(), gameID)
		if err == nil {
			if isHTMX {
				h.renderPartial(w, "research-game", view)
				return
			}
			page.GameResearch = view
		}
	}

	h.render(w, page)
}

// researchExplore returns the explorer partial for HTMX searches, chip
// changes, and sorts, and pushes the matching full-page URL into the address
// bar so the view survives a refresh or a shared link.
func (h *researchHandlers) researchExplore(w http.ResponseWriter, r *http.Request) {
	query := parseResearchQuery(r)
	playerName := strings.TrimSpace(r.URL.Query().Get("player"))
	explore := h.buildExplore(r.Context(), query, playerName)
	w.Header().Set("Hx-Push-Url", researchPushURL(queryFromExplore(explore)))
	h.renderPartial(w, "research-explore", explore)
}

// researchGameProps loads props for one targeted game on an explicit tap, then
// returns the explorer so any props that landed show up in place.
func (h *researchHandlers) researchGameProps(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("id")
	if gameID == "" || h.deps.Store == nil {
		http.NotFound(w, r)
		return
	}
	if _, err := h.deps.Store.GameByID(r.Context(), gameID); errors.Is(err, db.ErrGameNotFound) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		h.deps.Logger.Error().Err(err).Str("game_id", gameID).Msg("load game for props")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if h.deps.Refresh != nil {
		if err := h.deps.Refresh.SyncGameProps(r.Context(), gameID); err != nil {
			h.deps.Logger.Warn().Err(err).Str("game_id", gameID).Msg("sync game props from research")
		}
	}

	if r.Header.Get("Hx-Request") == "" {
		http.Redirect(w, r, "/research?game_id="+url.QueryEscape(gameID), http.StatusSeeOther)
		return
	}

	query := parseResearchQuery(r)
	query.GameID = gameID
	explore := h.buildExplore(r.Context(), query, "")
	w.Header().Set("Hx-Push-Url", researchPushURL(queryFromExplore(explore)))
	h.renderPartial(w, "research-explore", explore)
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

// parseResearchQuery reads the explorer's query from the URL or a submitted form.
func parseResearchQuery(r *http.Request) researchQuery {
	if r.Method == http.MethodPost && r.PostForm == nil {
		_ = r.ParseForm()
	}
	value := func(key string) string {
		if raw := r.URL.Query().Get(key); raw != "" {
			return raw
		}
		if r.PostForm != nil {
			return r.PostForm.Get(key)
		}
		return ""
	}
	q := researchQuery{
		Q:        strings.TrimSpace(value("q")),
		GameID:   strings.TrimSpace(value("game_id")),
		PlayerID: strings.TrimSpace(value("player_id")),
		Category: board.Category(strings.TrimSpace(value("cat"))),
		Grade:    normalizeResearchGrade(value("grade")),
		Signal:   normalizeResearchSignal(value("signal")),
		Sort:     board.ExploreSort(strings.TrimSpace(value("sort"))),
		Limit:    intFromString(value("limit")),
	}
	if !slices.Contains(board.Categories, q.Category) {
		q.Category = ""
	}
	if !slices.Contains(board.ExploreSorts, q.Sort) {
		q.Sort = ""
	}
	if q.Limit < 0 {
		q.Limit = 0
	}
	return q
}

// normalizeResearchGrade folds a grade chip value to its band. An unknown
// value reads as no grade filter.
func normalizeResearchGrade(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "a":
		return "a"
	case "ab", "a-b", "a–b":
		return "ab"
	case "c", "c+", "cplus":
		return "c"
	default:
		return ""
	}
}

// researchGradeLetters expands a grade chip band to the letters it keeps.
func researchGradeLetters(value string) []string {
	switch normalizeResearchGrade(value) {
	case "a":
		return []string{"A"}
	case "ab":
		return []string{"A", "B"}
	case "c":
		return []string{"A", "B", "C"}
	default:
		return nil
	}
}

// normalizeResearchSignal folds a signal chip value to its token. An unknown
// value reads as no signal filter.
func normalizeResearchSignal(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case board.SignalSmash, "smash matchup":
		return board.SignalSmash
	case board.SignalVolume, "high volume":
		return board.SignalVolume
	case board.SignalRoleBump, "role bump":
		return board.SignalRoleBump
	case board.SignalFitsSlip, "fits slip":
		return board.SignalFitsSlip
	default:
		return ""
	}
}

// researchURL builds one explorer URL. The path selects the partial or the full
// page; every filter that is set rides along.
func researchURL(path string, q researchQuery) string {
	values := url.Values{}
	if q.Q != "" {
		values.Set("q", q.Q)
	}
	if q.GameID != "" {
		values.Set("game_id", q.GameID)
	}
	if q.PlayerID != "" {
		values.Set("player_id", q.PlayerID)
	}
	if q.Category != "" {
		values.Set("cat", string(q.Category))
	}
	if q.Grade != "" {
		values.Set("grade", q.Grade)
	}
	if q.Signal != "" {
		values.Set("signal", q.Signal)
	}
	if q.Sort != "" {
		values.Set("sort", string(q.Sort))
	}
	if q.Limit > 0 {
		values.Set("limit", strconv.Itoa(q.Limit))
	}
	if encoded := values.Encode(); encoded != "" {
		return path + "?" + encoded
	}
	return path
}

// researchPushURL is the full page URL the explorer pushes into the address bar.
func researchPushURL(q researchQuery) string {
	return researchURL("/research", q)
}

// researchExploreURL is the HTMX partial URL for one query.
func researchExploreURL(q researchQuery) string {
	return researchURL("/research/explore", q)
}

// queryFromExplore returns the request a rendered explorer stands for, using
// the resolved game and player targets. The default page size stays out of the
// URL so shared links read cleanly.
func queryFromExplore(view ResearchExploreView) researchQuery {
	limit := view.Limit
	if limit <= 50 {
		limit = 0
	}
	return researchQuery{
		Q:        view.Search,
		GameID:   view.GameID,
		PlayerID: view.PlayerID,
		Category: board.Category(view.Category),
		Grade:    view.Grade,
		Signal:   view.Signal,
		Sort:     board.ExploreSort(view.Sort),
		Limit:    limit,
	}
}

// buildExplore loads one week of cached games, odds, and roster, scores the
// slate, and shapes the explorer partial. It never syncs anything, so browsing,
// searching, filtering, and sorting make zero paid API calls.
func (h *researchHandlers) buildExplore(
	ctx context.Context,
	q researchQuery,
	playerName string,
) ResearchExploreView {
	view := ResearchExploreView{
		Search:   q.Q,
		GameID:   q.GameID,
		PlayerID: q.PlayerID,
		Category: string(q.Category),
		Grade:    q.Grade,
		Signal:   q.Signal,
		Sort:     string(q.Sort),
		Limit:    q.Limit,
	}
	if view.Limit <= 0 {
		view.Limit = 50
	}
	result := board.Explore(nil, nil, nil, board.ExploreQuery{})

	if h.deps.Store == nil {
		finishResearchExplore(&view, result)
		return view
	}

	week, err := h.deps.Store.LatestWeek(ctx)
	if err != nil {
		view.NoGames = true
		finishResearchExplore(&view, result)
		return view
	}
	view.WeekLabel = weekLabel(week)

	games, err := h.deps.Store.GamesForWeek(ctx, week.ID)
	if err != nil {
		view.NoGames = true
		finishResearchExplore(&view, result)
		return view
	}
	view.Games = researchGameOptions(games, view.GameID)
	view.NoGames = len(games) == 0

	teams := weekTeams(games)
	roster, _ := h.deps.Store.RosterForTeams(ctx, teams)
	view.Players = researchPlayerOptions(roster)

	if view.PlayerID == "" && playerName != "" {
		norm := db.NormalizePlayerName(playerName)
		for _, player := range view.Players {
			if db.NormalizePlayerName(player.Name) == norm {
				view.PlayerID = player.ID
				break
			}
		}
		if view.PlayerID == "" && view.Search == "" {
			view.Search = playerName
		}
	}
	markResearchSelection(&view)

	now := h.deps.now()
	_, homeBook := h.researchViewer(ctx)
	odds, err := h.deps.Store.OddsForWeek(ctx, week.ID)
	if err != nil {
		view.NoLines = !view.NoGames
		finishResearchExplore(&view, result)
		return view
	}

	rows := board.Build(games, odds, roster, homeBook, nil, now)
	signals := h.researchSignals(ctx, week.ID, now, games, roster, rows)
	boardGames, boardStats := h.loadResearchBoardSignals(ctx, week.ID, now, games, rows)

	result = board.Explore(rows, boardGames, boardStats, board.ExploreQuery{
		Q:        view.Search,
		GameID:   view.GameID,
		PlayerID: view.PlayerID,
		Category: board.Category(view.Category),
		Grades:   researchGradeLetters(view.Grade),
		Signal:   view.Signal,
		Sort:     board.ExploreSort(view.Sort),
		Limit:    q.Limit,
		Signals:  signals,
	})

	view.Cards = researchCards(games, result.Rows)
	view.TotalMatched = result.TotalMatched
	view.HasMore = result.HasMore
	view.AllStarted = !view.NoGames && board.StartedCount(games, now) == len(games)
	view.NoLines = !view.NoGames && !view.AllStarted && len(rows) == 0
	view.Empty = len(result.Rows) == 0 && !view.NoGames && !view.NoLines && !view.AllStarted

	if result.HasMore {
		next := queryFromExplore(view)
		next.Limit = view.Limit + 50
		view.MoreHref = researchExploreURL(next)
		view.MorePushURL = researchPushURL(next)
	}
	finishResearchExploreTargets(&view, games, rows)
	finishResearchExplore(&view, result)
	return view
}

// researchGameOptions lists the week's games for the targeted picker.
func researchGameOptions(games []db.Game, selectedID string) []ResearchGameOption {
	options := make([]ResearchGameOption, 0, len(games))
	for _, game := range games {
		label := fmt.Sprintf("%s @ %s", game.AwayTeam, game.HomeTeam)
		if game.AwayTeam == "" || game.HomeTeam == "" {
			label = game.ID
		}
		options = append(options, ResearchGameOption{
			ID:       game.ID,
			Label:    label,
			Selected: game.ID == selectedID,
		})
	}
	return options
}

// researchPlayerOptions lists the week's rostered athletes for the targeted
// picker, sorted by name.
func researchPlayerOptions(roster []db.RosterPlayer) []ResearchPlayerOption {
	options := make([]ResearchPlayerOption, 0, len(roster))
	for _, player := range roster {
		options = append(options, ResearchPlayerOption{
			ID:       player.AthleteID,
			Name:     player.FullName,
			Team:     player.Team,
			Position: player.Position,
		})
	}
	slices.SortFunc(options, func(a, b ResearchPlayerOption) int {
		return strings.Compare(a.Name, b.Name)
	})
	return options
}

// markResearchSelection marks the resolved game and player in the pickers.
func markResearchSelection(view *ResearchExploreView) {
	for i := range view.Games {
		view.Games[i].Selected = view.Games[i].ID == view.GameID
	}
	for i := range view.Players {
		view.Players[i].Selected = view.Players[i].ID == view.PlayerID
	}
}

// researchViewer returns the signed-in person and their home book.
func (h *researchHandlers) researchViewer(ctx context.Context) (db.User, string) {
	if h.deps.UserFrom == nil {
		return db.User{}, ""
	}
	user, ok := h.deps.UserFrom(ctx)
	if !ok {
		return db.User{}, ""
	}
	homeBook := ""
	if user.HomeBook != nil {
		homeBook = *user.HomeBook
	}
	return user, homeBook
}

// loadResearchBoardSignals loads the nfelo, EPA, and player-stat context the
// analyzer scores rows with.
func (h *researchHandlers) loadResearchBoardSignals(
	ctx context.Context,
	weekID int64,
	now time.Time,
	games []db.Game,
	rows []board.Row,
) (map[string]analyzer.Game, map[string]analyzer.PlayerStats) {
	playerIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.PlayerID != "" {
			playerIDs = append(playerIDs, row.PlayerID)
		}
	}
	return analyzer.LoadBoardSignals(ctx, h.deps.Store, now, weekID, games, playerIDs)
}

// researchSignals loads the badges a row can earn: nflverse team EPA, player
// usage, game injuries, and the viewer's legs on open slips.
func (h *researchHandlers) researchSignals(
	ctx context.Context,
	weekID int64,
	now time.Time,
	games []db.Game,
	roster []db.RosterPlayer,
	rows []board.Row,
) board.BoardSignals {
	playerIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.PlayerID != "" {
			playerIDs = append(playerIDs, row.PlayerID)
		}
	}
	teamEPA, playerUsage := analyzer.LoadGameSignals(ctx, h.deps.Store, now, weekID, playerIDs)
	injuries := make(map[string][]espn.Injury)
	for _, game := range games {
		for team, teamInjuries := range analyzer.MergeGameInjuries(game, roster) {
			injuries[team] = append(injuries[team], teamInjuries...)
		}
	}
	return board.BoardSignals{
		TeamEPA:     teamEPA,
		PlayerUsage: playerUsage,
		Injuries:    injuries,
		SlipLegs:    h.researchSlipLegs(ctx, weekID),
	}
}

// researchSlipLegs collects the viewer's legs on open slips for this week, so
// the explorer can mark legs that fit what they are already building.
func (h *researchHandlers) researchSlipLegs(ctx context.Context, weekID int64) []analyzer.Leg {
	if h.deps.GroupFrom == nil || h.deps.UserFrom == nil {
		return nil
	}
	group, ok := h.deps.GroupFrom(ctx)
	if !ok {
		return nil
	}
	user, ok := h.deps.UserFrom(ctx)
	if !ok {
		return nil
	}
	weekSlips, err := h.deps.Store.SlipsForWeek(ctx, group.ID, weekID)
	if err != nil {
		return nil
	}
	var legs []analyzer.Leg
	for _, slip := range weekSlips {
		if slip.Status != db.SlipOpen {
			continue
		}
		slipLegs, err := h.deps.Store.LegsForSlip(ctx, slip.ID)
		if err != nil {
			continue
		}
		for _, leg := range slipLegs {
			if leg.UserID != user.ID {
				continue
			}
			legs = append(legs, analyzer.LegFromDB(leg, ""))
		}
	}
	return legs
}

// researchCards maps one explorer page to the candidate cards.
func researchCards(games []db.Game, rows []board.ScoredRow) []ResearchCardView {
	byID := make(map[string]db.Game, len(games))
	for _, game := range games {
		byID[game.ID] = game
	}
	cards := make([]ResearchCardView, 0, len(rows))
	for _, scored := range rows {
		row := scored.Row
		game := byID[row.GameID]
		card := ResearchCardView{
			GameID:          row.GameID,
			Market:          researchMarketLabel(row.Market),
			MarketType:      row.Market,
			Side:            row.Side,
			Team:            row.Team,
			Line:            researchLineText(row.Line),
			Selection:       row.Text,
			PlayerName:      row.PlayerName,
			PlayerID:        row.PlayerID,
			Position:        row.Position,
			HeadshotURL:     row.HeadshotURL,
			Initials:        Initials(row.PlayerName),
			Matchup:         game.AwayTeam + " @ " + game.HomeTeam,
			KickoffISO:      kickoffISO(row.Kickoff),
			KickoffFallback: kickoffFallback(row.Kickoff),
			Odds:            formatOdds(row.Odds),
			OddsValue:       row.Odds,
			GradeLetter:     scored.Grade.Letter,
			GradeScore:      scored.Grade.Score,
			GradeClass:      gradeClass(scored.Grade.Letter),
			Reason:          scored.Reason,
			Badge:           row.Badge,
			GameSheetHref:   "/research?game_id=" + url.QueryEscape(row.GameID),
			AwayLogo:        db.StringValue(game.AwayLogo),
			HomeLogo:        db.StringValue(game.HomeLogo),
		}
		if row.Odds == 0 {
			card.Odds = "—"
		}
		if scored.ImpliedPct > 0 {
			card.Implied = fmt.Sprintf("%d%%", scored.ImpliedPct)
		}
		if card.Matchup == " @ " {
			card.Matchup = game.ID
		}
		if row.PlayerID != "" {
			card.PlayerSheetHref = "/research?player_id=" + url.QueryEscape(row.PlayerID)
		}
		switch row.Team {
		case game.AwayTeam:
			card.TeamLogo = card.AwayLogo
		case game.HomeTeam:
			card.TeamLogo = card.HomeLogo
		}
		cards = append(cards, card)
	}
	return cards
}

// researchMarketLabel names a row's market on its card.
func researchMarketLabel(market string) string {
	switch market {
	case "SPREAD":
		return "Spread"
	case "ML":
		return "Moneyline"
	case "TOTAL":
		return "Total"
	case "TEAM_TOTAL":
		return "Team total"
	default:
		if label := db.MarketLabel(market); label != "" {
			return label
		}
		return market
	}
}

// researchLineText prints a row's line for a per-bet link.
func researchLineText(line *float64) string {
	if line == nil {
		return ""
	}
	return formatNumber(*line)
}

// finishResearchExploreTargets records the sheet links and the explicit props
// button a targeted game may need.
func finishResearchExploreTargets(
	view *ResearchExploreView,
	games []db.Game,
	rows []board.Row,
) {
	current := queryFromExplore(*view)
	current.Limit = 0
	if view.GameID != "" {
		view.GameSheetHref = researchPushURL(current)
	}
	if view.PlayerID != "" {
		view.PlayerSheetHref = researchPushURL(current)
	}
	back := current
	back.GameID = ""
	back.PlayerID = ""
	back.Limit = 0
	view.BackHref = researchPushURL(back)

	if view.GameID == "" {
		return
	}
	for _, game := range games {
		if game.ID == view.GameID {
			view.PropsMatchup = game.AwayTeam + " @ " + game.HomeTeam
			if view.PropsMatchup == " @ " {
				view.PropsMatchup = game.ID
			}
			break
		}
	}
	view.PropsGameID = view.GameID
	hasLines := false
	hasProps := false
	for _, row := range rows {
		if row.GameID != view.GameID {
			continue
		}
		hasLines = true
		if row.Category != board.CategoryLines {
			hasProps = true
			break
		}
	}
	view.GameNoProps = hasLines && !hasProps
}

// finishResearchExplore fills the chip bars and pagination links from one
// result and the resolved query.
func finishResearchExplore(view *ResearchExploreView, result board.ExploreResult) {
	current := queryFromExplore(*view)
	current.Limit = 0
	view.CategoryChips = researchCategoryChips(current, result)
	view.GradeChips = researchGradeChips(current, result)
	view.SignalChips = researchSignalChips(current, result)
	view.SortChips = researchSortChips(current)
}

// researchCategoryChips builds the bet-type chips with their counts.
func researchCategoryChips(q researchQuery, result board.ExploreResult) []ResearchChip {
	chips := make([]ResearchChip, 0, len(board.Categories)+1)
	all := 0
	for _, count := range result.CategoryCounts {
		all += count
	}
	next := q
	next.Category = ""
	chips = append(chips, researchChip(next, "All", all, q.Category == ""))
	for _, category := range board.Categories {
		next := q
		next.Category = category
		chips = append(
			chips,
			researchChip(next, category.Label(), result.CategoryCounts[category], q.Category == category),
		)
	}
	return chips
}

// researchChip fills one chip's links from the query it asks for.
func researchChip(next researchQuery, label string, count int, active bool) ResearchChip {
	next.Limit = 0
	return ResearchChip{
		Label:   label,
		Count:   count,
		Active:  active,
		Href:    researchExploreURL(next),
		PushURL: researchPushURL(next),
	}
}

// researchGradeChips builds the grade-band chips with their counts.
func researchGradeChips(q researchQuery, result board.ExploreResult) []ResearchChip {
	all := 0
	for _, count := range result.GradeCounts {
		all += count
	}
	bands := []struct {
		value string
		label string
	}{
		{"", "All"},
		{"a", "A"},
		{"ab", "A–B"},
		{"c", "C+"},
	}
	chips := make([]ResearchChip, 0, len(bands))
	for _, band := range bands {
		count := 0
		if band.value == "" {
			count = all
		} else {
			for _, letter := range researchGradeLetters(band.value) {
				count += result.GradeCounts[letter]
			}
		}
		next := q
		next.Grade = band.value
		chips = append(chips, researchChip(next, band.label, count, q.Grade == band.value))
	}
	return chips
}

// researchSignalChips builds the signal chips with their counts.
func researchSignalChips(q researchQuery, result board.ExploreResult) []ResearchChip {
	chips := make([]ResearchChip, 0, len(board.ExploreSignals)+1)
	next := q
	next.Signal = ""
	chips = append(chips, researchChip(next, "Any signal", 0, q.Signal == ""))
	for _, signal := range board.ExploreSignals {
		next := q
		next.Signal = signal
		chips = append(
			chips,
			researchChip(next, board.SignalLabel(signal), result.SignalCounts[signal], q.Signal == signal),
		)
	}
	return chips
}

// researchSortChips builds the sort links. An empty sort is the default grade
// sort, so Best grade stays current until another chip is picked.
func researchSortChips(q researchQuery) []ResearchChip {
	chips := make([]ResearchChip, 0, len(board.ExploreSorts))
	for _, sort := range board.ExploreSorts {
		next := q
		next.Sort = sort
		active := q.Sort == sort || (q.Sort == "" && sort == board.ExploreSortGrade)
		chips = append(chips, researchChip(next, sort.Label(), 0, active))
	}
	return chips
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

// loadGameResearch builds the GameResearchView for one game from cached data.
func (h *researchHandlers) loadGameResearch(ctx context.Context, gameID string) (*GameResearchView, error) {
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

// loadPlayerResearch builds the PlayerResearchView for one athlete from cached
// data.
func (h *researchHandlers) loadPlayerResearch(
	ctx context.Context,
	athleteID, legIDStr, marketParam, lineParam string,
) (*PlayerResearchView, error) {
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
	var oppDefLabel, oppDefRank string
	if sig.OppDefRank > 0 {
		if sig.OppDefSplit == "rush" {
			oppDefLabel = "Opp Rush Def"
		} else {
			oppDefLabel = "Opp Pass Def"
		}
		oppDefRank = fmt.Sprintf("#%d vs %s", sig.OppDefRank, sig.Opponent)
	}

	view.UsageTiles = buildUsageTiles(view.Position, sig, oppDefLabel, oppDefRank)

	normPos := db.NormalizePosition(view.Position)
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

	normPos := db.NormalizePosition(view.Position)
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

	view.PlayerChart = buildBarChart(rawBars, comparisonLine)
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
				bar.ResultClass = "bar-hit"
				bar.ResultText = "OVER"
			case bar.Value < *comparisonLine:
				bar.ResultClass = "bar-miss"
				bar.ResultText = "UNDER"
			default:
				bar.ResultClass = "bar-push"
				bar.ResultText = "PUSH"
			}
		} else {
			bar.ResultClass = "bar-neutral"
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

	normPos := db.NormalizePosition(position)
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
	badgeClass := "badge-void"
	switch {
	case rank <= 10:
		badgeClass = "badge-lost"
	case rank >= 23:
		badgeClass = "badge-won"
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
	p := Page{Title: "Research", Tab: TabResearch, Theme: ThemeFromRequest(r).ID}
	if h.deps.UserFrom != nil {
		if user, ok := h.deps.UserFrom(r.Context()); ok {
			view := NewUser(user)
			p.User = &view
			p.Theme = ThemeForRequest(r, user.Name).ID
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
