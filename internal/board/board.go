// Package board turns stored odds into the searchable rows the pick screen
// shows. It reads nothing itself, so its rules stay easy to test.
package board

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	// Embed the zone database so America/New_York resolves in a container
	// without a separate tzdata package.
	_ "time/tzdata"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
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
	WindowSundayMorning Window = "sun_morning"
	WindowSundayEarly   Window = "sun_early"
	WindowSundayLate    Window = "sun_late"
	WindowSundayNight   Window = "sun_night"
	WindowMonday        Window = "monday"
)

// Windows lists the window chips in display order.
var Windows = []Window{
	WindowSundayMorning,
	WindowSundayEarly,
	WindowSundayLate,
	WindowSundayNight,
	WindowMonday,
}

// Label is the chip text.
func (w Window) Label() string {
	switch w {
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
	SortKickoff  Sort = "kickoff"
	SortPlayer   Sort = "player"
	SortCategory Sort = "category"
	SortOdds     Sort = "odds"
)

// Sorts lists the sort chips in display order.
var Sorts = []Sort{SortKickoff, SortPlayer, SortCategory, SortOdds}

// Label is the chip text.
func (s Sort) Label() string {
	switch s {
	case SortKickoff:
		return "Kickoff"
	case SortPlayer:
		return "Player"
	case SortCategory:
		return "Category"
	case SortOdds:
		return "Odds"
	default:
		return string(s)
	}
}

// Row is one selection a person can add to their slip. The fields after Text
// are what the add-leg form posts.
type Row struct {
	GameID     string
	Market     string
	Team       string
	PlayerName string
	PlayerID   string
	Side       string
	Line       *float64
	Odds       int
	Category   Category
	Window     Window
	Kickoff    time.Time
	Text       string
	Picked     int64
}

// UnloadedGame is a game whose props were never fetched, offered as a prompt.
type UnloadedGame struct {
	GameID  string
	Matchup string
	Kickoff time.Time
}

// Query is one search, filter, and sort request.
type Query struct {
	Q        string
	Category Category
	Window   Window
	Sort     Sort
	GameID   string
	// Now is the app clock. A zero time uses time.Now.
	Now time.Time
}

// Result is the filtered rows and the prompts around them.
type Result struct {
	Rows     []Row
	Unloaded []UnloadedGame
	Started  int
}

// eastern is the zone the kickoff windows use.
var eastern = mustLoadLocation("America/New_York")

func mustLoadLocation(name string) *time.Location {
	location, err := time.LoadLocation(name)
	if err != nil {
		panic(fmt.Sprintf("load location %s: %v", name, err))
	}
	return location
}

// IsPropRow reports whether an odds row belongs to the prop sync group.
func IsPropRow(row db.Odds) bool {
	return row.MarketType == "TEAM_TOTAL" || row.PlayerName != "" || isPlayerMarket(row.MarketType)
}

// isPlayerMarket reports whether a market type targets an athlete rather than
// a game or team.
func isPlayerMarket(market string) bool {
	switch market {
	case "PASS_YDS", "PASS_TDS", "RUSH_YDS", "RUSH_ATT", "REC_YDS", "RECEPTIONS", "ANYTIME_TD", "KICKER_PTS":
		return true
	default:
		return false
	}
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

	type group struct {
		gameID, market, team, player, side string
		rows                               []db.Odds
	}
	groups := make(map[string]*group)
	order := make([]string, 0, len(odds))
	for _, row := range odds {
		if _, ok := byGame[row.GameID]; !ok {
			continue
		}
		key := strings.Join(
			[]string{row.GameID, row.MarketType, row.Team, row.PlayerName, row.Side}, "\x00")
		found, ok := groups[key]
		if !ok {
			found = &group{
				gameID: row.GameID,
				market: row.MarketType,
				team:   row.Team,
				player: row.PlayerName,
				side:   row.Side,
			}
			groups[key] = found
			order = append(order, key)
		}
		found.rows = append(found.rows, row)
	}

	rows := make([]Row, 0, len(groups))
	for _, key := range order {
		group := groups[key]
		game := byGame[group.gameID]
		line, price := selectionPrice(group.rows, homeBook)
		playerID := matchPlayerID(roster, group.player, group.team)
		rows = append(rows, Row{
			GameID:     group.gameID,
			Market:     group.market,
			Team:       group.team,
			PlayerName: group.player,
			PlayerID:   playerID,
			Side:       group.side,
			Line:       line,
			Odds:       price,
			Category:   categoryFor(group.market),
			Window:     windowFor(game.Kickoff),
			Kickoff:    game.Kickoff,
			Text:       rowText(group.market, group.team, group.player, group.side, line),
		})
	}

	for _, leg := range mine {
		for i := range rows {
			if rows[i].Picked != 0 {
				continue
			}
			if rows[i].MatchesLeg(leg) {
				rows[i].Picked = leg.ID
				break
			}
		}
	}

	sortRows(rows, SortKickoff)
	return rows
}

// MatchesLeg reports whether the row's selection is the leg's pick.
func (r Row) MatchesLeg(leg db.Leg) bool {
	return r.GameID == db.StringValue(leg.GameID) &&
		r.Market == leg.MarketType &&
		r.Team == db.StringValue(leg.Team) &&
		r.PlayerName == db.StringValue(leg.PlayerName) &&
		r.Side == db.StringValue(leg.Side) &&
		lineEqual(r.Line, leg.Line)
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

// windowFor maps a kickoff to its Eastern-time window chip. Monday covers the
// whole day, so a Monday-night game that lands on Tuesday in UTC still reads
// Monday here.
func windowFor(kickoff time.Time) Window {
	easternTime := kickoff.In(eastern)
	if easternTime.Weekday() == time.Monday {
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

// rowText is the full label a row shows.
func rowText(market, team, player, side string, line *float64) string {
	signed := ""
	if line != nil {
		signed = formatLine(*line)
	}
	switch market {
	case "SPREAD":
		if line == nil {
			return team
		}
		return team + " " + signedSpread(*line)
	case "ML":
		return team + " ML"
	case "TOTAL":
		if line == nil {
			return titleSide(side)
		}
		return titleSide(side) + " " + signed
	case "TEAM_TOTAL":
		text := team + " Team Total " + titleSide(side)
		if line != nil {
			text += " " + signed
		}
		return strings.TrimSpace(text)
	case "ANYTIME_TD":
		return player + " Anytime TD"
	default:
		label := marketLabel(market)
		text := player + " " + titleSide(side)
		if line != nil {
			text += " " + signed
		}
		if label != "" {
			text += " " + label
		}
		return strings.TrimSpace(text)
	}
}

// marketLabel is the stat name a prop row's text uses.
func marketLabel(market string) string {
	switch market {
	case "PASS_YDS":
		return "Pass Yds"
	case "PASS_TDS":
		return "Pass TDs"
	case "RUSH_YDS":
		return "Rush Yds"
	case "RUSH_ATT":
		return "Rush Att"
	case "REC_YDS":
		return "Rec Yds"
	case "RECEPTIONS":
		return "Receptions"
	case "KICKER_PTS":
		return "Kicking Pts"
	default:
		return ""
	}
}

// matchPlayerID finds the roster athlete for a prop row's player name and team.
// A name on the row's own team wins over the same name on another team.
func matchPlayerID(roster []db.RosterPlayer, name, team string) string {
	if name == "" {
		return ""
	}
	want := normalize(name)
	var fallback string
	for _, player := range roster {
		if normalize(player.FullName) != want && normalize(player.NormName) != want {
			continue
		}
		if team == "" || player.Team == team {
			return player.AthleteID
		}
		if fallback == "" {
			fallback = player.AthleteID
		}
	}
	return fallback
}

// Search filters and sorts built rows. It also names games whose props were
// never fetched when the query matches their team or a roster player on it.
func Search(rows []Row, games []db.Game, roster []db.RosterPlayer, q Query) Result {
	now := q.Now
	if now.IsZero() {
		now = time.Now()
	}

	byGame := make(map[string]db.Game, len(games))
	for _, game := range games {
		byGame[game.ID] = game
	}

	result := Result{Started: 0}
	for _, game := range games {
		if !now.Before(game.Kickoff) {
			result.Started++
		}
	}

	needle := normalize(q.Q)
	hasProps := make(map[string]bool, len(games))
	for _, row := range rows {
		if row.Category != CategoryLines {
			hasProps[row.GameID] = true
		}
	}

	for _, row := range rows {
		if !now.Before(row.Kickoff) {
			continue
		}
		if q.Category != "" && row.Category != q.Category {
			continue
		}
		if q.Window != "" && row.Window != q.Window {
			continue
		}
		if q.GameID != "" && row.GameID != q.GameID {
			continue
		}
		if needle != "" && !rowMatches(row, byGame[row.GameID], needle) {
			continue
		}
		result.Rows = append(result.Rows, row)
	}

	for _, game := range games {
		if !now.Before(game.Kickoff) || hasProps[game.ID] {
			continue
		}
		if !gameMatchesQuery(game, roster, needle) {
			continue
		}
		result.Unloaded = append(result.Unloaded, UnloadedGame{
			GameID:  game.ID,
			Matchup: game.AwayTeam + " @ " + game.HomeTeam,
			Kickoff: game.Kickoff,
		})
	}

	sortRows(result.Rows, q.Sort)
	sort.SliceStable(result.Unloaded, func(i, j int) bool {
		if !result.Unloaded[i].Kickoff.Equal(result.Unloaded[j].Kickoff) {
			return result.Unloaded[i].Kickoff.Before(result.Unloaded[j].Kickoff)
		}
		return result.Unloaded[i].GameID < result.Unloaded[j].GameID
	})
	return result
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

// gameMatchesQuery reports whether an empty query, a team, or a roster player
// makes a game worth naming as unloaded.
func gameMatchesQuery(game db.Game, roster []db.RosterPlayer, needle string) bool {
	if needle == "" {
		return true
	}
	for _, part := range []string{
		game.AwayTeam, game.HomeTeam, game.AwayName, game.HomeName,
		game.AwayTeam + " @ " + game.HomeTeam,
		game.AwayName + " at " + game.HomeName,
	} {
		if part != "" && strings.Contains(normalize(part), needle) {
			return true
		}
	}
	for _, player := range roster {
		if player.Team != game.AwayTeam && player.Team != game.HomeTeam {
			continue
		}
		name := normalize(player.FullName)
		if strings.Contains(name, needle) || strings.Contains(needle, name) {
			return true
		}
	}
	return false
}

// sortRows orders rows by sort, defaulting to kickoff.
func sortRows(rows []Row, mode Sort) {
	sort.SliceStable(rows, func(i, j int) bool {
		left, right := rows[i], rows[j]
		switch mode {
		case SortPlayer:
			if left.PlayerName != right.PlayerName {
				if left.PlayerName == "" {
					return false
				}
				if right.PlayerName == "" {
					return true
				}
				return left.PlayerName < right.PlayerName
			}
		case SortCategory:
			if left.Category != right.Category {
				return categoryOrder(left.Category) < categoryOrder(right.Category)
			}
		case SortOdds:
			leftDecimal := parlay.AmericanToDecimal(left.Odds)
			rightDecimal := parlay.AmericanToDecimal(right.Odds)
			if leftDecimal != rightDecimal {
				return leftDecimal < rightDecimal
			}
		}
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

// normalize folds accents, lowercases, and strips punctuation so a search is
// case- and accent-insensitive.
func normalize(value string) string {
	return db.NormalizePlayerName(foldAccents(value))
}

// foldAccents maps the common Latin accented letters to their plain form.
func foldAccents(value string) string {
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		b.WriteRune(plainRune(r))
	}
	return b.String()
}

// plainRune is the unaccented form of a letter.
func plainRune(r rune) rune {
	switch r {
	case 'á', 'à', 'â', 'ä', 'ã', 'å', 'Á', 'À', 'Â', 'Ä', 'Ã', 'Å':
		return 'a'
	case 'ç', 'Ç':
		return 'c'
	case 'é', 'è', 'ê', 'ë', 'É', 'È', 'Ê', 'Ë':
		return 'e'
	case 'í', 'ì', 'î', 'ï', 'Í', 'Ì', 'Î', 'Ï':
		return 'i'
	case 'ñ', 'Ñ':
		return 'n'
	case 'ó', 'ò', 'ô', 'ö', 'õ', 'Ó', 'Ò', 'Ô', 'Ö', 'Õ':
		return 'o'
	case 'ú', 'ù', 'û', 'ü', 'Ú', 'Ù', 'Û', 'Ü':
		return 'u'
	case 'ý', 'ÿ', 'Ý', 'Ÿ':
		return 'y'
	default:
		return r
	}
}

// titleSide turns OVER into "Over".
func titleSide(side string) string {
	if side == "" {
		return ""
	}
	return strings.ToUpper(side[:1]) + strings.ToLower(side[1:])
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

// lineEqual compares two optional lines exactly.
func lineEqual(left, right *float64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
