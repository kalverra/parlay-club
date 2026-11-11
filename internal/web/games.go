package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
)

// oddsSource is the sync_state row that records the last game-lines fetch.
const oddsSource = "odds:lines"

// gameHandlers serves the Games tab.
type gameHandlers struct {
	deps Dependencies
}

// Routes mounts the Games tab. It sits inside RequireUser and the group scope,
// like the slip screen.
func (h *gameHandlers) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /games", h.games)
	mux.HandleFunc("GET /games/{id}/props", h.props)
}

// GamesPage is the full Games tab: one card per game in the latest week and the
// slip quick-add targets.
type GamesPage struct {
	Page
	Games        []GameCardView
	SlipID       int64
	HasSlip      bool
	CanAdd       bool
	LinesAtISO   string
	LinesAtLabel string
}

// GamePropsView is the rendered partial for GET /games/{id}/props.
type GamePropsView struct {
	GameID     string
	SlipID     int64
	HasSlip    bool
	CanAdd     bool
	TeamTotals []GameLineView
	Players    []GamePlayerPropsView
}

// GamePlayerPropsView groups prop lines for one athlete.
type GamePlayerPropsView struct {
	Name     string
	Team     string
	PlayerID string
	Pills    []GameLineView
}

// GameCardView is one game card.
type GameCardView struct {
	ID           string
	AwayShort    string
	HomeShort    string
	AwayName     string
	HomeName     string
	KickoffISO   string
	KickoffLabel string
	Status       string
	StatusClass  string
	Score        string
	Spreads      []GameLineView
	Totals       []GameLineView
	Pills        []GameLineView
	Badges       []GameBadgeView
	HasLines     bool
}

// GameLineView is one market selection with the price quick-add posts.
type GameLineView struct {
	Text       string
	Price      string
	Market     string
	Team       string
	PlayerName string
	PlayerID   string
	Side       string
	Line       string
	Odds       int
}

// GameBadgeView is one weather or venue badge.
type GameBadgeView struct {
	Text  string
	Class string
}

// games renders the week's board.
func (h *gameHandlers) games(w http.ResponseWriter, r *http.Request) {
	view := GamesPage{Page: h.page(r)}

	if h.deps.Store == nil {
		h.render(w, view)
		return
	}

	week, err := h.deps.Store.LatestWeek(r.Context())
	if errors.Is(err, db.ErrWeekNotFound) {
		h.render(w, view)
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}
	view.WeekLabel = weekLabel(week)

	games, err := h.deps.Store.GamesForWeek(r.Context(), week.ID)
	if err != nil {
		h.serverError(w, err)
		return
	}

	user, hasUser := h.user(r.Context())
	homeBook := ""
	if hasUser && user.HomeBook != nil {
		homeBook = *user.HomeBook
	}

	var newestOdds time.Time
	for _, game := range games {
		rows, err := h.deps.Store.OddsForGame(r.Context(), game.ID)
		if err != nil {
			h.serverError(w, err)
			return
		}
		rows = currentOdds(rows)
		for _, row := range rows {
			if !isPropRow(row) && row.FetchedAt.After(newestOdds) {
				newestOdds = row.FetchedAt
			}
		}
		view.Games = append(view.Games, cardView(game, rows, homeBook))
	}

	h.addSlipTarget(r.Context(), &view, week.ID)
	view.LinesAtISO, view.LinesAtLabel = h.linesNote(r.Context(), newestOdds)

	h.render(w, view)
}

// addSlipTarget finds the newest slip of the week so quick-add pills have
// somewhere to add legs.
func (h *gameHandlers) addSlipTarget(ctx context.Context, view *GamesPage, weekID int64) {
	group, ok := h.group(ctx)
	if !ok {
		return
	}
	slips, err := h.deps.Store.SlipsForWeek(ctx, group.ID, weekID)
	if err != nil || len(slips) == 0 {
		return
	}
	view.SlipID = slips[0].ID
	view.HasSlip = true
	view.CanAdd = slips[0].Status == db.SlipOpen
}

// linesNote returns the newest time the board's lines were fetched, first from
// the games' own rows and otherwise from the odds sync state.
func (h *gameHandlers) linesNote(ctx context.Context, newest time.Time) (string, string) {
	if state, err := h.deps.Store.SyncState(ctx, oddsSource); err == nil {
		if state.FetchedAt.After(newest) {
			newest = state.FetchedAt
		}
	}
	if newest.IsZero() {
		return "", ""
	}
	newest = newest.UTC()
	return newest.Format(time.RFC3339), newest.Format("3:04 PM") + " UTC"
}

// cardView shapes one stored game and its odds rows for the template.
func cardView(game db.Game, rows []db.Odds, homeBook string) GameCardView {
	kickoff := game.Kickoff.UTC()
	status, statusClass := gameStatus(game.Status)
	card := GameCardView{
		ID:           game.ID,
		AwayShort:    game.AwayTeam,
		HomeShort:    game.HomeTeam,
		AwayName:     game.AwayName,
		HomeName:     game.HomeName,
		KickoffISO:   kickoff.Format(time.RFC3339),
		KickoffLabel: kickoff.Format("Mon 3:04 PM") + " UTC",
		Status:       status,
		StatusClass:  statusClass,
		Score:        gameScore(game),
		Badges:       gameBadges(game),
	}

	for _, team := range []string{game.AwayTeam, game.HomeTeam} {
		line, ok := pickGameLine(rowsFor(rows, "SPREAD", team, "TEAM"), homeBook)
		if !ok {
			continue
		}
		card.Spreads = append(card.Spreads, line)
		card.Pills = append(card.Pills, line)
		card.HasLines = true
	}
	for _, team := range []string{game.AwayTeam, game.HomeTeam} {
		line, ok := pickGameLine(rowsFor(rows, "ML", team, "TEAM"), homeBook)
		if !ok {
			continue
		}
		card.Pills = append(card.Pills, line)
		card.HasLines = true
	}
	for _, side := range []string{"OVER", "UNDER"} {
		line, ok := pickGameLine(rowsFor(rows, "TOTAL", "", side), homeBook)
		if !ok {
			continue
		}
		card.Totals = append(card.Totals, line)
		card.Pills = append(card.Pills, line)
		card.HasLines = true
	}
	return card
}

// isPropRow reports whether an odds row belongs to the prop sync group.
func isPropRow(row db.Odds) bool {
	return row.MarketType == "TEAM_TOTAL" || row.PlayerName != "" || isPlayerMarket(row.MarketType)
}

// currentOdds drops rows left behind by an earlier sync. Game lines and props
// are fetched independently and stamped with different fetch times; currentOdds
// computes the newest FetchedAt separately for game-line rows and prop rows,
// keeping rows that match the newest timestamp of their own sync group.
func currentOdds(rows []db.Odds) []db.Odds {
	if len(rows) == 0 {
		return rows
	}

	var newestGameLine, newestProp time.Time
	for _, row := range rows {
		if isPropRow(row) {
			if row.FetchedAt.After(newestProp) {
				newestProp = row.FetchedAt
			}
		} else {
			if row.FetchedAt.After(newestGameLine) {
				newestGameLine = row.FetchedAt
			}
		}
	}

	current := make([]db.Odds, 0, len(rows))
	for _, row := range rows {
		if isPropRow(row) {
			if !newestProp.IsZero() && row.FetchedAt.Equal(newestProp) {
				current = append(current, row)
			}
		} else {
			if !newestGameLine.IsZero() && row.FetchedAt.Equal(newestGameLine) {
				current = append(current, row)
			}
		}
	}
	return current
}

// rowsFor narrows a game's odds to one market and selection.
func rowsFor(rows []db.Odds, market, team, side string) []db.Odds {
	matches := make([]db.Odds, 0)
	for _, row := range rows {
		if row.MarketType != market || row.PlayerName != "" {
			continue
		}
		if row.Team != team || row.Side != side {
			continue
		}
		matches = append(matches, row)
	}
	return matches
}

// pickGameLine uses the viewer's home book when it prices the selection, and
// the median price and line across every book otherwise.
func pickGameLine(rows []db.Odds, homeBook string) (GameLineView, bool) {
	if len(rows) == 0 {
		return GameLineView{}, false
	}
	first := rows[0]

	line := medianLine(rows)
	price := parlay.DecimalToAmerican(medianDecimal(rows))
	if home := homeBookRow(rows, homeBook); home != nil {
		price = home.Price
		if home.Line != nil {
			line = home.Line
		}
	}

	view := GameLineView{
		Market: first.MarketType,
		Team:   first.Team,
		Side:   first.Side,
		Odds:   price,
		Price:  formatOdds(price),
	}
	if line != nil {
		view.Line = formatFloat(*line)
	}
	view.Text = lineLabel(first.MarketType, first.Team, first.Side, line)
	return view, true
}

// homeBookRow returns the viewer's book for a selection, or nil.
func homeBookRow(rows []db.Odds, homeBook string) *db.Odds {
	homeBook = strings.TrimSpace(homeBook)
	if homeBook == "" {
		return nil
	}
	for i := range rows {
		if strings.EqualFold(strings.TrimSpace(rows[i].Book), homeBook) {
			return &rows[i]
		}
	}
	return nil
}

// medianDecimal returns the median decimal price across the rows.
func medianDecimal(rows []db.Odds) float64 {
	prices := make([]float64, 0, len(rows))
	for _, row := range rows {
		prices = append(prices, parlay.AmericanToDecimal(row.Price))
	}
	return parlay.ReferencePrice(prices, 0)
}

// medianLine returns the median line across the rows that carry one.
func medianLine(rows []db.Odds) *float64 {
	lines := make([]float64, 0, len(rows))
	for _, row := range rows {
		if row.Line != nil {
			lines = append(lines, *row.Line)
		}
	}
	if len(lines) == 0 {
		return nil
	}

	median := analyzer.Median(lines)
	return &median
}

// lineLabel is the short text a card and pill show for one selection.
func lineLabel(market, team, side string, line *float64) string {
	switch market {
	case "SPREAD":
		if line == nil {
			return team
		}
		return team + " " + signedLine(*line)
	case "ML":
		return team + " ML"
	case "TOTAL":
		if line == nil {
			return titleSide(side)
		}
		return titleSide(side) + " " + formatFloat(*line)
	default:
		return team
	}
}

// gameStatus maps a stored status to the card's label and color.
func gameStatus(status db.GameStatus) (string, string) {
	switch status {
	case db.GameLive:
		return "Live", "border-sky-700 bg-sky-950 text-sky-300"
	case db.GameFinal:
		return "Final", "border-slate-700 bg-slate-800 text-slate-300"
	default:
		return "Upcoming", "border-slate-700 bg-slate-900 text-slate-400"
	}
}

// gameScore prints a live or final score, empty before the game starts.
func gameScore(game db.Game) string {
	if game.HomeScore == nil || game.AwayScore == nil {
		return ""
	}
	return fmt.Sprintf("%s %d - %s %d", game.AwayTeam, *game.AwayScore, game.HomeTeam, *game.HomeScore)
}

// gameBadges lists the venue and weather warnings a card shows.
func gameBadges(game db.Game) []GameBadgeView {
	badges := make([]GameBadgeView, 0, 3)
	if game.Indoor {
		badges = append(badges, GameBadgeView{
			Text:  "🏠 Indoor",
			Class: "border-slate-700 bg-slate-800 text-slate-300",
		})
	}
	if game.WindMPH != nil {
		class := "border-slate-700 bg-slate-800 text-slate-300"
		switch {
		case *game.WindMPH >= 20:
			class = "border-rose-700 bg-rose-950 text-rose-300"
		case *game.WindMPH >= 15:
			class = "border-amber-700 bg-amber-950 text-amber-300"
		}
		badges = append(badges, GameBadgeView{
			Text:  "💨 " + formatFloat(*game.WindMPH) + " mph",
			Class: class,
		})
	}
	if game.PrecipIN != nil && *game.PrecipIN >= 0.1 {
		badges = append(badges, GameBadgeView{
			Text:  "🌧",
			Class: "border-sky-700 bg-sky-950 text-sky-300",
		})
	}
	return badges
}

// page builds the shell's data for the Games tab.
func (h *gameHandlers) page(r *http.Request) Page {
	page := Page{Title: "Games"}
	if user, ok := h.user(r.Context()); ok {
		view := NewUser(user)
		page.User = &view
	}
	if h.deps.GroupsFrom != nil {
		page.Groups = h.deps.GroupsFrom(r.Context())
	}
	return page
}

// user returns the signed-in person.
func (h *gameHandlers) user(ctx context.Context) (db.User, bool) {
	if h.deps.UserFrom == nil {
		return db.User{}, false
	}
	return h.deps.UserFrom(ctx)
}

// group returns the active group.
func (h *gameHandlers) group(ctx context.Context) (db.Group, bool) {
	if h.deps.GroupFrom == nil {
		return db.Group{}, false
	}
	return h.deps.GroupFrom(ctx)
}

// render writes the Games page.
func (h *gameHandlers) render(w http.ResponseWriter, data GamesPage) {
	html, err := h.deps.Renderer.Render("games", data)
	if err != nil {
		h.deps.Logger.Error().Err(err).Str("page", "games").Msg("render page")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(html)
}

// renderPartial writes one HTMX fragment.
func (h *gameHandlers) renderPartial(w http.ResponseWriter, name string, data any) {
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

// props renders on-demand prop quick-add pills for one expanded game card.
func (h *gameHandlers) props(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("id")
	if gameID == "" || h.deps.Store == nil {
		http.NotFound(w, r)
		return
	}

	game, err := h.deps.Store.GameByID(r.Context(), gameID)
	if errors.Is(err, db.ErrGameNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}

	if h.deps.Refresh != nil {
		if err := h.deps.Refresh.SyncGameProps(r.Context(), gameID); err != nil {
			h.deps.Logger.Warn().Err(err).Str("game_id", gameID).Msg("sync game props on demand")
		}
	}

	rows, err := h.deps.Store.OddsForGame(r.Context(), gameID)
	if err != nil {
		h.serverError(w, err)
		return
	}

	user, hasUser := h.user(r.Context())
	homeBook := ""
	if hasUser && user.HomeBook != nil {
		homeBook = *user.HomeBook
	}

	view := GamePropsView{
		GameID: game.ID,
	}
	if group, ok := h.group(r.Context()); ok {
		slips, err := h.deps.Store.SlipsForWeek(r.Context(), group.ID, game.WeekID)
		if err == nil && len(slips) > 0 {
			view.SlipID = slips[0].ID
			view.HasSlip = true
			view.CanAdd = slips[0].Status == db.SlipOpen
		}
	}

	propRows := filterPropOdds(rows)
	propRows = currentOdds(propRows)

	view.TeamTotals = buildTeamTotals(game, propRows, homeBook)
	view.Players = h.buildPlayerProps(r.Context(), game, propRows, homeBook)

	h.renderPartial(w, "game-props", view)
}

func filterPropOdds(rows []db.Odds) []db.Odds {
	out := make([]db.Odds, 0, len(rows))
	for _, row := range rows {
		if isPropRow(row) {
			out = append(out, row)
		}
	}
	return out
}

func buildTeamTotals(game db.Game, rows []db.Odds, homeBook string) []GameLineView {
	var views []GameLineView
	for _, team := range []string{game.AwayTeam, game.HomeTeam} {
		if team == "" {
			continue
		}
		for _, side := range []string{"OVER", "UNDER"} {
			var matching []db.Odds
			for _, r := range rows {
				if r.MarketType == "TEAM_TOTAL" && r.Team == team && r.Side == side {
					matching = append(matching, r)
				}
			}
			if line, ok := pickGameLine(matching, homeBook); ok {
				line.Market = "TEAM_TOTAL"
				line.Team = team
				line.Side = side
				line.Text = team + " " + titleSide(side) + " " + line.Line
				views = append(views, line)
			}
		}
	}
	return views
}

var propMarketOrder = []string{
	"PASS_YDS",
	"PASS_TDS",
	"RUSH_YDS",
	"RUSH_ATT",
	"REC_YDS",
	"RECEPTIONS",
	"ANYTIME_TD",
	"KICKER_PTS",
}

func (h *gameHandlers) buildPlayerProps(
	ctx context.Context,
	game db.Game,
	rows []db.Odds,
	homeBook string,
) []GamePlayerPropsView {
	var playerNames []string
	seen := make(map[string]bool)
	for _, r := range rows {
		name := strings.TrimSpace(r.PlayerName)
		if name != "" && !seen[name] {
			seen[name] = true
			playerNames = append(playerNames, name)
		}
	}

	var players []GamePlayerPropsView
	for _, name := range playerNames {
		athleteID := ""
		team := ""
		if h.deps.Store != nil {
			aid, tm, err := h.deps.Store.ResolvePlayerMatch(ctx, name, game.HomeTeam, game.AwayTeam)
			if err == nil {
				athleteID = aid
				team = tm
			}
		}
		if team == "" {
			for _, r := range rows {
				if r.PlayerName == name && r.Team != "" {
					team = r.Team
					break
				}
			}
		}

		var pills []GameLineView
		for _, market := range propMarketOrder {
			if market == "ANYTIME_TD" {
				var matching []db.Odds
				for _, r := range rows {
					if r.PlayerName == name && r.MarketType == "ANYTIME_TD" && r.Side == "YES" {
						matching = append(matching, r)
					}
				}
				if pill, ok := pickGameLine(matching, homeBook); ok {
					pill.Market = "ANYTIME_TD"
					pill.PlayerName = name
					pill.PlayerID = athleteID
					pill.Team = team
					pill.Side = "YES"
					pill.Text = "Anytime TD"
					pills = append(pills, pill)
				}
			} else {
				for _, side := range []string{"OVER", "UNDER"} {
					var matching []db.Odds
					for _, r := range rows {
						if r.PlayerName == name && r.MarketType == market && r.Side == side {
							matching = append(matching, r)
						}
					}
					if pill, ok := pickGameLine(matching, homeBook); ok {
						pill.Market = market
						pill.PlayerName = name
						pill.PlayerID = athleteID
						pill.Team = team
						pill.Side = side
						label := marketLabels[market]
						if pill.Line != "" {
							pill.Text = titleSide(side) + " " + pill.Line + " " + label
						} else {
							pill.Text = titleSide(side) + " " + label
						}
						pills = append(pills, pill)
					}
				}
			}
		}

		if len(pills) > 0 {
			players = append(players, GamePlayerPropsView{
				Name:     name,
				Team:     team,
				PlayerID: athleteID,
				Pills:    pills,
			})
		}
	}

	slices.SortFunc(players, func(a, b GamePlayerPropsView) int {
		teamScore := func(t string) int {
			switch t {
			case game.AwayTeam:
				return 0
			case game.HomeTeam:
				return 1
			default:
				return 2
			}
		}
		sa, sb := teamScore(a.Team), teamScore(b.Team)
		if sa != sb {
			return sa - sb
		}
		return strings.Compare(a.Name, b.Name)
	})

	return players
}

// serverError logs err and answers 500.
func (h *gameHandlers) serverError(w http.ResponseWriter, err error) {
	h.deps.Logger.Error().Err(err).Msg("games request")
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}
