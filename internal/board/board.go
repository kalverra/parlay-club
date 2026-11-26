// Package board turns stored odds into the searchable rows the pick screen
// shows. It reads nothing itself, so its rules stay easy to test.
package board

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/espn"
	"github.com/kalverra/parlay-club/internal/parlay"
	"github.com/kalverra/parlay-club/internal/tz"
)

// Category groups a row for the category chips.
type Category string

// The categories the pick board filters by.
const (
	CategoryLines      Category = "lines"
	CategoryTeamTotals Category = "team_totals"
	CategoryPassing    Category = "passing"
	CategoryRushing    Category = "rushing"
	CategoryReceiving  Category = "receiving"
	CategoryTDs        Category = "tds"
	CategoryKicking    Category = "kicking"
)

// Categories lists the category chips in display order.
var Categories = []Category{
	CategoryLines,
	CategoryTeamTotals,
	CategoryPassing,
	CategoryRushing,
	CategoryReceiving,
	CategoryTDs,
	CategoryKicking,
}

// Label is the chip text.
func (c Category) Label() string {
	switch c {
	case CategoryLines:
		return "Lines"
	case CategoryTeamTotals:
		return "Team totals"
	case CategoryPassing:
		return "Passing"
	case CategoryRushing:
		return "Rushing"
	case CategoryReceiving:
		return "Receiving"
	case CategoryTDs:
		return "TDs"
	case CategoryKicking:
		return "Kicking"
	default:
		return string(c)
	}
}

// Window groups a row by kickoff time for the window chips.
type Window string

// The kickoff windows the pick board filters by, in Eastern time.
const (
	WindowThursday      Window = "thursday"
	WindowSundayMorning Window = "sun_morning"
	WindowSundayEarly   Window = "sun_early"
	WindowSundayLate    Window = "sun_late"
	WindowSundayNight   Window = "sun_night"
	WindowMonday        Window = "monday"
)

// Windows lists the window chips in display order.
var Windows = []Window{
	WindowThursday,
	WindowSundayMorning,
	WindowSundayEarly,
	WindowSundayLate,
	WindowSundayNight,
	WindowMonday,
}

// Label is the chip text.
func (w Window) Label() string {
	switch w {
	case WindowThursday:
		return "Thursday"
	case WindowSundayMorning:
		return "Sun morning"
	case WindowSundayEarly:
		return "Sun early"
	case WindowSundayLate:
		return "Sun late"
	case WindowSundayNight:
		return "Sun night"
	case WindowMonday:
		return "Monday"
	default:
		return string(w)
	}
}

// Sort orders the results.
type Sort string

// The ways the pick board can order its rows.
const (
	SortPlayer  Sort = "player"
	SortOdds    Sort = "odds"
	SortLine    Sort = "line"
	SortMatchup Sort = "matchup"
)

// DetailSorts lists the open game's sort chips in display order. One game has
// one kickoff, so kickoff drops out and player is the default.
var DetailSorts = []Sort{SortPlayer, SortOdds, SortLine, SortMatchup}

// Label is the chip text.
func (s Sort) Label() string {
	switch s {
	case SortPlayer:
		return "Player"
	case SortOdds:
		return "Odds"
	case SortLine:
		return "Line"
	case SortMatchup:
		return "Matchup"
	default:
		return string(s)
	}
}

// Row is one selection a person can add to their slip. The fields after Text
// are what the add-leg form posts.
type Row struct {
	GameID         string
	Market         string
	Team           string
	PlayerName     string
	PlayerID       string
	Position       string
	HeadshotURL    string
	Side           string
	Line           *float64
	Odds           int
	Category       Category
	Window         Window
	Kickoff        time.Time
	Text           string
	Picked         int64
	PickedLine     *float64
	PickedOdds     int
	Drifted        bool
	BoardDriftNote string
	TrayDriftNote  string
	Badge          string
}

// GameCard is one game on the slate: the matchup, its kickoff, the game lines
// at a glance, and how many props it carries.
type GameCard struct {
	ID          string
	Matchup     string
	Venue       string
	Indoor      bool
	TempF       *float64
	WindMPH     *float64
	WindGustMPH *float64
	PrecipIN    *float64
	Away        string
	Home        string
	AwayLogo    string
	HomeLogo    string
	Kickoff     time.Time
	Lines       GameLines
	Props       int
	HasProps    bool
	Unloaded    bool
}

// Slate is the pick board's game-card view for one week's games, along with
// the resolved open game, kickoff-window counts, and empty-state flags.
type Slate struct {
	Cards          []GameCard
	OpenGame       *GameCard
	ResolvedGameID string
	WindowCounts   map[Window]int
	Started        int
	NoGames        bool
	AllStarted     bool
	NoLines        bool
	Empty          bool
}

// GameLines is a game's spread, total, and moneyline summaries. A market the
// board never stored stays nil.
type GameLines struct {
	Spread    *LineSummary
	Total     *LineSummary
	Moneyline *LineSummary
}

// LineSummary is one game-line market printed as text. A spread uses the away
// and home fields, a total uses Line plus OverOdds and UnderOdds, and a
// moneyline uses only the two odds.
type LineSummary struct {
	AwayLine  string
	AwayOdds  string
	HomeLine  string
	HomeOdds  string
	Line      string
	OverOdds  string
	UnderOdds string
}

// Tab is one category tab in the open game's tab bar. Team totals fold into
// Lines, so there is no tab of their own.
type Tab struct {
	Category Category
	Label    string
	Count    int
	Active   bool
}

// LineGroup is one line and the sides a person can pick from it. Options holds
// one row when a side is unpriced and two when both sides are stored.
type LineGroup struct {
	Market   string
	Team     string
	Player   string
	PlayerID string
	Position string
	Label    string
	Line     *float64
	Options  []Row
}

// PlayerGroup is one player's card in a prop tab: who they are and the lines
// they carry across markets. Receiving yards and receptions share a card.
type PlayerGroup struct {
	Name        string
	PlayerID    string
	Position    string
	HeadshotURL string
	Team        string
	Tags        []string
	Lines       []LineGroup
}

// GameDetail is the open game's header, its tabs, and the rows of the active
// tab. Lines holds the game lines when Lines is active; Players holds the
// player cards otherwise.
type GameDetail struct {
	Game     GameCard
	Category Category
	Sort     Sort
	Tabs     []Tab
	Lines    []LineGroup
	Players  []PlayerGroup
}

// BoardSignals holds optional nflverse, injury, and open-slip context used to
// attach suggestion badges to board rows and rank rows by matchup strength.
type BoardSignals struct { //nolint:revive // Name specified in design.
	TeamEPA     map[string]*analyzer.TeamDefenseProfile
	PlayerUsage map[string]*analyzer.PlayerUsage
	Injuries    map[string][]espn.Injury
	SlipLegs    []analyzer.Leg
}

// Query is one search, filter, and sort request.
type Query struct {
	Q        string
	Category Category
	Window   Window
	Sort     Sort
	GameID   string
	// Now is the app clock. A zero time uses time.Now.
	Now     time.Time
	Signals BoardSignals
}

// eastern is the zone the kickoff windows use.
var eastern = tz.Eastern

// IsPropRow reports whether an odds row belongs to the prop sync group.
func IsPropRow(row db.Odds) bool {
	return db.IsPropRow(row)
}

// isPlayerMarket reports whether a market type targets an athlete rather than
// a game or team.
func isPlayerMarket(market string) bool {
	return db.IsPlayerMarket(market)
}

// Build turns one week's games and odds into rows. Games that kicked off
// before now are left out. mine marks the viewer's legs so a picked selection
// shows the leg to remove. The roster fills in player IDs for prop rows.
func Build(
	games []db.Game,
	odds []db.Odds,
	roster []db.RosterPlayer,
	homeBook string,
	mine []db.Leg,
	now time.Time,
) []Row {
	byGame := make(map[string]db.Game, len(games))
	for _, game := range games {
		if !now.IsZero() && !now.Before(game.Kickoff) {
			continue
		}
		byGame[game.ID] = game
	}

	index := newRosterIndex(roster)
	type group struct {
		gameID, market, team, player, side string
		playerID, position, headshotURL    string
		rows                               []db.Odds
	}
	groups := make(map[string]*group)
	order := make([]string, 0, len(odds))
	for _, row := range odds {
		if _, ok := byGame[row.GameID]; !ok {
			continue
		}
		var key, playerID, position, headshotURL string
		if isPlayerMarket(row.MarketType) || row.PlayerName != "" {
			playerID, position, headshotURL = matchPlayer(&index, row.PlayerName, row.Team)
			key = strings.Join(
				[]string{row.GameID, row.MarketType, canonicalPlayerKey(playerID, row.PlayerName, row.Team), row.Side},
				"\x00",
			)
		} else {
			key = strings.Join(
				[]string{row.GameID, row.MarketType, row.Team, row.PlayerName, row.Side},
				"\x00",
			)
		}
		found, ok := groups[key]
		if !ok {
			found = &group{
				gameID:      row.GameID,
				market:      row.MarketType,
				team:        row.Team,
				player:      row.PlayerName,
				side:        row.Side,
				playerID:    playerID,
				position:    position,
				headshotURL: headshotURL,
			}
			groups[key] = found
			order = append(order, key)
		} else {
			if found.team == "" && row.Team != "" {
				found.team = row.Team
			}
			if found.player == "" && row.PlayerName != "" {
				found.player = row.PlayerName
			}
		}
		found.rows = append(found.rows, row)
	}

	rows := make([]Row, 0, len(groups))
	for _, key := range order {
		group := groups[key]
		game := byGame[group.gameID]
		line, price := selectionPrice(group.rows, homeBook)
		player := group.player
		team := group.team
		if home := homeBookRow(group.rows, homeBook); home != nil {
			if home.PlayerName != "" {
				player = home.PlayerName
			}
			if home.Team != "" {
				team = home.Team
			}
		}
		rows = append(rows, Row{
			GameID:      group.gameID,
			Market:      group.market,
			Team:        team,
			PlayerName:  player,
			PlayerID:    group.playerID,
			Position:    group.position,
			HeadshotURL: group.headshotURL,
			Side:        group.side,
			Line:        line,
			Odds:        price,
			Category:    categoryFor(group.market),
			Window:      windowFor(game.Kickoff),
			Kickoff:     game.Kickoff,
			Text:        db.FormatSelection(group.market, team, player, group.side, line),
		})
	}

	for _, leg := range mine {
		for i := range rows {
			if rows[i].Picked != 0 {
				continue
			}
			if rows[i].MatchesLeg(leg) {
				rows[i].Picked = leg.ID
				if leg.Line != nil {
					line := *leg.Line
					rows[i].PickedLine = &line
				}
				rows[i].PickedOdds = leg.Odds
				if legDrifted(leg, rows[i]) {
					rows[i].Drifted = true
					rows[i].BoardDriftNote = boardDriftNote(leg, rows[i])
					rows[i].TrayDriftNote = trayDriftNote(leg, rows[i])
				}
				break
			}
		}
	}

	sortRows(rows)
	return rows
}

// legDrifted reports whether a saved leg's line or odds differ from the current
// board row.
func legDrifted(leg db.Leg, row Row) bool {
	if leg.MarketType == "OTHER" {
		return false
	}
	return !db.SameLine(leg.Line, row.Line) || leg.Odds != row.Odds
}

// boardDriftNote formats a concise drift notice for a picked board option.
func boardDriftNote(leg db.Leg, row Row) string {
	if !db.SameLine(leg.Line, row.Line) {
		slipLine := formatDriftLine(leg.MarketType, leg.Line)
		boardLine := formatDriftLine(leg.MarketType, row.Line)
		if slipLine != "" && boardLine != "" {
			return "Slip " + slipLine + " · now " + boardLine
		}
		return "Line moved"
	}
	if leg.Odds != row.Odds {
		return "Slip " + formatOdds(leg.Odds) + " · now " + formatOdds(row.Odds)
	}
	return ""
}

// trayDriftNote formats a concise drift notice for a leg in the pick tray.
func trayDriftNote(leg db.Leg, row Row) string {
	if !db.SameLine(leg.Line, row.Line) {
		boardLine := formatDriftLine(leg.MarketType, row.Line)
		if boardLine != "" {
			return "Now " + boardLine + " (" + formatOdds(row.Odds) + ")"
		}
		return "Line moved"
	}
	if leg.Odds != row.Odds {
		return "Now " + formatOdds(row.Odds)
	}
	return ""
}

func formatDriftLine(marketType string, line *float64) string {
	if line == nil {
		return ""
	}
	if marketType == "SPREAD" {
		return signedSpread(*line)
	}
	return formatLine(*line)
}

func formatOdds(odds int) string {
	if odds > 0 {
		return "+" + strconv.Itoa(odds)
	}
	return strconv.Itoa(odds)
}

// MatchesLeg reports whether the row's selection is the leg's pick, ignoring
// moved lines so picked legs stay bound on the board.
func (r Row) MatchesLeg(leg db.Leg) bool {
	return db.SameSelection(db.SelectionKey{
		GameID:     r.GameID,
		MarketType: r.Market,
		Team:       r.Team,
		PlayerName: r.PlayerName,
		PlayerID:   r.PlayerID,
		Side:       r.Side,
	}, db.LegSelection(leg))
}

// categoryFor maps an odds market to a chip category.
func categoryFor(market string) Category {
	switch market {
	case "SPREAD", "ML", "TOTAL":
		return CategoryLines
	case "TEAM_TOTAL":
		return CategoryTeamTotals
	case "PASS_YDS", "PASS_TDS":
		return CategoryPassing
	case "RUSH_YDS", "RUSH_ATT":
		return CategoryRushing
	case "REC_YDS", "RECEPTIONS":
		return CategoryReceiving
	case "ANYTIME_TD":
		return CategoryTDs
	case "KICKER_PTS":
		return CategoryKicking
	default:
		return CategoryLines
	}
}

// windowFor maps a kickoff to its Eastern-time window chip. Thursday and Monday
// cover the whole day, so a late game that lands on the next day in UTC still
// reads by its local Eastern day here.
func windowFor(kickoff time.Time) Window {
	easternTime := kickoff.In(eastern)
	switch easternTime.Weekday() {
	case time.Thursday:
		return WindowThursday
	case time.Monday:
		return WindowMonday
	}
	switch hour := easternTime.Hour(); {
	case hour < 12:
		return WindowSundayMorning
	case hour < 16:
		return WindowSundayEarly
	case hour < 19:
		return WindowSundayLate
	default:
		return WindowSundayNight
	}
}

// selectionPrice picks one line and price for a selection, using the viewer's
// home book when it prices it and the median across every book otherwise.
func selectionPrice(rows []db.Odds, homeBook string) (*float64, int) {
	if len(rows) == 0 {
		return nil, 0
	}
	line := medianLine(rows)
	price := parlay.DecimalToAmerican(medianDecimal(rows))
	if home := homeBookRow(rows, homeBook); home != nil {
		price = home.Price
		if home.Line != nil {
			line = home.Line
		}
	}
	return line, price
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

type rosterEntry struct {
	player   db.RosterPlayer
	normFull string
	normAlt  string
}

type rosterIndex struct {
	byTeamAndName map[string]db.RosterPlayer // team + "\x00" + norm
	byName        map[string]db.RosterPlayer // first match when team == "" or fallback
	byGameTeam    map[string][]rosterEntry   // team -> normalized entries for search
}

func newRosterIndex(roster []db.RosterPlayer) rosterIndex {
	if len(roster) == 0 {
		return rosterIndex{}
	}
	idx := rosterIndex{
		byTeamAndName: make(map[string]db.RosterPlayer, len(roster)),
		byName:        make(map[string]db.RosterPlayer, len(roster)),
		byGameTeam:    make(map[string][]rosterEntry),
	}
	for _, player := range roster {
		normFull := normalize(player.FullName)
		normAlt := ""
		if player.NormName != "" {
			normAlt = normalize(player.NormName)
		}
		idx.byGameTeam[player.Team] = append(idx.byGameTeam[player.Team], rosterEntry{
			player:   player,
			normFull: normFull,
			normAlt:  normAlt,
		})
		idx.addLookup(player, normFull)
		if normAlt != "" && normAlt != normFull {
			idx.addLookup(player, normAlt)
		}
	}
	return idx
}

func (idx *rosterIndex) addLookup(player db.RosterPlayer, norm string) {
	if norm == "" {
		return
	}
	if player.Team != "" {
		key := player.Team + "\x00" + norm
		if _, exists := idx.byTeamAndName[key]; !exists {
			idx.byTeamAndName[key] = player
		}
	}
	if _, exists := idx.byName[norm]; !exists {
		idx.byName[norm] = player
	}
}

// matchPlayer finds the roster athlete for a prop row's player name and team,
// returning its ID, position, and headshot URL. A name on the row's own team
// wins over the same name on another team.
func matchPlayer(index *rosterIndex, name, team string) (string, string, string) {
	if name == "" || index == nil {
		return "", "", ""
	}
	want := normalize(name)
	if want == "" {
		return "", "", ""
	}
	if team == "" {
		_, team = db.StripPlayerTag(name)
	}
	if team != "" {
		if player, ok := index.byTeamAndName[team+"\x00"+want]; ok {
			return player.AthleteID, player.Position, player.HeadshotURL
		}
	}
	if player, ok := index.byName[want]; ok {
		return player.AthleteID, player.Position, player.HeadshotURL
	}
	return "", "", ""
}

// StartedCount counts the games that had kicked off by now, so the slate can
// name how many it hides.
func StartedCount(games []db.Game, now time.Time) int {
	if now.IsZero() {
		now = time.Now()
	}
	count := 0
	for _, game := range games {
		if !now.Before(game.Kickoff) {
			count++
		}
	}
	return count
}

// Slates shapes one week's built rows into the game cards the slate shows.
// It keeps games that have not kicked off, filters by the query's kickoff
// window and search text, and orders the cards by kickoff.
func Slates(games []db.Game, rows []Row, roster []db.RosterPlayer, q Query) []GameCard {
	now := q.Now
	if now.IsZero() {
		now = time.Now()
	}
	needle := normalize(q.Q)
	var index *rosterIndex
	if needle != "" {
		built := newRosterIndex(roster)
		index = &built
	}

	byGame := make(map[string][]Row, len(games))
	for _, row := range rows {
		byGame[row.GameID] = append(byGame[row.GameID], row)
	}

	cards := make([]GameCard, 0, len(games))
	for _, game := range games {
		if !now.Before(game.Kickoff) {
			continue
		}
		if q.Window != "" && windowFor(game.Kickoff) != q.Window {
			continue
		}
		if !gameMatchesQuery(game, index, needle) {
			continue
		}
		gameRows := byGame[game.ID]
		cards = append(cards, cardFor(game, gameRows))
	}

	sort.SliceStable(cards, func(i, j int) bool {
		if !cards[i].Kickoff.Equal(cards[j].Kickoff) {
			return cards[i].Kickoff.Before(cards[j].Kickoff)
		}
		if cards[i].Matchup != cards[j].Matchup {
			return cards[i].Matchup < cards[j].Matchup
		}
		return cards[i].ID < cards[j].ID
	})
	return cards
}

// cardFor shapes one game and its built rows into a slate card.
func cardFor(game db.Game, rows []Row) GameCard {
	card := GameCard{
		ID:          game.ID,
		Matchup:     game.AwayTeam + " @ " + game.HomeTeam,
		Venue:       db.StringValue(game.VenueName),
		Indoor:      game.Indoor,
		TempF:       game.TempF,
		WindMPH:     game.WindMPH,
		WindGustMPH: game.WindGustMPH,
		PrecipIN:    game.PrecipIN,
		Away:        game.AwayTeam,
		Home:        game.HomeTeam,
		AwayLogo:    db.StringValue(game.AwayLogo),
		HomeLogo:    db.StringValue(game.HomeLogo),
		Kickoff:     game.Kickoff,
		Lines:       gameLines(game, rows),
		Props:       propCount(rows),
	}
	card.HasProps = card.Props > 0
	card.Unloaded = !card.HasProps && len(rows) > 0
	return card
}

// BuildSlate assembles the slate cards, resolved open game, kickoff-window
// counts, and empty-state flags for one week's games and built rows.
func BuildSlate(games []db.Game, rows []Row, roster []db.RosterPlayer, q Query) Slate {
	now := q.Now
	if now.IsZero() {
		now = time.Now()
	}
	started := StartedCount(games, now)

	// The detail exists even when the current search matches only one of the
	// open game's rows, so resolve the open game from a slate without q.Q.
	openCards := Slates(games, rows, roster, Query{Window: q.Window, Now: now})
	var openGame *GameCard
	resolvedGameID := ""
	if q.GameID != "" {
		for i := range openCards {
			if openCards[i].ID == q.GameID {
				card := openCards[i]
				openGame = &card
				resolvedGameID = card.ID
				break
			}
		}
	}

	cards := openCards
	if q.Q != "" {
		cards = Slates(games, rows, roster, q)
	}

	windowCounts := make(map[Window]int)
	for _, row := range rows {
		windowCounts[row.Window]++
	}

	noGames := len(games) == 0
	allStarted := len(games) > 0 && started == len(games)
	noLines := !noGames && !allStarted && len(rows) == 0
	empty := len(cards) == 0 && !noGames && !noLines && !allStarted

	return Slate{
		Cards:          cards,
		OpenGame:       openGame,
		ResolvedGameID: resolvedGameID,
		WindowCounts:   windowCounts,
		Started:        started,
		NoGames:        noGames,
		AllStarted:     allStarted,
		NoLines:        noLines,
		Empty:          empty,
	}
}

// Detail shapes one game's built rows into the header, tabs, and lines the open
// game shows. The query supplies the search text, the open tab, and the sort;
// an unknown or empty category falls back to the first tab with rows.
func Detail(game db.Game, rows []Row, q Query) GameDetail {
	gameRows := rowsForGame(rows, game.ID)
	needle := normalize(q.Q)
	visible := make([]Row, 0, len(gameRows))
	for _, row := range gameRows {
		if needle != "" && !rowMatches(row, game, needle) {
			continue
		}
		visible = append(visible, row)
	}

	counts := tabCounts(visible)
	category := tabCategory(q.Category, counts)
	mode := detailSort(q.Sort)
	detail := GameDetail{
		Game:     cardFor(game, gameRows),
		Category: category,
		Sort:     mode,
	}
	for _, known := range tabCategories {
		if counts[known] == 0 && known != category {
			continue
		}
		detail.Tabs = append(detail.Tabs, Tab{
			Category: known,
			Label:    known.Label(),
			Count:    counts[known],
			Active:   known == category,
		})
	}

	tabRows := make([]Row, 0, len(visible))
	for _, row := range visible {
		if foldCategory(row.Category) == category {
			tabRows = append(tabRows, row)
		}
	}
	tabRows = ApplySignals(game, tabRows, q.Signals)
	if category == CategoryLines {
		detail.Lines = lineGroups(game, tabRows, mode, q.Signals)
	} else {
		detail.Players = playerGroups(game, tabRows, mode, q.Signals)
	}
	return detail
}

// tabCategories lists the open game's tabs in display order.
var tabCategories = []Category{
	CategoryLines,
	CategoryPassing,
	CategoryRushing,
	CategoryReceiving,
	CategoryTDs,
	CategoryKicking,
}

// foldCategory maps a row's category to the tab that shows it. Team totals fold
// into Lines, and an unknown category reads as Lines.
func foldCategory(category Category) Category {
	switch category {
	case CategoryTeamTotals:
		return CategoryLines
	case CategoryPassing, CategoryRushing, CategoryReceiving, CategoryTDs, CategoryKicking:
		return category
	default:
		return CategoryLines
	}
}

// tabCounts counts the visible rows in each tab, folding team totals into
// Lines.
func tabCounts(rows []Row) map[Category]int {
	counts := make(map[Category]int, len(tabCategories))
	for _, row := range rows {
		counts[foldCategory(row.Category)]++
	}
	return counts
}

// tabCategory picks the tab to show. A requested tab with rows wins; otherwise
// the detail falls back to Lines, then to the first tab with rows.
func tabCategory(requested Category, counts map[Category]int) Category {
	wanted := foldCategory(requested)
	if counts[wanted] > 0 {
		return wanted
	}
	if counts[CategoryLines] > 0 {
		return CategoryLines
	}
	for _, category := range tabCategories {
		if counts[category] > 0 {
			return category
		}
	}
	return CategoryLines
}

// detailSort resolves the open game's sort. Kickoff and category drop out, so
// anything unknown reads as player.
func detailSort(mode Sort) Sort {
	switch mode {
	case SortPlayer, SortOdds, SortLine, SortMatchup:
		return mode
	default:
		return SortPlayer
	}
}

// rowsForGame returns one game's built rows.
func rowsForGame(rows []Row, gameID string) []Row {
	gameRows := make([]Row, 0, len(rows))
	for _, row := range rows {
		if row.GameID == gameID {
			gameRows = append(gameRows, row)
		}
	}
	return gameRows
}

// Pair folds one game's built rows into one group per line. Spreads and
// moneylines key on market alone, game totals key on market and number, team
// totals key on market, team, and number, and player props key on market and
// canonical player, so an Over and an Under for the same player prop sit on
// one row even across differing book lines. Each group's options come back in
// a fixed order: away before home for a spread or moneyline, Over before Under
// for a total or prop, and the single Yes of an anytime touchdown. A line with
// only one stored side keeps one option.
func Pair(game db.Game, rows []Row) []LineGroup {
	groups := make([]LineGroup, 0, len(rows))
	index := make(map[string]int, len(rows))
	for _, row := range rows {
		key := lineKey(row)
		if at, ok := index[key]; ok {
			groups[at].Options = append(groups[at].Options, row)
			continue
		}
		index[key] = len(groups)
		groups = append(groups, newLineGroup(row))
	}
	for i := range groups {
		sortOptions(game, groups[i].Options)
	}
	return groups
}

// HoldsLeg reports whether the line group carries the leg's selection. A picked
// option matches by ID; a just-deleted or unpicked leg matches by its fields.
func (g LineGroup) HoldsLeg(leg db.Leg) bool {
	for _, option := range g.Options {
		if (leg.ID != 0 && option.Picked == leg.ID) || option.MatchesLeg(leg) {
			return true
		}
	}
	return false
}

// LineGroupForLeg applies signals to game's rows, pairs them into line groups,
// and returns the group holding leg, if any.
func LineGroupForLeg(game db.Game, rows []Row, leg db.Leg, signals BoardSignals) (LineGroup, bool) {
	gameRows := ApplySignals(game, rowsForGame(rows, game.ID), signals)
	for _, group := range Pair(game, gameRows) {
		if group.HoldsLeg(leg) {
			return group, true
		}
	}
	return LineGroup{}, false
}

// lineGroups pairs rows into one group per line and then orders the groups for
// the detail's sort.
func lineGroups(game db.Game, rows []Row, mode Sort, signals BoardSignals) []LineGroup {
	groups := Pair(game, rows)
	sortLineGroups(game, groups, mode, signals)
	return groups
}

// lineKey names the line a row sits on.
func lineKey(row Row) string {
	if isPlayerMarket(row.Market) || row.PlayerName != "" {
		return row.Market + "|" + canonicalPlayerKey(row.PlayerID, row.PlayerName, row.Team)
	}
	return LineKey(row.Market, row.Team, row.PlayerName, row.Line)
}

// LineKey names the line a selection sits on. A spread or moneyline puts both
// teams on one line, a total keys on its number, a team total keys on its team
// and number, and a player prop keys on its market and canonical player.
func LineKey(market, team, player string, line *float64) string {
	switch market {
	case "SPREAD", "ML":
		return market
	case "TOTAL":
		return market + "|" + lineText(line)
	default:
		if isPlayerMarket(market) || player != "" {
			return market + "|" + canonicalPlayerKey("", player, team)
		}
		return strings.Join([]string{market, team, player, lineText(line)}, "|")
	}
}

// newLineGroup starts a group from its first row. A game-wide line carries no
// team of its own.
func newLineGroup(row Row) LineGroup {
	group := LineGroup{
		Market:   row.Market,
		Player:   row.PlayerName,
		PlayerID: row.PlayerID,
		Position: row.Position,
		Label:    marketHeading(row.Market),
		Line:     row.Line,
		Options:  []Row{row},
	}
	switch row.Market {
	case "SPREAD", "ML", "TOTAL":
		// The teams and the number already live on the options.
	default:
		group.Team = row.Team
	}
	return group
}

// marketHeading is the label a line group shows above its sides.
func marketHeading(market string) string {
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
		return db.MarketLabel(market)
	}
}

// sortOptions orders one line's sides: away then home for a spread or
// moneyline, Over then Under for a total or a prop, and the single Yes for an
// anytime touchdown. The remaining fields break a tie the same way every time.
func sortOptions(game db.Game, options []Row) {
	sort.SliceStable(options, func(i, j int) bool {
		left, right := options[i], options[j]
		if optionRank(game, left) != optionRank(game, right) {
			return optionRank(game, left) < optionRank(game, right)
		}
		if left.Side != right.Side {
			return left.Side < right.Side
		}
		if left.Team != right.Team {
			return left.Team < right.Team
		}
		if a, b := lineValue(left.Line), lineValue(right.Line); a != b {
			return a < b
		}
		return left.Text < right.Text
	})
}

// optionRank is a side's position on its line.
func optionRank(game db.Game, row Row) int {
	switch row.Market {
	case "SPREAD", "ML":
		switch row.Team {
		case game.AwayTeam:
			return 0
		case game.HomeTeam:
			return 1
		default:
			return 2
		}
	default:
		switch row.Side {
		case "OVER", "YES":
			return 0
		case "UNDER":
			return 1
		default:
			return 2
		}
	}
}

// lineText prints an optional line for a grouping key.
func lineText(line *float64) string {
	if line == nil {
		return ""
	}
	return formatLine(*line)
}

// sortLineGroups orders line groups by the detail's sort, falling back to the
// market order for ties.
func sortLineGroups(game db.Game, groups []LineGroup, mode Sort, signals BoardSignals) {
	sort.SliceStable(groups, func(i, j int) bool {
		left, right := groups[i], groups[j]
		switch mode {
		case SortOdds:
			if a, b := shortestPrice(left), shortestPrice(right); a != b {
				return a < b
			}
		case SortLine:
			if a, b := highestLine(left), highestLine(right); a != b {
				return a > b
			}
		case SortMatchup:
			if a, b := groupSignalScore(game, left, signals), groupSignalScore(game, right, signals); a != b {
				return a > b
			}
		}
		return lineGroupLess(left, right)
	})
}

// lineGroupLess is the natural order: the game lines first, then props by
// category and stat, then by team and number.
func lineGroupLess(left, right LineGroup) bool {
	if marketRank(left.Market) != marketRank(right.Market) {
		return marketRank(left.Market) < marketRank(right.Market)
	}
	if categoryOrder(categoryFor(left.Market)) != categoryOrder(categoryFor(right.Market)) {
		return categoryOrder(categoryFor(left.Market)) < categoryOrder(categoryFor(right.Market))
	}
	if left.Label != right.Label {
		return left.Label < right.Label
	}
	if left.Team != right.Team {
		return left.Team < right.Team
	}
	if a, b := lineValue(left.Line), lineValue(right.Line); a != b {
		return a < b
	}
	if left.Player != right.Player {
		return left.Player < right.Player
	}
	return left.PlayerID < right.PlayerID
}

// marketRank is a market's position inside the Lines tab: the game lines first
// in display order, then team totals, then every prop.
func marketRank(market string) int {
	switch market {
	case "SPREAD":
		return 0
	case "ML":
		return 1
	case "TOTAL":
		return 2
	case "TEAM_TOTAL":
		return 3
	default:
		return 4
	}
}

// shortestPrice is a line's best price as decimal odds, preferring a priced
// side. A line with no price sorts last.
func shortestPrice(group LineGroup) float64 {
	best := math.Inf(1)
	for _, option := range group.Options {
		if option.Odds == 0 {
			continue
		}
		if decimal := parlay.AmericanToDecimal(option.Odds); decimal < best {
			best = decimal
		}
	}
	return best
}

// highestLine is a line's largest number, with a missing line sorting last.
func highestLine(group LineGroup) float64 {
	highest := math.Inf(-1)
	for _, option := range group.Options {
		if option.Line != nil && *option.Line > highest {
			highest = *option.Line
		}
	}
	return highest
}

// lineValue is an optional line as a number, with a missing line sorting first.
func lineValue(line *float64) float64 {
	if line == nil {
		return math.Inf(-1)
	}
	return *line
}

// playerGroups folds prop rows into one card per player, clustering a player's
// markets under their name.
func playerGroups(game db.Game, rows []Row, mode Sort, signals BoardSignals) []PlayerGroup {
	order := make([]string, 0, len(rows))
	byPlayer := make(map[string][]Row, len(rows))
	for _, row := range rows {
		key := playerKey(row)
		if _, ok := byPlayer[key]; !ok {
			order = append(order, key)
		}
		byPlayer[key] = append(byPlayer[key], row)
	}

	players := make([]PlayerGroup, 0, len(order))
	for _, key := range order {
		playerRows := byPlayer[key]
		first := playerRows[0]
		players = append(players, PlayerGroup{
			Name:        first.PlayerName,
			PlayerID:    first.PlayerID,
			Position:    first.Position,
			HeadshotURL: first.HeadshotURL,
			Team:        first.Team,
			Tags:        playerTags(game, playerRows, signals),
			Lines:       lineGroups(game, playerRows, mode, signals),
		})
	}
	sortPlayers(game, players, mode, signals)
	return players
}

// playerKey identifies one player's card. A roster ID wins; otherwise the name
// and team keep two players with the same name apart.
func playerKey(row Row) string {
	return canonicalPlayerKey(row.PlayerID, row.PlayerName, row.Team)
}

// canonicalPlayerKey identifies one player across raw name spellings. A roster
// ID wins; otherwise the normalized name and team keep two players with the
// same name apart.
func canonicalPlayerKey(playerID, playerName, team string) string {
	if playerID != "" {
		return "id:" + playerID
	}
	return "name:" + normalize(playerName) + "|" + team
}

// sortPlayers orders player cards for the sort, falling back to the player's
// name.
func sortPlayers(game db.Game, players []PlayerGroup, mode Sort, signals BoardSignals) {
	sort.SliceStable(players, func(i, j int) bool {
		left, right := players[i], players[j]
		switch mode {
		case SortOdds:
			if a, b := playerPrice(left), playerPrice(right); a != b {
				return a < b
			}
		case SortLine:
			if a, b := playerLine(left), playerLine(right); a != b {
				return a > b
			}
		case SortMatchup:
			if a, b := playerSignalScore(game, left, signals), playerSignalScore(game, right, signals); a != b {
				return a > b
			}
		}
		return playerLess(left, right)
	})
}

// playerLess orders players by name, with an unnamed card last.
func playerLess(left, right PlayerGroup) bool {
	if left.Name != right.Name {
		if left.Name == "" {
			return false
		}
		if right.Name == "" {
			return true
		}
		return left.Name < right.Name
	}
	if left.Team != right.Team {
		return left.Team < right.Team
	}
	return left.PlayerID < right.PlayerID
}

// playerPrice is a player's best price across their lines.
func playerPrice(player PlayerGroup) float64 {
	best := math.Inf(1)
	for _, line := range player.Lines {
		if price := shortestPrice(line); price < best {
			best = price
		}
	}
	return best
}

// playerLine is a player's largest line across their markets.
func playerLine(player PlayerGroup) float64 {
	highest := math.Inf(-1)
	for _, line := range player.Lines {
		if guess := highestLine(line); guess > highest {
			highest = guess
		}
	}
	return highest
}

// gameLines folds one game's built rows into its spread, total, and moneyline
// summaries. A market with no stored row stays nil.
func gameLines(game db.Game, rows []Row) GameLines {
	var lines GameLines
	for _, row := range rows {
		switch row.Market {
		case "SPREAD":
			switch row.Team {
			case game.AwayTeam:
				if lines.Spread == nil {
					lines.Spread = &LineSummary{}
				}
				lines.Spread.AwayLine = spreadLine(row.Line)
				lines.Spread.AwayOdds = priceText(row.Odds)
			case game.HomeTeam:
				if lines.Spread == nil {
					lines.Spread = &LineSummary{}
				}
				lines.Spread.HomeLine = spreadLine(row.Line)
				lines.Spread.HomeOdds = priceText(row.Odds)
			}
		case "ML":
			switch row.Team {
			case game.AwayTeam:
				if lines.Moneyline == nil {
					lines.Moneyline = &LineSummary{}
				}
				lines.Moneyline.AwayOdds = priceText(row.Odds)
			case game.HomeTeam:
				if lines.Moneyline == nil {
					lines.Moneyline = &LineSummary{}
				}
				lines.Moneyline.HomeOdds = priceText(row.Odds)
			}
		case "TOTAL":
			if lines.Total == nil {
				lines.Total = &LineSummary{}
			}
			if row.Line != nil {
				lines.Total.Line = formatLine(*row.Line)
			}
			switch row.Side {
			case "OVER":
				lines.Total.OverOdds = priceText(row.Odds)
			case "UNDER":
				lines.Total.UnderOdds = priceText(row.Odds)
			}
		}
	}
	return lines
}

// propCount counts a game's prop rows, the same rows the props sync owns.
func propCount(rows []Row) int {
	count := 0
	for _, row := range rows {
		if row.Category != CategoryLines {
			count++
		}
	}
	return count
}

// spreadLine prints a spread with its sign, or nothing when the line is
// missing.
func spreadLine(line *float64) string {
	if line == nil {
		return ""
	}
	return signedSpread(*line)
}

// priceText prints an American price with a plus sign when positive. A zero
// price reads as a missing one.
func priceText(odds int) string {
	if odds == 0 {
		return ""
	}
	if odds > 0 {
		return "+" + strconv.Itoa(odds)
	}
	return strconv.Itoa(odds)
}

// rowMatches reports whether needle appears in a row's text, teams, player, or
// matchup. needle is already normalized.
func rowMatches(row Row, game db.Game, needle string) bool {
	haystack := []string{
		row.Text,
		row.Team,
		row.PlayerName,
		game.AwayTeam,
		game.HomeTeam,
		game.AwayName,
		game.HomeName,
		game.AwayTeam + " @ " + game.HomeTeam,
		game.AwayName + " at " + game.HomeName,
	}
	for _, part := range haystack {
		if part != "" && strings.Contains(normalize(part), needle) {
			return true
		}
	}
	return false
}

// gameMatchesQuery reports whether an empty query, a team, a venue, or a
// roster player makes a game worth showing.
func gameMatchesQuery(game db.Game, index *rosterIndex, needle string) bool {
	if needle == "" {
		return true
	}
	for _, part := range []string{
		game.AwayTeam, game.HomeTeam, game.AwayName, game.HomeName,
		db.StringValue(game.VenueName), db.StringValue(game.VenueID),
		game.AwayTeam + " @ " + game.HomeTeam,
		game.AwayName + " at " + game.HomeName,
	} {
		if part != "" && strings.Contains(normalize(part), needle) {
			return true
		}
	}
	if index == nil {
		return false
	}
	for _, team := range [2]string{game.AwayTeam, game.HomeTeam} {
		for _, entry := range index.byGameTeam[team] {
			if entry.normFull == "" {
				continue
			}
			if strings.Contains(entry.normFull, needle) || strings.Contains(needle, entry.normFull) {
				return true
			}
		}
	}
	return false
}

// sortRows orders rows by kickoff, then by the stable tiebreaks a list needs.
func sortRows(rows []Row) {
	sort.SliceStable(rows, func(i, j int) bool {
		left, right := rows[i], rows[j]
		if !left.Kickoff.Equal(right.Kickoff) {
			return left.Kickoff.Before(right.Kickoff)
		}
		if left.GameID != right.GameID {
			return left.GameID < right.GameID
		}
		if categoryOrder(left.Category) != categoryOrder(right.Category) {
			return categoryOrder(left.Category) < categoryOrder(right.Category)
		}
		return left.Text < right.Text
	})
}

// categoryOrder is a category's position in the chip list.
func categoryOrder(category Category) int {
	for i, known := range Categories {
		if known == category {
			return i
		}
	}
	return len(Categories)
}

// normalize delegates to db.NormalizePlayerName so board matching and ingest
// share one canonical normalizer.
func normalize(value string) string {
	return db.NormalizePlayerName(value)
}

// signedSpread prints a spread with a plus on a positive number.
func signedSpread(line float64) string {
	if line > 0 {
		return "+" + formatLine(line)
	}
	return formatLine(line)
}

// formatLine prints a line without a trailing zero.
func formatLine(line float64) string {
	return strconv.FormatFloat(line, 'g', -1, 64)
}
