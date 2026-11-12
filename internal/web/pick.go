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

	"github.com/kalverra/parlay-club/internal/board"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/slips"
)

// PickPage is the focused pick board: the game slate, the open game's detail,
// and the tray.
type PickPage struct {
	Page
	SlipID   int64
	SlipName string
	Body     PickBodyView
	Tray     PickTrayView
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
	SelfURL  string
	Slate    PickSlateView
	Detail   *PickDetailView
}

// PickSlateView is the game list: the search box, kickoff-window chips, and one
// card per upcoming game.
type PickSlateView struct {
	SlipID     int64
	Search     string
	Window     string
	Game       string
	SelfURL    string
	Cards      []PickGameCardView
	Windows    []PickChip
	Unloaded   []board.UnloadedGame
	Started    int
	Empty      bool
	NoGames    bool
	NoLines    bool
	AllStarted bool
}

// PickGameCardView is one game on the slate, with its lines at a glance.
type PickGameCardView struct {
	ID              string
	Matchup         string
	Venue           string
	Away            string
	Home            string
	KickoffISO      string
	KickoffFallback string
	Spread          string
	Total           string
	Moneyline       string
	Props           int
	HasProps        bool
	Href            string
	PushURL         string
	Active          bool
}

// PickDetailView is the open game's header, chips, and rows.
type PickDetailView struct {
	SlipID          int64
	GameID          string
	Matchup         string
	Venue           string
	KickoffISO      string
	KickoffFallback string
	Search          string
	Category        string
	Sort            string
	BackHref        string
	BackPushURL     string
	Categories      []PickChip
	Sorts           []PickChip
	GameGroups      []PickGameGroup
	Empty           bool
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
	KickoffISO string
	Matchup    string
	AtLimit    bool
	SwapURL    string
}

// PickGameGroup is one game's rows shown in the one-game view, with both sides
// of each line beside each other.
type PickGameGroup struct {
	GameID     string
	Matchup    string
	KickoffISO string
	Options    []PickRowView
}

// PickTrayView is the viewer's own picks and the one thing to do next.
type PickTrayView struct {
	SlipID      int64
	Picked      int
	Limit       int
	Legs        []TrayLegView
	Warnings    []AnalyzerWarningView
	ActionLabel string
	ActionHref  string
}

// TrayLegView is one of the viewer's legs in the tray.
type TrayLegView struct {
	ID        int64
	Selection string
	Odds      string
	Locked    bool
}

// PickSwapView is the bottom sheet that swaps one leg for a new pick.
type PickSwapView struct {
	SlipID int64
	Text   string
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
	data, err := h.pickPageData(r.Context(), slip, pickQuery(r, h.deps.now()))
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
	data, err := h.pickPageData(r.Context(), slip, pickQuery(r, h.deps.now()))
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.renderPartial(w, "pick-body", data.Body)
}

// pickTray returns the tray partial.
func (h *slipHandlers) pickTray(w http.ResponseWriter, r *http.Request) {
	slip, _, _, ok := h.loadSlip(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	tray, err := h.pickTrayData(r.Context(), slip)
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.renderPartial(w, "pick-tray", tray)
}

// pickSwap shows the swap sheet for one unpicked row.
func (h *slipHandlers) pickSwap(w http.ResponseWriter, r *http.Request) {
	slip, _, _, ok := h.loadSlip(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	tray, err := h.pickTrayData(r.Context(), slip)
	if err != nil {
		h.serverError(w, err)
		return
	}
	// Only unlocked legs can be swapped out. A kicked-off leg stays visible in
	// the tray, but it is not a choice here.
	legs := make([]TrayLegView, 0, len(tray.Legs))
	for _, leg := range tray.Legs {
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
	games, err := h.deps.Store.GamesForWeek(ctx, slip.WeekID)
	if err != nil {
		return PickPage{}, err
	}
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
	result := board.Search(rows, games, roster, q)
	tray, err := h.trayFrom(ctx, slip, legs, games, mine)
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
	slateCards := board.Slates(games, rows, roster, q)
	slate := PickSlateView{
		SlipID:  slip.ID,
		Search:  q.Q,
		Window:  string(q.Window),
		Game:    q.GameID,
		SelfURL: pickHref(slip.ID, q, "", ""),
		Windows: windowChips(slip.ID, rows, q),
		Started: result.Started,
	}
	for _, card := range slateCards {
		slate.Cards = append(slate.Cards, pickGameCardView(slip.ID, card, q))
	}
	slate.Unloaded = result.Unloaded
	slate.NoGames = len(games) == 0
	slate.AllStarted = len(games) > 0 && result.Started == len(games)
	slate.NoLines = !slate.NoGames && !slate.AllStarted && len(rows) == 0
	if slate.NoLines {
		// With no lines at all, one "Load them" prompt per game is noise. The
		// week's empty state says what is missing once.
		slate.Unloaded = nil
	}
	slate.Empty = len(slate.Cards) == 0 && len(slate.Unloaded) == 0 &&
		!slate.NoGames && !slate.NoLines && !slate.AllStarted

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
		SelfURL:  pickHref(slip.ID, q, "", ""),
		Slate:    slate,
	}
	if card, ok := openByID[q.GameID]; ok {
		detail := &PickDetailView{
			SlipID:          slip.ID,
			GameID:          card.ID,
			Matchup:         card.Matchup,
			Venue:           card.Venue,
			KickoffISO:      kickoffISO(card.Kickoff),
			KickoffFallback: kickoffFallback(card.Kickoff),
			Search:          q.Q,
			Category:        string(q.Category),
			Sort:            string(q.Sort),
			BackHref:        pickBackHref(slip.ID, q),
			BackPushURL:     pickBackPushURL(slip.ID, q),
			Categories:      categoryChips(slip.ID, rowsForGame(rows, q.GameID), q),
			Sorts:           sortChips(slip.ID, q),
		}
		rowViews := make([]PickRowView, 0, len(result.Rows))
		for _, row := range result.Rows {
			rowViews = append(rowViews, pickRowView(slip.ID, row, gamesByID[row.GameID], atLimit, q))
		}
		detail.GameGroups = groupByGame(rowViews)
		detail.Empty = len(rowViews) == 0
		body.Detail = detail
	}
	return PickPage{
		SlipID:   slip.ID,
		SlipName: slip.Name,
		Body:     body,
		Tray:     tray,
	}, nil
}

// rowsForGame returns one game's built rows for its own category counts.
func rowsForGame(rows []board.Row, gameID string) []board.Row {
	gameRows := make([]board.Row, 0)
	for _, row := range rows {
		if row.GameID == gameID {
			gameRows = append(gameRows, row)
		}
	}
	return gameRows
}

// pickGameCardView maps one slate card to the template's fields.
func pickGameCardView(slipID int64, card board.GameCard, q board.Query) PickGameCardView {
	return PickGameCardView{
		ID:              card.ID,
		Matchup:         card.Matchup,
		Venue:           card.Venue,
		Away:            card.Away,
		Home:            card.Home,
		KickoffISO:      kickoffISO(card.Kickoff),
		KickoffFallback: kickoffFallback(card.Kickoff),
		Spread:          spreadSummary(card),
		Total:           totalSummary(card),
		Moneyline:       moneylineSummary(card),
		Props:           card.Props,
		HasProps:        card.HasProps,
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

// pickTrayData loads the tray for a request.
func (h *slipHandlers) pickTrayData(ctx context.Context, slip db.Slip) (PickTrayView, error) {
	legs, err := h.deps.Store.LegsForSlip(ctx, slip.ID)
	if err != nil {
		return PickTrayView{}, err
	}
	games, err := h.deps.Store.GamesForWeek(ctx, slip.WeekID)
	if err != nil {
		return PickTrayView{}, err
	}
	user, _ := h.user(ctx)
	return h.trayFrom(ctx, slip, legs, games, legsForUser(legs, user.ID))
}

// trayFrom shapes the viewer's legs and their analyzer warnings.
func (h *slipHandlers) trayFrom(
	ctx context.Context,
	slip db.Slip,
	legs []db.Leg,
	games []db.Game,
	mine []db.Leg,
) (PickTrayView, error) {
	members, err := h.deps.Store.SlipMembers(ctx, slip.ID)
	if err != nil {
		return PickTrayView{}, err
	}
	names := make(map[int64]string, len(members))
	for _, member := range members {
		names[member.User.ID] = member.User.Name
	}
	user, _ := h.user(ctx)
	viewerID := user.ID

	past, err := h.deps.Store.WeekPast(ctx, slip.WeekID, h.deps.now())
	if err != nil {
		return PickTrayView{}, err
	}
	now := h.deps.now()
	lockedGames := make(map[string]bool, len(games))
	for _, game := range games {
		if !past && !now.Before(game.Kickoff) {
			lockedGames[game.ID] = true
		}
	}

	view := PickTrayView{
		SlipID: slip.ID,
		Picked: len(mine),
		Limit:  slip.LegsPerMember,
	}
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
	}

	nfelo := h.nfeloForGames(ctx, games)
	gamesByID := analyzerGameMap(games, nfelo)
	stats := h.playerStatsForLegs(ctx, legs)
	analysis := analyzerView(legs, gamesByID, stats, names)
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

// categoryChips builds the category filter links.
func categoryChips(slipID int64, rows []board.Row, q board.Query) []PickChip {
	counts := make(map[board.Category]int)
	for _, row := range rows {
		counts[row.Category]++
	}
	chips := []PickChip{{
		Value:   "",
		Label:   "All",
		Href:    pickHref(slipID, q, "cat", ""),
		PushURL: pickPushURL(slipID, q, "cat", ""),
		Active:  q.Category == "",
	}}
	for _, category := range board.Categories {
		if counts[category] == 0 && q.Category != category {
			continue
		}
		chips = append(chips, PickChip{
			Value:   string(category),
			Label:   category.Label(),
			Href:    pickHref(slipID, q, "cat", string(category)),
			PushURL: pickPushURL(slipID, q, "cat", string(category)),
			Count:   counts[category],
			Active:  q.Category == category,
		})
	}
	return chips
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

// sortChips builds the sort links.
func sortChips(slipID int64, q board.Query) []PickChip {
	sort := q.Sort
	if sort == "" {
		sort = board.SortKickoff
	}
	chips := make([]PickChip, 0, len(board.Sorts))
	for _, option := range board.Sorts {
		chips = append(chips, PickChip{
			Value:   string(option),
			Label:   option.Label(),
			Href:    pickHref(slipID, q, "sort", string(option)),
			PushURL: pickPushURL(slipID, q, "sort", string(option)),
			Active:  sort == option,
		})
	}
	return chips
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

// pickRowView maps a built row to the template's fields.
func pickRowView(slipID int64, row board.Row, game db.Game, atLimit bool, q board.Query) PickRowView {
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
		Matchup:    game.AwayTeam + " @ " + game.HomeTeam,
		AtLimit:    atLimit,
	}
	if row.Line != nil {
		view.Line = formatFloat(*row.Line)
	}
	view.KickoffISO = kickoffISO(row.Kickoff)
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

// groupByGame pairs both sides of each line for the one-game view.
func groupByGame(rows []PickRowView) []PickGameGroup {
	groups := make([]PickGameGroup, 0)
	index := make(map[string]int)
	for _, row := range rows {
		key := pairKey(row)
		position, ok := index[key]
		if !ok {
			position = len(groups)
			index[key] = position
			groups = append(groups, PickGameGroup{
				GameID:     row.GameID,
				Matchup:    row.Matchup,
				KickoffISO: row.KickoffISO,
			})
		}
		groups[position].Options = append(groups[position].Options, row)
	}
	return groups
}

// pairKey groups the two sides of one line together.
func pairKey(row PickRowView) string {
	switch row.Market {
	case "SPREAD", "ML":
		return row.GameID + "|" + row.Market
	case "TOTAL":
		return row.GameID + "|TOTAL|" + row.Line
	case "TEAM_TOTAL":
		return row.GameID + "|TEAM_TOTAL|" + row.Team + "|" + row.Line
	default:
		return row.GameID + "|" + row.Market + "|" + row.PlayerName + "|" + row.Line
	}
}

// pickRowFromLeg shapes a row for the add or remove response. The leg carries
// every field the add form posted, so no board rebuild is needed.
func (h *slipHandlers) pickRowFromLeg(ctx context.Context, slip db.Slip, leg db.Leg) PickRowView {
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
	if leg.GameID != nil {
		if game, err := h.deps.Store.GameByID(ctx, *leg.GameID); err == nil {
			view.Matchup = game.AwayTeam + " @ " + game.HomeTeam
			view.KickoffISO = kickoffISO(game.Kickoff)
		}
	}
	return view
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
		q.Sort = board.SortKickoff
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
	return slices.Contains(board.Sorts, sort)
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
