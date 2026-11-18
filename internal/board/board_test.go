package board_test

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/board"
	"github.com/kalverra/parlay-club/internal/db"
)

// eastern is the zone the kickoff windows use.
var eastern = mustLoad("America/New_York")

func mustLoad(name string) *time.Location {
	location, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return location
}

// at builds an Eastern wall-clock time in 2026.
func at(t *testing.T, day, hour, minute int) time.Time {
	t.Helper()
	return time.Date(2026, time.November, day, hour, minute, 0, 0, eastern)
}

// game builds one stored game.
func game(id string, kickoff time.Time) db.Game {
	return db.Game{
		ID:       id,
		AwayTeam: "MIA",
		HomeTeam: "BUF",
		AwayName: "Miami Dolphins",
		HomeName: "Buffalo Bills",
		Kickoff:  kickoff,
		Status:   db.GameScheduled,
	}
}

// odds builds one stored price.
func odds(gameID, book, market, team, player, side string, line *float64, price int) db.Odds {
	return db.Odds{
		GameID:     gameID,
		Book:       book,
		MarketType: market,
		Team:       team,
		PlayerName: player,
		Side:       side,
		Line:       line,
		Price:      price,
		FetchedAt:  time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC),
	}
}

func TestBuildMapsCategoriesWindowsAndText(t *testing.T) {
	t.Parallel()

	thursday := at(t, 5, 20, 15)
	sunday := at(t, 8, 13, 0)
	monday := at(t, 9, 20, 15)
	games := []db.Game{
		game("thu", thursday),
		game("sun", sunday),
		game("mon", monday),
	}
	rows := []db.Odds{
		odds("thu", "draftkings", "ML", "BAL", "", "TEAM", nil, -130),
		odds("sun", "draftkings", "SPREAD", "BUF", "", "TEAM", new(-3.0), -110),
		odds("sun", "draftkings", "ML", "BUF", "", "TEAM", nil, -150),
		odds("sun", "draftkings", "TOTAL", "", "", "OVER", new(47.5), -110),
		odds("sun", "draftkings", "TEAM_TOTAL", "BUF", "", "OVER", new(24.5), -110),
		odds("sun", "draftkings", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
		odds("sun", "draftkings", "RUSH_YDS", "BUF", "James Cook", "OVER", new(64.5), -110),
		odds("sun", "draftkings", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(52.5), -110),
		odds("sun", "draftkings", "ANYTIME_TD", "BUF", "James Cook", "YES", nil, 150),
		odds("sun", "draftkings", "KICKER_PTS", "BUF", "Tyler Bass", "OVER", new(7.5), -110),
		odds("mon", "draftkings", "ML", "MIA", "", "TEAM", nil, 130),
	}

	built := board.Build(games, rows, nil, "", nil, thursday.Add(-24*time.Hour))
	require.Len(t, built, 11)

	byText := make(map[string]board.Row, len(built))
	for _, row := range built {
		byText[row.Text] = row
	}
	assert.Equal(t, board.CategoryLines, byText["BAL ML"].Category)
	assert.Equal(t, board.CategoryLines, byText["BUF -3"].Category)
	assert.Equal(t, board.CategoryLines, byText["BUF ML"].Category)
	assert.Equal(t, board.CategoryLines, byText["Over 47.5"].Category)
	assert.Equal(t, board.CategoryTeamTotals, byText["BUF Team Total Over 24.5"].Category)
	assert.Equal(t, board.CategoryPassing, byText["Josh Allen Over 245.5 Pass Yds"].Category)
	assert.Equal(t, board.CategoryRushing, byText["James Cook Over 64.5 Rush Yds"].Category)
	assert.Equal(t, board.CategoryReceiving, byText["Khalil Shakir Over 52.5 Rec Yds"].Category)
	assert.Equal(t, board.CategoryTDs, byText["James Cook Anytime TD"].Category)
	assert.Equal(t, board.CategoryKicking, byText["Tyler Bass Over 7.5 Kicking Pts"].Category)

	assert.Equal(t, board.WindowThursday, byText["BAL ML"].Window,
		"a Thursday-night kickoff that lands on Friday in UTC still reads Thursday")
	assert.Equal(t, thursday.UTC(), byText["BAL ML"].Kickoff.UTC())
	assert.Equal(t, board.WindowSundayEarly, byText["BUF ML"].Window)
	assert.Equal(t, board.WindowMonday, byText["MIA ML"].Window,
		"a Monday-night kickoff that lands on Tuesday in UTC still reads Monday")
	assert.Equal(t, monday.UTC(), byText["MIA ML"].Kickoff.UTC())
}

func TestWindowsSplitTheDay(t *testing.T) {
	t.Parallel()

	games := []db.Game{
		game("thursday", at(t, 5, 20, 15)),
		game("morning", at(t, 8, 9, 30)),
		game("early", at(t, 8, 13, 0)),
		game("late", at(t, 8, 16, 25)),
		game("night", at(t, 8, 20, 20)),
		game("monday", at(t, 9, 13, 0)),
	}
	var rows []db.Odds
	for _, game := range games {
		rows = append(rows, odds(game.ID, "draftkings", "ML", "BUF", "", "TEAM", nil, -110))
	}

	built := board.Build(games, rows, nil, "", nil, at(t, 4, 12, 0))
	want := map[string]board.Window{
		"thursday": board.WindowThursday,
		"morning":  board.WindowSundayMorning,
		"early":    board.WindowSundayEarly,
		"late":     board.WindowSundayLate,
		"night":    board.WindowSundayNight,
		"monday":   board.WindowMonday,
	}
	for _, row := range built {
		assert.Equal(t, want[row.GameID], row.Window, row.GameID)
	}
}

func TestWindowChipsAndLabels(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []board.Window{
		board.WindowThursday,
		board.WindowSundayMorning,
		board.WindowSundayEarly,
		board.WindowSundayLate,
		board.WindowSundayNight,
		board.WindowMonday,
	}, board.Windows)

	assert.Equal(t, "Thursday", board.WindowThursday.Label())
	assert.Equal(t, "Sun morning", board.WindowSundayMorning.Label())
	assert.Equal(t, "Sun early", board.WindowSundayEarly.Label())
	assert.Equal(t, "Sun late", board.WindowSundayLate.Label())
	assert.Equal(t, "Sun night", board.WindowSundayNight.Label())
	assert.Equal(t, "Monday", board.WindowMonday.Label())
	assert.Equal(t, "other", board.Window("other").Label())
}

func TestStartedGamesAreHiddenAndCounted(t *testing.T) {
	t.Parallel()

	started := game("started", at(t, 8, 13, 0))
	upcoming := game("upcoming", at(t, 8, 16, 0))
	games := []db.Game{started, upcoming}
	rows := []db.Odds{
		odds("started", "draftkings", "ML", "BUF", "", "TEAM", nil, -110),
		odds("upcoming", "draftkings", "ML", "BUF", "", "TEAM", nil, -110),
	}
	now := at(t, 8, 13, 30)
	built := board.Build(games, rows, nil, "", nil, now)
	require.Len(t, built, 1, "the started game's rows are not built")

	cards := board.Slates(games, built, nil, board.Query{Now: now})
	require.Len(t, cards, 1)
	assert.Equal(t, "upcoming", cards[0].ID)
	assert.Equal(t, 1, board.StartedCount(games, now))
}

func TestHomeBookPriceWinsOverMedian(t *testing.T) {
	t.Parallel()

	game := game("401", at(t, 8, 13, 0))
	games := []db.Game{game}
	rows := []db.Odds{
		odds("401", "draftkings", "ML", "BUF", "", "TEAM", nil, -110),
		odds("401", "fanduel", "ML", "BUF", "", "TEAM", nil, -105),
	}
	built := board.Build(games, rows, nil, "fanduel", nil, at(t, 7, 12, 0))
	require.Len(t, built, 1)
	assert.Equal(t, -105, built[0].Odds)
}

func TestPickedRowsCarryTheLegID(t *testing.T) {
	t.Parallel()

	game := game("401", at(t, 8, 13, 0))
	games := []db.Game{game}
	rows := []db.Odds{
		odds("401", "draftkings", "ML", "BUF", "", "TEAM", nil, -110),
	}
	mine := []db.Leg{{
		ID:         7,
		GameID:     new("401"),
		MarketType: "ML",
		Team:       new("BUF"),
		Side:       new("TEAM"),
		Odds:       -110,
	}}
	built := board.Build(games, rows, nil, "", mine, at(t, 7, 12, 0))
	require.Len(t, built, 1)
	assert.Equal(t, int64(7), built[0].Picked)
}

func TestBuildCopiesPositionOntoPropRows(t *testing.T) {
	t.Parallel()

	games := []db.Game{game("401", at(t, 8, 13, 0))}
	rows := []db.Odds{
		odds("401", "draftkings", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
		odds("401", "draftkings", "ML", "BUF", "", "TEAM", nil, -150),
	}
	roster := []db.RosterPlayer{
		{AthleteID: "1", Team: "BUF", FullName: "Josh Allen", Position: "QB"},
	}
	built := board.Build(games, rows, roster, "", nil, at(t, 7, 12, 0))
	require.Len(t, built, 2)

	byText := make(map[string]board.Row, len(built))
	for _, row := range built {
		byText[row.Text] = row
	}
	prop := byText["Josh Allen Over 245.5 Pass Yds"]
	assert.Equal(t, "1", prop.PlayerID)
	assert.Equal(t, "QB", prop.Position)
	assert.Empty(t, byText["BUF ML"].Position)
}

func TestSlatesOrderByKickoff(t *testing.T) {
	t.Parallel()

	games := []db.Game{
		game("monday", at(t, 9, 20, 15)),
		game("late", at(t, 8, 16, 25)),
		game("early", at(t, 8, 13, 0)),
	}
	rows := []db.Odds{
		odds("monday", "draftkings", "ML", "BUF", "", "TEAM", nil, -110),
		odds("late", "draftkings", "ML", "BUF", "", "TEAM", nil, -110),
		odds("early", "draftkings", "ML", "BUF", "", "TEAM", nil, -110),
	}
	now := at(t, 7, 12, 0)
	built := board.Build(games, rows, nil, "", nil, now)

	cards := board.Slates(games, built, nil, board.Query{Now: now})
	require.Len(t, cards, 3)
	assert.Equal(t, []string{"early", "late", "monday"},
		[]string{cards[0].ID, cards[1].ID, cards[2].ID}, "the earliest kickoff comes first")
	assert.Equal(t, "MIA @ BUF", cards[0].Matchup)
	assert.Equal(t, "MIA", cards[0].Away)
	assert.Equal(t, "BUF", cards[0].Home)
	assert.Equal(t, at(t, 8, 13, 0).UTC(), cards[0].Kickoff.UTC())
	assert.Nil(t, cards[0].Lines.Spread, "a game with only a moneyline has no spread")
	assert.Nil(t, cards[0].Lines.Total)
	assert.Equal(t, "-110", cards[0].Lines.Moneyline.HomeOdds)
	assert.Empty(t, cards[0].Lines.Moneyline.AwayOdds, "an unpriced side stays blank")
	assert.False(t, cards[0].HasProps)
}

func TestSlatesSummarizeTheGameLines(t *testing.T) {
	t.Parallel()

	g := game("401", at(t, 8, 13, 0))
	g.VenueName = new("Highmark Stadium")
	games := []db.Game{g}
	rows := []db.Odds{
		odds("401", "draftkings", "SPREAD", "MIA", "", "TEAM", new(3.0), -105),
		odds("401", "draftkings", "SPREAD", "BUF", "", "TEAM", new(-3.0), -115),
		odds("401", "draftkings", "TOTAL", "", "", "OVER", new(47.5), -110),
		odds("401", "draftkings", "TOTAL", "", "", "UNDER", new(47.5), -115),
		odds("401", "draftkings", "ML", "MIA", "", "TEAM", nil, 130),
		odds("401", "draftkings", "ML", "BUF", "", "TEAM", nil, -150),
	}
	now := at(t, 7, 12, 0)
	built := board.Build(games, rows, nil, "", nil, now)

	cards := board.Slates(games, built, nil, board.Query{Now: now})
	require.Len(t, cards, 1)
	card := cards[0]
	assert.Equal(t, "Highmark Stadium", card.Venue)

	require.NotNil(t, card.Lines.Spread)
	assert.Equal(t, "+3", card.Lines.Spread.AwayLine)
	assert.Equal(t, "-105", card.Lines.Spread.AwayOdds)
	assert.Equal(t, "-3", card.Lines.Spread.HomeLine)
	assert.Equal(t, "-115", card.Lines.Spread.HomeOdds)

	require.NotNil(t, card.Lines.Total)
	assert.Equal(t, "47.5", card.Lines.Total.Line)
	assert.Equal(t, "-110", card.Lines.Total.OverOdds)
	assert.Equal(t, "-115", card.Lines.Total.UnderOdds)

	require.NotNil(t, card.Lines.Moneyline)
	assert.Equal(t, "+130", card.Lines.Moneyline.AwayOdds)
	assert.Equal(t, "-150", card.Lines.Moneyline.HomeOdds)
	assert.Empty(t, card.Lines.Moneyline.AwayLine)
}

func TestSlatesCountPropRows(t *testing.T) {
	t.Parallel()

	games := []db.Game{
		game("401", at(t, 8, 13, 0)),
		game("402", at(t, 8, 16, 0)),
	}
	rows := []db.Odds{
		odds("401", "draftkings", "ML", "BUF", "", "TEAM", nil, -150),
		odds("401", "draftkings", "TEAM_TOTAL", "BUF", "", "OVER", new(24.5), -110),
		odds("401", "draftkings", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
		odds("401", "draftkings", "ANYTIME_TD", "BUF", "James Cook", "YES", nil, 150),
		odds("402", "draftkings", "ML", "MIA", "", "TEAM", nil, 130),
	}
	now := at(t, 7, 12, 0)
	built := board.Build(games, rows, nil, "", nil, now)

	cards := board.Slates(games, built, nil, board.Query{Now: now})
	require.Len(t, cards, 2)

	byID := make(map[string]board.GameCard, len(cards))
	for _, card := range cards {
		byID[card.ID] = card
	}
	assert.Equal(t, 3, byID["401"].Props, "team totals and player markets count as props")
	assert.True(t, byID["401"].HasProps)
	assert.Zero(t, byID["402"].Props)
	assert.False(t, byID["402"].HasProps)
}

func TestSlatesFilterByWindowAndQuery(t *testing.T) {
	t.Parallel()

	thu := game("thu", at(t, 5, 20, 15))
	thu.AwayTeam = "BAL"
	thu.HomeTeam = "JAX"
	thu.AwayName = "Baltimore Ravens"
	thu.HomeName = "Jacksonville Jaguars"
	early := game("early", at(t, 8, 13, 0))
	early.VenueName = new("Highmark Stadium")
	late := game("late", at(t, 8, 16, 25))
	late.HomeTeam = "KC"
	late.HomeName = "Kansas City Chiefs"
	games := []db.Game{thu, early, late}
	rows := []db.Odds{
		odds("thu", "draftkings", "ML", "BAL", "", "TEAM", nil, -150),
		odds("early", "draftkings", "ML", "BUF", "", "TEAM", nil, -150),
		odds("late", "draftkings", "ML", "KC", "", "TEAM", nil, -150),
	}
	roster := []db.RosterPlayer{
		{AthleteID: "1", Team: "BUF", FullName: "Josh Allen", Position: "QB"},
	}
	now := at(t, 4, 12, 0)
	built := board.Build(games, rows, roster, "", nil, now)

	windowThu := board.Slates(games, built, roster, board.Query{
		Window: board.WindowThursday,
		Now:    now,
	})
	require.Len(t, windowThu, 1)
	assert.Equal(t, "thu", windowThu[0].ID)

	window := board.Slates(games, built, roster, board.Query{
		Window: board.WindowSundayLate,
		Now:    now,
	})
	require.Len(t, window, 1)
	assert.Equal(t, "late", window[0].ID)

	byTeam := board.Slates(games, built, roster, board.Query{Q: "Buffalo", Now: now})
	require.Len(t, byTeam, 1)
	assert.Equal(t, "early", byTeam[0].ID)

	byMatchup := board.Slates(games, built, roster, board.Query{Q: "MIA @ BUF", Now: now})
	require.Len(t, byMatchup, 1)
	assert.Equal(t, "early", byMatchup[0].ID)

	byNames := board.Slates(games, built, roster, board.Query{Q: "Dolphins at Buffalo", Now: now})
	require.Len(t, byNames, 1)
	assert.Equal(t, "early", byNames[0].ID)

	byVenue := board.Slates(games, built, roster, board.Query{Q: "highmark", Now: now})
	require.Len(t, byVenue, 1)
	assert.Equal(t, "early", byVenue[0].ID)

	byPlayer := board.Slates(games, built, roster, board.Query{Q: "JOSH ALLEN", Now: now})
	require.Len(t, byPlayer, 1)
	assert.Equal(t, "early", byPlayer[0].ID)

	noMatch := board.Slates(games, built, roster, board.Query{Q: "Seattle", Now: now})
	assert.Empty(t, noMatch)
}

func TestSlatesDropKickedOffGames(t *testing.T) {
	t.Parallel()

	games := []db.Game{
		game("started", at(t, 8, 13, 0)),
		game("upcoming", at(t, 8, 16, 0)),
	}
	rows := []db.Odds{
		odds("started", "draftkings", "ML", "BUF", "", "TEAM", nil, -110),
		odds("upcoming", "draftkings", "ML", "BUF", "", "TEAM", nil, -110),
	}
	now := at(t, 8, 13, 30)
	built := board.Build(games, rows, nil, "", nil, now)

	cards := board.Slates(games, built, nil, board.Query{Now: now})
	require.Len(t, cards, 1)
	assert.Equal(t, "upcoming", cards[0].ID)
}

func TestDetailOrdersTabsAndFoldsTeamTotals(t *testing.T) {
	t.Parallel()

	g := game("401", at(t, 8, 13, 0))
	games := []db.Game{g}
	rows := []db.Odds{
		odds("401", "draftkings", "ML", "BUF", "", "TEAM", nil, -150),
		odds("401", "draftkings", "TEAM_TOTAL", "BUF", "", "OVER", new(24.5), -110),
		odds("401", "draftkings", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
		odds("401", "draftkings", "KICKER_PTS", "BUF", "Tyler Bass", "OVER", new(7.5), -110),
	}
	now := at(t, 7, 12, 0)
	built := board.Build(games, rows, nil, "", nil, now)

	detail := board.Detail(g, built, board.Query{Now: now})
	assert.Equal(t, board.CategoryLines, detail.Category, "an empty category opens Lines")
	assert.Equal(t, board.SortPlayer, detail.Sort, "player is the default detail sort")
	assert.Equal(t, "401", detail.Game.ID)
	assert.Equal(t, "MIA @ BUF", detail.Game.Matchup)

	categories := make([]board.Category, 0, len(detail.Tabs))
	counts := make([]int, 0, len(detail.Tabs))
	for _, tab := range detail.Tabs {
		categories = append(categories, tab.Category)
		counts = append(counts, tab.Count)
	}
	assert.Equal(t, []board.Category{
		board.CategoryLines, board.CategoryPassing, board.CategoryKicking,
	}, categories, "tabs follow the chip order and empty ones stay hidden")
	assert.Equal(t, []int{2, 1, 1}, counts, "the moneyline and the team total share Lines")
	assert.True(t, detail.Tabs[0].Active)
	assert.Equal(t, "Lines", detail.Tabs[0].Label)
}

func TestDetailLinesShowGameLinesThenTeamTotals(t *testing.T) {
	t.Parallel()

	g := game("401", at(t, 8, 13, 0))
	games := []db.Game{g}
	rows := []db.Odds{
		odds("401", "draftkings", "TEAM_TOTAL", "BUF", "", "UNDER", new(24.5), -110),
		odds("401", "draftkings", "TEAM_TOTAL", "BUF", "", "OVER", new(24.5), -110),
		odds("401", "draftkings", "SPREAD", "MIA", "", "TEAM", new(7.5), -105),
		odds("401", "draftkings", "SPREAD", "BUF", "", "TEAM", new(-7.5), -115),
		odds("401", "draftkings", "TOTAL", "", "", "UNDER", new(47.5), -110),
		odds("401", "draftkings", "TOTAL", "", "", "OVER", new(47.5), -105),
		odds("401", "draftkings", "ML", "MIA", "", "TEAM", nil, 260),
		odds("401", "draftkings", "ML", "BUF", "", "TEAM", nil, -320),
	}
	now := at(t, 7, 12, 0)
	built := board.Build(games, rows, nil, "", nil, now)

	detail := board.Detail(g, built, board.Query{Now: now})
	require.Len(t, detail.Lines, 4, "a team total stays on the Lines tab")

	assert.Equal(t, "SPREAD", detail.Lines[0].Market)
	assert.Equal(t, "Spread", detail.Lines[0].Label)
	require.Len(t, detail.Lines[0].Options, 2)
	assert.Equal(t, "MIA", detail.Lines[0].Options[0].Team, "the away side comes first")
	assert.Equal(t, "BUF", detail.Lines[0].Options[1].Team)
	assert.Empty(t, detail.Lines[0].Team, "a game-wide line carries no team")

	assert.Equal(t, "ML", detail.Lines[1].Market)
	require.Len(t, detail.Lines[1].Options, 2)
	assert.Equal(t, "MIA", detail.Lines[1].Options[0].Team)

	assert.Equal(t, "TOTAL", detail.Lines[2].Market)
	require.Len(t, detail.Lines[2].Options, 2)
	assert.Equal(t, "OVER", detail.Lines[2].Options[0].Side, "Over comes before Under")
	assert.Equal(t, "UNDER", detail.Lines[2].Options[1].Side)
	assert.Empty(t, detail.Lines[2].Team)

	assert.Equal(t, "TEAM_TOTAL", detail.Lines[3].Market)
	assert.Equal(t, "Team total", detail.Lines[3].Label)
	assert.Equal(t, "BUF", detail.Lines[3].Team)
	require.Len(t, detail.Lines[3].Options, 2)
	assert.Equal(t, "OVER", detail.Lines[3].Options[0].Side)
	assert.Equal(t, "UNDER", detail.Lines[3].Options[1].Side)
}

func TestDetailClustersPlayerLinesAcrossMarkets(t *testing.T) {
	t.Parallel()

	g := game("401", at(t, 8, 13, 0))
	games := []db.Game{g}
	rows := []db.Odds{
		odds("401", "draftkings", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(52.5), -110),
		odds("401", "draftkings", "REC_YDS", "BUF", "Khalil Shakir", "UNDER", new(52.5), -110),
		odds("401", "draftkings", "RECEPTIONS", "BUF", "Khalil Shakir", "OVER", new(4.5), -120),
		odds("401", "draftkings", "REC_YDS", "BUF", "Keon Coleman", "OVER", new(41.5), -110),
	}
	roster := []db.RosterPlayer{
		{AthleteID: "10", Team: "BUF", FullName: "Khalil Shakir", Position: "WR"},
		{AthleteID: "11", Team: "BUF", FullName: "Keon Coleman", Position: "WR"},
	}
	now := at(t, 7, 12, 0)
	built := board.Build(games, rows, roster, "", nil, now)

	detail := board.Detail(g, built, board.Query{Category: board.CategoryReceiving, Now: now})
	require.Len(t, detail.Players, 2, "each receive picks one player card")
	assert.Empty(t, detail.Lines)

	assert.Equal(t, "Keon Coleman", detail.Players[0].Name)
	shakir := detail.Players[1]
	assert.Equal(t, "Khalil Shakir", shakir.Name)
	assert.Equal(t, "10", shakir.PlayerID)
	assert.Equal(t, "WR", shakir.Position)
	assert.Equal(t, "BUF", shakir.Team)

	require.Len(t, shakir.Lines, 2, "yards and receptions share one card")
	assert.Equal(t, "Rec Yds", shakir.Lines[0].Label)
	require.Len(t, shakir.Lines[0].Options, 2)
	assert.Equal(t, "OVER", shakir.Lines[0].Options[0].Side)
	assert.Equal(t, "UNDER", shakir.Lines[0].Options[1].Side)
	assert.Equal(t, "Receptions", shakir.Lines[1].Label)
	require.Len(t, shakir.Lines[1].Options, 1, "an unpriced side leaves one button")
}

func TestDetailFoldsTeamTotalsIntoTheOpenTab(t *testing.T) {
	t.Parallel()

	g := game("401", at(t, 8, 13, 0))
	games := []db.Game{g}
	rows := []db.Odds{
		odds("401", "draftkings", "TEAM_TOTAL", "BUF", "", "OVER", new(24.5), -110),
	}
	now := at(t, 7, 12, 0)
	built := board.Build(games, rows, nil, "", nil, now)

	detail := board.Detail(g, built, board.Query{
		Category: board.CategoryTeamTotals,
		Now:      now,
	})
	assert.Equal(t, board.CategoryLines, detail.Category, "team totals open the Lines tab")
	require.Len(t, detail.Tabs, 1)
	assert.Equal(t, board.CategoryLines, detail.Tabs[0].Category)
	assert.True(t, detail.Tabs[0].Active)
	assert.Len(t, detail.Lines, 1)
	assert.Equal(t, "TEAM_TOTAL", detail.Lines[0].Market)
}

func TestDetailFallsBackToTheFirstTabWithRows(t *testing.T) {
	t.Parallel()

	g := game("401", at(t, 8, 13, 0))
	games := []db.Game{g}
	rows := []db.Odds{
		odds("401", "draftkings", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
		odds("401", "draftkings", "ANYTIME_TD", "BUF", "James Cook", "YES", nil, 150),
	}
	now := at(t, 7, 12, 0)
	built := board.Build(games, rows, nil, "", nil, now)

	detail := board.Detail(g, built, board.Query{Category: "bogus", Now: now})
	assert.Equal(t, board.CategoryPassing, detail.Category,
		"an unknown category falls back past empty Lines to the first tab with rows")
	require.Len(t, detail.Tabs, 2)
	assert.Equal(t, board.CategoryPassing, detail.Tabs[0].Category)
	assert.True(t, detail.Tabs[0].Active)
	assert.Equal(t, board.CategoryTDs, detail.Tabs[1].Category)
	require.Len(t, detail.Players, 1)
	assert.Equal(t, "Josh Allen", detail.Players[0].Name)

	tds := board.Detail(g, built, board.Query{Category: board.CategoryTDs, Now: now})
	require.Len(t, tds.Players, 1)
	require.Len(t, tds.Players[0].Lines, 1)
	assert.Equal(t, "Anytime TD", tds.Players[0].Lines[0].Label)
}

func TestDetailSorts(t *testing.T) {
	t.Parallel()

	g := game("401", at(t, 8, 13, 0))
	games := []db.Game{g}
	rows := []db.Odds{
		odds("401", "draftkings", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(72.5), 120),
		odds("401", "draftkings", "REC_YDS", "BUF", "Keon Coleman", "OVER", new(41.5), -150),
	}
	now := at(t, 7, 12, 0)
	built := board.Build(games, rows, nil, "", nil, now)

	player := board.Detail(g, built, board.Query{Sort: board.Sort("kickoff"), Now: now})
	assert.Equal(t, board.SortPlayer, player.Sort, "kickoff drops out for one game")
	require.Len(t, player.Players, 2)
	assert.Equal(t, "Keon Coleman", player.Players[0].Name)

	oddsSort := board.Detail(g, built, board.Query{Sort: board.SortOdds, Now: now})
	require.Len(t, oddsSort.Players, 2)
	assert.Equal(t, "Keon Coleman", oddsSort.Players[0].Name, "the shortest price comes first")

	lineSort := board.Detail(g, built, board.Query{Sort: board.SortLine, Now: now})
	assert.Equal(t, board.SortLine, lineSort.Sort)
	require.Len(t, lineSort.Players, 2)
	assert.Equal(t, "Khalil Shakir", lineSort.Players[0].Name, "the highest line comes first")
}

func TestDetailFiltersRowsBySearch(t *testing.T) {
	t.Parallel()

	g := game("401", at(t, 8, 13, 0))
	games := []db.Game{g}
	rows := []db.Odds{
		odds("401", "draftkings", "SPREAD", "BUF", "", "TEAM", new(-3.0), -110),
		odds("401", "draftkings", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
		odds("401", "draftkings", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(52.5), -110),
	}
	now := at(t, 7, 12, 0)
	built := board.Build(games, rows, nil, "", nil, now)

	detail := board.Detail(g, built, board.Query{Q: "shakir", Now: now})
	require.Len(t, detail.Tabs, 1, "a tab with no matching row stays hidden")
	assert.Equal(t, board.CategoryReceiving, detail.Tabs[0].Category)
	require.Len(t, detail.Players, 1)
	assert.Equal(t, "Khalil Shakir", detail.Players[0].Name)
	require.NotNil(t, detail.Game.Lines.Spread, "the header keeps the whole game")

	byTeam := board.Detail(g, built, board.Query{Q: "bills", Now: now})
	require.Len(t, byTeam.Tabs, 3, "a team name matches every row of the game")
	assert.Len(t, byTeam.Lines, 1)
}

func TestDetailOfAGameWithNoRows(t *testing.T) {
	t.Parallel()

	g := game("401", at(t, 8, 13, 0))
	other := game("402", at(t, 8, 16, 0))
	rows := []db.Odds{
		odds("402", "draftkings", "ML", "BUF", "", "TEAM", nil, -110),
	}
	built := board.Build([]db.Game{g, other}, rows, nil, "", nil, at(t, 7, 12, 0))

	detail := board.Detail(g, built, board.Query{})
	assert.Equal(t, "401", detail.Game.ID)
	assert.Zero(t, detail.Game.Props)
	assert.False(t, detail.Game.HasProps)
	require.Len(t, detail.Tabs, 1)
	assert.Equal(t, board.CategoryLines, detail.Tabs[0].Category)
	assert.Zero(t, detail.Tabs[0].Count)
	assert.Empty(t, detail.Lines, "another game's rows stay out of the detail")
	assert.Empty(t, detail.Players)
}

func TestPairKeepsBothSidesOfOneLine(t *testing.T) {
	t.Parallel()

	g := game("401", at(t, 8, 13, 0))
	games := []db.Game{g}
	rows := []db.Odds{
		odds("401", "draftkings", "TOTAL", "", "", "OVER", new(47.5), -105),
		odds("401", "draftkings", "TOTAL", "", "", "UNDER", new(47.5), -110),
		odds("401", "draftkings", "REC_YDS", "BUF", "Khalil Shakir", "UNDER", new(52.5), -115),
		odds("401", "draftkings", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(52.5), -105),
	}
	built := board.Build(games, rows, nil, "", nil, at(t, 7, 12, 0))

	groups := board.Pair(g, built)
	require.Len(t, groups, 2, "the total and the prop keep their own rows")

	total := pairGroup(t, groups, "TOTAL", "", "")
	require.Len(t, total.Options, 2)
	assert.Equal(t, []string{"OVER", "UNDER"}, optionSides(total), "Over comes before Under")
	assert.Equal(t, total.Options[0].Line, total.Options[1].Line, "both sides key on the same number")

	prop := pairGroup(t, groups, "REC_YDS", "BUF", "Khalil Shakir")
	require.Len(t, prop.Options, 2)
	assert.Equal(t, []string{"OVER", "UNDER"}, optionSides(prop))
}

func TestPairPutsTheAwaySideFirst(t *testing.T) {
	t.Parallel()

	g := game("401", at(t, 8, 13, 0))
	games := []db.Game{g}
	rows := []db.Odds{
		odds("401", "draftkings", "SPREAD", "BUF", "", "TEAM", new(-3.0), -115),
		odds("401", "draftkings", "SPREAD", "MIA", "", "TEAM", new(3.0), -105),
		odds("401", "draftkings", "ML", "BUF", "", "TEAM", nil, -150),
		odds("401", "draftkings", "ML", "MIA", "", "TEAM", nil, 130),
	}
	built := board.Build(games, rows, nil, "", nil, at(t, 7, 12, 0))

	groups := board.Pair(g, built)
	require.Len(t, groups, 2)

	spread := pairGroup(t, groups, "SPREAD", "", "")
	assert.Equal(t, []string{"MIA", "BUF"}, optionTeams(spread), "the away side comes first")
	require.Len(t, spread.Options, 2)
	assert.InDelta(t, -3.0, *spread.Options[1].Line, 1e-9)

	moneyline := pairGroup(t, groups, "ML", "", "")
	assert.Equal(t, []string{"MIA", "BUF"}, optionTeams(moneyline))
	assert.Equal(t, []string{"TEAM", "TEAM"}, optionSides(moneyline))
}

func TestPairKeepsTheSingleAnytimeTDSide(t *testing.T) {
	t.Parallel()

	g := game("401", at(t, 8, 13, 0))
	games := []db.Game{g}
	rows := []db.Odds{
		odds("401", "draftkings", "ANYTIME_TD", "BUF", "James Cook", "YES", nil, 150),
	}
	built := board.Build(games, rows, nil, "", nil, at(t, 7, 12, 0))

	groups := board.Pair(g, built)
	require.Len(t, groups, 1)
	group := groups[0]
	assert.Equal(t, "ANYTIME_TD", group.Market)
	assert.Equal(t, "Anytime TD", group.Label)
	require.Len(t, group.Options, 1, "an anytime touchdown has one side")
	assert.Equal(t, "YES", group.Options[0].Side)
}

func TestPairKeepsAMissingSide(t *testing.T) {
	t.Parallel()

	g := game("401", at(t, 8, 13, 0))
	games := []db.Game{g}
	rows := []db.Odds{
		odds("401", "draftkings", "TOTAL", "", "", "OVER", new(47.5), -105),
		odds("401", "draftkings", "SPREAD", "MIA", "", "TEAM", new(3.0), -105),
		odds("401", "draftkings", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(52.5), -110),
	}
	built := board.Build(games, rows, nil, "", nil, at(t, 7, 12, 0))

	groups := board.Pair(g, built)
	require.Len(t, groups, 3)

	for _, group := range groups {
		assert.Len(t, group.Options, 1, "%s keeps its one stored side", group.Market)
	}
}

func TestPairOptionOrderIsDeterministic(t *testing.T) {
	t.Parallel()

	g := game("401", at(t, 8, 13, 0))
	games := []db.Game{g}
	rows := []db.Odds{
		odds("401", "draftkings", "SPREAD", "BUF", "", "TEAM", new(-3.0), -115),
		odds("401", "draftkings", "SPREAD", "MIA", "", "TEAM", new(3.0), -105),
		odds("401", "draftkings", "ML", "BUF", "", "TEAM", nil, -150),
		odds("401", "draftkings", "ML", "MIA", "", "TEAM", nil, 130),
		odds("401", "draftkings", "TOTAL", "", "", "OVER", new(47.5), -105),
		odds("401", "draftkings", "TOTAL", "", "", "UNDER", new(47.5), -110),
		odds("401", "draftkings", "ANYTIME_TD", "BUF", "James Cook", "YES", nil, 150),
	}
	built := board.Build(games, rows, nil, "", nil, at(t, 7, 12, 0))
	reversed := make([]board.Row, len(built))
	for i := range built {
		reversed[len(built)-1-i] = built[i]
	}

	first := board.Pair(g, built)
	second := board.Pair(g, reversed)
	require.Len(t, first, len(second))
	for _, group := range first {
		same := pairGroup(t, second, group.Market, group.Team, group.Player)
		assert.Equal(t, optionSides(group), optionSides(same),
			"%s keeps its option order whatever the input order", group.Market)
	}
}

// pairGroup finds one line group by its market, team, and player.
func pairGroup(t *testing.T, groups []board.LineGroup, market, team, player string) board.LineGroup {
	t.Helper()
	for _, group := range groups {
		if group.Market == market && group.Team == team && group.Player == player {
			return group
		}
	}
	t.Fatalf("no %s group for team %q player %q", market, team, player)
	return board.LineGroup{}
}

// optionSides lists a group's option sides in order.
func optionSides(group board.LineGroup) []string {
	sides := make([]string, 0, len(group.Options))
	for _, option := range group.Options {
		sides = append(sides, option.Side)
	}
	return sides
}

// optionTeams lists a group's option teams in order.
func optionTeams(group board.LineGroup) []string {
	teams := make([]string, 0, len(group.Options))
	for _, option := range group.Options {
		teams = append(teams, option.Team)
	}
	return teams
}

func TestLogosAndHeadshots(t *testing.T) {
	t.Parallel()

	g := game("401", at(t, 8, 13, 0))
	g.AwayLogo = new("https://a.espncdn.com/mia.png")
	g.HomeLogo = new("https://a.espncdn.com/buf.png")

	roster := []db.RosterPlayer{
		{
			AthleteID:   "3918298",
			Team:        "BUF",
			FullName:    "Josh Allen",
			NormName:    "josh allen",
			Position:    "QB",
			HeadshotURL: "https://a.espncdn.com/allen.png",
		},
		{
			AthleteID: "4241463",
			Team:      "BUF",
			FullName:  "James Cook",
			NormName:  "james cook",
			Position:  "RB",
		},
	}
	rows := []db.Odds{
		odds("401", "fanduel", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(250.5), -110),
		odds("401", "fanduel", "RUSH_YDS", "BUF", "James Cook", "OVER", new(65.5), -110),
		odds("401", "fanduel", "REC_YDS", "MIA", "Unmatched Player", "OVER", new(45.5), -110),
	}

	built := board.Build([]db.Game{g}, rows, roster, "fanduel", nil, at(t, 7, 12, 0))

	cards := board.Slates([]db.Game{g}, built, roster, board.Query{Now: at(t, 8, 10, 0)})
	require.Len(t, cards, 1)
	assert.Equal(t, "https://a.espncdn.com/mia.png", cards[0].AwayLogo)
	assert.Equal(t, "https://a.espncdn.com/buf.png", cards[0].HomeLogo)

	detail := board.Detail(g, built, board.Query{Category: board.CategoryPassing})
	require.Len(t, detail.Players, 1)
	assert.Equal(t, "https://a.espncdn.com/allen.png", detail.Players[0].HeadshotURL)
	assert.Equal(t, "https://a.espncdn.com/allen.png", detail.Players[0].Lines[0].Options[0].HeadshotURL)

	rushing := board.Detail(g, built, board.Query{Category: board.CategoryRushing})
	require.Len(t, rushing.Players, 1)
	assert.Empty(t, rushing.Players[0].HeadshotURL)

	receiving := board.Detail(g, built, board.Query{Category: board.CategoryReceiving})
	require.Len(t, receiving.Players, 1)
	assert.Empty(t, receiving.Players[0].HeadshotURL)
}

func TestBuildMatchPlayerTeamPriorityAndFallback(t *testing.T) {
	t.Parallel()

	kickoff := at(t, 8, 13, 0)
	games := []db.Game{game("401", kickoff)}
	roster := []db.RosterPlayer{
		{AthleteID: "other-1", Team: "MIA", FullName: "Josh Allen", Position: "DE"},
		{AthleteID: "buf-1", Team: "BUF", FullName: "Josh Allen", Position: "QB"},
		{AthleteID: "other-2", Team: "KC", FullName: "Patrick Mahomes II", NormName: "patrick mahomes", Position: "QB"},
	}
	rows := []db.Odds{
		odds("401", "draftkings", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
		odds("401", "draftkings", "PASS_YDS", "MIA", "Patrick Mahomes", "OVER", new(275.5), -110),
	}

	built := board.Build(games, rows, roster, "", nil, kickoff.Add(-time.Hour))
	require.Len(t, built, 2)

	byPlayer := make(map[string]board.Row, len(built))
	for _, row := range built {
		byPlayer[row.PlayerName] = row
	}
	assert.Equal(t, "buf-1", byPlayer["Josh Allen"].PlayerID, "same-team match wins over earlier roster entry")
	assert.Equal(t, "other-2", byPlayer["Patrick Mahomes"].PlayerID, "falls back to another team via normalized name")
}

func TestBuildAndDetailNormalizeAccentsAndTeamTags(t *testing.T) {
	t.Parallel()

	kickoff := at(t, 8, 13, 0)
	g := game("401", kickoff)
	roster := []db.RosterPlayer{
		{AthleteID: "mia-1", Team: "MIA", FullName: "José Alvarado", Position: "RB"},
	}
	rows := []db.Odds{
		odds("401", "draftkings", "RUSH_YDS", "MIA", "Jose Alvarado (MIA)", "OVER", new(45.5), -110),
		odds("401", "draftkings", "RUSH_YDS", "MIA", "Ollie Gordon II (MIA)", "OVER", new(35.5), -110),
		odds("401", "fanduel", "RUSH_YDS", "MIA", "Ollie Gordon", "OVER", new(35.5), -105),
	}

	built := board.Build([]db.Game{g}, rows, roster, "draftkings", nil, kickoff.Add(-time.Hour))
	require.Len(t, built, 2)

	// Accent + team tag matches roster entry.
	assert.Equal(t, "mia-1", built[0].PlayerID)

	// Unmatched rows with and without (TEAM) tag and suffix fold into one player card
	// and one line group per market.
	detail := board.Detail(g, built, board.Query{Category: board.CategoryRushing, Now: kickoff.Add(-time.Hour)})
	require.Len(t, detail.Players, 2)
	for _, player := range detail.Players {
		assert.Len(t, player.Lines, 1)
	}

	// Accent-insensitive search finds José Alvarado via unaccented query.
	filtered := board.Detail(
		g,
		built,
		board.Query{Category: board.CategoryRushing, Q: "jose", Now: kickoff.Add(-time.Hour)},
	)
	require.Len(t, filtered.Players, 1)
	assert.Equal(t, "mia-1", filtered.Players[0].PlayerID)
}

func TestBuildAndPairCollapsePlayerPropsAcrossBooksAndSpellings(t *testing.T) {
	t.Parallel()

	kickoff := at(t, 8, 13, 0)
	g := game("401", kickoff)
	roster := []db.RosterPlayer{
		{AthleteID: "mia-ollie", Team: "MIA", FullName: "Ollie Gordon II", Position: "RB"},
	}
	rows := []db.Odds{
		odds("401", "fanduel", "RUSH_YDS", "MIA", "Ollie Gordon", "OVER", new(72.5), -110),
		odds("401", "fanduel", "RUSH_YDS", "MIA", "Ollie Gordon", "UNDER", new(72.5), -110),
		odds("401", "hardrockbet", "RUSH_YDS", "MIA", "Ollie Gordon II (MIA)", "OVER", new(73.5), -115),
		odds("401", "hardrockbet", "RUSH_YDS", "MIA", "Ollie Gordon II (MIA)", "UNDER", new(73.5), -105),
		odds("401", "draftkings", "RUSH_YDS", "MIA", "Ollie Gordon II", "OVER", new(75.5), -110),
		odds("401", "draftkings", "RUSH_YDS", "MIA", "Ollie Gordon II", "UNDER", new(75.5), -110),
	}

	built := board.Build([]db.Game{g}, rows, roster, "", nil, kickoff.Add(-time.Hour))
	require.Len(t, built, 2, "all books collapse into one OVER and one UNDER row")

	detail := board.Detail(g, built, board.Query{Category: board.CategoryRushing, Now: kickoff.Add(-time.Hour)})
	require.Len(t, detail.Players, 1)
	require.Len(t, detail.Players[0].Lines, 1, "one LineGroup per player and prop market")

	lineGroup := detail.Players[0].Lines[0]
	assert.Equal(t, "RUSH_YDS", lineGroup.Market)
	require.Len(t, lineGroup.Options, 2)
	assert.Equal(t, []string{"OVER", "UNDER"}, optionSides(lineGroup))
	assert.InDelta(t, 73.5, *lineGroup.Options[0].Line, 1e-9, "median across 72.5, 73.5, 75.5")
	assert.InDelta(t, 73.5, *lineGroup.Options[1].Line, 1e-9)
}

func TestPairOneSidedHomeBookStillPairsWithOtherBook(t *testing.T) {
	t.Parallel()

	kickoff := at(t, 8, 13, 0)
	g := game("401", kickoff)
	rows := []db.Odds{
		odds("401", "prizepicks", "RUSH_YDS", "MIA", "Ollie Gordon", "OVER", new(73.5), -110),
		odds("401", "draftkings", "RUSH_YDS", "MIA", "Ollie Gordon II", "OVER", new(75.5), -110),
		odds("401", "draftkings", "RUSH_YDS", "MIA", "Ollie Gordon II", "UNDER", new(75.5), -110),
	}

	built := board.Build([]db.Game{g}, rows, nil, "prizepicks", nil, kickoff.Add(-time.Hour))
	require.Len(t, built, 2)

	groups := board.Pair(g, built)
	require.Len(t, groups, 1, "OVER 73.5 and UNDER 75.5 pair into one LineGroup")
	require.Len(t, groups[0].Options, 2)
	assert.Equal(t, []string{"OVER", "UNDER"}, optionSides(groups[0]))
	assert.InDelta(t, 73.5, *groups[0].Options[0].Line, 1e-9)
	assert.InDelta(t, 75.5, *groups[0].Options[1].Line, 1e-9)
}

func TestPairByPlayerIDAcrossDifferingNamesAndTeams(t *testing.T) {
	t.Parallel()

	g := game("401", at(t, 8, 13, 0))
	paired := board.Pair(g, []board.Row{
		{
			GameID:     "401",
			Market:     "RUSH_YDS",
			Team:       "MIA",
			PlayerName: "Ollie Gordon",
			PlayerID:   "mia-ollie",
			Side:       "OVER",
			Line:       new(73.5),
		},
		{
			GameID:     "401",
			Market:     "RUSH_YDS",
			Team:       "",
			PlayerName: "Ollie Gordon II",
			PlayerID:   "mia-ollie",
			Side:       "UNDER",
			Line:       new(75.5),
		},
	})
	require.Len(t, paired, 1)
	require.Len(t, paired[0].Options, 2)
	assert.Equal(t, []string{"OVER", "UNDER"}, optionSides(paired[0]))
}

func TestLineKeyCanonicalPlayerPropsAndGameLines(t *testing.T) {
	t.Parallel()

	assert.Equal(
		t,
		board.LineKey("RUSH_YDS", "MIA", "Ollie Gordon", new(72.5)),
		board.LineKey("RUSH_YDS", "MIA", "Ollie Gordon II (MIA)", new(75.5)),
		"player props ignore line value and normalize player name",
	)
	assert.NotEqual(
		t,
		board.LineKey("RUSH_YDS", "MIA", "Ollie Gordon", new(72.5)),
		board.LineKey("REC_YDS", "MIA", "Ollie Gordon", new(72.5)),
	)
	assert.NotEqual(
		t,
		board.LineKey("TOTAL", "", "", new(47.5)),
		board.LineKey("TOTAL", "", "", new(48.5)),
		"game totals still key on line value",
	)
	assert.NotEqual(
		t,
		board.LineKey("TEAM_TOTAL", "BUF", "", new(24.5)),
		board.LineKey("TEAM_TOTAL", "BUF", "", new(25.5)),
		"team totals still key on line value",
	)
	assert.Equal(
		t,
		board.LineKey("SPREAD", "BUF", "", new(-3.5)),
		board.LineKey("SPREAD", "MIA", "", new(3.5)),
	)
}

func prodScaleBoardFixture(b *testing.B) ([]db.Game, []db.Odds, []db.RosterPlayer, time.Time) {
	b.Helper()

	now := time.Date(2026, time.November, 7, 12, 0, 0, 0, eastern)
	kickoff := time.Date(2026, time.November, 8, 13, 0, 0, 0, eastern)

	const (
		numGames       = 14
		playersPerTeam = 75 // 28 teams * 75 = 2,100 roster rows
	)

	teams := make([]string, numGames*2)
	for i := range teams {
		teams[i] = fmt.Sprintf("T%02d", i)
	}

	games := make([]db.Game, numGames)
	for i := range games {
		away := teams[i*2]
		home := teams[i*2+1]
		games[i] = db.Game{
			ID:       fmt.Sprintf("40167%04d", i),
			AwayTeam: away,
			HomeTeam: home,
			AwayName: "Away " + away,
			HomeName: "Home " + home,
			Kickoff:  kickoff.Add(time.Duration(i%3) * 3 * time.Hour),
			Status:   db.GameScheduled,
		}
	}

	positions := []string{"QB", "RB", "WR", "TE", "K"}
	roster := make([]db.RosterPlayer, 0, len(teams)*playersPerTeam)
	for tIdx, team := range teams {
		for p := range playersPerTeam {
			id := strconv.Itoa(tIdx*100 + p + 1)
			fullName := fmt.Sprintf("Player %s %02d Jr.", team, p)
			if p%10 == 0 {
				fullName = fmt.Sprintf("José %s %02d", team, p)
			}
			roster = append(roster, db.RosterPlayer{
				AthleteID:   id,
				Team:        team,
				FullName:    fullName,
				NormName:    db.NormalizePlayerName(fullName),
				Position:    positions[p%len(positions)],
				HeadshotURL: "https://a.espncdn.com/" + id + ".png",
			})
		}
	}

	books := []string{"draftkings", "fanduel"}
	markets := []string{"PASS_YDS", "RUSH_YDS", "REC_YDS", "RECEPTIONS", "ANYTIME_TD", "PASS_TDS"}
	oddsRows := make([]db.Odds, 0, 15500)
	for _, g := range games {
		for _, book := range books {
			oddsRows = append(oddsRows,
				odds(g.ID, book, "ML", g.AwayTeam, "", "TEAM", nil, 120),
				odds(g.ID, book, "ML", g.HomeTeam, "", "TEAM", nil, -140),
				odds(g.ID, book, "SPREAD", g.AwayTeam, "", "TEAM", new(2.5), -110),
				odds(g.ID, book, "SPREAD", g.HomeTeam, "", "TEAM", new(-2.5), -110),
				odds(g.ID, book, "TOTAL", "", "", "OVER", new(45.5), -110),
				odds(g.ID, book, "TOTAL", "", "", "UNDER", new(45.5), -110),
			)
		}
		for p := range 46 {
			team := g.AwayTeam
			if p%2 == 1 {
				team = g.HomeTeam
			}
			player := fmt.Sprintf("Player %s %02d Jr.", team, p)
			for _, m := range markets {
				for _, book := range books {
					if m == "ANYTIME_TD" {
						oddsRows = append(oddsRows, odds(g.ID, book, m, team, player, "YES", nil, 150))
						continue
					}
					line := new(45.5)
					oddsRows = append(oddsRows,
						odds(g.ID, book, m, team, player, "OVER", line, -110),
						odds(g.ID, book, m, team, player, "UNDER", line, -110),
					)
				}
			}
		}
	}

	return games, oddsRows, roster, now
}

func BenchmarkBuild(b *testing.B) {
	games, oddsRows, roster, now := prodScaleBoardFixture(b)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		rows := board.Build(games, oddsRows, roster, "fanduel", nil, now)
		if len(rows) == 0 {
			b.Fatal("expected rows")
		}
	}
}

func BenchmarkSlates(b *testing.B) {
	games, oddsRows, roster, now := prodScaleBoardFixture(b)
	built := board.Build(games, oddsRows, roster, "fanduel", nil, now)
	q := board.Query{Q: "Player T03 11", Now: now}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		cards := board.Slates(games, built, roster, q)
		if len(cards) == 0 {
			b.Fatal("expected cards")
		}
	}
}

func BenchmarkDetail(b *testing.B) {
	games, oddsRows, roster, now := prodScaleBoardFixture(b)
	built := board.Build(games, oddsRows, roster, "fanduel", nil, now)
	q := board.Query{Category: board.CategoryReceiving, Now: now}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		detail := board.Detail(games[0], built, q)
		if len(detail.Players) == 0 {
			b.Fatal("expected players")
		}
	}
}

func TestRowMatchesLegAndBuildStayBoundAcrossLineMoves(t *testing.T) {
	t.Parallel()

	kickoff := at(t, 8, 13, 0)
	g := game("401", kickoff)
	roster := []db.RosterPlayer{
		{
			AthleteID: "mia-ollie",
			Team:      "MIA",
			FullName:  "Ollie Gordon II",
			NormName:  db.NormalizePlayerName("Ollie Gordon II"),
			Position:  "RB",
		},
	}
	rows := []db.Odds{
		odds("401", "fanduel", "RUSH_YDS", "MIA", "Ollie Gordon II", "OVER", new(73.5), -110),
		odds("401", "fanduel", "RUSH_YDS", "MIA", "Ollie Gordon II", "UNDER", new(73.5), -110),
		odds("401", "fanduel", "SPREAD", "BUF", "", "TEAM", new(-4.0), -110),
		odds("401", "fanduel", "TOTAL", "", "", "OVER", new(48.5), -110),
	}
	mine := []db.Leg{
		{
			ID:         11,
			GameID:     new("401"),
			MarketType: "RUSH_YDS",
			Team:       new("MIA"),
			PlayerName: new("Ollie Gordon"),
			Side:       new("OVER"),
			Line:       new(72.5),
		},
		{
			ID:         12,
			GameID:     new("401"),
			MarketType: "SPREAD",
			Team:       new("BUF"),
			Side:       new("TEAM"),
			Line:       new(-3.5),
		},
		{
			ID:         13,
			GameID:     new("401"),
			MarketType: "TOTAL",
			Side:       new("OVER"),
			Line:       new(47.5),
		},
	}

	built := board.Build([]db.Game{g}, rows, roster, "fanduel", mine, kickoff.Add(-time.Hour))
	pickedByKey := make(map[string]int64, len(built))
	for _, r := range built {
		pickedByKey[r.Market+"|"+r.Side] = r.Picked
	}
	assert.Equal(t, int64(11), pickedByKey["RUSH_YDS|OVER"])
	assert.Equal(t, int64(0), pickedByKey["RUSH_YDS|UNDER"])
	assert.Equal(t, int64(12), pickedByKey["SPREAD|TEAM"])
	assert.Equal(t, int64(13), pickedByKey["TOTAL|OVER"])
}

func TestBuildPopulatesPickedDriftFields(t *testing.T) {
	t.Parallel()

	kickoff := at(t, 8, 13, 0)
	g := game("401", kickoff)
	rows := []db.Odds{
		odds("401", "fanduel", "SPREAD", "BUF", "", "TEAM", new(-4.5), -110),
		odds("401", "fanduel", "ML", "MIA", "", "TEAM", nil, 165),
		odds("401", "fanduel", "TOTAL", "", "", "OVER", new(47.5), -110),
	}
	mine := []db.Leg{
		{
			ID:         1,
			GameID:     new("401"),
			MarketType: "SPREAD",
			Team:       new("BUF"),
			Side:       new("TEAM"),
			Line:       new(-3.5),
			Odds:       -110,
		},
		{
			ID:         2,
			GameID:     new("401"),
			MarketType: "ML",
			Team:       new("MIA"),
			Side:       new("TEAM"),
			Odds:       145,
		},
		{
			ID:         3,
			GameID:     new("401"),
			MarketType: "TOTAL",
			Side:       new("OVER"),
			Line:       new(47.5),
			Odds:       -110,
		},
	}

	built := board.Build([]db.Game{g}, rows, nil, "fanduel", mine, kickoff.Add(-time.Hour))
	byMarket := make(map[string]board.Row, len(built))
	for _, r := range built {
		byMarket[r.Market] = r
	}

	spread := byMarket["SPREAD"]
	require.Equal(t, int64(1), spread.Picked)
	require.NotNil(t, spread.PickedLine)
	assert.InDelta(t, -3.5, *spread.PickedLine, 0.001)
	assert.Equal(t, -110, spread.PickedOdds)
	assert.True(t, spread.Drifted)
	assert.Equal(t, "Slip -3.5 · now -4.5", spread.BoardDriftNote)
	assert.Equal(t, "Now -4.5 (-110)", spread.TrayDriftNote)

	ml := byMarket["ML"]
	require.Equal(t, int64(2), ml.Picked)
	assert.Nil(t, ml.PickedLine)
	assert.Equal(t, 145, ml.PickedOdds)
	assert.True(t, ml.Drifted)
	assert.Equal(t, "Slip +145 · now +165", ml.BoardDriftNote)
	assert.Equal(t, "Now +165", ml.TrayDriftNote)

	total := byMarket["TOTAL"]
	require.Equal(t, int64(3), total.Picked)
	require.NotNil(t, total.PickedLine)
	assert.InDelta(t, 47.5, *total.PickedLine, 0.001)
	assert.Equal(t, -110, total.PickedOdds)
	assert.False(t, total.Drifted)
	assert.Empty(t, total.BoardDriftNote)
	assert.Empty(t, total.TrayDriftNote)
}

func TestBuildSlateResolvesOpenGameUnloadedAndStates(t *testing.T) {
	t.Parallel()

	thu := at(t, 5, 20, 15)
	sunEarly := at(t, 8, 13, 0)
	sunLate := at(t, 8, 16, 25)
	now := thu.Add(time.Hour) // Thursday game has started; Sunday games remain.

	gThu := game("thu", thu)
	gEarly := game("early", sunEarly)
	gLate := db.Game{
		ID:       "late",
		AwayTeam: "KC",
		HomeTeam: "DEN",
		AwayName: "Kansas City Chiefs",
		HomeName: "Denver Broncos",
		Kickoff:  sunLate,
		Status:   db.GameScheduled,
	}
	games := []db.Game{gThu, gEarly, gLate}

	roster := []db.RosterPlayer{
		{
			AthleteID: "kc-mahomes",
			Team:      "KC",
			FullName:  "Patrick Mahomes",
			NormName:  db.NormalizePlayerName("Patrick Mahomes"),
			Position:  "QB",
		},
	}

	rows := board.Build(games, []db.Odds{
		odds("early", "fanduel", "SPREAD", "BUF", "", "TEAM", new(-3.5), -110),
		odds("late", "fanduel", "SPREAD", "KC", "", "TEAM", new(-6.5), -110),
		odds("late", "fanduel", "PASS_YDS", "KC", "Patrick Mahomes", "OVER", new(275.5), -110),
	}, roster, "fanduel", nil, now)

	t.Run("resolves open game, unloaded flag, and window counts", func(t *testing.T) {
		t.Parallel()
		slate := board.BuildSlate(games, rows, roster, board.Query{
			GameID: "early",
			Q:      "Mahomes",
			Now:    now,
		})
		assert.Equal(t, 1, slate.Started)
		assert.Equal(t, "early", slate.ResolvedGameID)
		require.NotNil(t, slate.OpenGame)
		assert.Equal(t, "early", slate.OpenGame.ID)
		assert.True(t, slate.OpenGame.Unloaded, "game with lines and no props is unloaded")
		assert.Equal(t, 1, slate.WindowCounts[board.WindowSundayEarly])
		assert.Equal(t, 2, slate.WindowCounts[board.WindowSundayLate])
		assert.False(t, slate.NoGames)
		assert.False(t, slate.AllStarted)
		assert.False(t, slate.NoLines)
		assert.False(t, slate.Empty)
		require.Len(t, slate.Cards, 1)
		assert.Equal(t, "late", slate.Cards[0].ID)
		assert.False(t, slate.Cards[0].Unloaded, "game with props is not unloaded")
	})

	t.Run("clears resolved game when kicked off or outside window", func(t *testing.T) {
		t.Parallel()
		startedSlate := board.BuildSlate(games, rows, nil, board.Query{GameID: "thu", Now: now})
		assert.Nil(t, startedSlate.OpenGame)
		assert.Empty(t, startedSlate.ResolvedGameID)

		wrongWindow := board.BuildSlate(games, rows, nil, board.Query{
			GameID: "early",
			Window: board.WindowSundayLate,
			Now:    now,
		})
		assert.Nil(t, wrongWindow.OpenGame)
		assert.Empty(t, wrongWindow.ResolvedGameID)
	})

	t.Run("empty state flags", func(t *testing.T) {
		t.Parallel()
		noGames := board.BuildSlate(nil, nil, nil, board.Query{Now: now})
		assert.True(t, noGames.NoGames)
		assert.False(t, noGames.AllStarted)
		assert.False(t, noGames.NoLines)
		assert.False(t, noGames.Empty)

		allStarted := board.BuildSlate([]db.Game{gThu}, nil, nil, board.Query{Now: now})
		assert.True(t, allStarted.AllStarted)
		assert.False(t, allStarted.NoLines)
		assert.False(t, allStarted.Empty)

		noLines := board.BuildSlate([]db.Game{gEarly}, nil, nil, board.Query{Now: now})
		assert.True(t, noLines.NoLines)
		assert.False(t, noLines.Empty)

		filteredEmpty := board.BuildSlate(games, rows, nil, board.Query{Q: "nonexistent-team", Now: now})
		assert.True(t, filteredEmpty.Empty)
	})
}

func TestLineGroupForLegAndHoldsLeg(t *testing.T) {
	t.Parallel()

	kickoff := at(t, 8, 13, 0)
	g := game("401", kickoff)
	raw := []db.Odds{
		odds("401", "fanduel", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(255.5), -110),
		odds("401", "fanduel", "PASS_YDS", "BUF", "Josh Allen", "UNDER", new(255.5), -110),
		odds("401", "fanduel", "SPREAD", "BUF", "", "TEAM", new(-3.5), -110),
		odds("401", "fanduel", "SPREAD", "MIA", "", "TEAM", new(3.5), -110),
	}
	roster := []db.RosterPlayer{
		{
			AthleteID: "qb1",
			FullName:  "Josh Allen",
			NormName:  db.NormalizePlayerName("Josh Allen"),
			Team:      "BUF",
			Position:  "QB",
		},
	}
	pickedLeg := db.Leg{
		ID:         42,
		GameID:     new("401"),
		MarketType: "PASS_YDS",
		Team:       new("BUF"),
		PlayerName: new("Josh Allen"),
		PlayerID:   new("qb1"),
		Side:       new("OVER"),
		Line:       new(255.5),
		Odds:       -110,
	}
	rows := board.Build([]db.Game{g}, raw, roster, "fanduel", []db.Leg{pickedLeg}, kickoff.Add(-time.Hour))
	signals := board.BoardSignals{
		TeamEPA: map[string]*analyzer.TeamDefenseProfile{
			"MIA": {DefPassEPAAllowedRank: 30},
		},
		PlayerUsage: map[string]*analyzer.PlayerUsage{
			"qb1": {
				SeasonTargetShare: 0.25,
				Weeks:             []analyzer.PlayerWeekUsage{{WeekNum: 1, TargetShare: 0.25}},
			},
		},
	}

	group, ok := board.LineGroupForLeg(g, rows, pickedLeg, signals)
	require.True(t, ok)
	assert.Equal(t, "PASS_YDS", group.Market)
	require.Len(t, group.Options, 2)
	assert.Equal(t, int64(42), group.Options[0].Picked)
	assert.Equal(t, board.BadgeSmashMatchup, group.Options[0].Badge)
	assert.True(t, group.HoldsLeg(pickedLeg))

	// A just-deleted leg (not in mine, so Picked == 0 on rows) still matches by selection fields.
	deletedSpread := db.Leg{
		ID:         99,
		GameID:     new("401"),
		MarketType: "SPREAD",
		Team:       new("MIA"),
		Side:       new("TEAM"),
		Line:       new(3.5),
		Odds:       -110,
	}
	spreadGroup, ok := board.LineGroupForLeg(g, rows, deletedSpread, signals)
	require.True(t, ok)
	assert.Equal(t, "SPREAD", spreadGroup.Market)
	assert.True(t, spreadGroup.HoldsLeg(deletedSpread))
	assert.False(t, spreadGroup.HoldsLeg(pickedLeg))

	missingLeg := db.Leg{
		ID:         100,
		GameID:     new("401"),
		MarketType: "TOTAL",
		Side:       new("OVER"),
		Line:       new(47.5),
	}
	_, ok = board.LineGroupForLeg(g, rows, missingLeg, signals)
	assert.False(t, ok)
}
