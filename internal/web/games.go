package web

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kalverra/parlay-club/internal/board"
	"github.com/kalverra/parlay-club/internal/db"
)

// gameHandlers serves the one props-loading route. The old Games board is a
// redirect now that the pick screen owns the week.
type gameHandlers struct {
	deps Dependencies
}

// Routes mounts the props route and the Games redirect. They sit inside
// RequireUser and the group scope, like the slip screen.
func (h *gameHandlers) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /games", h.games)
	mux.HandleFunc("GET /games/{id}/props", h.props)
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
	Name        string
	Team        string
	PlayerID    string
	HeadshotURL string
	Initials    string
	Pills       []GameLineView
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

// games sends an old Games link to the active group's week. The pick board is
// where picks happen now.
func (h *gameHandlers) games(w http.ResponseWriter, r *http.Request) {
	if group, ok := h.group(r.Context()); ok {
		h.redirect(w, r, "/groups/"+strconv.FormatInt(group.ID, 10)+"/week")
		return
	}
	h.redirect(w, r, "/")
}

// props loads props on demand and either retriggers the pick board's search or
// renders the standalone prop partial. Passing ?slip= names the slip whose
// board should refresh.
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

	if rawSlip := strings.TrimSpace(r.URL.Query().Get("slip")); rawSlip != "" {
		// The board owns the results. Tell it the props changed and let its
		// own results request re-run the current search.
		w.Header().Set("Hx-Trigger", "board-updated")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(""))
		return
	}

	rows, err := h.deps.Store.OddsForGame(r.Context(), gameID)
	if err != nil {
		h.serverError(w, err)
		return
	}
	rows = currentOdds(rows)

	user, hasUser := h.user(r.Context())
	homeBook := ""
	if hasUser && user.HomeBook != nil {
		homeBook = *user.HomeBook
	}

	view := GamePropsView{GameID: game.ID}
	if group, ok := h.group(r.Context()); ok {
		slips, err := h.deps.Store.SlipsForWeek(r.Context(), group.ID, game.WeekID)
		if err == nil && len(slips) > 0 {
			view.SlipID = slips[0].ID
			view.HasSlip = true
			view.CanAdd = slips[0].Status == db.SlipOpen
		}
	}

	roster, err := h.deps.Store.RosterForTeams(r.Context(), []string{game.AwayTeam, game.HomeTeam})
	if err != nil {
		h.serverError(w, err)
		return
	}
	now := time.Time{}
	built := board.Build([]db.Game{game}, rows, roster, homeBook, nil, now)
	view.TeamTotals, view.Players = groupProps(game, built)

	h.renderPartial(w, "game-props", view)
}

// groupProps splits built rows into team totals and per-player pills.
func groupProps(game db.Game, rows []board.Row) ([]GameLineView, []GamePlayerPropsView) {
	var totals []GameLineView
	byPlayer := make(map[string]*GamePlayerPropsView)
	var order []string
	for _, row := range rows {
		line := gameLineView(row)
		switch {
		case row.Category == board.CategoryTeamTotals:
			// The card groups by team, so the pill reads "BUF Over 24.5".
			if line.Line != "" {
				line.Text = row.Team + " " + titleSide(row.Side) + " " + line.Line
			}
			totals = append(totals, line)
		case row.PlayerName != "":
			// The player's header names them, so the pill drops the prefix.
			line.Text = strings.TrimPrefix(row.Text, row.PlayerName+" ")
			player, ok := byPlayer[row.PlayerName]
			if !ok {
				player = &GamePlayerPropsView{
					Name:        row.PlayerName,
					Team:        row.Team,
					PlayerID:    row.PlayerID,
					HeadshotURL: row.HeadshotURL,
					Initials:    Initials(row.PlayerName),
				}
				byPlayer[row.PlayerName] = player
				order = append(order, row.PlayerName)
			} else if player.HeadshotURL == "" && row.HeadshotURL != "" {
				player.HeadshotURL = row.HeadshotURL
			}
			player.Pills = append(player.Pills, line)
		}
	}

	players := make([]GamePlayerPropsView, 0, len(order))
	for _, name := range order {
		player := byPlayer[name]
		slices.SortStableFunc(player.Pills, func(a, b GameLineView) int {
			left, right := propMarketRank(a.Market), propMarketRank(b.Market)
			if left != right {
				return left - right
			}
			return strings.Compare(a.Text, b.Text)
		})
		players = append(players, *player)
	}

	slices.SortStableFunc(players, func(a, b GamePlayerPropsView) int {
		teamScore := func(team string) int {
			switch team {
			case game.AwayTeam:
				return 0
			case game.HomeTeam:
				return 1
			default:
				return 2
			}
		}
		if teamScore(a.Team) != teamScore(b.Team) {
			return teamScore(a.Team) - teamScore(b.Team)
		}
		return strings.Compare(a.Name, b.Name)
	})
	return totals, players
}

// propMarketRank orders one player's pills the way the card reads.
func propMarketRank(market string) int {
	switch market {
	case "PASS_YDS":
		return 0
	case "PASS_TDS":
		return 1
	case "RUSH_YDS":
		return 2
	case "RUSH_ATT":
		return 3
	case "REC_YDS":
		return 4
	case "RECEPTIONS":
		return 5
	case "ANYTIME_TD":
		return 6
	case "KICKER_PTS":
		return 7
	default:
		return 8
	}
}

// gameLineView maps a built row to the quick-add pill's fields.
func gameLineView(row board.Row) GameLineView {
	view := GameLineView{
		Text:       row.Text,
		Price:      formatOdds(row.Odds),
		Market:     row.Market,
		Team:       row.Team,
		PlayerName: row.PlayerName,
		PlayerID:   row.PlayerID,
		Side:       row.Side,
		Odds:       row.Odds,
	}
	if row.Line != nil {
		view.Line = formatFloat(*row.Line)
	}
	return view
}

// isPropRow reports whether an odds row belongs to the prop sync group.
func isPropRow(row db.Odds) bool {
	return board.IsPropRow(row)
}

// currentOdds drops rows left behind by an earlier sync. Game lines and props
// are fetched independently and stamped with different fetch times, and props
// are synced one game at a time, so currentOdds computes the newest FetchedAt
// separately for each game's game-line rows and prop rows. Keeping the game in
// the key means loading one game's props later does not hide another game's
// props.
func currentOdds(rows []db.Odds) []db.Odds {
	if len(rows) == 0 {
		return rows
	}

	type syncGroup struct {
		gameID string
		props  bool
	}
	newest := make(map[syncGroup]time.Time, len(rows))
	for _, row := range rows {
		group := syncGroup{row.GameID, isPropRow(row)}
		if row.FetchedAt.After(newest[group]) {
			newest[group] = row.FetchedAt
		}
	}

	current := make([]db.Odds, 0, len(rows))
	for _, row := range rows {
		group := syncGroup{row.GameID, isPropRow(row)}
		if !newest[group].IsZero() && row.FetchedAt.Equal(newest[group]) {
			current = append(current, row)
		}
	}
	return current
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

// redirect sends the browser to target, or HTMX if the request came from it.
func (h *gameHandlers) redirect(w http.ResponseWriter, r *http.Request, target string) {
	if strings.EqualFold(r.Header.Get("Hx-Request"), "true") {
		w.Header().Set("Hx-Redirect", target)
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// serverError logs err and answers 500.
func (h *gameHandlers) serverError(w http.ResponseWriter, err error) {
	h.deps.Logger.Error().Err(err).Msg("games request")
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}
