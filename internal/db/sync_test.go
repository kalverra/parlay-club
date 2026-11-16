package db_test

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
)

func TestSyncStateReportsAMissingSource(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)

	_, err := store.SyncState(t.Context(), "espn:scoreboard")
	require.ErrorIs(t, err, db.ErrSyncStateNotFound)
}

func TestRecordSyncCreatesAndUpdatesTheRow(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	first := time.Date(2026, time.November, 7, 12, 0, 0, 0, time.UTC)
	remaining := 500

	require.NoError(t, store.RecordSync(t.Context(), "odds:lines", first, &remaining))
	state, err := store.SyncState(t.Context(), "odds:lines")
	require.NoError(t, err)
	assert.Equal(t, "odds:lines", state.Source)
	assert.Equal(t, first, state.FetchedAt)
	require.NotNil(t, state.OddsCreditsRemaining)
	assert.Equal(t, 500, *state.OddsCreditsRemaining)

	second := first.Add(time.Hour)
	remaining = 480
	require.NoError(t, store.RecordSync(t.Context(), "odds:lines", second, &remaining))

	state, err = store.SyncState(t.Context(), "odds:lines")
	require.NoError(t, err)
	assert.Equal(t, second, state.FetchedAt)
	assert.Equal(t, 480, *state.OddsCreditsRemaining)

	// A nil balance leaves the stored count alone: a sync that skipped the paid
	// call cannot erase what the last paid call reported.
	require.NoError(t, store.RecordSync(t.Context(), "odds:lines", second.Add(time.Hour), nil))
	state, err = store.SyncState(t.Context(), "odds:lines")
	require.NoError(t, err)
	assert.Equal(t, second.Add(time.Hour), state.FetchedAt)
	require.NotNil(t, state.OddsCreditsRemaining)
	assert.Equal(t, 480, *state.OddsCreditsRemaining)

	require.NoError(t, store.RecordSync(t.Context(), "espn:scoreboard", second, nil))
	state, err = store.SyncState(t.Context(), "espn:scoreboard")
	require.NoError(t, err)
	assert.Nil(t, state.OddsCreditsRemaining)
}

func TestSetSyncCreditsLeavesFetchedAtAlone(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	fetchedAt := time.Date(2026, time.November, 7, 12, 0, 0, 0, time.UTC)
	remaining := 500
	require.NoError(t, store.RecordSync(t.Context(), "odds:lines", fetchedAt, &remaining))

	require.NoError(t, store.SetSyncCredits(t.Context(), "odds:lines", 450))

	state, err := store.SyncState(t.Context(), "odds:lines")
	require.NoError(t, err)
	assert.Equal(t, fetchedAt, state.FetchedAt)
	assert.Equal(t, 450, *state.OddsCreditsRemaining)

	// A source with no row is untouched; there is nothing to refresh yet.
	require.NoError(t, store.SetSyncCredits(t.Context(), "odds:props:x", 450))
	_, err = store.SyncState(t.Context(), "odds:props:x")
	require.ErrorIs(t, err, db.ErrSyncStateNotFound)
}

func TestGamesForWeekReturnsKickoffOrder(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	week := createTestWeek(t, store, 9)

	late := testSlateGame(week)
	late.ID = "late"
	late.Kickoff = late.Kickoff.Add(3 * time.Hour)
	early := testSlateGame(week)
	early.ID = "early"

	_, err := store.UpsertGame(t.Context(), late)
	require.NoError(t, err)
	_, err = store.UpsertGame(t.Context(), early)
	require.NoError(t, err)

	games, err := store.GamesForWeek(t.Context(), week.ID)
	require.NoError(t, err)
	require.Len(t, games, 2)
	assert.Equal(t, "early", games[0].ID)
	assert.Equal(t, "late", games[1].ID)
}

func TestSetGameWeatherReportsRealChanges(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	week := createTestWeek(t, store, 9)
	game, err := store.UpsertGame(t.Context(), testSlateGame(week))
	require.NoError(t, err)

	first := time.Date(2026, time.November, 7, 13, 0, 0, 0, time.UTC)
	changed, err := store.SetGameWeather(t.Context(), game.ID, 48, 18, 29, 0.08, first)
	require.NoError(t, err)
	assert.True(t, changed)

	stored, err := store.GameByID(t.Context(), game.ID)
	require.NoError(t, err)
	assert.Equal(t, new(48.0), stored.TempF)
	assert.Equal(t, new(18.0), stored.WindMPH)
	assert.Equal(t, new(29.0), stored.WindGustMPH)
	assert.Equal(t, new(0.08), stored.PrecipIN)
	assert.Equal(t, first, stored.UpdatedAt)

	changed, err = store.SetGameWeather(t.Context(), game.ID, 48, 18, 29, 0.08, first.Add(time.Hour))
	require.NoError(t, err)
	assert.False(t, changed)
	stored, err = store.GameByID(t.Context(), game.ID)
	require.NoError(t, err)
	assert.Equal(t, first, stored.UpdatedAt, "an unchanged forecast leaves updated_at alone")

	changed, err = store.SetGameWeather(t.Context(), game.ID, 40, 22, 35, 0, first.Add(2*time.Hour))
	require.NoError(t, err)
	assert.True(t, changed)
}

func TestSetGameWeatherReportsAMissingGame(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)

	_, err := store.SetGameWeather(t.Context(), "missing", 48, 18, 29, 0, time.Now())
	require.ErrorIs(t, err, db.ErrGameNotFound)
}

func TestSetGameOddsEventReportsRealChanges(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	week := createTestWeek(t, store, 9)
	game, err := store.UpsertGame(t.Context(), testSlateGame(week))
	require.NoError(t, err)

	changed, err := store.SetGameOddsEvent(t.Context(), game.ID, "evt-1")
	require.NoError(t, err)
	assert.True(t, changed)

	stored, err := store.GameByID(t.Context(), game.ID)
	require.NoError(t, err)
	assert.Equal(t, new("evt-1"), stored.OddsEventID)

	changed, err = store.SetGameOddsEvent(t.Context(), game.ID, "evt-1")
	require.NoError(t, err)
	assert.False(t, changed)

	changed, err = store.SetGameOddsEvent(t.Context(), game.ID, "evt-2")
	require.NoError(t, err)
	assert.True(t, changed)
}

func TestSetGameInjuriesReportsRealChanges(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	week := createTestWeek(t, store, 9)
	game, err := store.UpsertGame(t.Context(), testSlateGame(week))
	require.NoError(t, err)

	changed, err := store.SetGameInjuries(t.Context(), game.ID, `[{"athlete":"Player 1"}]`)
	require.NoError(t, err)
	assert.True(t, changed)

	stored, err := store.GameByID(t.Context(), game.ID)
	require.NoError(t, err)
	assert.Equal(t, new(`[{"athlete":"Player 1"}]`), stored.InjuriesJSON)

	changed, err = store.SetGameInjuries(t.Context(), game.ID, `[{"athlete":"Player 1"}]`)
	require.NoError(t, err)
	assert.False(t, changed)

	changed, err = store.SetGameInjuries(t.Context(), game.ID, `[{"athlete":"Player 2"}]`)
	require.NoError(t, err)
	assert.True(t, changed)
}

func TestSetGameInjuriesReportsAMissingGame(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)

	_, err := store.SetGameInjuries(t.Context(), "missing", `[]`)
	require.ErrorIs(t, err, db.ErrGameNotFound)
}

func TestUpsertOddsInsertsUpdatesAndLists(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	week := createTestWeek(t, store, 9)
	game, err := store.UpsertGame(t.Context(), testSlateGame(week))
	require.NoError(t, err)
	fetchedAt := time.Date(2026, time.November, 7, 12, 0, 0, 0, time.UTC)

	rows := []db.Odds{
		{
			GameID: game.ID, Book: "draftkings", MarketType: "SPREAD", Team: "BUF",
			Side: "TEAM", Line: new(-2.5), Price: -110, FetchedAt: fetchedAt,
		},
		{
			GameID: game.ID, Book: "draftkings", MarketType: "ML", Team: "BUF",
			Side: "TEAM", Price: -155, FetchedAt: fetchedAt,
		},
	}
	changed, err := store.UpsertOdds(t.Context(), rows)
	require.NoError(t, err)
	assert.True(t, changed)

	stored, err := store.OddsForGame(t.Context(), game.ID)
	require.NoError(t, err)
	require.Len(t, stored, 2)
	assert.Equal(t, "ML", stored[0].MarketType, "the list is ordered by market")
	assert.Equal(t, "SPREAD", stored[1].MarketType)

	changed, err = store.UpsertOdds(t.Context(), rows)
	require.NoError(t, err)
	assert.False(t, changed, "the same prices at the same fetch time change nothing")

	rows[1].Price = -160
	changed, err = store.UpsertOdds(t.Context(), rows)
	require.NoError(t, err)
	assert.True(t, changed)
}

func TestUpsertOddsStoresAndRefreshesLinksWithPrices(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	week := createTestWeek(t, store, 9)
	game, err := store.UpsertGame(t.Context(), testSlateGame(week))
	require.NoError(t, err)
	fetchedAt := time.Date(2026, time.November, 7, 12, 0, 0, 0, time.UTC)

	rows := []db.Odds{
		{
			GameID: game.ID, Book: "draftkings", MarketType: "SPREAD", Team: "BUF",
			Side: "TEAM", Line: new(-2.5), Price: -110,
			Link: new("https://sportsbook.draftkings.com/?outcomes=0HC1P250_1"), Sid: new("0HC1P250_1"),
			EventSid: new("34118128"), FetchedAt: fetchedAt,
		},
		{
			GameID: game.ID, Book: "betmgm", MarketType: "ML", Team: "BUF",
			Side: "TEAM", Price: -155, FetchedAt: fetchedAt,
		},
	}
	changed, err := store.UpsertOdds(t.Context(), rows)
	require.NoError(t, err)
	assert.True(t, changed)

	stored, err := store.OddsForGame(t.Context(), game.ID)
	require.NoError(t, err)
	require.Len(t, stored, 2)
	byBook := make(map[string]db.Odds, len(stored))
	for _, row := range stored {
		byBook[row.Book] = row
	}
	assert.Equal(t, new("https://sportsbook.draftkings.com/?outcomes=0HC1P250_1"), byBook["draftkings"].Link)
	assert.Equal(t, new("0HC1P250_1"), byBook["draftkings"].Sid)
	assert.Equal(t, new("34118128"), byBook["draftkings"].EventSid)
	assert.Nil(t, byBook["betmgm"].Link, "a book without a link stores NULL")
	assert.Nil(t, byBook["betmgm"].Sid, "a book without a sid stores NULL")
	assert.Nil(t, byBook["betmgm"].EventSid, "a book without an event id stores NULL")

	changed, err = store.UpsertOdds(t.Context(), rows)
	require.NoError(t, err)
	assert.False(t, changed, "the same link and price at the same fetch time change nothing")

	rows[0].Price = -115
	rows[0].Link = new("https://sportsbook.draftkings.com/?outcomes=0HC1P250_2")
	rows[0].Sid = new("0HC1P250_2")
	rows[0].EventSid = new("34118129")
	changed, err = store.UpsertOdds(t.Context(), rows)
	require.NoError(t, err)
	assert.True(t, changed)

	stored, err = store.OddsForGame(t.Context(), game.ID)
	require.NoError(t, err)
	require.Len(t, stored, 2)
	for _, row := range stored {
		if row.Book == "draftkings" {
			assert.Equal(t, -115, row.Price)
			assert.Equal(t, new("https://sportsbook.draftkings.com/?outcomes=0HC1P250_2"), row.Link)
			assert.Equal(t, new("0HC1P250_2"), row.Sid)
			assert.Equal(t, new("34118129"), row.EventSid)
		}
		if row.Book == "betmgm" {
			assert.Equal(t, -155, row.Price, "another book's row is untouched")
			assert.Nil(t, row.Link)
		}
	}

	rows[0].Link = nil
	rows[0].Sid = nil
	rows[0].EventSid = nil
	changed, err = store.UpsertOdds(t.Context(), rows)
	require.NoError(t, err)
	assert.True(t, changed, "losing a link is a change")

	stored, err = store.OddsForGame(t.Context(), game.ID)
	require.NoError(t, err)
	for _, row := range stored {
		if row.Book == "draftkings" {
			assert.Nil(t, row.Link, "a vanished link becomes NULL")
			assert.Nil(t, row.Sid)
			assert.Nil(t, row.EventSid, "a vanished event id becomes NULL")
		}
	}
}

func openBenchStore(b *testing.B) *db.Store {
	b.Helper()

	database, err := db.Open(filepath.Join(b.TempDir(), "bench.db"))
	require.NoError(b, err)
	require.NoError(b, db.Migrate(b.Context(), database))

	store := db.NewStore(database)
	b.Cleanup(func() {
		_ = store.Close()
		_ = database.Close()
	})
	return store
}

func benchOddsBatch(gameID, awayTeam, homeTeam string, count int, fetchedAt time.Time) []db.Odds {
	books := []string{"draftkings", "fanduel"}
	markets := []string{"PASS_YDS", "RUSH_YDS", "REC_YDS", "RECEPTIONS", "ANYTIME_TD", "PASS_TDS"}
	rows := make([]db.Odds, 0, count)

	for _, book := range books {
		rows = append(rows,
			db.Odds{
				GameID: gameID, Book: book, MarketType: "ML", Team: awayTeam,
				Side: "TEAM", Price: 120, FetchedAt: fetchedAt,
			},
			db.Odds{
				GameID: gameID, Book: book, MarketType: "ML", Team: homeTeam,
				Side: "TEAM", Price: -140, FetchedAt: fetchedAt,
			},
			db.Odds{
				GameID: gameID, Book: book, MarketType: "SPREAD", Team: awayTeam,
				Side: "TEAM", Line: new(2.5), Price: -110, FetchedAt: fetchedAt,
			},
			db.Odds{
				GameID: gameID, Book: book, MarketType: "SPREAD", Team: homeTeam,
				Side: "TEAM", Line: new(-2.5), Price: -110, FetchedAt: fetchedAt,
			},
			db.Odds{
				GameID: gameID, Book: book, MarketType: "TOTAL", Team: "",
				Side: "OVER", Line: new(45.5), Price: -110, FetchedAt: fetchedAt,
			},
			db.Odds{
				GameID: gameID, Book: book, MarketType: "TOTAL", Team: "",
				Side: "UNDER", Line: new(45.5), Price: -110, FetchedAt: fetchedAt,
			},
		)
	}

	for p := range 60 {
		team := awayTeam
		if p%2 == 1 {
			team = homeTeam
		}
		player := fmt.Sprintf("Player %s %02d", team, p)
		for _, market := range markets {
			for _, book := range books {
				if market == "ANYTIME_TD" {
					rows = append(rows, db.Odds{
						GameID: gameID, Book: book, MarketType: market, Team: team,
						PlayerName: player, Side: "YES", Price: 150, FetchedAt: fetchedAt,
					})
				} else {
					line := new(45.5)
					rows = append(rows,
						db.Odds{
							GameID: gameID, Book: book, MarketType: market, Team: team,
							PlayerName: player, Side: "OVER", Line: line, Price: -110, FetchedAt: fetchedAt,
						},
						db.Odds{
							GameID: gameID, Book: book, MarketType: market, Team: team,
							PlayerName: player, Side: "UNDER", Line: line, Price: -110, FetchedAt: fetchedAt,
						},
					)
				}
				if len(rows) >= count {
					return rows[:count]
				}
			}
		}
	}
	return rows
}

func seedProdScaleWeekOdds(b *testing.B, store *db.Store, numGames, totalOdds int) (db.Week, []db.Game) {
	b.Helper()

	week, err := store.UpsertWeek(b.Context(), 2026, 2, 9)
	require.NoError(b, err)

	fetchedAt := time.Date(2026, time.November, 8, 11, 0, 0, 0, time.UTC)
	games := make([]db.Game, 0, numGames)
	perGame := totalOdds / numGames
	remainder := totalOdds % numGames

	for i := range numGames {
		away := fmt.Sprintf("T%02d", i*2)
		home := fmt.Sprintf("T%02d", i*2+1)
		game := testSlateGame(week)
		game.ID = fmt.Sprintf("40167%04d", i)
		game.AwayTeam = away
		game.HomeTeam = home
		game.AwayName = "Away " + away
		game.HomeName = "Home " + home
		game.Kickoff = game.Kickoff.Add(time.Duration(i%3) * 3 * time.Hour)

		stored, err := store.UpsertGame(b.Context(), game)
		require.NoError(b, err)
		games = append(games, stored)

		count := perGame
		if i < remainder {
			count++
		}
		_, err = store.UpsertOdds(b.Context(), benchOddsBatch(stored.ID, away, home, count, fetchedAt))
		require.NoError(b, err)
	}
	return week, games
}

func BenchmarkOddsForWeek(b *testing.B) {
	store := openBenchStore(b)
	week, _ := seedProdScaleWeekOdds(b, store, 14, 15500)

	b.ReportAllocs()
	for b.Loop() {
		rows, err := store.OddsForWeek(b.Context(), week.ID)
		if err != nil {
			b.Fatal(err)
		}
		if len(rows) != 15500 {
			b.Fatalf("got %d rows, want 15500", len(rows))
		}
	}
}

func BenchmarkUpsertOdds(b *testing.B) {
	store := openBenchStore(b)
	week, games := seedProdScaleWeekOdds(b, store, 1, 1200)
	_ = week

	fetchedAt := time.Date(2026, time.November, 8, 12, 0, 0, 0, time.UTC)
	batch := benchOddsBatch(games[0].ID, games[0].AwayTeam, games[0].HomeTeam, 1200, fetchedAt)

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		price := -110 - (i%2)*5
		for j := 0; j < len(batch); j += 10 {
			batch[j].Price = price
		}
		if _, err := store.UpsertOdds(b.Context(), batch); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkConcurrentReadsDuringWrite(b *testing.B) {
	store := openBenchStore(b)
	week, games := seedProdScaleWeekOdds(b, store, 14, 15500)

	fetchedAt := time.Date(2026, time.November, 8, 12, 0, 0, 0, time.UTC)
	writeBatch := benchOddsBatch(games[0].ID, games[0].AwayTeam, games[0].HomeTeam, 1200, fetchedAt)

	writerCtx, cancelWriter := context.WithCancel(b.Context())
	var wg sync.WaitGroup
	wg.Go(func() {
		for i := 0; writerCtx.Err() == nil; i++ {
			price := -110 - (i%2)*5
			for j := 0; j < len(writeBatch); j += 10 {
				writeBatch[j].Price = price
			}
			if _, err := store.UpsertOdds(writerCtx, writeBatch); err != nil {
				return
			}
		}
	})

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rows, err := store.OddsForWeek(b.Context(), week.ID)
			if err != nil {
				b.Error(err)
				return
			}
			if len(rows) < 15500 {
				b.Errorf("got %d rows, want >= 15500", len(rows))
				return
			}
		}
	})

	b.StopTimer()
	cancelWriter()
	wg.Wait()
}
