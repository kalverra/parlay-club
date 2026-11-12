package board_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	sunday := at(t, 8, 13, 0)
	monday := at(t, 9, 20, 15)
	games := []db.Game{
		game("sun", sunday),
		game("mon", monday),
	}
	rows := []db.Odds{
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

	built := board.Build(games, rows, nil, "", nil, sunday.Add(-24*time.Hour))
	require.Len(t, built, 10)

	byText := make(map[string]board.Row, len(built))
	for _, row := range built {
		byText[row.Text] = row
	}
	assert.Equal(t, board.CategoryLines, byText["BUF -3"].Category)
	assert.Equal(t, board.CategoryLines, byText["BUF ML"].Category)
	assert.Equal(t, board.CategoryLines, byText["Over 47.5"].Category)
	assert.Equal(t, board.CategoryTeamTotals, byText["BUF Team Total Over 24.5"].Category)
	assert.Equal(t, board.CategoryPassing, byText["Josh Allen Over 245.5 Pass Yds"].Category)
	assert.Equal(t, board.CategoryRushing, byText["James Cook Over 64.5 Rush Yds"].Category)
	assert.Equal(t, board.CategoryReceiving, byText["Khalil Shakir Over 52.5 Rec Yds"].Category)
	assert.Equal(t, board.CategoryTDs, byText["James Cook Anytime TD"].Category)
	assert.Equal(t, board.CategoryKicking, byText["Tyler Bass Over 7.5 Kicking Pts"].Category)

	assert.Equal(t, board.WindowSundayEarly, byText["BUF ML"].Window)
	assert.Equal(t, board.WindowMonday, byText["MIA ML"].Window,
		"a Monday-night kickoff that lands on Tuesday in UTC still reads Monday")
	assert.Equal(t, monday.UTC(), byText["MIA ML"].Kickoff.UTC())
}

func TestWindowsSplitTheDay(t *testing.T) {
	t.Parallel()

	games := []db.Game{
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

	built := board.Build(games, rows, nil, "", nil, at(t, 7, 12, 0))
	want := map[string]board.Window{
		"morning": board.WindowSundayMorning,
		"early":   board.WindowSundayEarly,
		"late":    board.WindowSundayLate,
		"night":   board.WindowSundayNight,
		"monday":  board.WindowMonday,
	}
	for _, row := range built {
		assert.Equal(t, want[row.GameID], row.Window, row.GameID)
	}
}

func TestSorts(t *testing.T) {
	t.Parallel()

	early := at(t, 8, 13, 0)
	late := at(t, 8, 16, 0)
	games := []db.Game{game("late", late), game("early", early)}
	rows := []db.Odds{
		odds("late", "draftkings", "ML", "BUF", "", "TEAM", nil, 150),
		odds("early", "draftkings", "ML", "MIA", "", "TEAM", nil, -200),
		odds("early", "draftkings", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
	}
	built := board.Build(games, rows, nil, "", nil, at(t, 7, 12, 0))

	kickoff := board.Search(built, games, nil, board.Query{
		Sort: board.SortKickoff,
		Now:  at(t, 7, 12, 0),
	})
	require.Len(t, kickoff.Rows, 3)
	assert.Equal(t, "early", kickoff.Rows[0].GameID)
	assert.Equal(t, "late", kickoff.Rows[2].GameID)

	player := board.Search(built, games, nil, board.Query{Sort: board.SortPlayer, Now: at(t, 7, 12, 0)})
	require.Len(t, player.Rows, 3)
	assert.Equal(t, "Josh Allen", player.Rows[0].PlayerName)
	assert.Empty(t, player.Rows[1].PlayerName)

	oddsSort := board.Search(built, games, nil, board.Query{Sort: board.SortOdds, Now: at(t, 7, 12, 0)})
	require.Len(t, oddsSort.Rows, 3)
	assert.Equal(t, -200, oddsSort.Rows[0].Odds, "the shortest price comes first")
	assert.Equal(t, 150, oddsSort.Rows[2].Odds)

	category := board.Search(built, games, nil, board.Query{Sort: board.SortCategory, Now: at(t, 7, 12, 0)})
	require.Len(t, category.Rows, 3)
	assert.Equal(t, board.CategoryLines, category.Rows[0].Category)
	assert.Equal(t, board.CategoryPassing, category.Rows[2].Category)
}

func TestSearchIsAccentInsensitive(t *testing.T) {
	t.Parallel()

	game := game("401", at(t, 8, 13, 0))
	games := []db.Game{game}
	rows := []db.Odds{
		odds("401", "draftkings", "REC_YDS", "CIN", "Ja'Marr Chase", "OVER", new(72.5), -110),
		odds("401", "draftkings", "REC_YDS", "CIN", "José Ramírez", "OVER", new(42.5), -110),
	}
	roster := []db.RosterPlayer{
		{AthleteID: "1", Team: "CIN", FullName: "Ja'Marr Chase"},
	}
	built := board.Build(games, rows, roster, "", nil, at(t, 7, 12, 0))

	for _, query := range []string{"Chase", "jamarr", "Ja'Marr", "JOSE", "ramirez"} {
		result := board.Search(built, games, roster, board.Query{
			Q:    query,
			Sort: board.SortKickoff,
			Now:  at(t, 7, 12, 0),
		})
		assert.NotEmpty(t, result.Rows, "query %q finds a row", query)
	}
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

	result := board.Search(built, games, nil, board.Query{Now: now})
	require.Len(t, result.Rows, 1)
	assert.Equal(t, "upcoming", result.Rows[0].GameID)
	assert.Equal(t, 1, result.Started)
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

func TestSearchNamesAGameWhosePropsAreUnloaded(t *testing.T) {
	t.Parallel()

	game := game("401", at(t, 8, 13, 0))
	games := []db.Game{game}
	rows := []db.Odds{
		odds("401", "draftkings", "ML", "BUF", "", "TEAM", nil, -110),
	}
	roster := []db.RosterPlayer{
		{AthleteID: "1", Team: "BUF", FullName: "Josh Allen"},
	}
	built := board.Build(games, rows, roster, "", nil, at(t, 7, 12, 0))

	result := board.Search(built, games, roster, board.Query{
		Q:   "Josh Allen",
		Now: at(t, 7, 12, 0),
	})
	require.Len(t, result.Unloaded, 1)
	assert.Equal(t, "401", result.Unloaded[0].GameID)
	assert.Equal(t, "MIA @ BUF", result.Unloaded[0].Matchup)

	byTeam := board.Search(built, games, roster, board.Query{
		Q:   "Buffalo",
		Now: at(t, 7, 12, 0),
	})
	assert.Len(t, byTeam.Unloaded, 1)

	noMatch := board.Search(built, games, roster, board.Query{
		Q:   "Kansas City",
		Now: at(t, 7, 12, 0),
	})
	assert.Empty(t, noMatch.Unloaded)
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

	early := game("early", at(t, 8, 13, 0))
	early.VenueName = new("Highmark Stadium")
	late := game("late", at(t, 8, 16, 25))
	late.HomeTeam = "KC"
	late.HomeName = "Kansas City Chiefs"
	games := []db.Game{early, late}
	rows := []db.Odds{
		odds("early", "draftkings", "ML", "BUF", "", "TEAM", nil, -150),
		odds("late", "draftkings", "ML", "KC", "", "TEAM", nil, -150),
	}
	roster := []db.RosterPlayer{
		{AthleteID: "1", Team: "BUF", FullName: "Josh Allen", Position: "QB"},
	}
	now := at(t, 7, 12, 0)
	built := board.Build(games, rows, roster, "", nil, now)

	window := board.Slates(games, built, roster, board.Query{
		Window: board.WindowSundayLate,
		Now:    now,
	})
	require.Len(t, window, 1)
	assert.Equal(t, "late", window[0].ID)

	byTeam := board.Slates(games, built, roster, board.Query{Q: "Buffalo", Now: now})
	require.Len(t, byTeam, 1)
	assert.Equal(t, "early", byTeam[0].ID)

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
