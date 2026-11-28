package web

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/board"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
	"github.com/kalverra/parlay-club/internal/slips"
)

// PickPage is the focused pick board: the game slate, the open game's detail,
// and the dock with its tray sheet.
type PickPage struct {
	Page
	LiveURL  string
	SlipID   int64
	SlipName string
	Body     PickBodyView
	Dock     PickDockView
}

// PickBodyView is the #pick-body partial: the slate and, when a game is open,
// its detail.
type PickBodyView struct {
	SlipID   int64
	Search   string
	GroupBy  string
	Category string
	Window   string
	Sort     string
	Game     string
	Slate    PickSlateView
	Detail   *PickDetailView
}

// PickSlateView is the game list: the search box, kickoff-window chips, and one
// card per upcoming game.
type PickSlateView struct {
	SlipID     int64
	Search     string
	GroupBy    string
	Category   string
	Window     string
	Sort       string
	Game       string
	RefreshURL string
	CanRefresh bool
	Detail     bool
	Cards      []PickGameCardView
	Windows    []PickChip
	Started    int
	Empty      bool
	NoGames    bool
	NoLines    bool
	AllStarted bool
}

// GameGradeBadge is one non-zero grade-tier count pill on a game card.
type GameGradeBadge struct {
	Letter    string
	Count     int
	Class     string
	AriaLabel string
}

// PickGameCardView is one game on the slate, with its lines at a glance. A game
// with lines but no prop rows offers to load them.
type PickGameCardView struct {
	SlipID            int64
	ID                string
	Matchup           string
	Venue             string
	Weather           string
	Away              string
	Home              string
	AwayLogo          string
	HomeLogo          string
	KickoffISO        string
	KickoffFallback   string
	Spread            string
	SpreadSplit       string
	Total             string
	TotalSplit        string
	Moneyline         string
	Props             int
	HasProps          bool
	Unloaded          bool
	GradeBadges       []GameGradeBadge
	TopPlayersSummary string
	Href              string
	PushURL           string
	Active            bool
}

// PickDetailView is the open game's pinned header, its tab bar, and the rows
// of the active tab. The Lines tab holds the game lines and the team totals; a
// prop tab holds one card per player with that player's lines.
type PickDetailView struct {
	SlipID          int64
	GameID          string
	Matchup         string
	Away            string
	Home            string
	AwayLogo        string
	HomeLogo        string
	Venue           string
	KickoffISO      string
	KickoffFallback string
	Search          string
	GroupBy         string
	Category        string
	Sort            string
	RefreshURL      string
	CanRefresh      bool
	OOB             bool
	BackHref        string
	BackPushURL     string
	Groups          []PickChip
	Tabs            []PickChip
	Sorts           []PickChip
	Unloaded        bool
	Highlights      HighlightsView
	Sections        []PickDetailSectionView
	Empty           bool
}

// PickDetailSectionView is one grouped section in the open game's detail.
type PickDetailSectionView struct {
	Key        string
	Label      string
	Badge      string
	BadgeClass string
	Count      int
	Lines      []PickLineGroupView
	Players    []PickPlayerView
	Options    []PickRowView
}

// PickLineGroupView is one line and the sides a person can pick from it.
type PickLineGroupView struct {
	ID         string
	Label      string
	IsOU       bool
	SharedLine string
	SplitLine  string
	Options    []PickRowView
}

// PickPlayerView is one player's card in a prop tab. A player's markets share
// the card, so Receiving Yards and Receptions sit together.
type PickPlayerView struct {
	Name        string
	Initials    string
	HeadshotURL string
	Position    string
	Team        string
	TeamLogo    string
	Tags        []string
	Lines       []PickLineGroupView
}

// PickChip is one filter or sort link.
type PickChip struct {
	Value   string
	Label   string
	Href    string
	PushURL string
	Count   int
	Active  bool
}

// PickRowView is one selection a person can add, remove, or swap.
type PickRowView struct {
	SlipID        int64
	GameID        string
	Market        string
	Team          string
	PlayerName    string
	PlayerID      string
	Side          string
	Line          string
	Odds          int
	Price         string
	ImpliedChance string
	ImpliedHuman  string
	Text          string
	PickID        int64
	UpdateID      int64
	AtLimit       bool
	SwapURL       string
	ResearchURL   string
	Badge         string
	ComboWarning  string
	SplitLine     string
	DriftNote     string
	CanRefresh    bool
	FromDetail    bool
	Grade         GradeView
}

// PickDockView is the viewer's own picks: the fixed dock's count and combined
// price, and the tray sheet's legs, warnings, and one next step.
type PickDockView struct {
	SlipID               int64
	Picked               int
	Limit                int
	Combined             string
	CombinedAmerican     string
	CombinedImplied      string
	CombinedImpliedHuman string
	Legs                 []TrayLegView
	Grade                GradeView
	Warnings             []AnalyzerWarningView
	ActionLabel          string
	ActionHref           string
}

// TrayLegView is one of the viewer's legs in the tray.
type TrayLegView struct {
	ID            int64
	Selection     string
	SideBadge     string
	SideClass     string
	Odds          string
	ImpliedChance string
	ImpliedHuman  string
	Grade         GradeView
	Locked        bool
	SplitLine     string
	DriftNote     string
	Refresh       *LegRefreshView
}

// LegRefreshView carries the current board fields needed to sync a drifted leg
// to the latest line and price with one tap.
type LegRefreshView struct {
	GameID     string
	Market     string
	Team       string
	PlayerName string
	PlayerID   string
	Side       string
	Line       string
	Odds       int
}

// PickSwapView is the bottom sheet that swaps one leg for a new pick.
type PickSwapView struct {
	SlipID int64
	Text   string
	Target string
	Fields PickRowView
	Legs   []TrayLegView
	Q      string
	Group  string
	Cat    string
	Win    string
	Sort   string
	Game   string
}

// pickBoard shows the focused pick screen.
func (h *slipHandlers) pickBoard(w http.ResponseWriter, r *http.Request) {
	slip, _, _, ok := h.loadSlip(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	q := pickQuery(r, h.deps.now())
	if err := h.kickPickIfOpen(r.Context(), slip, q, true); err != nil {
		h.serverError(w, err)
		return
	}
	data, err := h.pickPageData(r.Context(), slip, q)
	if err != nil {
		h.serverError(w, err)
		return
	}
	page := h.pageForGroup(r, "Pick", slip.GroupID)
	page.Tab = ""
	page.Wide = true
	data.Page = page
	h.renderPageCtx(r.Context(), w, "pick", data)
}

// pickResults returns the #pick-body partial for HTMX searches, filters, game
// switches, and sorts.
func (h *slipHandlers) pickResults(w http.ResponseWriter, r *http.Request) {
	slip, _, _, ok := h.loadSlip(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	q := pickQuery(r, h.deps.now())
	if err := h.kickPickIfOpen(r.Context(), slip, q, false); err != nil {
		h.serverError(w, err)
		return
	}
	data, err := h.pickPageData(r.Context(), slip, q)
	if err != nil {
		h.serverError(w, err)
		return
	}
	// A search or filter swap replaces the body, so the browser should carry the
	// full page URL rather than the partial URL the request used.
	self := board.Query{
		Q:        data.Body.Search,
		GroupBy:  q.GroupBy,
		Category: board.Category(data.Body.Category),
		Window:   board.Window(data.Body.Window),
		Sort:     board.Sort(data.Body.Sort),
		GameID:   data.Body.Game,
	}
	w.Header().Set("Hx-Push-Url", pickPushURL(slip.ID, self, "", ""))
	h.renderPartialCtx(r.Context(), w, "pick-body", data.Body)
}

// pickSlate returns the slate alone, so a live board signal refreshes the game
// cards without touching the open game's tab or sort.
func (h *slipHandlers) pickSlate(w http.ResponseWriter, r *http.Request) {
	slip, _, _, ok := h.loadSlip(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	slate, err := h.pickSlateData(r.Context(), slip, pickQuery(r, h.deps.now()))
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.renderPartialCtx(r.Context(), w, "pick-slate", slate)
}

// pickDetail returns the #pick-detail partial for a tab or sort change. It
// leaves the slate alone, so its scroll and the open game stay put.
func (h *slipHandlers) pickDetail(w http.ResponseWriter, r *http.Request) {
	slip, _, _, ok := h.loadSlip(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	q := pickQuery(r, h.deps.now())
	if err := h.kickPickIfOpen(r.Context(), slip, q, false); err != nil {
		h.serverError(w, err)
		return
	}
	detail, ok, err := h.pickDetailData(r.Context(), slip, q)
	if err != nil {
		h.serverError(w, err)
		return
	}
	if !ok {
		h.renderPartialCtx(r.Context(), w, "pick-detail-empty", nil)
		return
	}
	// The tab and sort live in the detail's URL, but the search form still posts
	// hidden copies. Swap them out of band so the next search keeps the tab.
	detail.OOB = true
	h.renderPartialCtx(r.Context(), w, "pick-detail", detail)
}

// pickRefresh triggers a manual pick-time odds refresh on an open slip and
// returns the updated #pick-body partial.
func (h *slipHandlers) pickRefresh(w http.ResponseWriter, r *http.Request) {
	slip, _, _, ok := h.loadSlip(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	if slip.Status != db.SlipOpen {
		h.respondError(w, slips.ErrSlipNotOpen)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.writeFormError(w, "Bad form")
		return
	}
	q := pickQuery(r, h.deps.now())
	if h.deps.Refresh != nil {
		hasUpcoming, gameUpcoming, err := h.syncPickScope(r.Context(), slip, q.GameID, q.Now)
		if err != nil {
			h.serverError(w, err)
			return
		}
		if hasUpcoming {
			refreshGameID := ""
			if gameUpcoming {
				refreshGameID = q.GameID
			}
			if err := h.deps.Refresh.ForcePickRefresh(r.Context(), slip.WeekID, refreshGameID); err != nil {
				h.deps.Logger.Warn().
					Err(err).
					Int64("slip_id", slip.ID).
					Str("game_id", refreshGameID).
					Msg("force pick refresh")
			}
		}
	}
	data, err := h.pickPageData(r.Context(), slip, q)
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.renderPartialCtx(r.Context(), w, "pick-body", data.Body)
}

// kickPickIfOpen starts a background pick-time odds refresh when slip is OPEN
// and upcoming games remain in its scope, while the response renders from cached
// rows. When kickLines is false, it refreshes only when an upcoming game is
// selected.
func (h *slipHandlers) kickPickIfOpen(ctx context.Context, slip db.Slip, q board.Query, kickLines bool) error {
	if h.deps.Refresh == nil || slip.Status != db.SlipOpen {
		return nil
	}
	if !kickLines && q.GameID == "" {
		return nil
	}
	hasUpcoming, gameUpcoming, err := h.syncPickScope(ctx, slip, q.GameID, q.Now)
	if err != nil {
		return err
	}
	if gameUpcoming {
		h.deps.Refresh.KickPickGame(ctx, slip.WeekID, q.GameID)
		return nil
	}
	if kickLines && hasUpcoming {
		h.deps.Refresh.KickPickLines(ctx, slip.WeekID)
	}
	return nil
}

// syncPickScope reports whether slip has any upcoming game in its slate scope
// and whether gameID itself is an upcoming game in that scope.
func (h *slipHandlers) syncPickScope(
	ctx context.Context,
	slip db.Slip,
	gameID string,
	now time.Time,
) (hasUpcoming, gameUpcoming bool, err error) {
	if slip.Status != db.SlipOpen || h.deps.Store == nil {
		return false, false, nil
	}
	allGames, err := h.deps.Store.GamesForWeek(ctx, slip.WeekID)
	if err != nil {
		return false, false, err
	}
	games := slips.FilterGamesForSlip(allGames, slip)
	for _, game := range games {
		if now.Before(game.Kickoff) {
			hasUpcoming = true
			if gameID != "" && game.ID == gameID {
				gameUpcoming = true
			}
		}
	}
	return hasUpcoming, gameUpcoming, nil
}

// pickTray returns the tray partial for the dock's sheet.
func (h *slipHandlers) pickTray(w http.ResponseWriter, r *http.Request) {
	slip, _, _, ok := h.loadSlip(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	dock, err := h.pickDockData(r.Context(), slip)
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.renderPartial(w, "pick-tray", dock)
}

// pickDock returns the fixed dock partial, so a leg change updates the count
// and the combined price without a page reload.
func (h *slipHandlers) pickDock(w http.ResponseWriter, r *http.Request) {
	slip, _, _, ok := h.loadSlip(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	dock, err := h.pickDockData(r.Context(), slip)
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.renderPartial(w, "pick-dock", dock)
}

// pickSwap shows the swap sheet for one unpicked row.
func (h *slipHandlers) pickSwap(w http.ResponseWriter, r *http.Request) {
	slip, _, _, ok := h.loadSlip(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	dock, err := h.pickDockData(r.Context(), slip)
	if err != nil {
		h.serverError(w, err)
		return
	}
	// Only unlocked legs can be swapped out. A kicked-off leg stays visible in
	// the tray, but it is not a choice here.
	legs := make([]TrayLegView, 0, len(dock.Legs))
	for _, leg := range dock.Legs {
		if !leg.Locked {
			legs = append(legs, leg)
		}
	}
	view := PickSwapView{
		SlipID: slip.ID,
		Text:   r.URL.Query().Get("text"),
		Legs:   legs,
		Q:      r.URL.Query().Get("q"),
		Group:  r.URL.Query().Get("group"),
		Cat:    r.URL.Query().Get("cat"),
		Win:    r.URL.Query().Get("win"),
		Sort:   r.URL.Query().Get("sort"),
		Game:   r.URL.Query().Get("game"),
	}
	view.Fields = PickRowView{
		SlipID:     slip.ID,
		GameID:     r.URL.Query().Get("game_id"),
		Market:     r.URL.Query().Get("market"),
		Team:       r.URL.Query().Get("team"),
		PlayerName: r.URL.Query().Get("player_name"),
		PlayerID:   r.URL.Query().Get("player_id"),
		Side:       r.URL.Query().Get("side"),
		Line:       r.URL.Query().Get("line"),
		Odds:       intFromString(r.URL.Query().Get("odds")),
		Text:       r.URL.Query().Get("text"),
	}
	if view.Group == string(board.GroupByGrade) {
		view.Target = "#pick-detail"
	} else {
		view.Target = "#" + pickLineID(
			view.Fields.GameID,
			view.Fields.Market,
			view.Fields.Team,
			view.Fields.PlayerName,
			floatPointer(view.Fields.Line),
		)
	}
	h.renderPartial(w, "pick-swap", view)
}

// pickPageData loads and shapes one board request.
func (h *slipHandlers) pickPageData(
	ctx context.Context,
	slip db.Slip,
	q board.Query,
) (PickPage, error) {
	dbStart := time.Now()
	legs, err := h.deps.Store.LegsForSlip(ctx, slip.ID)
	if err != nil {
		return PickPage{}, err
	}
	allGames, err := h.deps.Store.GamesForWeek(ctx, slip.WeekID)
	if err != nil {
		return PickPage{}, err
	}
	games := slips.FilterGamesForSlip(allGames, slip)
	odds, err := h.deps.Store.OddsForWeek(ctx, slip.WeekID)
	if err != nil {
		return PickPage{}, err
	}
	roster, err := h.deps.Store.RosterForTeams(ctx, weekTeams(games))
	if err != nil {
		return PickPage{}, err
	}
	AddDBTiming(ctx, time.Since(dbStart))

	user, homeBook, q := h.pickViewer(ctx, q)
	mine := legsForUser(legs, user.ID)
	now := q.Now

	boardStart := time.Now()
	rows := board.BuildDualBook(games, odds, roster, homeBook, mine, now)
	AddBoardTiming(ctx, time.Since(boardStart))

	dockStart := time.Now()
	dock, err := h.dockFrom(ctx, slip, legs, allGames, mine, rows, homeBook)
	AddDBTiming(ctx, time.Since(dockStart))
	if err != nil {
		return PickPage{}, err
	}

	slateStart := time.Now()
	slateModel := board.BuildSlate(games, rows, roster, q)
	var boardGames map[string]analyzer.Game
	var boardStats map[string]analyzer.PlayerStats
	var insights map[string]gameCardInsights
	if slip.Status == db.SlipOpen && len(rows) > 0 {
		sigStart := time.Now()
		boardGames, boardStats = h.loadBoardSignalsForRows(ctx, now, slip.WeekID, games, rows)
		AddDBTiming(ctx, time.Since(sigStart))
		rows = board.ScoreRows(rows, boardGames, boardStats, h.deps.GradingEngine)
		insights = gameCardInsightsByGame(games, rows, boardGames, boardStats)
	}
	slate := buildPickSlate(slip, slateModel, rows, homeBook, q, insights)
	AddBoardTiming(ctx, time.Since(slateStart))
	q.GameID = slateModel.ResolvedGameID

	remainingSlots := max(slip.LegsPerMember-len(mine), 0)
	atLimit := remainingSlots == 0
	body := PickBodyView{
		SlipID:   slip.ID,
		Search:   q.Q,
		GroupBy:  string(q.GroupBy),
		Category: string(q.Category),
		Window:   string(q.Window),
		Sort:     string(q.Sort),
		Game:     q.GameID,
		Slate:    slate,
	}
	if slateModel.OpenGame != nil {
		gamesByID := make(map[string]db.Game, len(games))
		for _, game := range games {
			gamesByID[game.ID] = game
		}
		game := gamesByID[slateModel.OpenGame.ID]
		detail := h.buildPickDetail(
			ctx, slip, game, *slateModel.OpenGame, rows, legs, q, now,
			atLimit, remainingSlots, boardGames, boardStats, homeBook,
		)
		body.Detail = &detail
		body.GroupBy = detail.GroupBy
		body.Category = detail.Category
		body.Sort = detail.Sort
	}
	body.Slate.GroupBy = body.GroupBy
	body.Slate.Category = body.Category
	body.Slate.Sort = body.Sort
	body.Slate.Detail = body.Detail != nil
	return PickPage{
		LiveURL:  slipLiveURL(slip),
		SlipID:   slip.ID,
		SlipName: slip.Name,
		Body:     body,
		Dock:     dock,
	}, nil
}

// pickSlateData loads only the slate for pickSlate, skipping dockFrom and Detail.
func (h *slipHandlers) pickSlateData(
	ctx context.Context,
	slip db.Slip,
	q board.Query,
) (PickSlateView, error) {
	dbStart := time.Now()
	allGames, err := h.deps.Store.GamesForWeek(ctx, slip.WeekID)
	if err != nil {
		return PickSlateView{}, err
	}
	games := slips.FilterGamesForSlip(allGames, slip)
	odds, err := h.deps.Store.OddsForWeek(ctx, slip.WeekID)
	if err != nil {
		return PickSlateView{}, err
	}
	roster, err := h.deps.Store.RosterForTeams(ctx, weekTeams(games))
	if err != nil {
		return PickSlateView{}, err
	}
	var legs []db.Leg
	if slip.Status == db.SlipOpen {
		legs, err = h.deps.Store.LegsForSlip(ctx, slip.ID)
		if err != nil {
			return PickSlateView{}, err
		}
	}
	AddDBTiming(ctx, time.Since(dbStart))

	user, homeBook, q := h.pickViewer(ctx, q)
	mine := legsForUser(legs, user.ID)
	now := q.Now

	boardStart := time.Now()
	rows := board.BuildDualBook(games, odds, roster, homeBook, mine, now)
	var insights map[string]gameCardInsights
	if slip.Status == db.SlipOpen && len(rows) > 0 {
		sigStart := time.Now()
		boardGames, boardStats := h.loadBoardSignalsForRows(ctx, now, slip.WeekID, games, rows)
		AddDBTiming(ctx, time.Since(sigStart))
		rows = board.ScoreRows(rows, boardGames, boardStats, h.deps.GradingEngine)
		insights = gameCardInsightsByGame(games, rows, boardGames, boardStats)
	}
	slate := buildPickSlate(slip, board.BuildSlate(games, rows, roster, q), rows, homeBook, q, insights)
	AddBoardTiming(ctx, time.Since(boardStart))
	return slate, nil
}

// pickDetailData loads only the open game's odds and roster for pickDetail.
func (h *slipHandlers) pickDetailData(
	ctx context.Context,
	slip db.Slip,
	q board.Query,
) (PickDetailView, bool, error) {
	if q.GameID == "" {
		return PickDetailView{}, false, nil
	}
	dbStart := time.Now()
	game, err := h.deps.Store.GameByID(ctx, q.GameID)
	if errors.Is(err, db.ErrGameNotFound) {
		return PickDetailView{}, false, nil
	}
	if err != nil {
		return PickDetailView{}, false, err
	}
	if game.WeekID != slip.WeekID || len(slips.FilterGamesForSlip([]db.Game{game}, slip)) == 0 {
		return PickDetailView{}, false, nil
	}

	legs, err := h.deps.Store.LegsForSlip(ctx, slip.ID)
	if err != nil {
		return PickDetailView{}, false, err
	}
	odds, err := h.deps.Store.OddsForGame(ctx, game.ID)
	if err != nil {
		return PickDetailView{}, false, err
	}
	roster, err := h.deps.Store.RosterForTeams(ctx, []string{game.AwayTeam, game.HomeTeam})
	if err != nil {
		return PickDetailView{}, false, err
	}
	AddDBTiming(ctx, time.Since(dbStart))

	user, homeBook, q := h.pickViewer(ctx, q)
	mine := legsForUser(legs, user.ID)
	now := q.Now

	boardStart := time.Now()
	gameSlice := []db.Game{game}
	rows := board.BuildDualBook(gameSlice, odds, roster, homeBook, mine, now)
	openCards := board.Slates(gameSlice, rows, roster, board.Query{Window: q.Window, Now: now})
	AddBoardTiming(ctx, time.Since(boardStart))
	if len(openCards) == 0 {
		return PickDetailView{}, false, nil
	}

	remainingSlots := max(slip.LegsPerMember-len(mine), 0)
	atLimit := remainingSlots == 0
	var boardGames map[string]analyzer.Game
	var boardStats map[string]analyzer.PlayerStats
	if slip.Status == db.SlipOpen && len(rows) > 0 {
		sigStart := time.Now()
		boardGames, boardStats = h.loadBoardSignalsForRows(ctx, now, slip.WeekID, gameSlice, rows)
		AddDBTiming(ctx, time.Since(sigStart))
		rows = board.ScoreRows(rows, boardGames, boardStats, h.deps.GradingEngine)
	}
	detail := h.buildPickDetail(
		ctx, slip, game, openCards[0], rows, legs, q, now,
		atLimit, remainingSlots, boardGames, boardStats, homeBook,
	)
	return detail, true, nil
}

func (h *slipHandlers) pickViewer(ctx context.Context, q board.Query) (db.User, string, board.Query) {
	user, _ := h.user(ctx)
	homeBook := ""
	if user.HomeBook != nil {
		homeBook = *user.HomeBook
	}
	if q.Now.IsZero() {
		q.Now = h.deps.now()
	}
	return user, homeBook, q
}

func buildPickSlate(
	slip db.Slip,
	model board.Slate,
	rows []board.Row,
	homeBook string,
	q board.Query,
	insightsByGame map[string]gameCardInsights,
) PickSlateView {
	q.GameID = model.ResolvedGameID
	slate := PickSlateView{
		SlipID:     slip.ID,
		Search:     q.Q,
		GroupBy:    string(q.GroupBy),
		Category:   string(q.Category),
		Window:     string(q.Window),
		Sort:       string(q.Sort),
		Game:       model.ResolvedGameID,
		RefreshURL: slateHref(slip.ID),
		Windows:    windowChips(slip.ID, model.WindowCounts, q),
		Started:    model.Started,
		NoGames:    model.NoGames,
		AllStarted: model.AllStarted,
		NoLines:    model.NoLines,
		Empty:      model.Empty,
		CanRefresh: slip.Status == db.SlipOpen && !model.NoGames && !model.AllStarted,
		Detail:     model.ResolvedGameID != "",
	}
	splitsByGame := gameCardSplitLines(model.Cards, rows, homeBook)
	for _, card := range model.Cards {
		view := pickGameCardView(slip.ID, card, q)
		if split, ok := splitsByGame[card.ID]; ok {
			view.SpreadSplit = split.spread
			view.TotalSplit = split.total
		}
		if info, ok := insightsByGame[card.ID]; ok {
			view.GradeBadges = info.Grades
			view.TopPlayersSummary = info.TopPlayersSummary
		}
		slate.Cards = append(slate.Cards, view)
	}
	return slate
}

type gameCardSplit struct {
	spread string
	total  string
}

func gameCardSplitLines(cards []board.GameCard, rows []board.Row, homeBook string) map[string]gameCardSplit {
	cardByID := make(map[string]board.GameCard, len(cards))
	for _, card := range cards {
		cardByID[card.ID] = card
	}
	out := make(map[string]gameCardSplit, len(cards))
	for _, row := range rows {
		if !row.LinesDiffer {
			continue
		}
		card, ok := cardByID[row.GameID]
		if !ok {
			continue
		}
		split := out[row.GameID]
		switch row.Market {
		case "SPREAD":
			if split.spread != "" {
				continue
			}
			wantTeam := favoredSpreadTeam(card)
			if wantTeam != "" && row.Team != wantTeam {
				continue
			}
			if badge := splitLineBadge(row.Market, row.DK, row.FD, row.LinesDiffer, homeBook); badge != "" {
				split.spread = row.Team + " " + badge
			}
		case "TOTAL":
			if split.total != "" {
				continue
			}
			if row.Side != "" && row.Side != "OVER" {
				continue
			}
			if badge := splitLineBadge(row.Market, row.DK, row.FD, row.LinesDiffer, homeBook); badge != "" {
				split.total = "O/U " + badge
			}
		}
		out[row.GameID] = split
	}
	return out
}

func favoredSpreadTeam(card board.GameCard) string {
	spread := card.Lines.Spread
	if spread == nil {
		return ""
	}
	if spread.HomeLine != "" && strings.HasPrefix(spread.HomeLine, "-") {
		return card.Home
	}
	if spread.AwayLine != "" {
		return card.Away
	}
	if spread.HomeLine != "" {
		return card.Home
	}
	return ""
}

func (h *slipHandlers) buildPickDetail(
	ctx context.Context,
	slip db.Slip,
	game db.Game,
	card board.GameCard,
	rows []board.Row,
	legs []db.Leg,
	q board.Query,
	now time.Time,
	atLimit bool,
	remainingSlots int,
	boardGames map[string]analyzer.Game,
	boardStats map[string]analyzer.PlayerStats,
	homeBook string,
) PickDetailView {
	dbStart := time.Now()
	signals := h.boardSignalsForGame(ctx, slip.WeekID, game, legs, boardRowPlayerIDs(game.ID, rows))
	AddDBTiming(ctx, time.Since(dbStart))

	boardStart := time.Now()
	model := board.Detail(game, rows, board.Query{
		Q:        q.Q,
		GroupBy:  q.GroupBy,
		Category: q.Category,
		Sort:     q.Sort,
		Now:      now,
		Signals:  signals,
	})
	detail := pickDetailView(
		slip.ID,
		model,
		q,
		atLimit,
		card.Unloaded,
		slip.Status == db.SlipOpen,
		homeBook,
	)
	detail.CanRefresh = slip.Status == db.SlipOpen
	detail.Highlights = gameHighlightsView(slip, game.ID, rows, remainingSlots, boardGames, boardStats, homeBook)
	AddBoardTiming(ctx, time.Since(boardStart))
	return detail
}

// pickDetailView maps one game's detail model to the template's fields. The
// links keep the resolved tab and sort, so changing one keeps the other.
func pickDetailView(
	slipID int64,
	model board.GameDetail,
	q board.Query,
	atLimit bool,
	unloaded bool,
	canRefresh bool,
	homeBook string,
) PickDetailView {
	activeCategory := q.Category
	if activeCategory == board.CategoryTeamTotals {
		activeCategory = board.CategoryLines
	}

	linkQuery := q
	linkQuery.GroupBy = q.GroupBy
	linkQuery.Category = activeCategory
	linkQuery.Sort = model.Sort

	view := PickDetailView{
		SlipID:          slipID,
		GameID:          model.Game.ID,
		Matchup:         model.Game.Matchup,
		Away:            model.Game.Away,
		Home:            model.Game.Home,
		AwayLogo:        model.Game.AwayLogo,
		HomeLogo:        model.Game.HomeLogo,
		Venue:           model.Game.Venue,
		KickoffISO:      kickoffISO(model.Game.Kickoff),
		KickoffFallback: kickoffFallback(model.Game.Kickoff),
		Search:          q.Q,
		GroupBy:         string(model.GroupBy),
		Category:        string(activeCategory),
		Sort:            string(model.Sort),
		RefreshURL:      detailHref(slipID, linkQuery, "", ""),
		BackHref:        pickBackHref(slipID, q),
		BackPushURL:     pickBackPushURL(slipID, q),
		Unloaded:        unloaded,
	}
	for _, option := range board.DetailGroupBys {
		view.Groups = append(view.Groups, detailChip(
			slipID, linkQuery, "group", string(option), option.Label(), 0, option == model.GroupBy))
	}
	totalCount := 0
	for _, tab := range model.Tabs {
		totalCount += tab.Count
	}
	view.Tabs = append(view.Tabs, detailChip(
		slipID, linkQuery, "cat", "", "All", totalCount, activeCategory == ""))
	for _, tab := range model.Tabs {
		view.Tabs = append(view.Tabs, detailChip(
			slipID, linkQuery, "cat", string(tab.Category), tab.Label, tab.Count, tab.Category == activeCategory))
	}
	for _, option := range board.DetailSorts {
		view.Sorts = append(view.Sorts, detailChip(
			slipID, linkQuery, "sort", string(option), option.Label(), 0, option == model.Sort))
	}

	var pickedByLine map[string]int64
	var pickedCountByLine map[string]int
	if model.GroupBy == board.GroupByGrade {
		pickedByLine = make(map[string]int64)
		pickedCountByLine = make(map[string]int)
		for _, sec := range model.Sections {
			for _, row := range sec.Options {
				if row.Picked != 0 {
					key := row.PlayerID + "|" + board.LineKey(row.Market, row.Team, row.PlayerName, row.Line)
					pickedByLine[key] = row.Picked
					pickedCountByLine[key]++
				}
			}
		}
	}
	for _, sec := range model.Sections {
		secView := PickDetailSectionView{
			Key:   sec.Key,
			Label: sec.Label,
			Badge: sec.Badge,
			Count: sec.Count,
		}
		if sec.Badge != "" {
			secView.BadgeClass = gameGradeBadgeClass(sec.Badge)
		}
		for _, group := range sec.Lines {
			secView.Lines = append(
				secView.Lines,
				pickLineGroupView(slipID, group, atLimit, linkQuery, false, canRefresh, homeBook),
			)
		}
		for _, player := range sec.Players {
			secView.Players = append(
				secView.Players,
				pickPlayerView(slipID, player, model.Game, atLimit, linkQuery, canRefresh, homeBook),
			)
		}
		for _, row := range sec.Options {
			option := pickRowView(slipID, row, atLimit, linkQuery, canRefresh, homeBook)
			option.FromDetail = model.GroupBy == board.GroupByGrade
			if option.PickID == 0 && model.GroupBy == board.GroupByGrade {
				key := row.PlayerID + "|" + board.LineKey(row.Market, row.Team, row.PlayerName, row.Line)
				if pickedCountByLine[key] == 1 {
					option.UpdateID = pickedByLine[key]
				}
			}
			secView.Options = append(secView.Options, option)
		}
		view.Sections = append(view.Sections, secView)
	}

	view.Empty = len(view.Sections) == 0
	return view
}

func pickPlayerView(
	slipID int64,
	player board.PlayerGroup,
	game board.GameCard,
	atLimit bool,
	linkQuery board.Query,
	canRefresh bool,
	homeBook string,
) PickPlayerView {
	card := PickPlayerView{
		Name:        player.Name,
		Initials:    Initials(player.Name),
		HeadshotURL: player.HeadshotURL,
		Position:    player.Position,
		Team:        player.Team,
		TeamLogo:    teamLogoForGame(player.Team, game),
		Tags:        player.Tags,
	}
	for _, group := range player.Lines {
		card.Lines = append(
			card.Lines,
			pickLineGroupView(slipID, group, atLimit, linkQuery, true, canRefresh, homeBook),
		)
	}
	return card
}

// teamLogoForGame resolves a player's team logo from the game's away/home team.
func teamLogoForGame(team string, game board.GameCard) string {
	switch {
	case team == "":
		return ""
	case strings.EqualFold(team, game.Away):
		return game.AwayLogo
	case strings.EqualFold(team, game.Home):
		return game.HomeLogo
	default:
		return ""
	}
}

// detailChip builds one tab or sort link for the open game's detail.
func detailChip(
	slipID int64,
	q board.Query,
	key, value, label string,
	count int,
	active bool,
) PickChip {
	return PickChip{
		Value:   value,
		Label:   label,
		Href:    detailHref(slipID, q, key, value),
		PushURL: detailPushURL(slipID, q, key, value),
		Count:   count,
		Active:  active,
	}
}

// pickLineGroupView maps one line's sides to the template. A prop line shows a
// short option such as "Over 245.5" because the player card already says whose
// line it is. When one side is picked, the other side's button updates that leg
// through PUT instead of adding a second one.
func pickLineGroupView(
	slipID int64,
	group board.LineGroup,
	atLimit bool,
	q board.Query,
	prop bool,
	canRefresh bool,
	homeBook string,
) PickLineGroupView {
	view := PickLineGroupView{Label: lineGroupLabel(group)}
	if len(group.Options) > 0 {
		view.ID = pickLineID(
			group.Options[0].GameID,
			group.Market,
			group.Team,
			group.Player,
			group.Line,
		)
	}
	pickedID := int64(0)
	picked := 0
	for _, row := range group.Options {
		if row.Picked != 0 {
			pickedID = row.Picked
			picked++
		}
	}
	for _, row := range group.Options {
		option := pickRowView(slipID, row, atLimit, q, canRefresh, homeBook)
		if option.PickID == 0 && picked == 1 {
			option.UpdateID = pickedID
		}
		if prop {
			option.Text = propOptionText(row)
		}
		view.Options = append(view.Options, option)
	}
	if len(view.Options) == 2 && view.Options[0].Side == "OVER" && view.Options[1].Side == "UNDER" {
		view.IsOU = true
		if view.Options[0].Line != "" && view.Options[0].Line == view.Options[1].Line {
			view.SharedLine = view.Options[0].Line
		}
		for _, opt := range view.Options {
			if opt.PickID != 0 && opt.SplitLine != "" {
				view.SplitLine = opt.SplitLine
				break
			}
		}
		if view.SplitLine == "" {
			for _, opt := range view.Options {
				if opt.SplitLine != "" {
					view.SplitLine = opt.SplitLine
					break
				}
			}
		}
	}
	return view
}

// lineGroupLabel names a line above its sides. A team total reads with its team
// so the Lines tab does not show two anonymous totals.
func lineGroupLabel(group board.LineGroup) string {
	if group.Market == "TEAM_TOTAL" && group.Team != "" {
		return group.Team + " team total"
	}
	return group.Label
}

// propOptionText is a prop side's short label inside a player card.
func propOptionText(row board.Row) string {
	if row.Market == "ANYTIME_TD" || row.Side == "YES" {
		return "Yes"
	}
	text := titleSide(row.Side)
	if row.Line != nil {
		text += " " + formatFloat(*row.Line)
	}
	return text
}

// pickGameCardView maps one slate card to the template's fields.
func pickGameCardView(slipID int64, card board.GameCard, q board.Query) PickGameCardView {
	return PickGameCardView{
		SlipID:          slipID,
		ID:              card.ID,
		Matchup:         card.Matchup,
		Venue:           card.Venue,
		Weather:         gameWeatherSummary(card),
		Away:            card.Away,
		Home:            card.Home,
		AwayLogo:        card.AwayLogo,
		HomeLogo:        card.HomeLogo,
		KickoffISO:      kickoffISO(card.Kickoff),
		KickoffFallback: kickoffFallback(card.Kickoff),
		Spread:          spreadSummary(card),
		Total:           totalSummary(card),
		Moneyline:       moneylineSummary(card),
		Props:           card.Props,
		HasProps:        card.HasProps,
		Unloaded:        card.Unloaded,
		Href:            pickHref(slipID, q, "game", card.ID),
		PushURL:         pickPushURL(slipID, q, "game", card.ID),
		Active:          q.GameID == card.ID,
	}
}

// gameWeatherSummary formats a compact dome/weather badge with an emoji.
func gameWeatherSummary(card board.GameCard) string {
	if card.Indoor {
		return "🏟️ Dome"
	}
	var emoji string
	switch {
	case card.PrecipIN != nil && *card.PrecipIN > 0:
		if card.TempF != nil && *card.TempF <= 32 {
			emoji = "❄️"
		} else {
			emoji = "🌧️"
		}
	case (card.WindMPH != nil && *card.WindMPH >= 15) || (card.WindGustMPH != nil && *card.WindGustMPH >= 25):
		emoji = "💨"
	case card.TempF != nil && *card.TempF <= 32:
		emoji = "🥶"
	case card.TempF != nil:
		emoji = "☀️"
	}
	var parts []string
	if card.TempF != nil {
		parts = append(parts, fmt.Sprintf("%.0f°F", *card.TempF))
	}
	if card.WindMPH != nil && *card.WindMPH >= 5 {
		parts = append(parts, fmt.Sprintf("%.0f mph", *card.WindMPH))
	}
	metrics := strings.Join(parts, " · ")
	if emoji != "" && metrics != "" {
		return emoji + " " + metrics
	}
	if emoji != "" {
		return emoji
	}
	return metrics
}

// spreadSummary prints the favored side of the spread (or the available side
// when only one is stored), omitting standard -110 juice.
func spreadSummary(card board.GameCard) string {
	spread := card.Lines.Spread
	if spread == nil {
		return ""
	}
	if spread.HomeLine != "" && strings.HasPrefix(spread.HomeLine, "-") {
		return withNonStandardPrice(card.Home+" "+spread.HomeLine, spread.HomeOdds)
	}
	if spread.AwayLine != "" {
		return withNonStandardPrice(card.Away+" "+spread.AwayLine, spread.AwayOdds)
	}
	if spread.HomeLine != "" {
		return withNonStandardPrice(card.Home+" "+spread.HomeLine, spread.HomeOdds)
	}
	return ""
}

// totalSummary prints the over/under total line without redundant -110 juice.
func totalSummary(card board.GameCard) string {
	total := card.Lines.Total
	if total == nil {
		return ""
	}
	if total.Line != "" {
		return "O/U " + total.Line
	}
	return "O/U"
}

// moneylineSummary prints away / home moneyline odds concisely.
func moneylineSummary(card board.GameCard) string {
	moneyline := card.Lines.Moneyline
	if moneyline == nil {
		return ""
	}
	if moneyline.AwayOdds != "" && moneyline.HomeOdds != "" {
		return moneyline.AwayOdds + " / " + moneyline.HomeOdds
	}
	if moneyline.AwayOdds != "" {
		return card.Away + " " + moneyline.AwayOdds
	}
	if moneyline.HomeOdds != "" {
		return card.Home + " " + moneyline.HomeOdds
	}
	return ""
}

// withNonStandardPrice appends a price in parentheses when it differs from -110.
func withNonStandardPrice(label, price string) string {
	if price == "" || price == "-110" {
		return label
	}
	return label + " (" + price + ")"
}

// kickoffISO is the UTC value a time element carries.
func kickoffISO(kickoff time.Time) string {
	if kickoff.IsZero() {
		return ""
	}
	return kickoff.UTC().Format(timeRFC3339)
}

// eastern is the zone the server's no-JavaScript fallback reads in.
var eastern = loadEastern()

func loadEastern() *time.Location {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		return time.UTC
	}
	return location
}

// kickoffFallback is the neutral ET time the server renders before app.js
// replaces it with the reader's local zone.
func kickoffFallback(kickoff time.Time) string {
	if kickoff.IsZero() {
		return ""
	}
	return kickoff.In(eastern).Format("Mon 3:04 PM") + " ET"
}

// pickDockData loads the dock and tray for a request.
func (h *slipHandlers) pickDockData(ctx context.Context, slip db.Slip) (PickDockView, error) {
	legs, err := h.deps.Store.LegsForSlip(ctx, slip.ID)
	if err != nil {
		return PickDockView{}, err
	}
	games, err := h.deps.Store.GamesForWeek(ctx, slip.WeekID)
	if err != nil {
		return PickDockView{}, err
	}
	games = slips.FilterGamesForSlip(games, slip)
	user, homeBook, q := h.pickViewer(ctx, board.Query{})
	mine := legsForUser(legs, user.ID)

	var boardRows []board.Row
	if slip.Status == db.SlipOpen && len(mine) > 0 {
		odds, err := h.deps.Store.OddsForWeek(ctx, slip.WeekID)
		if err != nil {
			return PickDockView{}, err
		}
		roster, err := h.deps.Store.RosterForTeams(ctx, weekTeams(games))
		if err != nil {
			return PickDockView{}, err
		}
		boardRows = board.BuildDualBook(games, odds, roster, homeBook, mine, q.Now)
	}
	return h.dockFrom(ctx, slip, legs, games, mine, boardRows, homeBook)
}

// dockFrom shapes the viewer's legs, their analyzer warnings, and the combined
// price the dock shows.
func (h *slipHandlers) dockFrom(
	ctx context.Context,
	slip db.Slip,
	legs []db.Leg,
	games []db.Game,
	mine []db.Leg,
	boardRows []board.Row,
	homeBook string,
) (PickDockView, error) {
	members, err := h.deps.Store.SlipMembers(ctx, slip.ID)
	if err != nil {
		return PickDockView{}, err
	}
	names := make(map[int64]string, len(members))
	for _, member := range members {
		names[member.User.ID] = member.User.Name
	}
	user, _ := h.user(ctx)
	viewerID := user.ID

	past, err := h.deps.Store.WeekPast(ctx, slip.WeekID, h.deps.now())
	if err != nil {
		return PickDockView{}, err
	}
	now := h.deps.now()
	lockedGames := make(map[string]bool, len(games))
	for _, game := range games {
		if !past && !now.Before(game.Kickoff) {
			lockedGames[game.ID] = true
		}
	}

	rowByLegID := make(map[int64]board.Row, len(mine))
	for _, row := range boardRows {
		if row.Picked != 0 {
			rowByLegID[row.Picked] = row
		}
	}

	view := PickDockView{
		SlipID: slip.ID,
		Picked: len(mine),
		Limit:  slip.LegsPerMember,
	}
	combinedLegs := make([]parlay.Leg, 0, len(mine))
	for _, leg := range mine {
		locked := false
		if leg.GameID != nil && lockedGames[*leg.GameID] {
			locked = true
		}
		sideBadge, sideClass := legSideBadge(leg.Side)
		trayLeg := TrayLegView{
			ID:            leg.ID,
			Selection:     leg.Selection,
			SideBadge:     sideBadge,
			SideClass:     sideClass,
			Odds:          formatOdds(leg.Odds),
			ImpliedChance: formatImpliedChance(parlay.ImpliedProb(leg.Odds)),
			ImpliedHuman:  formatImpliedHuman(parlay.ImpliedProb(leg.Odds)),
			Locked:        locked,
		}
		if row, ok := rowByLegID[leg.ID]; ok {
			trayLeg.SplitLine = splitLineBadge(row.Market, row.DK, row.FD, row.LinesDiffer, homeBook)
			if row.Drifted {
				trayLeg.DriftNote = row.TrayDriftNote
				if !locked && slip.Status == db.SlipOpen {
					trayLeg.Refresh = refreshViewFromRow(row)
				}
			}
		}
		view.Legs = append(view.Legs, trayLeg)
		combinedLegs = append(combinedLegs, parlay.Leg{
			Decimal: parlay.AmericanToDecimal(leg.Odds),
			Result:  leg.Result,
		})
	}
	combined := parlay.CombinedOdds(combinedLegs)
	view.Combined = fmt.Sprintf("%.2fx", combined)
	view.CombinedAmerican = formatOdds(parlay.DecimalToAmerican(combined))
	view.CombinedImplied = formatImpliedChance(parlay.DecimalImpliedProb(combined))
	view.CombinedImpliedHuman = formatImpliedHuman(parlay.DecimalImpliedProb(combined))

	gamesByID, stats := analyzer.LoadSlipSignals(ctx, h.deps.Store, now, slip.WeekID, games, legs)
	analysis, legGrades := analyzerView(legs, gamesByID, stats, names, h.deps.GradingEngine)
	view.Grade = analysis.Grade
	for i := range view.Legs {
		view.Legs[i].Grade = legGrades[view.Legs[i].ID]
	}
	for _, warning := range analysis.Warnings {
		for _, leg := range warning.Legs {
			if leg.OwnerID == viewerID {
				view.Warnings = append(view.Warnings, warning)
				break
			}
		}
	}

	next := slips.NextStep(slip, members, legs, games, viewerID, now)
	view.ActionHref = fmt.Sprintf("/slips/%d", slip.ID)
	switch next.Step {
	case slips.StepLockIn:
		view.ActionLabel = "Review and lock in"
	default:
		view.ActionLabel = "Done"
	}
	return view, nil
}

// windowChips builds the kickoff-window filter links.
func windowChips(slipID int64, counts map[board.Window]int, q board.Query) []PickChip {
	chips := []PickChip{{
		Value:   "",
		Label:   "Any time",
		Href:    pickHref(slipID, q, "win", ""),
		PushURL: pickPushURL(slipID, q, "win", ""),
		Active:  q.Window == "",
	}}
	for _, window := range board.Windows {
		if counts[window] == 0 && q.Window != window {
			continue
		}
		chips = append(chips, PickChip{
			Value:   string(window),
			Label:   window.Label(),
			Href:    pickHref(slipID, q, "win", string(window)),
			PushURL: pickPushURL(slipID, q, "win", string(window)),
			Count:   counts[window],
			Active:  q.Window == window,
		})
	}
	return chips
}

// slateHref is the slate's own refresh URL. The element includes the search
// form's current values, so a tab or sort swap reaches the slate too.
func slateHref(slipID int64) string {
	return fmt.Sprintf("/slips/%d/pick/slate", slipID)
}

// pickHref builds a results URL that changes one filter and keeps the rest.
func pickHref(slipID int64, q board.Query, key, value string) string {
	values := url.Values{}
	if q.Q != "" {
		values.Set("q", q.Q)
	}
	set := func(name, current string) {
		if name == key {
			if value != "" {
				values.Set(name, value)
			}
			return
		}
		if current != "" {
			values.Set(name, current)
		}
	}
	set("group", string(q.GroupBy))
	set("cat", string(q.Category))
	set("win", string(q.Window))
	set("sort", string(q.Sort))
	set("game", q.GameID)
	encoded := values.Encode()
	target := fmt.Sprintf("/slips/%d/pick/results", slipID)
	if encoded != "" {
		target += "?" + encoded
	}
	return target
}

// pickPushURL is the full page URL a chip pushes into the address bar.
func pickPushURL(slipID int64, q board.Query, key, value string) string {
	return strings.Replace(
		pickHref(slipID, q, key, value),
		"/pick/results",
		"/pick",
		1,
	)
}

// detailHref builds a detail URL that changes one tab or sort and keeps the
// rest of the query, so the open game and its search stay put.
func detailHref(slipID int64, q board.Query, key, value string) string {
	values := url.Values{}
	if q.Q != "" {
		values.Set("q", q.Q)
	}
	set := func(name, current string) {
		if name == key {
			if value != "" {
				values.Set(name, value)
			}
			return
		}
		if current != "" {
			values.Set(name, current)
		}
	}
	set("group", string(q.GroupBy))
	set("cat", string(q.Category))
	set("sort", string(q.Sort))
	set("win", string(q.Window))
	set("game", q.GameID)
	encoded := values.Encode()
	target := fmt.Sprintf("/slips/%d/pick/detail", slipID)
	if encoded != "" {
		target += "?" + encoded
	}
	return target
}

// detailPushURL is the full page URL a tab or sort chip pushes.
func detailPushURL(slipID int64, q board.Query, key, value string) string {
	return strings.Replace(
		detailHref(slipID, q, key, value),
		"/pick/detail",
		"/pick",
		1,
	)
}

// pickBackHref returns to the slate, dropping the open game and its category
// while keeping the search and kickoff window.
func pickBackHref(slipID int64, q board.Query) string {
	values := url.Values{}
	if q.Q != "" {
		values.Set("q", q.Q)
	}
	if q.Window != "" {
		values.Set("win", string(q.Window))
	}
	target := fmt.Sprintf("/slips/%d/pick/results", slipID)
	if encoded := values.Encode(); encoded != "" {
		target += "?" + encoded
	}
	return target
}

// pickBackPushURL is the full page URL the back link pushes.
func pickBackPushURL(slipID int64, q board.Query) string {
	return strings.Replace(pickBackHref(slipID, q), "/pick/results", "/pick", 1)
}

// pickLineID is the DOM id of one line's pair row. The swap sheet targets it so
// an update swaps that row and nothing else.
func pickLineID(gameID, market, team, player string, line *float64) string {
	key := gameID + "|" + board.LineKey(market, team, player, line)
	slug := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		default:
			return '-'
		}
	}, key)
	return "pick-line-" + slug
}

// pickRowView maps a built row to the template's fields.
func pickRowView(
	slipID int64,
	row board.Row,
	atLimit bool,
	q board.Query,
	canRefresh bool,
	homeBook string,
) PickRowView {
	view := PickRowView{
		SlipID:        slipID,
		GameID:        row.GameID,
		Market:        row.Market,
		Team:          row.Team,
		PlayerName:    row.PlayerName,
		PlayerID:      row.PlayerID,
		Side:          row.Side,
		Odds:          row.Odds,
		Price:         formatOdds(row.Odds),
		ImpliedChance: formatImpliedChance(parlay.ImpliedProb(row.Odds)),
		ImpliedHuman:  formatImpliedHuman(parlay.ImpliedProb(row.Odds)),
		Text:          row.Text,
		PickID:        row.Picked,
		AtLimit:       atLimit,
		ResearchURL: betResearchHref(
			db.SelectionKey{
				GameID:     row.GameID,
				MarketType: row.Market,
				Team:       row.Team,
				PlayerName: row.PlayerName,
				PlayerID:   row.PlayerID,
				Side:       row.Side,
			},
			pickFromURL(slipID, row.GameID),
			slipID,
		),
		Badge:        row.Badge,
		ComboWarning: row.ComboWarning,
		SplitLine:    splitLineBadge(row.Market, row.DK, row.FD, row.LinesDiffer, homeBook),
	}
	if row.Line != nil {
		view.Line = formatFloat(*row.Line)
	}
	if row.Grade.Letter != "" {
		view.Grade = LegGradeView(row.Grade)
	}
	if row.Picked != 0 && row.Drifted {
		view.DriftNote = row.BoardDriftNote
		view.CanRefresh = canRefresh
	}
	if atLimit && row.Picked == 0 {
		view.SwapURL = swapURL(slipID, view, q)
	}
	return view
}

// splitLineBadge formats an explicit DK vs. FD split-line note ordered with
// the viewer's homeBook first whenever both books quote different lines.
func splitLineBadge(market string, dk, fd *board.BookQuote, linesDiffer bool, homeBook string) string {
	if !linesDiffer || dk == nil || fd == nil {
		return ""
	}
	dkPart := formatBookQuote("DK", market, dk)
	fdPart := formatBookQuote("FD", market, fd)
	if db.NormalizeBook(homeBook) == "fanduel" {
		return fdPart + " · " + dkPart
	}
	return dkPart + " · " + fdPart
}

func formatBookQuote(label, market string, quote *board.BookQuote) string {
	price := formatOdds(quote.Odds)
	if quote.Line == nil {
		return label + " " + price
	}
	lineText := formatFloat(*quote.Line)
	if market == "SPREAD" && *quote.Line > 0 {
		lineText = "+" + lineText
	}
	return fmt.Sprintf("%s %s (%s)", label, lineText, price)
}

// timeRFC3339 is the layout every time element uses.
const timeRFC3339 = time.RFC3339

// swapURL names one row for the swap sheet, keeping the current search so the
// results come back the same way after the swap.
func swapURL(slipID int64, row PickRowView, q board.Query) string {
	values := url.Values{}
	values.Set("game_id", row.GameID)
	values.Set("market", row.Market)
	values.Set("team", row.Team)
	values.Set("player_name", row.PlayerName)
	values.Set("player_id", row.PlayerID)
	values.Set("side", row.Side)
	values.Set("line", row.Line)
	values.Set("odds", strconv.Itoa(row.Odds))
	values.Set("text", row.Text)
	if q.Q != "" {
		values.Set("q", q.Q)
	}
	if q.GroupBy != "" {
		values.Set("group", string(q.GroupBy))
	}
	if q.Category != "" {
		values.Set("cat", string(q.Category))
	}
	if q.Window != "" {
		values.Set("win", string(q.Window))
	}
	if q.Sort != "" {
		values.Set("sort", string(q.Sort))
	}
	if q.GameID != "" {
		values.Set("game", q.GameID)
	}
	return fmt.Sprintf("/slips/%d/pick/swap?%s", slipID, values.Encode())
}

// pickRowFromLeg shapes one option from a stored leg. It is the fallback for a
// line the board can no longer rebuild, such as a custom pick with no odds.
func pickRowFromLeg(slip db.Slip, leg db.Leg) PickRowView {
	view := PickRowView{
		SlipID:        slip.ID,
		GameID:        db.StringValue(leg.GameID),
		Market:        leg.MarketType,
		Team:          db.StringValue(leg.Team),
		PlayerName:    db.StringValue(leg.PlayerName),
		PlayerID:      db.StringValue(leg.PlayerID),
		Side:          db.StringValue(leg.Side),
		Odds:          leg.Odds,
		Price:         formatOdds(leg.Odds),
		ImpliedChance: formatImpliedChance(parlay.ImpliedProb(leg.Odds)),
		Text:          leg.Selection,
		PickID:        leg.ID,
		ResearchURL:   betResearchHref(db.LegSelection(leg), pickFromURL(slip.ID, db.StringValue(leg.GameID)), slip.ID),
	}
	if leg.Line != nil {
		view.Line = formatFloat(*leg.Line)
	}
	return view
}

func pickFromURL(slipID int64, gameID string) string {
	if gameID == "" {
		return fmt.Sprintf("/slips/%d/pick", slipID)
	}
	return fmt.Sprintf("/slips/%d/pick?game=%s", slipID, url.QueryEscape(gameID))
}

// pickPairRow rebuilds one line's row after a pick from the board, so both
// sides come back with their current picked state. picked says whether the leg
// is still on the slip; a deleted leg has no option left in the rebuilt board.
func (h *slipHandlers) pickPairRow(
	ctx context.Context,
	slip db.Slip,
	leg db.Leg,
	picked bool,
) PickLineGroupView {
	fallback := func() PickLineGroupView {
		row := pickRowFromLeg(slip, leg)
		if !picked {
			row.PickID = 0
		}
		return PickLineGroupView{
			ID: pickLineID(
				db.StringValue(leg.GameID),
				leg.MarketType,
				db.StringValue(leg.Team),
				db.StringValue(leg.PlayerName),
				leg.Line,
			),
			Options: []PickRowView{row},
		}
	}
	gameID := db.StringValue(leg.GameID)
	if gameID == "" {
		return fallback()
	}
	game, err := h.deps.Store.GameByID(ctx, gameID)
	if err != nil {
		return fallback()
	}
	odds, err := h.deps.Store.OddsForGame(ctx, gameID)
	if err != nil {
		return fallback()
	}
	roster, err := h.deps.Store.RosterForTeams(ctx, []string{game.AwayTeam, game.HomeTeam})
	if err != nil {
		return fallback()
	}
	legs, err := h.deps.Store.LegsForSlip(ctx, slip.ID)
	if err != nil {
		return fallback()
	}
	user, _ := h.user(ctx)
	homeBook := ""
	if user.HomeBook != nil {
		homeBook = *user.HomeBook
	}
	mine := legsForUser(legs, user.ID)
	now := h.deps.now()
	rows := board.BuildDualBook(
		[]db.Game{game}, odds, roster, homeBook, mine, now)
	if slip.Status == db.SlipOpen && len(rows) > 0 {
		boardGames, boardStats := h.loadBoardSignalsForRows(ctx, now, slip.WeekID, []db.Game{game}, rows)
		rows = board.ScoreRows(rows, boardGames, boardStats, h.deps.GradingEngine)
	}
	var playerIDs []string
	if leg.PlayerID != nil && *leg.PlayerID != "" {
		playerIDs = []string{*leg.PlayerID}
	}
	signals := h.boardSignalsForGame(ctx, slip.WeekID, game, legs, playerIDs)
	if group, ok := board.LineGroupForLeg(game, rows, leg, signals); ok {
		return pickLineGroupView(
			slip.ID,
			group,
			len(mine) >= slip.LegsPerMember,
			board.Query{},
			isPlayerMarket(group.Market),
			slip.Status == db.SlipOpen,
			homeBook,
		)
	}
	return fallback()
}

// refreshViewFromRow builds the form payload to sync a drifted leg to row.
func refreshViewFromRow(row board.Row) *LegRefreshView {
	line := ""
	if row.Line != nil {
		line = formatFloat(*row.Line)
	}
	return &LegRefreshView{
		GameID:     row.GameID,
		Market:     row.Market,
		Team:       row.Team,
		PlayerName: row.PlayerName,
		PlayerID:   row.PlayerID,
		Side:       row.Side,
		Line:       line,
		Odds:       row.Odds,
	}
}

// sameBoardLine reports whether two legs sit on the same board line group, so a
// switch needs only that row refreshed. A leg that moves to another market,
// player, or game leaves a stale row behind, and the board needs a full refresh.
func sameBoardLine(left, right db.Leg) bool {
	return db.SameLineGroup(db.LegSelection(left), db.LegSelection(right))
}

// pickQuery reads the board's query from the URL or a submitted form.
func pickQuery(r *http.Request, now time.Time) board.Query {
	value := func(key string) string {
		if raw := r.URL.Query().Get(key); raw != "" {
			return raw
		}
		if r.PostForm != nil {
			return r.PostForm.Get(key)
		}
		return ""
	}
	q := board.Query{
		Q:        strings.TrimSpace(value("q")),
		GroupBy:  board.GroupBy(value("group")),
		Category: board.Category(value("cat")),
		Window:   board.Window(value("win")),
		Sort:     board.Sort(value("sort")),
		GameID:   strings.TrimSpace(value("game")),
		Now:      now,
	}
	if !knownGroupBy(q.GroupBy) {
		q.GroupBy = ""
	}
	if !knownCategory(q.Category) {
		q.Category = ""
	}
	if !knownWindow(q.Window) {
		q.Window = ""
	}
	if !knownSort(q.Sort) {
		q.Sort = ""
	}
	return q
}

func knownGroupBy(groupBy board.GroupBy) bool {
	if groupBy == "" {
		return true
	}
	return slices.Contains(board.DetailGroupBys, groupBy)
}

func knownCategory(category board.Category) bool {
	if category == "" {
		return true
	}
	return slices.Contains(board.Categories, category)
}

func knownWindow(window board.Window) bool {
	if window == "" {
		return true
	}
	return slices.Contains(board.Windows, window)
}

func knownSort(sort board.Sort) bool {
	if sort == "" {
		return true
	}
	return slices.Contains(board.DetailSorts, sort)
}

// legsForUser returns one person's legs.
func legsForUser(legs []db.Leg, userID int64) []db.Leg {
	mine := make([]db.Leg, 0)
	for _, leg := range legs {
		if leg.UserID == userID {
			mine = append(mine, leg)
		}
	}
	return mine
}

// weekTeams lists the teams playing in a week.
func weekTeams(games []db.Game) []string {
	seen := make(map[string]bool, len(games)*2)
	teams := make([]string, 0, len(games)*2)
	for _, game := range games {
		for _, team := range []string{game.AwayTeam, game.HomeTeam} {
			if team == "" || seen[team] {
				continue
			}
			seen[team] = true
			teams = append(teams, team)
		}
	}
	return teams
}

// intFromString reads an integer, defaulting to zero.
func intFromString(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0
	}
	return value
}

// floatPointer parses a decimal, or returns nil when it is blank or invalid.
func floatPointer(raw string) *float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return nil
	}
	return &value
}

func boardRowPlayerIDs(gameID string, rows []board.Row) []string {
	seen := make(map[string]struct{})
	var ids []string
	for _, row := range rows {
		if row.GameID != gameID || row.PlayerID == "" {
			continue
		}
		if _, ok := seen[row.PlayerID]; ok {
			continue
		}
		seen[row.PlayerID] = struct{}{}
		ids = append(ids, row.PlayerID)
	}
	return ids
}

// boardSignalsForGame loads optional nflverse team EPA, player usage, game
// injuries, and open-slip legs for one game's pick board. Missing or stale
// nflverse data fails soft with nil maps.
func (h *slipHandlers) boardSignalsForGame(
	ctx context.Context,
	weekID int64,
	game db.Game,
	legs []db.Leg,
	playerIDs []string,
) board.BoardSignals {
	slipLegs := make([]analyzer.Leg, 0, len(legs))
	for _, leg := range legs {
		slipLegs = append(slipLegs, analyzer.LegFromDB(leg, ""))
	}
	var rosters []db.RosterPlayer
	if loaded, err := h.deps.Store.RosterForTeams(ctx, []string{game.AwayTeam, game.HomeTeam}); err == nil {
		rosters = loaded
	}
	teamEPA, playerUsage := analyzer.LoadGameSignals(ctx, h.deps.Store, h.deps.now(), weekID, playerIDs)
	signals := board.BoardSignals{
		TeamEPA:  teamEPA,
		Injuries: analyzer.MergeGameInjuries(game, rosters),
		SlipLegs: slipLegs,
	}
	if len(playerUsage) > 0 {
		signals.PlayerUsage = playerUsage
	}
	return signals
}

func (h *slipHandlers) loadBoardSignalsForRows(
	ctx context.Context,
	now time.Time,
	weekID int64,
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

type gameCardInsights struct {
	Grades            []GameGradeBadge
	TopPlayersSummary string
}

var gameCardGradeOrder = []string{"A", "B", "C", "D", "F"}

func gameGradeBadgeClass(letter string) string {
	switch letter {
	case "A":
		return "grade-a"
	case "B":
		return "grade-b"
	case "C":
		return "grade-c"
	case "D":
		return "grade-d"
	case "F":
		return "grade-f"
	default:
		return "grade-neutral"
	}
}

func gameCardInsightsByGame(
	games []db.Game,
	rows []board.Row,
	boardGames map[string]analyzer.Game,
	boardStats map[string]analyzer.PlayerStats,
) map[string]gameCardInsights {
	if len(games) == 0 || len(rows) == 0 || len(boardGames) == 0 {
		return nil
	}
	rowsByGame := make(map[string][]board.Row, len(games))
	for _, row := range rows {
		if row.GameID != "" {
			rowsByGame[row.GameID] = append(rowsByGame[row.GameID], row)
		}
	}
	insights := make(map[string]gameCardInsights, len(games))
	for _, game := range games {
		gameRows := rowsByGame[game.ID]
		if len(gameRows) == 0 {
			continue
		}
		gradeCounts := make(map[string]int, len(gameCardGradeOrder))
		playerScores := make(map[string]int)
		for _, row := range gameRows {
			if row.Picked != 0 || row.Market == "OTHER" {
				continue
			}
			grade := row.Grade
			if grade.Letter == "" {
				grade = analyzer.ScoreLeg(board.RowToAnalyzerLeg(row), boardGames, boardStats)
			}
			gradeCounts[grade.Letter]++
			if row.Category != board.CategoryLines && row.PlayerName != "" {
				if prev, ok := playerScores[row.PlayerName]; !ok || grade.Score > prev {
					playerScores[row.PlayerName] = grade.Score
				}
			}
		}

		var badges []GameGradeBadge
		for _, letter := range gameCardGradeOrder {
			count := gradeCounts[letter]
			if count == 0 {
				continue
			}
			badges = append(badges, GameGradeBadge{
				Letter:    letter,
				Count:     count,
				Class:     gameGradeBadgeClass(letter),
				AriaLabel: fmt.Sprintf("%d %s-grade", count, letter),
			})
		}

		type playerRank struct {
			name  string
			score int
		}
		ranked := make([]playerRank, 0, len(playerScores))
		for name, score := range playerScores {
			ranked = append(ranked, playerRank{name: name, score: score})
		}
		slices.SortFunc(ranked, func(a, b playerRank) int {
			if c := cmp.Compare(b.score, a.score); c != 0 {
				return c
			}
			return cmp.Compare(a.name, b.name)
		})
		const maxTopPlayers = 3
		if len(ranked) > maxTopPlayers {
			ranked = ranked[:maxTopPlayers]
		}
		topNames := make([]string, 0, len(ranked))
		for _, p := range ranked {
			topNames = append(topNames, abbreviatePlayerName(p.name))
		}

		if len(badges) > 0 || len(topNames) > 0 {
			insights[game.ID] = gameCardInsights{
				Grades:            badges,
				TopPlayersSummary: strings.Join(topNames, " · "),
			}
		}
	}
	return insights
}

func gameHighlightsView(
	slip db.Slip,
	gameID string,
	rows []board.Row,
	remainingSlots int,
	boardGames map[string]analyzer.Game,
	boardStats map[string]analyzer.PlayerStats,
	homeBook string,
) HighlightsView {
	if slip.Status != db.SlipOpen || remainingSlots <= 0 || len(rows) == 0 || len(boardGames) == 0 {
		return HighlightsView{}
	}
	var gameRows []board.Row
	for _, row := range rows {
		if row.GameID == gameID {
			gameRows = append(gameRows, row)
		}
	}
	if len(gameRows) == 0 {
		return HighlightsView{}
	}
	set := board.SelectHighlights(gameRows, boardGames, boardStats, board.HighlightOptions{
		MaxLegs:       3,
		MaxParlays:    1,
		SameGameOnly:  true,
		ExcludePicked: true,
	})
	view := highlightsView(set, remainingSlots, homeBook, slip.ID, pickFromURL(slip.ID, gameID))
	view.SlipID = slip.ID
	view.FromBoard = true
	return view
}
