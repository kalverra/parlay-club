package web

import (
	"context"
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
	"github.com/kalverra/parlay-club/internal/ingest/espn"
	"github.com/kalverra/parlay-club/internal/parlay"
	"github.com/kalverra/parlay-club/internal/slips"
)

// PickPage is the focused pick board: the game slate, the open game's detail,
// and the dock with its tray sheet.
type PickPage struct {
	Page
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

// PickGameCardView is one game on the slate, with its lines at a glance. A game
// with lines but no prop rows offers to load them.
type PickGameCardView struct {
	SlipID          int64
	ID              string
	Matchup         string
	Venue           string
	Away            string
	Home            string
	AwayLogo        string
	HomeLogo        string
	KickoffISO      string
	KickoffFallback string
	Spread          string
	Total           string
	Moneyline       string
	Props           int
	HasProps        bool
	Unloaded        bool
	Href            string
	PushURL         string
	Active          bool
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
	Category        string
	Sort            string
	RefreshURL      string
	CanRefresh      bool
	OOB             bool
	BackHref        string
	BackPushURL     string
	Tabs            []PickChip
	Sorts           []PickChip
	Unloaded        bool
	Lines           []PickLineGroupView
	Players         []PickPlayerView
	Empty           bool
}

// PickLineGroupView is one line and the sides a person can pick from it.
type PickLineGroupView struct {
	ID      string
	Label   string
	Options []PickRowView
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
	SlipID     int64
	GameID     string
	Market     string
	Team       string
	PlayerName string
	PlayerID   string
	Side       string
	Line       string
	Odds       int
	Price      string
	Text       string
	PickID     int64
	UpdateID   int64
	AtLimit    bool
	SwapURL    string
	Badge      string
}

// PickDockView is the viewer's own picks: the fixed dock's count and combined
// price, and the tray sheet's legs, warnings, and one next step.
type PickDockView struct {
	SlipID           int64
	Picked           int
	Limit            int
	Combined         string
	CombinedAmerican string
	Legs             []TrayLegView
	Grade            GradeView
	Warnings         []AnalyzerWarningView
	ActionLabel      string
	ActionHref       string
}

// TrayLegView is one of the viewer's legs in the tray.
type TrayLegView struct {
	ID        int64
	Selection string
	Odds      string
	Grade     GradeView
	Locked    bool
}

// PickSwapView is the bottom sheet that swaps one leg for a new pick.
type PickSwapView struct {
	SlipID int64
	Text   string
	Target string
	Fields PickRowView
	Legs   []TrayLegView
	Q      string
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
	if err := h.syncPickIfOpen(r.Context(), slip, q, true); err != nil {
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
	h.renderPage(w, "pick", data)
}

// pickResults returns the #pick-body partial for HTMX searches, filters, game
// switches, and sorts.
func (h *slipHandlers) pickResults(w http.ResponseWriter, r *http.Request) {
	slip, _, _, ok := h.loadSlip(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	q := pickQuery(r, h.deps.now())
	if err := h.syncPickIfOpen(r.Context(), slip, q, false); err != nil {
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
		Category: board.Category(data.Body.Category),
		Window:   board.Window(data.Body.Window),
		Sort:     board.Sort(data.Body.Sort),
		GameID:   data.Body.Game,
	}
	w.Header().Set("Hx-Push-Url", pickPushURL(slip.ID, self, "", ""))
	h.renderPartial(w, "pick-body", data.Body)
}

// pickSlate returns the slate alone, so a live board signal refreshes the game
// cards without touching the open game's tab or sort.
func (h *slipHandlers) pickSlate(w http.ResponseWriter, r *http.Request) {
	slip, _, _, ok := h.loadSlip(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	data, err := h.pickPageData(r.Context(), slip, pickQuery(r, h.deps.now()))
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.renderPartial(w, "pick-slate", data.Body.Slate)
}

// pickDetail returns the #pick-detail partial for a tab or sort change. It
// leaves the slate alone, so its scroll and the open game stay put.
func (h *slipHandlers) pickDetail(w http.ResponseWriter, r *http.Request) {
	slip, _, _, ok := h.loadSlip(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	q := pickQuery(r, h.deps.now())
	if err := h.syncPickIfOpen(r.Context(), slip, q, false); err != nil {
		h.serverError(w, err)
		return
	}
	data, err := h.pickPageData(r.Context(), slip, q)
	if err != nil {
		h.serverError(w, err)
		return
	}
	if data.Body.Detail == nil {
		h.renderPartial(w, "pick-detail-empty", nil)
		return
	}
	detail := *data.Body.Detail
	// The tab and sort live in the detail's URL, but the search form still posts
	// hidden copies. Swap them out of band so the next search keeps the tab.
	detail.OOB = true
	h.renderPartial(w, "pick-detail", detail)
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
	h.renderPartial(w, "pick-body", data.Body)
}

// syncPickIfOpen triggers pick-time odds syncs only when slip is OPEN and
// upcoming games remain in its scope. When syncLines is false, it syncs only
// when an upcoming game is selected.
func (h *slipHandlers) syncPickIfOpen(ctx context.Context, slip db.Slip, q board.Query, syncLines bool) error {
	if h.deps.Refresh == nil || slip.Status != db.SlipOpen {
		return nil
	}
	if !syncLines && q.GameID == "" {
		return nil
	}
	hasUpcoming, gameUpcoming, err := h.syncPickScope(ctx, slip, q.GameID, q.Now)
	if err != nil {
		return err
	}
	if gameUpcoming {
		if err := h.deps.Refresh.SyncPickGame(ctx, slip.WeekID, q.GameID); err != nil {
			h.deps.Logger.Warn().Err(err).Int64("slip_id", slip.ID).Str("game_id", q.GameID).Msg("sync pick game")
		}
		return nil
	}
	if syncLines && hasUpcoming {
		if err := h.deps.Refresh.SyncPickLines(ctx, slip.WeekID); err != nil {
			h.deps.Logger.Warn().Err(err).Int64("slip_id", slip.ID).Msg("sync pick lines")
		}
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
	games := slips.FilterGamesByScope(allGames, slip.SlateScope)
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
	view.Target = "#" + pickLineID(
		view.Fields.GameID,
		view.Fields.Market,
		view.Fields.Team,
		view.Fields.PlayerName,
		floatPointer(view.Fields.Line),
	)
	h.renderPartial(w, "pick-swap", view)
}

// pickPageData loads and shapes one board request.
func (h *slipHandlers) pickPageData(
	ctx context.Context,
	slip db.Slip,
	q board.Query,
) (PickPage, error) {
	legs, err := h.deps.Store.LegsForSlip(ctx, slip.ID)
	if err != nil {
		return PickPage{}, err
	}
	allGames, err := h.deps.Store.GamesForWeek(ctx, slip.WeekID)
	if err != nil {
		return PickPage{}, err
	}
	games := slips.FilterGamesByScope(allGames, slip.SlateScope)
	odds, err := h.deps.Store.OddsForWeek(ctx, slip.WeekID)
	if err != nil {
		return PickPage{}, err
	}
	roster, err := h.deps.Store.RosterForTeams(ctx, weekTeams(games))
	if err != nil {
		return PickPage{}, err
	}

	user, _ := h.user(ctx)
	homeBook := ""
	if user.HomeBook != nil {
		homeBook = *user.HomeBook
	}
	mine := legsForUser(legs, user.ID)
	now := q.Now
	if now.IsZero() {
		now = h.deps.now()
	}
	q.Now = now

	rows := board.Build(games, currentOdds(odds), roster, homeBook, mine, now)
	started := board.StartedCount(games, now)
	dock, err := h.dockFrom(ctx, slip, legs, allGames, mine)
	if err != nil {
		return PickPage{}, err
	}

	// The detail exists even when the current search matches only one of the
	// open game's rows, so open it from a slate that ignores the search text.
	openCards := board.Slates(games, rows, roster, board.Query{Window: q.Window, Now: now})
	openByID := make(map[string]board.GameCard, len(openCards))
	for _, card := range openCards {
		openByID[card.ID] = card
	}
	if _, ok := openByID[q.GameID]; !ok {
		// An unknown game, a kicked-off game, or a window change falls back to
		// the slate instead of 404ing.
		q.GameID = ""
	}

	atLimit := len(mine) >= slip.LegsPerMember
	// A game can only offer to load props when it already has lines. A game
	// with no rows at all is a week-level "no lines" state, not a prompt.
	rowGames := make(map[string]bool, len(games))
	for _, row := range rows {
		rowGames[row.GameID] = true
	}
	slateCards := board.Slates(games, rows, roster, q)
	slate := PickSlateView{
		SlipID:     slip.ID,
		Search:     q.Q,
		Window:     string(q.Window),
		Game:       q.GameID,
		RefreshURL: slateHref(slip.ID),
		Windows:    windowChips(slip.ID, rows, q),
		Started:    started,
	}
	for _, card := range slateCards {
		unloaded := !card.HasProps && rowGames[card.ID]
		slate.Cards = append(slate.Cards, pickGameCardView(slip.ID, card, q, unloaded))
	}
	slate.NoGames = len(games) == 0
	slate.AllStarted = len(games) > 0 && started == len(games)
	slate.NoLines = !slate.NoGames && !slate.AllStarted && len(rows) == 0
	slate.Empty = len(slate.Cards) == 0 &&
		!slate.NoGames && !slate.NoLines && !slate.AllStarted
	slate.CanRefresh = slip.Status == db.SlipOpen && !slate.NoGames && !slate.AllStarted

	gamesByID := make(map[string]db.Game, len(games))
	for _, game := range games {
		gamesByID[game.ID] = game
	}
	body := PickBodyView{
		SlipID:   slip.ID,
		Search:   q.Q,
		Category: string(q.Category),
		Window:   string(q.Window),
		Sort:     string(q.Sort),
		Game:     q.GameID,
		Slate:    slate,
	}
	if card, ok := openByID[q.GameID]; ok {
		game := gamesByID[card.ID]
		signals := h.boardSignalsForGame(ctx, slip.WeekID, game, rows, mine)
		model := board.Detail(game, rows, board.Query{
			Q:        q.Q,
			Category: q.Category,
			Sort:     q.Sort,
			Now:      now,
			Signals:  signals,
		})
		detail := pickDetailView(slip.ID, model, q, atLimit, !card.HasProps && rowGames[card.ID])
		detail.CanRefresh = slip.Status == db.SlipOpen
		body.Detail = &detail
		body.Category = string(model.Category)
		body.Sort = string(model.Sort)
	}
	body.Slate.Category = body.Category
	body.Slate.Sort = body.Sort
	body.Slate.Detail = body.Detail != nil
	return PickPage{
		SlipID:   slip.ID,
		SlipName: slip.Name,
		Body:     body,
		Dock:     dock,
	}, nil
}

// pickDetailView maps one game's detail model to the template's fields. The
// links keep the resolved tab and sort, so changing one keeps the other.
func pickDetailView(
	slipID int64,
	model board.GameDetail,
	q board.Query,
	atLimit bool,
	unloaded bool,
) PickDetailView {
	linkQuery := q
	linkQuery.Category = model.Category
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
		Category:        string(model.Category),
		Sort:            string(model.Sort),
		RefreshURL:      detailHref(slipID, linkQuery, "", ""),
		BackHref:        pickBackHref(slipID, q),
		BackPushURL:     pickBackPushURL(slipID, q),
		Unloaded:        unloaded,
	}
	for _, tab := range model.Tabs {
		view.Tabs = append(view.Tabs, detailChip(
			slipID, linkQuery, "cat", string(tab.Category), tab.Label, tab.Count, tab.Active))
	}
	for _, option := range board.DetailSorts {
		view.Sorts = append(view.Sorts, detailChip(
			slipID, linkQuery, "sort", string(option), option.Label(), 0, option == model.Sort))
	}
	for _, group := range model.Lines {
		view.Lines = append(view.Lines, pickLineGroupView(slipID, group, atLimit, linkQuery, false))
	}
	for _, player := range model.Players {
		card := PickPlayerView{
			Name:        player.Name,
			Initials:    Initials(player.Name),
			HeadshotURL: player.HeadshotURL,
			Position:    player.Position,
			Team:        player.Team,
			TeamLogo:    teamLogoForGame(player.Team, model.Game),
			Tags:        player.Tags,
		}
		for _, group := range player.Lines {
			card.Lines = append(card.Lines, pickLineGroupView(slipID, group, atLimit, linkQuery, true))
		}
		view.Players = append(view.Players, card)
	}
	view.Empty = len(view.Lines) == 0 && len(view.Players) == 0
	return view
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
		option := pickRowView(slipID, row, atLimit, q)
		if option.PickID == 0 && picked == 1 {
			option.UpdateID = pickedID
		}
		if prop {
			option.Text = propOptionText(row)
		}
		view.Options = append(view.Options, option)
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
func pickGameCardView(slipID int64, card board.GameCard, q board.Query, unloaded bool) PickGameCardView {
	return PickGameCardView{
		SlipID:          slipID,
		ID:              card.ID,
		Matchup:         card.Matchup,
		Venue:           card.Venue,
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
		Unloaded:        unloaded,
		Href:            pickHref(slipID, q, "game", card.ID),
		PushURL:         pickPushURL(slipID, q, "game", card.ID),
		Active:          q.GameID == card.ID,
	}
}

// spreadSummary prints both sides of a spread, or nothing when it is missing.
func spreadSummary(card board.GameCard) string {
	spread := card.Lines.Spread
	if spread == nil {
		return ""
	}
	parts := make([]string, 0, 2)
	if spread.AwayLine != "" {
		parts = append(parts, withPrice(card.Away+" "+spread.AwayLine, spread.AwayOdds))
	}
	if spread.HomeLine != "" {
		parts = append(parts, withPrice(card.Home+" "+spread.HomeLine, spread.HomeOdds))
	}
	return strings.Join(parts, " · ")
}

// totalSummary prints the over/under line, or nothing when it is missing.
func totalSummary(card board.GameCard) string {
	total := card.Lines.Total
	if total == nil {
		return ""
	}
	parts := make([]string, 0, 2)
	if total.OverOdds != "" {
		parts = append(parts, "O "+total.OverOdds)
	}
	if total.UnderOdds != "" {
		parts = append(parts, "U "+total.UnderOdds)
	}
	label := "O/U"
	if total.Line != "" {
		label += " " + total.Line
	}
	if len(parts) > 0 {
		label += " · " + strings.Join(parts, " · ")
	}
	return label
}

// moneylineSummary prints both moneylines, or nothing when they are missing.
func moneylineSummary(card board.GameCard) string {
	moneyline := card.Lines.Moneyline
	if moneyline == nil {
		return ""
	}
	parts := make([]string, 0, 2)
	if moneyline.AwayOdds != "" {
		parts = append(parts, card.Away+" "+moneyline.AwayOdds)
	}
	if moneyline.HomeOdds != "" {
		parts = append(parts, card.Home+" "+moneyline.HomeOdds)
	}
	return strings.Join(parts, " · ")
}

// withPrice appends a price in parentheses when there is one.
func withPrice(label, price string) string {
	if price == "" {
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
	games = slips.FilterGamesByScope(games, slip.SlateScope)
	user, _ := h.user(ctx)
	return h.dockFrom(ctx, slip, legs, games, legsForUser(legs, user.ID))
}

// dockFrom shapes the viewer's legs, their analyzer warnings, and the combined
// price the dock shows.
func (h *slipHandlers) dockFrom(
	ctx context.Context,
	slip db.Slip,
	legs []db.Leg,
	games []db.Game,
	mine []db.Leg,
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
		view.Legs = append(view.Legs, TrayLegView{
			ID:        leg.ID,
			Selection: leg.Selection,
			Odds:      formatOdds(leg.Odds),
			Locked:    locked,
		})
		combinedLegs = append(combinedLegs, parlay.Leg{
			Decimal: parlay.AmericanToDecimal(leg.Odds),
			Result:  leg.Result,
		})
	}
	combined := parlay.CombinedOdds(combinedLegs)
	view.Combined = fmt.Sprintf("%.2fx", combined)
	view.CombinedAmerican = formatOdds(parlay.DecimalToAmerican(combined))

	nfelo := h.nfeloForGames(ctx, games)
	teamEPA, usageByPlayer := h.nflverseForSlip(ctx, slip.WeekID, legs)
	gamesByID := analyzerGameMap(games, nfelo, teamEPA)
	stats := h.playerStatsForLegs(ctx, legs, usageByPlayer)
	analysis, legGrades := analyzerView(legs, gamesByID, stats, names)
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
func windowChips(slipID int64, rows []board.Row, q board.Query) []PickChip {
	counts := make(map[board.Window]int)
	for _, row := range rows {
		counts[row.Window]++
	}
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
func pickRowView(slipID int64, row board.Row, atLimit bool, q board.Query) PickRowView {
	view := PickRowView{
		SlipID:     slipID,
		GameID:     row.GameID,
		Market:     row.Market,
		Team:       row.Team,
		PlayerName: row.PlayerName,
		PlayerID:   row.PlayerID,
		Side:       row.Side,
		Odds:       row.Odds,
		Price:      formatOdds(row.Odds),
		Text:       row.Text,
		PickID:     row.Picked,
		AtLimit:    atLimit,
		Badge:      row.Badge,
	}
	if row.Line != nil {
		view.Line = formatFloat(*row.Line)
	}
	if atLimit && row.Picked == 0 {
		view.SwapURL = swapURL(slipID, view, q)
	}
	return view
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
		SlipID:     slip.ID,
		GameID:     db.StringValue(leg.GameID),
		Market:     leg.MarketType,
		Team:       db.StringValue(leg.Team),
		PlayerName: db.StringValue(leg.PlayerName),
		PlayerID:   db.StringValue(leg.PlayerID),
		Side:       db.StringValue(leg.Side),
		Odds:       leg.Odds,
		Price:      formatOdds(leg.Odds),
		Text:       leg.Selection,
		PickID:     leg.ID,
	}
	if leg.Line != nil {
		view.Line = formatFloat(*leg.Line)
	}
	return view
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
	rows := board.Build(
		[]db.Game{game}, currentOdds(odds), roster, homeBook, mine, h.deps.now())
	signals := h.boardSignalsForGame(ctx, slip.WeekID, game, rows, mine)
	rows = board.ApplySignals(game, rows, signals)
	for _, group := range board.Pair(game, rows) {
		if !groupHoldsLeg(group, leg) {
			continue
		}
		return pickLineGroupView(
			slip.ID,
			group,
			len(mine) >= slip.LegsPerMember,
			board.Query{},
			isPlayerMarket(group.Market),
		)
	}
	return fallback()
}

// sameBoardLine reports whether two legs sit on the same line, so a switch
// needs only that row refreshed. A leg that moves to another line or game
// leaves a stale row behind, and the board needs a full refresh.
func sameBoardLine(left, right db.Leg) bool {
	if db.StringValue(left.GameID) != db.StringValue(right.GameID) {
		return false
	}
	return board.LineKey(
		left.MarketType, db.StringValue(left.Team), db.StringValue(left.PlayerName), left.Line,
	) == board.LineKey(
		right.MarketType, db.StringValue(right.Team), db.StringValue(right.PlayerName), right.Line,
	)
}

// groupHoldsLeg reports whether a line group carries the leg's selection. A
// picked option matches by id; a just-deleted leg matches by its fields.
func groupHoldsLeg(group board.LineGroup, leg db.Leg) bool {
	for _, option := range group.Options {
		if option.Picked == leg.ID || option.MatchesLeg(leg) {
			return true
		}
	}
	return false
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
		Category: board.Category(value("cat")),
		Window:   board.Window(value("win")),
		Sort:     board.Sort(value("sort")),
		GameID:   strings.TrimSpace(value("game")),
		Now:      now,
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

// boardSignalsForGame loads optional nflverse team EPA, player usage, game
// injuries, and open-slip legs for one game's pick board. Missing or stale
// nflverse data fails soft with nil maps.
func (h *slipHandlers) boardSignalsForGame(
	ctx context.Context,
	weekID int64,
	game db.Game,
	rows []board.Row,
	mine []db.Leg,
) board.BoardSignals {
	slipLegs := make([]analyzer.Leg, 0, len(mine))
	for _, leg := range mine {
		slipLegs = append(slipLegs, analyzerLeg(leg, nil))
	}
	signals := board.BoardSignals{
		Injuries: parseGameInjuries(game),
		SlipLegs: slipLegs,
	}

	week, err := h.deps.Store.WeekByID(ctx, weekID)
	if err != nil {
		return signals
	}
	teamEPA, fresh := loadFreshNFLverseTeamProfiles(ctx, h.deps.Store, h.deps.now(), week)
	if !fresh {
		return signals
	}
	signals.TeamEPA = teamEPA

	seen := make(map[string]struct{})
	playerUsage := make(map[string]*analyzer.PlayerUsage)
	for _, row := range rows {
		if row.GameID != game.ID || row.PlayerID == "" {
			continue
		}
		if _, ok := seen[row.PlayerID]; ok {
			continue
		}
		seen[row.PlayerID] = struct{}{}
		pWeeks := loadNFLversePlayerWeeks(ctx, h.deps.Store, row.PlayerID, week.Season, week.SeasonType)
		if usage := buildPlayerUsage(pWeeks); usage != nil {
			playerUsage[row.PlayerID] = usage
		}
	}
	if len(playerUsage) > 0 {
		signals.PlayerUsage = playerUsage
	}
	return signals
}

func parseGameInjuries(game db.Game) map[string][]espn.Injury {
	if game.InjuriesJSON == nil || *game.InjuriesJSON == "" {
		return nil
	}
	teamInjuries, err := espn.ParseInjuries(*game.InjuriesJSON)
	if err != nil || len(teamInjuries) == 0 {
		return nil
	}
	byTeam := make(map[string][]espn.Injury, len(teamInjuries))
	for _, ti := range teamInjuries {
		if ti.Abbreviation != "" && len(ti.Injuries) > 0 {
			byTeam[ti.Abbreviation] = ti.Injuries
		}
	}
	return byTeam
}
