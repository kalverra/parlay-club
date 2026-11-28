package web

import (
	"cmp"
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
	"github.com/kalverra/parlay-club/internal/parlay"
	"github.com/kalverra/parlay-club/internal/slips"
)

// researchHandlers serves the Research tab and partials.
type researchHandlers struct {
	deps Dependencies
}

// Routes mounts the Research tab and endpoints.
func (h *researchHandlers) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /research", h.research)
	mux.HandleFunc("GET /research/bet", h.researchBet)
	mux.HandleFunc("GET /research/explore", h.researchExplore)
	mux.HandleFunc("GET /research/games/{id}", h.researchGame)
	mux.HandleFunc("POST /research/games/{id}/props", h.researchGameProps)
	mux.HandleFunc("GET /research/players/{espnID}", h.researchPlayer)
	mux.HandleFunc("POST /research/matches", h.saveMatch)
	mux.HandleFunc("GET /bets/add-sheet", h.betAddSheet)
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

	// GameResearch and PlayerResearch are the resolved target's sheet. The
	// explorer partial renders them out of band when OOB is set so a search or
	// chip change replaces an already-open sheet instead of leaving it on the
	// old target.
	GameResearch   *GameResearchView
	PlayerResearch *PlayerResearchView
	OOB            bool
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
	ImpliedHuman    string
	GradeLetter     string
	GradeDisplay    string
	GradeDescriptor string
	GradeScore      int
	GradeClass      string
	Reason          string
	Badge           string
	SplitLine       string
	BetHref         string
	AddHref         string
	AddPostURL      string
	ReturnURL       string
	GameSheetHref   string
	PlayerSheetHref string
}

// BetResearchPage is the full per-bet research page.
type BetResearchPage struct {
	Page
	View BetResearchView
}

// BetResearchView is one bet's summary, cached book prices, analyzer grade,
// and the research that fits its market. Missing says the selection could not
// be matched to a cached row, so the page renders a clean not-found state.
type BetResearchView struct {
	GameID          string
	MarketType      string
	MarketLabel     string
	Side            string
	SideBadge       string
	SideClass       string
	Team            string
	PlayerID        string
	PlayerName      string
	Position        string
	Selection       string
	Badge           string
	SplitLine       string
	Matchup         string
	HomeTeam        string
	AwayTeam        string
	HomeLogo        string
	AwayLogo        string
	HeadshotURL     string
	Initials        string
	TeamLogo        string
	KickoffISO      string
	KickoffFallback string
	Odds            int
	OddsText        string
	Implied         string
	ImpliedHuman    string
	LineInput       string
	Reason          string
	Grade           GradeView
	Warnings        []AnalyzerWarningView
	Pricing         BetPricingView
	Player          *PlayerResearchView
	TeamContext     *TeamBetContextView
	Related         []ResearchCardView
	AddSlips        []BetAddSlipView
	BackHref        string
	ReturnURL       string
	PlayerSheetHref string
	GameSheetHref   string
	Missing         bool
	MissingMessage  string
}

// BetPricingView compares the cached books for one selection.
type BetPricingView struct {
	HasPrices             bool
	HomeBook              string
	HomeLine              string
	HomePrice             string
	ConsensusLine         string
	ConsensusPrice        string
	ConsensusImplied      string
	ConsensusImpliedHuman string
	BestBook              string
	BestLine              string
	BestPrice             string
	BestImplied           string
	BestImpliedHuman      string
	DevigEdge             string
	DevigNote             string
	SnapshotBooks         []BetBookView
	BookCount             int
	Books                 []BetBookView
}

// BetBookView is one book's cached line and price for the bet.
type BetBookView struct {
	Book         string
	Line         string
	Price        string
	Implied      string
	ImpliedHuman string
	Home         bool
	Best         bool
}

// BetAddSlipView is one open slip the viewer can add this bet to.
type BetAddSlipView struct {
	SlipID   int64
	SlipName string
	PostURL  string
	Summary  string
}

// BetAddSheetView is the #bet-add-dialog-body partial for choosing an
// eligible open slip to add a bet to.
type BetAddSheetView struct {
	GameID     string
	MarketType string
	Team       string
	Side       string
	PlayerID   string
	PlayerName string
	Line       string
	Odds       string
	Selection  string
	Matchup    string
	ReturnURL  string
	WeekID     int64
	KickedOff  bool
	AddSlips   []BetAddSlipView
}

// TeamBetContextView is the cached game context a game-market bet shows:
// nfelo's projection versus the market, both teams' EPA ranks, weather, and
// key injuries.
type TeamBetContextView struct {
	GameID          string
	Matchup         string
	Venue           string
	Weather         string
	Indoor          bool
	KickoffISO      string
	KickoffFallback string
	HasModel        bool
	ModelHomeSpread string
	ModelHomeWin    string
	ModelAwayWin    string
	ModelBetSpread  string
	ModelBetWin     string
	BetTeam         string
	MarketLabel     string
	MarketValue     string
	Teams           []TeamEPAContextView
	Injuries        []TeamInjuriesView
}

// TeamEPAContextView is one team's offensive and defensive EPA ranks.
type TeamEPAContextView struct {
	Team       string
	Name       string
	Logo       string
	HasEPA     bool
	OffPass    string
	OffRush    string
	DefPass    string
	DefRush    string
	DefOverall string
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
	explore = h.researchPanel(r.Context(), r, explore)
	explore.OOB = true
	w.Header().Set("Hx-Push-Url", researchPushURL(queryFromExplore(explore)))
	h.renderPartial(w, "research-explore", explore)
}

// researchPanel resolves the sheet for an explorer target so an HTMX search or
// chip change can replace an already-open #research-panel out of band. With no
// target it stays empty, so the previous sheet goes away instead of lingering
// under a list that now names something else.
func (h *researchHandlers) researchPanel(
	ctx context.Context,
	r *http.Request,
	view ResearchExploreView,
) ResearchExploreView {
	if h.deps.Store == nil {
		return view
	}
	market := strings.TrimSpace(r.URL.Query().Get("market"))
	lineStr := strings.TrimSpace(r.URL.Query().Get("line"))
	legIDStr := strings.TrimSpace(r.URL.Query().Get("leg_id"))
	switch {
	case view.PlayerID != "":
		if player, err := h.loadPlayerResearch(ctx, view.PlayerID, legIDStr, market, lineStr); err == nil {
			view.PlayerResearch = player
		}
	case view.GameID != "":
		if game, err := h.loadGameResearch(ctx, view.GameID); err == nil {
			view.GameResearch = game
		}
	}
	return view
}

// researchBet shows one bet's cached prices, analyzer grade, and the deeper
// research for its market. It reads only cached SQLite data and never syncs.
func (h *researchHandlers) researchBet(w http.ResponseWriter, r *http.Request) {
	page := h.page(r)
	page.Title = "Bet research"
	if h.deps.Store != nil {
		if week, err := h.deps.Store.LatestWeek(r.Context()); err == nil {
			page.WeekLabel = weekLabel(week)
		}
	}
	view := h.buildBetResearch(r.Context(), r)
	h.renderBet(w, BetResearchPage{Page: page, View: view})
}

// buildBetResearch loads and shapes one bet. A selection that is not on the
// cached slate comes back with Missing set.
func (h *researchHandlers) buildBetResearch(ctx context.Context, r *http.Request) BetResearchView {
	query := r.URL.Query()
	selection := db.SelectionKey{
		GameID:     strings.TrimSpace(query.Get("game_id")),
		MarketType: strings.ToUpper(strings.TrimSpace(query.Get("market"))),
		Team:       strings.TrimSpace(query.Get("team")),
		PlayerName: strings.TrimSpace(query.Get("player_name")),
		PlayerID:   strings.TrimSpace(query.Get("player_id")),
		Side:       strings.ToUpper(strings.TrimSpace(query.Get("side"))),
	}
	view := BetResearchView{
		GameID:         selection.GameID,
		MarketType:     selection.MarketType,
		MarketLabel:    researchMarketLabel(selection.MarketType),
		Side:           selection.Side,
		Team:           selection.Team,
		PlayerID:       selection.PlayerID,
		PlayerName:     selection.PlayerName,
		BackHref:       safeReturnPath(query.Get("from")),
		ReturnURL:      r.URL.RequestURI(),
		Missing:        true,
		MissingMessage: "We couldn't match that bet to this week's cached lines.",
	}
	if view.BackHref == "" {
		view.BackHref = "/research"
	}
	if h.deps.Store == nil || selection.GameID == "" || selection.MarketType == "" {
		return view
	}

	game, err := h.deps.Store.GameByID(ctx, selection.GameID)
	if errors.Is(err, db.ErrGameNotFound) {
		return view
	}
	if err != nil {
		h.deps.Logger.Error().Err(err).Str("game_id", selection.GameID).Msg("load game for bet research")
		view.MissingMessage = "We couldn't load that game's cached odds."
		return view
	}
	now := h.deps.now()
	if !now.Before(game.Kickoff) {
		view.MissingMessage = "That game has kicked off, so its cached lines are closed."
		return view
	}

	odds, err := h.deps.Store.OddsForGame(ctx, game.ID)
	if err != nil {
		h.deps.Logger.Error().Err(err).Str("game_id", game.ID).Msg("load odds for bet research")
		return view
	}
	roster, _ := h.deps.Store.RosterForTeams(ctx, []string{game.HomeTeam, game.AwayTeam})
	homeBook := h.researchViewer(ctx)
	rows := board.BuildDualBook([]db.Game{game}, odds, roster, homeBook, nil, now)

	var bet *board.Row
	for i := range rows {
		if db.SameSelection(selection, betRowSelection(rows[i])) {
			bet = &rows[i]
			break
		}
	}
	if bet == nil {
		return view
	}

	games, stats := analyzer.LoadBoardSignals(
		ctx, h.deps.Store, now, game.WeekID, []db.Game{game}, rowPlayerIDs([]board.Row{*bet}),
	)
	grade := analyzer.ScoreLeg(board.RowToAnalyzerLeg(*bet), games, stats)

	view.Missing = false
	view.MissingMessage = ""
	view.Selection = bet.Text
	view.MarketType = bet.Market
	view.MarketLabel = researchMarketLabel(bet.Market)
	view.Side = bet.Side
	view.SideBadge, view.SideClass = legSideBadge(&bet.Side)
	view.Team = bet.Team
	view.PlayerID = bet.PlayerID
	view.PlayerName = bet.PlayerName
	view.Position = bet.Position
	view.HeadshotURL = bet.HeadshotURL
	view.Initials = Initials(bet.PlayerName)
	view.Badge = bet.Badge
	view.Matchup = game.AwayTeam + " @ " + game.HomeTeam
	view.HomeTeam = game.HomeTeam
	view.AwayTeam = game.AwayTeam
	view.HomeLogo = db.StringValue(game.HomeLogo)
	view.AwayLogo = db.StringValue(game.AwayLogo)
	view.KickoffISO = kickoffISO(bet.Kickoff)
	view.KickoffFallback = kickoffFallback(bet.Kickoff)
	view.Odds = bet.Odds
	view.OddsText = formatOdds(bet.Odds)
	view.Implied = formatImpliedChance(parlay.ImpliedProb(bet.Odds))
	view.ImpliedHuman = formatImpliedHuman(parlay.ImpliedProb(bet.Odds))
	view.LineInput = researchLineText(bet.Line)
	view.SplitLine = splitLineBadge(bet.Market, bet.DK, bet.FD, bet.LinesDiffer, homeBook)
	view.Grade = LegGradeView(grade)
	view.Reason = board.ExploreReason(grade)
	view.Warnings = betWarnings(*bet, games, stats)
	switch bet.Team {
	case game.AwayTeam:
		view.TeamLogo = view.AwayLogo
	case game.HomeTeam:
		view.TeamLogo = view.HomeLogo
	}
	view.Pricing = buildBetPricing(odds, *bet, game, homeBook)

	if db.IsPlayerMarket(bet.Market) {
		playerID := bet.PlayerID
		if playerID == "" {
			playerID = selection.PlayerID
		}
		if playerID != "" {
			if player, err := h.loadPlayerResearch(
				ctx, playerID, "", bet.Market, researchLineText(bet.Line),
			); err == nil {
				view.Player = player
				view.PlayerSheetHref = "/research?player_id=" + url.QueryEscape(playerID)
			}
		}
	} else {
		if gameResearch, err := h.loadGameResearch(ctx, game.ID); err == nil {
			view.TeamContext = buildTeamBetContext(game, gameResearch, *bet, games[game.ID])
			view.GameSheetHref = "/research?game_id=" + url.QueryEscape(game.ID)
		}
	}

	rawSlipID := query.Get("slip_id")
	var slipID int64
	if parsed, err := strconv.ParseInt(strings.TrimSpace(rawSlipID), 10, 64); err == nil && parsed > 0 {
		slipID = parsed
	}
	view.AddSlips = preferBetSlip(h.betAddSlips(ctx, game), rawSlipID)
	activeAddURL := ""
	if slipID > 0 {
		for _, option := range view.AddSlips {
			if option.SlipID == slipID {
				activeAddURL = option.PostURL
				break
			}
		}
	}
	view.Related = relatedBetCards(
		game, rows, games, stats, *bet, view.BackHref, view.ReturnURL, homeBook, slipID, activeAddURL,
	)
	return view
}

// preferBetSlip narrows the offered slips to the one a link asked for, when
// that slip is still eligible.
func preferBetSlip(options []BetAddSlipView, raw string) []BetAddSlipView {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return options
	}
	for _, option := range options {
		if option.SlipID == id {
			return []BetAddSlipView{option}
		}
	}
	return options
}

// renderBet writes the per-bet research page, answering 404 when the selection
// is not on the cached slate.
func (h *researchHandlers) renderBet(w http.ResponseWriter, page BetResearchPage) {
	html, err := h.deps.Renderer.Render("research-bet", page)
	if err != nil {
		h.deps.Logger.Error().Err(err).Msg("render bet research page")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if page.View.Missing {
		w.WriteHeader(http.StatusNotFound)
	}
	_, _ = w.Write(html)
}

// betRowSelection is the selection identity of one cached board row.
func betRowSelection(row board.Row) db.SelectionKey {
	return db.SelectionKey{
		GameID:     row.GameID,
		MarketType: row.Market,
		Team:       row.Team,
		PlayerName: row.PlayerName,
		PlayerID:   row.PlayerID,
		Side:       row.Side,
	}
}

// rowPlayerIDs lists the distinct player IDs the analyzer can use for a set of
// rows.
func rowPlayerIDs(rows []board.Row) []string {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.PlayerID == "" || slices.Contains(ids, row.PlayerID) {
			continue
		}
		ids = append(ids, row.PlayerID)
	}
	return ids
}

// betWarnings runs the analyzer over the bet on its own and returns the
// warnings that name it.
func betWarnings(
	row board.Row,
	games map[string]analyzer.Game,
	stats map[string]analyzer.PlayerStats,
) []AnalyzerWarningView {
	leg := board.RowToAnalyzerLeg(row)
	if leg.ID == 0 {
		leg.ID = 1
	}
	evaluation := analyzer.EvaluateSlip([]analyzer.Leg{leg}, games, stats)
	warnings := make([]AnalyzerWarningView, 0, len(evaluation.Warnings))
	for _, warning := range evaluation.Warnings {
		if len(warning.LegIDs) > 0 && !slices.Contains(warning.LegIDs, leg.ID) {
			continue
		}
		label, class := analyzerSeverity(warning.Severity)
		warnings = append(warnings, AnalyzerWarningView{
			Rule:     warning.Rule,
			Severity: label,
			Class:    class,
			Message:  warning.Message,
		})
	}
	return warnings
}

// betBookPrice is one book's chosen cached price for the bet.
type betBookPrice struct {
	book string
	key  string
	odds int
	line *float64
}

// buildBetPricing compares the cached books for one bet and removes the vig
// when a book prices both sides.
func buildBetPricing(odds []db.Odds, row board.Row, game db.Game, homeBook string) BetPricingView {
	prices := betBookPrices(odds, betRowSelection(row), row.Line)
	if len(prices) == 0 {
		return BetPricingView{}
	}

	pool := prices
	sameLine := make([]betBookPrice, 0, len(prices))
	for _, price := range prices {
		if db.SameLine(price.line, row.Line) {
			sameLine = append(sameLine, price)
		}
	}
	if len(sameLine) > 0 {
		pool = sameLine
	}

	line := betMedianLine(pool)
	view := BetPricingView{
		HasPrices: true,
		Books:     make([]BetBookView, 0, len(prices)),
		HomeBook:  strings.TrimSpace(homeBook),
	}
	view.ConsensusLine = researchLineText(line)
	view.ConsensusPrice, view.ConsensusImplied = betPriceTexts(betMedianPrice(pool))
	view.ConsensusImpliedHuman = formatImpliedHuman(parlay.ImpliedProb(betMedianPrice(pool)))

	best := pool[0]
	for _, price := range pool[1:] {
		if parlay.AmericanToDecimal(price.odds) > parlay.AmericanToDecimal(best.odds) {
			best = price
		}
	}
	view.BestBook = best.book
	view.BestLine = researchLineText(best.line)
	view.BestPrice, view.BestImplied = betPriceTexts(best.odds)
	view.BestImpliedHuman = formatImpliedHuman(parlay.ImpliedProb(best.odds))

	homeKey := db.NormalizeBook(view.HomeBook)
	bestKey := best.key
	for _, price := range prices {
		bookView := BetBookView{
			Book: price.book,
			Line: researchLineText(price.line),
		}
		bookView.Price, bookView.Implied = betPriceTexts(price.odds)
		bookView.ImpliedHuman = formatImpliedHuman(parlay.ImpliedProb(price.odds))
		bookView.Home = homeKey != "" && price.key == homeKey
		bookView.Best = price.key == bestKey
		view.Books = append(view.Books, bookView)
		if bookView.Home {
			view.HomeBook = price.book
			view.HomeLine = bookView.Line
			view.HomePrice = bookView.Price
		}
	}
	slices.SortStableFunc(view.Books, func(left, right BetBookView) int {
		if left.Home != right.Home {
			if left.Home {
				return -1
			}
			return 1
		}
		if left.Best != right.Best {
			if left.Best {
				return -1
			}
			return 1
		}
		return strings.Compare(left.Book, right.Book)
	})

	view.BookCount = len(view.Books)
	view.SnapshotBooks = extractSnapshotBooks(view.Books, homeKey)
	view.DevigEdge, view.DevigNote = betDevigEdge(odds, row, game, best.odds)
	return view
}

// extractSnapshotBooks returns the explicit FanDuel and DraftKings quotes
// ordered with the viewer's homeBook first when it is one of them.
func extractSnapshotBooks(books []BetBookView, homeKey string) []BetBookView {
	var fd, dk *BetBookView
	for i := range books {
		switch db.NormalizeBook(books[i].Book) {
		case "fanduel":
			fd = &books[i]
		case "draftkings":
			dk = &books[i]
		}
	}
	out := make([]BetBookView, 0, 2)
	if homeKey == "draftkings" {
		if dk != nil {
			out = append(out, *dk)
		}
		if fd != nil {
			out = append(out, *fd)
		}
		return out
	}
	if fd != nil {
		out = append(out, *fd)
	}
	if dk != nil {
		out = append(out, *dk)
	}
	return out
}

// betBookPrices groups one selection's cached odds by book, choosing the book's
// line closest to the bet's line when it prices more than one.
func betBookPrices(odds []db.Odds, selection db.SelectionKey, want *float64) []betBookPrice {
	byBook := make(map[string][]db.Odds)
	display := make(map[string]string)
	for _, o := range odds {
		if !db.SameSelection(selection, db.OddsSelection(o)) {
			continue
		}
		key := db.NormalizeBook(o.Book)
		if key == "" {
			continue
		}
		if _, ok := display[key]; !ok {
			display[key] = o.Book
		}
		byBook[key] = append(byBook[key], o)
	}
	prices := make([]betBookPrice, 0, len(byBook))
	for key, rows := range byBook {
		chosen := closestBetLine(rows, want)
		prices = append(prices, betBookPrice{
			book: display[key],
			key:  key,
			odds: chosen.Price,
			line: chosen.Line,
		})
	}
	slices.SortFunc(prices, func(left, right betBookPrice) int {
		return strings.Compare(left.key, right.key)
	})
	return prices
}

// closestBetLine picks the book's row on want's line, or the nearest line.
func closestBetLine(rows []db.Odds, want *float64) db.Odds {
	chosen := rows[0]
	bestDelta := math.Inf(1)
	for _, o := range rows {
		if want != nil && o.Line != nil && *o.Line == *want {
			return o
		}
		if want == nil {
			if o.Line == nil {
				return o
			}
			continue
		}
		if o.Line == nil {
			continue
		}
		delta := math.Abs(*o.Line - *want)
		if delta < bestDelta {
			bestDelta = delta
			chosen = o
		}
	}
	return chosen
}

// betMedianPrice is the median American price across the books.
func betMedianPrice(prices []betBookPrice) int {
	decimals := make([]float64, 0, len(prices))
	for _, price := range prices {
		if price.odds == 0 {
			continue
		}
		decimals = append(decimals, parlay.AmericanToDecimal(price.odds))
	}
	if len(decimals) == 0 {
		return 0
	}
	return parlay.DecimalToAmerican(parlay.ReferencePrice(decimals, 0))
}

// betMedianLine is the median cached line across the books.
func betMedianLine(prices []betBookPrice) *float64 {
	lines := make([]float64, 0, len(prices))
	for _, price := range prices {
		if price.line != nil {
			lines = append(lines, *price.line)
		}
	}
	if len(lines) == 0 {
		return nil
	}
	median := analyzer.Median(lines)
	return &median
}

// betPriceTexts formats one American price and its implied chance.
func betPriceTexts(odds int) (string, string) {
	if odds == 0 {
		return "", ""
	}
	return formatOdds(odds), formatImpliedChance(parlay.ImpliedProb(odds))
}

// betDevigEdge removes the vig from books that price both sides and reports the
// edge between fair probability and the best price's implied probability.
func betDevigEdge(odds []db.Odds, row board.Row, game db.Game, bestPrice int) (string, string) {
	opposite, ok := betOppositeSelection(row, game)
	if !ok || bestPrice == 0 {
		return "", ""
	}
	selection := betRowSelection(row)
	type twoWay struct {
		sel      *db.Odds
		opposite *db.Odds
	}
	byBook := make(map[string]*twoWay)
	for i := range odds {
		o := &odds[i]
		key := db.NormalizeBook(o.Book)
		if key == "" || !betSameMarketLine(*o, row) {
			continue
		}
		entry, exists := byBook[key]
		if !exists {
			entry = &twoWay{}
			byBook[key] = entry
		}
		if entry.sel == nil && db.SameSelection(selection, db.OddsSelection(*o)) {
			entry.sel = o
		}
		if entry.opposite == nil && db.SameSelection(opposite, db.OddsSelection(*o)) {
			entry.opposite = o
		}
	}

	var fairProbs []float64
	for _, entry := range byBook {
		if entry.sel == nil || entry.opposite == nil {
			continue
		}
		probability, _, ok := parlay.DevigTwoWay(entry.sel.Price, entry.opposite.Price)
		if ok {
			fairProbs = append(fairProbs, probability)
		}
	}
	if len(fairProbs) == 0 {
		return "", ""
	}
	fair := analyzer.Mean(fairProbs)
	implied := parlay.ImpliedProb(bestPrice)
	edge := fair - implied
	if math.Abs(edge) < 0.001 {
		return "Even", fmt.Sprintf("De-vig fair %s at the best price", formatImpliedChance(fair))
	}
	return fmt.Sprintf("%+.1f%%", edge*100),
		fmt.Sprintf("De-vig fair %s at the best price", formatImpliedChance(fair))
}

// betSameMarketLine reports whether a two-way book row sits on the bet's line.
// Spreads and moneylines have no shared number, so they always match.
func betSameMarketLine(o db.Odds, row board.Row) bool {
	switch row.Market {
	case "SPREAD", "ML":
		return true
	default:
		return db.SameLine(o.Line, row.Line)
	}
}

// betOppositeSelection names the other side of the bet's two-way market.
func betOppositeSelection(row board.Row, game db.Game) (db.SelectionKey, bool) {
	side := oppositeBetSide(row.Side)
	if side == "" {
		return db.SelectionKey{}, false
	}
	key := betRowSelection(row)
	key.Side = side
	switch row.Market {
	case "SPREAD", "ML":
		switch row.Team {
		case game.HomeTeam:
			key.Team = game.AwayTeam
		case game.AwayTeam:
			key.Team = game.HomeTeam
		default:
			return db.SelectionKey{}, false
		}
		key.PlayerID = ""
		key.PlayerName = ""
	case "TOTAL":
		key.Team = ""
	case "TEAM_TOTAL":
		key.PlayerID = ""
		key.PlayerName = ""
	case "ANYTIME_TD":
		return db.SelectionKey{}, false
	default:
		if !db.IsPlayerMarket(row.Market) {
			return db.SelectionKey{}, false
		}
	}
	return key, true
}

// oppositeBetSide flips a two-way side.
func oppositeBetSide(side string) string {
	switch side {
	case "OVER":
		return "UNDER"
	case "UNDER":
		return "OVER"
	case "TEAM":
		return "TEAM"
	default:
		return ""
	}
}

// buildTeamBetContext shapes a game-market bet's cached nfelo, EPA, weather,
// and injury context.
func buildTeamBetContext(
	game db.Game,
	research *GameResearchView,
	row board.Row,
	analyzed analyzer.Game,
) *TeamBetContextView {
	view := &TeamBetContextView{
		GameID:          game.ID,
		Matchup:         game.AwayTeam + " @ " + game.HomeTeam,
		Venue:           db.StringValue(game.VenueName),
		Weather:         betWeatherText(game),
		Indoor:          game.Indoor,
		KickoffISO:      kickoffISO(game.Kickoff),
		KickoffFallback: kickoffFallback(game.Kickoff),
		BetTeam:         row.Team,
	}
	if view.Matchup == " @ " {
		view.Matchup = game.ID
	}
	if research != nil {
		view.Injuries = research.Injuries
	}

	if analyzed.Nfelo != nil {
		view.HasModel = true
		if spread, ok := analyzed.ModelSpread(game.HomeTeam); ok {
			view.ModelHomeSpread = game.HomeTeam + " " + formatSignedNumber(spread)
		}
		view.ModelHomeWin = fmt.Sprintf("%s %.0f%%", game.HomeTeam, analyzed.Nfelo.HomeWinProb*100)
		view.ModelAwayWin = fmt.Sprintf("%s %.0f%%", game.AwayTeam, (1-analyzed.Nfelo.HomeWinProb)*100)
		if row.Team != "" {
			if spread, ok := analyzed.ModelSpread(row.Team); ok {
				view.ModelBetSpread = formatSignedNumber(spread)
			}
			if probability, ok := analyzed.ModelWinProb(row.Team); ok {
				view.ModelBetWin = fmt.Sprintf("%.0f%%", probability*100)
			}
		}
	}
	view.MarketLabel, view.MarketValue = betMarketLine(row)
	view.Teams = []TeamEPAContextView{
		teamEPAContext(game.AwayTeam, db.StringValue(game.AwayLogo), analyzed),
		teamEPAContext(game.HomeTeam, db.StringValue(game.HomeLogo), analyzed),
	}
	return view
}

// teamEPAContext formats one team's offensive and defensive EPA ranks.
func teamEPAContext(team, logo string, analyzed analyzer.Game) TeamEPAContextView {
	view := TeamEPAContextView{Team: team, Logo: logo}
	profile := analyzed.TeamEPA(team)
	if profile == nil {
		return view
	}
	view.HasEPA = true
	view.OffPass = betRank(profile.OffPassEPARank)
	view.OffRush = betRank(profile.OffRushEPARank)
	view.DefPass = betRank(profile.DefPassEPAAllowedRank)
	view.DefRush = betRank(profile.DefRushEPAAllowedRank)
	view.DefOverall = betRank(profile.DefOverallEPARank)
	return view
}

// betRank prints one league rank.
func betRank(rank int) string {
	if rank <= 0 {
		return "—"
	}
	return fmt.Sprintf("#%d", rank)
}

// betMarketLine names the bet's own market line for the team context card.
func betMarketLine(row board.Row) (string, string) {
	line := researchLineText(row.Line)
	switch row.Market {
	case "SPREAD":
		return "Spread", strings.TrimSpace(row.Team + " " + lineValueOf(row.Line))
	case "ML":
		return "Moneyline", strings.TrimSpace(row.Team + " " + formatOdds(row.Odds))
	case "TOTAL":
		return "Total", strings.TrimSpace(betSideLabel(row.Side) + " " + line)
	case "TEAM_TOTAL":
		return "Team total", strings.TrimSpace(row.Team + " " + betSideLabel(row.Side) + " " + line)
	default:
		return researchMarketLabel(row.Market), line
	}
}

// lineValueOf prints a signed spread line, or nothing when it is missing.
func lineValueOf(line *float64) string {
	if line == nil {
		return ""
	}
	return formatSignedNumber(*line)
}

// betSideLabel title-cases a side for display.
func betSideLabel(side string) string {
	switch side {
	case "OVER":
		return "Over"
	case "UNDER":
		return "Under"
	case "YES":
		return "Yes"
	case "TEAM":
		return ""
	default:
		return side
	}
}

// betWeatherText prints a game's venue and weather.
func betWeatherText(game db.Game) string {
	if game.Indoor {
		return "Indoor"
	}
	var parts []string
	if game.TempF != nil {
		parts = append(parts, fmt.Sprintf("%.0f°F", *game.TempF))
	}
	if game.WindMPH != nil {
		parts = append(parts, fmt.Sprintf("%.0f mph wind", *game.WindMPH))
	}
	if game.WindGustMPH != nil {
		parts = append(parts, fmt.Sprintf("%.0f mph gusts", *game.WindGustMPH))
	}
	if game.PrecipIN != nil && *game.PrecipIN > 0 {
		parts = append(parts, fmt.Sprintf("%.2f in rain", *game.PrecipIN))
	}
	return strings.Join(parts, " · ")
}

// relatedBetCards lists the next-best cached bets in the same game, putting the
// same player's other markets first.
func relatedBetCards(
	game db.Game,
	rows []board.Row,
	games map[string]analyzer.Game,
	stats map[string]analyzer.PlayerStats,
	current board.Row,
	from, returnURL, homeBook string,
	slipID int64,
	activeAddURL string,
) []ResearchCardView {
	result := board.Explore(rows, games, stats, board.ExploreQuery{
		GameID: game.ID,
		Sort:   board.ExploreSortGrade,
		Limit:  len(rows),
	})
	related := make([]board.ScoredRow, 0, len(result.Rows))
	for _, scored := range result.Rows {
		// The other side of the same market is a hedge, not a related bet.
		if scored.Row.Market == current.Market {
			continue
		}
		related = append(related, scored)
	}
	slices.SortStableFunc(related, func(left, right board.ScoredRow) int {
		leftSame := current.PlayerID != "" && left.Row.PlayerID == current.PlayerID
		rightSame := current.PlayerID != "" && right.Row.PlayerID == current.PlayerID
		if leftSame != rightSame {
			if leftSame {
				return -1
			}
			return 1
		}
		if c := cmp.Compare(right.Grade.Score, left.Grade.Score); c != 0 {
			return c
		}
		return cmp.Compare(left.Row.Text, right.Row.Text)
	})
	const maxRelated = 5
	if len(related) > maxRelated {
		related = related[:maxRelated]
	}
	cards := researchCards([]db.Game{game}, related, from, homeBook, slipID)
	if returnURL == "" {
		returnURL = from
	}
	for i := range cards {
		cards[i].ReturnURL = returnURL
		if activeAddURL != "" {
			cards[i].AddPostURL = activeAddURL
			cards[i].AddHref = ""
			continue
		}
		sel := db.SelectionKey{
			GameID:     cards[i].GameID,
			MarketType: cards[i].MarketType,
			Team:       cards[i].Team,
			PlayerName: cards[i].PlayerName,
			PlayerID:   cards[i].PlayerID,
			Side:       cards[i].Side,
		}
		cards[i].AddHref = betAddSheetHref(sel, cards[i].Line, cards[i].OddsValue, returnURL)
	}
	return cards
}

// betAddSheet renders the eligible-slip chooser dialog body for one bet.
func (h *researchHandlers) betAddSheet(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	gameID := strings.TrimSpace(query.Get("game_id"))
	market := strings.ToUpper(strings.TrimSpace(query.Get("market")))
	if market == "" {
		market = strings.ToUpper(strings.TrimSpace(query.Get("market_type")))
	}
	if h.deps.Store == nil || gameID == "" || market == "" || market == "OTHER" {
		http.NotFound(w, r)
		return
	}
	game, err := h.deps.Store.GameByID(r.Context(), gameID)
	if errors.Is(err, db.ErrGameNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		h.deps.Logger.Error().Err(err).Str("game_id", gameID).Msg("load game for bet add sheet")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	side := strings.ToUpper(strings.TrimSpace(query.Get("side")))
	team := strings.TrimSpace(query.Get("team"))
	playerID := strings.TrimSpace(query.Get("player_id"))
	playerName := strings.TrimSpace(query.Get("player_name"))
	lineStr := strings.TrimSpace(query.Get("line"))
	oddsStr := strings.TrimSpace(query.Get("odds"))
	selection := strings.TrimSpace(query.Get("selection"))
	if selection == "" {
		var lineVal *float64
		if lineStr != "" {
			if parsed, err := strconv.ParseFloat(lineStr, 64); err == nil {
				lineVal = &parsed
			}
		}
		selection = db.BuildLegSelection(market, team, playerName, side, lineVal)
	}
	returnURL := safeReturnPath(query.Get("return"))
	if returnURL == "" {
		returnURL = "/research"
	}
	matchup := game.AwayTeam + " @ " + game.HomeTeam
	if matchup == " @ " {
		matchup = game.ID
	}
	now := h.deps.now()
	view := BetAddSheetView{
		GameID:     game.ID,
		MarketType: market,
		Team:       team,
		Side:       side,
		PlayerID:   playerID,
		PlayerName: playerName,
		Line:       lineStr,
		Odds:       oddsStr,
		Selection:  selection,
		Matchup:    matchup,
		ReturnURL:  returnURL,
		WeekID:     game.WeekID,
		KickedOff:  !now.Before(game.Kickoff),
		AddSlips:   h.betAddSlips(r.Context(), game),
	}
	h.renderPartial(w, "bet-add-sheet", view)
}

// betAddSlips lists the viewer's open slips that are still in the pick step and
// include the bet's game.
func (h *researchHandlers) betAddSlips(ctx context.Context, game db.Game) []BetAddSlipView {
	if h.deps.Store == nil || h.deps.UserFrom == nil || h.deps.GroupFrom == nil {
		return nil
	}
	user, ok := h.deps.UserFrom(ctx)
	if !ok {
		return nil
	}
	group, ok := h.deps.GroupFrom(ctx)
	if !ok {
		return nil
	}
	weekSlips, err := h.deps.Store.SlipsForWeek(ctx, group.ID, game.WeekID)
	if err != nil {
		return nil
	}
	now := h.deps.now()
	views := make([]BetAddSlipView, 0, len(weekSlips))
	for _, slip := range weekSlips {
		if slip.Status != db.SlipOpen {
			continue
		}
		if len(slips.FilterGamesForSlip([]db.Game{game}, slip)) == 0 {
			continue
		}
		members, err := h.deps.Store.SlipMembers(ctx, slip.ID)
		if err != nil {
			continue
		}
		legs, err := h.deps.Store.LegsForSlip(ctx, slip.ID)
		if err != nil {
			continue
		}
		next := slips.NextStep(slip, members, legs, []db.Game{game}, user.ID, now)
		if next.Step != slips.StepPick {
			continue
		}
		picked := 0
		for _, leg := range legs {
			if leg.UserID == user.ID {
				picked++
			}
		}
		views = append(views, BetAddSlipView{
			SlipID:   slip.ID,
			SlipName: slip.Name,
			PostURL:  fmt.Sprintf("/slips/%d/legs", slip.ID),
			Summary:  fmt.Sprintf("%d/%d picked", picked, slip.LegsPerMember),
		})
	}
	return views
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

	query := parseResearchQuery(r)
	query.GameID = gameID
	explore := h.buildExplore(r.Context(), query, "")
	explore = h.researchPanel(r.Context(), r, explore)
	explore.OOB = true
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
	if limit <= board.DefaultExploreLimit {
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
		view.Limit = board.DefaultExploreLimit
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
	homeBook := h.researchViewer(ctx)
	odds, err := h.deps.Store.OddsForWeek(ctx, week.ID)
	if err != nil {
		view.NoLines = !view.NoGames
		finishResearchExplore(&view, result)
		return view
	}

	rows := board.BuildDualBook(games, odds, roster, homeBook, nil, now)
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

	view.Cards = researchCards(games, result.Rows, researchPushURL(queryFromExplore(view)), homeBook, 0)
	view.TotalMatched = result.TotalMatched
	view.HasMore = result.HasMore
	view.AllStarted = !view.NoGames && board.StartedCount(games, now) == len(games)
	view.NoLines = !view.NoGames && !view.AllStarted && len(rows) == 0
	view.Empty = len(result.Rows) == 0 && !view.NoGames && !view.NoLines && !view.AllStarted

	if result.HasMore {
		next := queryFromExplore(view)
		next.Limit = view.Limit + board.DefaultExploreLimit
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

// researchViewer returns the signed-in person's home book.
func (h *researchHandlers) researchViewer(ctx context.Context) string {
	if h.deps.UserFrom == nil {
		return ""
	}
	user, ok := h.deps.UserFrom(ctx)
	if !ok || user.HomeBook == nil {
		return ""
	}
	return *user.HomeBook
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
	return analyzer.LoadBoardSignals(ctx, h.deps.Store, now, weekID, games, rowPlayerIDs(rows))
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
	teamEPA, playerUsage := analyzer.LoadGameSignals(ctx, h.deps.Store, now, weekID, rowPlayerIDs(rows))
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

// researchCards maps one explorer page to the candidate cards. from is the
// explorer URL a card's per-bet page returns to.
func researchCards(games []db.Game, rows []board.ScoredRow, from, homeBook string, slipID int64) []ResearchCardView {
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
			ImpliedHuman:    formatImpliedHuman(parlay.ImpliedProb(row.Odds)),
			GradeLetter:     scored.Grade.Letter,
			GradeDisplay:    scored.Grade.Diagnostics.DisplayGrade,
			GradeDescriptor: scored.Grade.Diagnostics.Descriptor,
			GradeScore:      scored.Grade.Score,
			GradeClass:      gradeClass(scored.Grade.Letter),
			Reason:          scored.Reason,
			Badge:           row.Badge,
			SplitLine:       splitLineBadge(row.Market, row.DK, row.FD, row.LinesDiffer, homeBook),
			GameSheetHref:   "/research?game_id=" + url.QueryEscape(row.GameID),
			AwayLogo:        db.StringValue(game.AwayLogo),
			HomeLogo:        db.StringValue(game.HomeLogo),
		}
		sel := db.SelectionKey{
			GameID:     row.GameID,
			MarketType: row.Market,
			Team:       row.Team,
			PlayerName: row.PlayerName,
			PlayerID:   row.PlayerID,
			Side:       row.Side,
		}
		card.BetHref = betResearchHref(sel, from, slipID)
		card.AddHref = betAddSheetHref(sel, card.Line, row.Odds, from)
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

// betResearchHref builds a per-bet research link from a SelectionKey. Line and
// price stay out of the URL so a shared link survives line drift.
func betResearchHref(sel db.SelectionKey, from string, slipID int64) string {
	if sel.GameID == "" || sel.MarketType == "" || sel.MarketType == "OTHER" {
		return ""
	}
	values := url.Values{}
	values.Set("game_id", sel.GameID)
	values.Set("market", sel.MarketType)
	if sel.Side != "" {
		values.Set("side", sel.Side)
	}
	if sel.Team != "" {
		values.Set("team", sel.Team)
	}
	if sel.PlayerID != "" {
		values.Set("player_id", sel.PlayerID)
	}
	if sel.PlayerName != "" {
		values.Set("player_name", sel.PlayerName)
	}
	if from != "" {
		values.Set("from", from)
	}
	if slipID > 0 {
		values.Set("slip_id", strconv.FormatInt(slipID, 10))
	}
	return "/research/bet?" + values.Encode()
}

// betAddSheetHref builds the HTMX URL for the eligible-slip chooser dialog.
// Returns "" when the selection cannot be added to a slip.
func betAddSheetHref(sel db.SelectionKey, line string, odds int, returnURL string) string {
	if sel.GameID == "" || sel.MarketType == "" || sel.MarketType == "OTHER" {
		return ""
	}
	values := url.Values{}
	values.Set("game_id", sel.GameID)
	values.Set("market", sel.MarketType)
	if sel.Side != "" {
		values.Set("side", sel.Side)
	}
	if sel.Team != "" {
		values.Set("team", sel.Team)
	}
	if sel.PlayerID != "" {
		values.Set("player_id", sel.PlayerID)
	}
	if sel.PlayerName != "" {
		values.Set("player_name", sel.PlayerName)
	}
	if line != "" {
		values.Set("line", line)
	}
	if odds != 0 {
		values.Set("odds", strconv.Itoa(odds))
	}
	if returnURL != "" {
		values.Set("return", returnURL)
	}
	return "/bets/add-sheet?" + values.Encode()
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
// data, falling back to cached nflverse weekly stats when ESPN game logs are
// missing or lack the active market.
func (h *researchHandlers) loadPlayerResearch(
	ctx context.Context,
	athleteID, legIDStr, marketParam, lineParam string,
) (*PlayerResearchView, error) {
	player, playerErr := h.deps.Store.RosterPlayerByID(ctx, athleteID)
	gamelog, logErr := h.deps.Store.PlayerGameLog(ctx, athleteID)
	pWeeks := h.loadNFLversePlayerWeeks(ctx, athleteID)

	if playerErr != nil && logErr != nil && len(pWeeks) == 0 {
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
	if len(pWeeks) > 0 {
		if view.Name == "" {
			view.Name = pWeeks[0].PlayerName
		}
		if view.Team == "" {
			view.Team = pWeeks[0].Team
		}
		if view.Position == "" {
			view.Position = pWeeks[0].Position
		}
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

	nflvMarkets, nflvEntries := buildNFLverseResearchGameLog(pWeeks, view.Position)
	if len(markets) == 0 && len(nflvMarkets) > 0 {
		markets = nflvMarkets
		entries = nflvEntries
		if view.GamesPlayed == 0 {
			view.GamesPlayed = len(pWeeks)
		}
	}

	optionMarkets := markets
	if len(nflvMarkets) > 0 {
		optionMarkets = make(map[string][]float64, len(markets)+len(nflvMarkets))
		for k, v := range nflvMarkets {
			if len(v) > 0 {
				optionMarkets[k] = v
			}
		}
		for k, v := range markets {
			if len(v) > 0 {
				optionMarkets[k] = v
			}
		}
	}

	activeMarket, comparisonLine := h.resolveMarketAndLine(
		ctx, athleteID, player, view.Position, optionMarkets, legIDStr, marketParam, lineParam,
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

	view.AvailableMarkets = buildMarketOptions(athleteID, optionMarkets, activeMarket)

	h.populateMatchupContext(ctx, view, logos)
	sig := h.populateNFLverseResearch(ctx, view, activeMarket, comparisonLine)

	chartGames := markets[activeMarket]
	chartEntries := entries
	if len(chartGames) == 0 && len(nflvMarkets[activeMarket]) > 0 {
		chartGames = nflvMarkets[activeMarket]
		chartEntries = nflvEntries
		if view.GamesPlayed == 0 {
			view.GamesPlayed = len(pWeeks)
		}
	}
	if len(chartGames) > 0 {
		buildPlayerChart(view, chartGames, chartEntries, comparisonLine, sig)
	}

	return view, nil
}

func (h *researchHandlers) loadNFLversePlayerWeeks(ctx context.Context, athleteID string) []db.NFLversePlayerWeek {
	if athleteID == "" || h.deps.Store == nil {
		return nil
	}
	week, err := h.deps.Store.LatestWeek(ctx)
	if err != nil {
		return nil
	}
	seasonTypes := analyzer.NFLverseSeasonTypes(week.SeasonType)
	rows, err := h.deps.Store.NFLversePlayerWeeksForPlayerTypes(ctx, athleteID, week.Season, seasonTypes...)
	if err != nil || len(rows) == 0 {
		return nil
	}
	return rows
}

func buildNFLverseResearchGameLog(
	rows []db.NFLversePlayerWeek,
	position string,
) (map[string][]float64, []espn.GameLogEntry) {
	if len(rows) == 0 {
		return nil, nil
	}
	normPos := db.NormalizePosition(position)
	if normPos == "" {
		normPos = db.NormalizePosition(rows[0].Position)
	}

	n := len(rows)
	all := map[string][]float64{
		"PASS_YDS":   make([]float64, 0, n),
		"PASS_TDS":   make([]float64, 0, n),
		"RUSH_YDS":   make([]float64, 0, n),
		"RUSH_ATT":   make([]float64, 0, n),
		"REC_YDS":    make([]float64, 0, n),
		"RECEPTIONS": make([]float64, 0, n),
		"ANYTIME_TD": make([]float64, 0, n),
		"KICKER_PTS": make([]float64, 0, n),
	}
	entries := make([]espn.GameLogEntry, 0, n)
	hasNonZero := make(map[string]bool, len(all))
	for _, r := range rows {
		vals := map[string]float64{
			"PASS_YDS":   float64(r.PassYds),
			"PASS_TDS":   float64(r.PassTDs),
			"RUSH_YDS":   float64(r.RushYds),
			"RUSH_ATT":   float64(r.Carries),
			"REC_YDS":    float64(r.RecYds),
			"RECEPTIONS": float64(r.Receptions),
			"ANYTIME_TD": float64(r.RushTDs + r.RecTDs),
			"KICKER_PTS": float64(r.KickerPts),
		}
		for m, v := range vals {
			all[m] = append(all[m], v)
			if v != 0 {
				hasNonZero[m] = true
			}
		}
		entries = append(entries, espn.GameLogEntry{
			Week:     r.WeekNum,
			Opponent: r.Opponent,
		})
	}

	markets := make(map[string][]float64, len(all))
	for _, m := range positionPrimaryMarkets(normPos) {
		markets[m] = all[m]
	}
	for m, vals := range all {
		if hasNonZero[m] {
			markets[m] = vals
		}
	}
	if len(markets) == 0 {
		markets = all
	}
	return markets, entries
}

func positionPrimaryMarkets(normPos string) []string {
	switch normPos {
	case "QB":
		return []string{"PASS_YDS", "PASS_TDS", "RUSH_YDS", "RUSH_ATT", "ANYTIME_TD"}
	case "RB":
		return []string{"RUSH_YDS", "RUSH_ATT", "REC_YDS", "RECEPTIONS", "ANYTIME_TD"}
	case "WR", "TE":
		return []string{"REC_YDS", "RECEPTIONS", "ANYTIME_TD"}
	case "K":
		return []string{"KICKER_PTS"}
	default:
		return nil
	}
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
	oppRoleLabel := sig.OppRoleLabel
	if oppRoleLabel == "" {
		oppRoleLabel = normPos
	}
	if sig.Opponent != "" && oppRoleLabel != "" && view.ActiveMarketName != "" {
		view.OppDefenseTitle = fmt.Sprintf(
			"%s Defense · Last 5 vs %s · %s",
			sig.Opponent,
			oppRoleLabel,
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
	position string,
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
		activeMarket = defaultResearchMarket(position, markets)
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

func defaultResearchMarket(position string, markets map[string][]float64) string {
	for _, m := range positionPrimaryMarkets(db.NormalizePosition(position)) {
		if len(markets[m]) > 0 {
			return m
		}
	}
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
