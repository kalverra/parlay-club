package refresh

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/archive"
	"github.com/kalverra/parlay-club/internal/ingest/odds"
)

// oddsTestStore writes the week and the one stored game the odds tests use.
func oddsTestStore(t *testing.T, now time.Time) (*db.Store, db.Game) {
	t.Helper()

	store := openTestStore(t)
	week, err := store.UpsertWeek(t.Context(), 2026, 2, 9)
	require.NoError(t, err)
	game, err := store.UpsertGame(t.Context(), storedGame(week.ID, now))
	require.NoError(t, err)
	return store, game
}

// primaryOdds is Parlay API's usual board: FanDuel rows carry links, DraftKings
// rows do not, and no row has a useful event ID for DraftKings.
func primaryOdds(game db.Game) *fakeLines {
	return &fakeLines{
		events: []odds.Event{{
			ID: "evt-primary", HomeTeam: game.HomeName, AwayTeam: game.AwayName,
			CommenceTime: game.Kickoff,
		}},
		eventsCredits: odds.Credits{Remaining: 400},
		lines: []odds.Line{
			{
				EventID: "evt-primary", Book: "fanduel", MarketType: odds.MarketML,
				Team: game.HomeName, Side: odds.SideTeam, Price: -155,
				Link: "https://sportsbook.fanduel.com/buf", Sid: "fd-buf", EventSid: "fd-event",
			},
			{
				EventID: "evt-primary", Book: "draftkings", MarketType: odds.MarketML,
				Team: game.HomeName, Side: odds.SideTeam, Price: -150,
			},
			{
				EventID: "evt-primary", Book: "draftkings", MarketType: odds.MarketTotal,
				Side: odds.SideOver, Line: new(45.5), Price: -108,
			},
		},
		lineCredits: odds.Credits{Remaining: 397},
	}
}

// secondaryOdds is The Odds API board, carrying DraftKings links, a different
// total number, and a book Parlay API did not quote.
func secondaryOdds(game db.Game) *fakeLines {
	return &fakeLines{
		events: []odds.Event{{
			ID: "evt-secondary", HomeTeam: game.HomeName, AwayTeam: game.AwayName,
			CommenceTime: game.Kickoff,
		}},
		eventsCredits: odds.Credits{Remaining: 320},
		lines: []odds.Line{
			{
				EventID: "evt-secondary", Book: "draftkings", MarketType: odds.MarketML,
				Team: game.HomeName, Side: odds.SideTeam, Price: -148,
				Link: "https://sportsbook.draftkings.com/?outcomes=dk-buf", Sid: "dk-buf",
				EventSid: "34118128",
			},
			{
				EventID: "evt-secondary", Book: "draftkings", MarketType: odds.MarketTotal,
				Side: odds.SideOver, Line: new(46.0), Price: -105,
				Link: "https://sportsbook.draftkings.com/?outcomes=dk-over", Sid: "dk-over",
				EventSid: "34118128",
			},
			{
				EventID: "evt-secondary", Book: "fanduel", MarketType: odds.MarketML,
				Team: game.HomeName, Side: odds.SideTeam, Price: -160,
				Link: "https://sportsbook.fanduel.com/other", Sid: "fd-other", EventSid: "fd-event",
			},
			{
				EventID: "evt-secondary", Book: "betmgm", MarketType: odds.MarketML,
				Team: game.AwayName, Side: odds.SideTeam, Price: 130,
				Link: "https://sports.betmgm.com/mia", Sid: "mgm-mia", EventSid: "mgm-event",
			},
		},
		lineCredits: odds.Credits{Remaining: 317},
	}
}

func TestMergeOddsEnrichesUnlinkedRowsAndKeepsLinkedPrimary(t *testing.T) {
	t.Parallel()

	now := novemberTime(7, 11, 0)
	primary := []db.Odds{
		{
			GameID: "401", Book: "fanduel", MarketType: odds.MarketML,
			Team: "BUF", Side: odds.SideTeam, Price: -155,
			Link: new("https://sportsbook.fanduel.com/buf"), Sid: new("fd-buf"),
			EventSid: new("fd-event"), FetchedAt: now,
		},
		{
			GameID: "401", Book: "draftkings", MarketType: odds.MarketML,
			Team: "BUF", Side: odds.SideTeam, Price: -150, FetchedAt: now,
		},
		{
			GameID: "401", Book: "draftkings", MarketType: odds.MarketSpread,
			Team: "MIA", Side: odds.SideTeam, Line: new(3.0), Price: -110, FetchedAt: now,
		},
	}
	secondary := []db.Odds{
		{
			GameID: "401", Book: "fanduel", MarketType: odds.MarketML,
			Team: "BUF", Side: odds.SideTeam, Price: -160,
			Link: new("https://sportsbook.fanduel.com/other"), Sid: new("fd-other"),
			EventSid: new("fd-event"), FetchedAt: now,
		},
		{
			// Attaching DraftKings' link also adopts its half point.
			GameID: "401", Book: "draftkings", MarketType: odds.MarketSpread,
			Team: "MIA", Side: odds.SideTeam, Line: new(3.5), Price: -105,
			Link: new("https://sportsbook.draftkings.com/?outcomes=dk-mia"), Sid: new("dk-mia"),
			EventSid: new("34118128"), FetchedAt: now,
		},
		{
			// This row's event ID backfills the unlinked DraftKings moneyline too.
			GameID: "401", Book: "draftkings", MarketType: odds.MarketTotal,
			Side: odds.SideOver, Line: new(46.0), Price: -105,
			Link: new("https://sportsbook.draftkings.com/?outcomes=dk-over"), Sid: new("dk-over"),
			EventSid: new("34118128"), FetchedAt: now,
		},
		{
			GameID: "401", Book: "betmgm", MarketType: odds.MarketML,
			Team: "MIA", Side: odds.SideTeam, Price: 130,
			Link: new("https://sports.betmgm.com/mia"), Sid: new("mgm-mia"),
			EventSid: new("mgm-event"), FetchedAt: now,
		},
	}

	rows := mergeOdds(primary, secondary)
	require.Len(t, rows, 5, "secondary-only markets are appended")

	bySelection := make(map[string]db.Odds, len(rows))
	for _, row := range rows {
		bySelection[row.Book+"|"+row.MarketType+"|"+row.Team+"|"+row.Side] = row
	}

	fanDuel := bySelection["fanduel|ML|BUF|TEAM"]
	assert.Equal(t, -155, fanDuel.Price, "a linked primary keeps its price")
	assert.Equal(t, new("https://sportsbook.fanduel.com/buf"), fanDuel.Link)
	assert.Equal(t, new("fd-buf"), fanDuel.Sid)

	draftKingsML := bySelection["draftkings|ML|BUF|TEAM"]
	assert.Nil(t, draftKingsML.Link, "a secondary row for another selection does not link it")
	assert.Equal(t, new("34118128"), draftKingsML.EventSid,
		"the book's event ID backfills from another market")

	draftKingsSpread := bySelection["draftkings|SPREAD|MIA|TEAM"]
	assert.Equal(t, new(3.5), draftKingsSpread.Line, "the adopted link carries its own line")
	assert.Equal(t, -105, draftKingsSpread.Price)
	assert.Equal(t, new("https://sportsbook.draftkings.com/?outcomes=dk-mia"), draftKingsSpread.Link)
	assert.Equal(t, new("dk-mia"), draftKingsSpread.Sid)
	assert.Equal(t, new("34118128"), draftKingsSpread.EventSid)

	draftKingsTotal := bySelection["draftkings|TOTAL||OVER"]
	assert.Equal(t, new(46.0), draftKingsTotal.Line)
	assert.Equal(t, new("dk-over"), draftKingsTotal.Sid)

	assert.Equal(t, new("mgm-mia"), bySelection["betmgm|ML|MIA|TEAM"].Sid)
}

func TestMergeOddsReturnsTheProviderThatHasRows(t *testing.T) {
	t.Parallel()

	row := db.Odds{GameID: "401", Book: "draftkings", MarketType: odds.MarketML, Team: "BUF"}

	assert.Equal(t, []db.Odds{row}, mergeOdds(nil, []db.Odds{row}))
	assert.Equal(t, []db.Odds{row}, mergeOdds([]db.Odds{row}, nil))
	assert.Empty(t, mergeOdds(nil, nil))
}

func TestSyncOddsEnrichesParlayLinesWithTheOddsAPI(t *testing.T) {
	t.Parallel()

	now := novemberTime(7, 11, 0)
	store, game := oddsTestStore(t, now)
	primary := primaryOdds(game)
	fallback := secondaryOdds(game)
	service := New(Deps{
		Store: store, Lines: primary, FallbackLines: fallback, Logger: zerolog.Nop(),
	})

	changed, err := service.syncOdds(t.Context(), []db.Game{game}, now)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, 1, fallback.lineCalls, "a healthy primary still allows enrichment")

	stored, err := store.GameByID(t.Context(), game.ID)
	require.NoError(t, err)
	assert.Equal(t, new("evt-primary"), stored.OddsEventID)

	rows, err := store.OddsForGame(t.Context(), game.ID)
	require.NoError(t, err)
	require.Len(t, rows, 4, "the betmgm market exists only in The Odds API")

	bySelection := make(map[string]db.Odds, len(rows))
	for _, row := range rows {
		bySelection[row.Book+"|"+row.MarketType+"|"+row.Team+"|"+row.Side] = row
	}

	fanDuel := bySelection["fanduel|ML|BUF|TEAM"]
	assert.Equal(t, -155, fanDuel.Price)
	assert.Equal(t, new("https://sportsbook.fanduel.com/buf"), fanDuel.Link)
	assert.Equal(t, new("fd-buf"), fanDuel.Sid)

	draftKingsML := bySelection["draftkings|ML|BUF|TEAM"]
	assert.Equal(t, -150, draftKingsML.Price, "a same-line link keeps the primary price")
	assert.Equal(t, new("https://sportsbook.draftkings.com/?outcomes=dk-buf"), draftKingsML.Link)
	assert.Equal(t, new("dk-buf"), draftKingsML.Sid)
	assert.Equal(t, new("34118128"), draftKingsML.EventSid)

	draftKingsTotal := bySelection["draftkings|TOTAL||OVER"]
	assert.Equal(t, new(46.0), draftKingsTotal.Line)
	assert.Equal(t, new("https://sportsbook.draftkings.com/?outcomes=dk-over"), draftKingsTotal.Link)

	assert.Equal(t, new("mgm-mia"), bySelection["betmgm|ML|MIA|TEAM"].Sid)

	linesState, err := store.SyncState(t.Context(), OddsLinesSource)
	require.NoError(t, err)
	require.NotNil(t, linesState.OddsCreditsRemaining)
	assert.Equal(t, 397, *linesState.OddsCreditsRemaining)
	assert.Equal(t, now, linesState.FetchedAt)

	fallbackState, err := store.SyncState(t.Context(), TheOddsAPISource)
	require.NoError(t, err)
	require.NotNil(t, fallbackState.OddsCreditsRemaining)
	assert.Equal(t, 317, *fallbackState.OddsCreditsRemaining)
	assert.Equal(t, now, fallbackState.FetchedAt)
}

func TestSyncOddsArchivesEachProvidersCallsUnderItsOwnKey(t *testing.T) {
	t.Parallel()

	now := novemberTime(7, 11, 0)
	store, game := oddsTestStore(t, now)

	server := func(eventID string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Requests-Remaining", "300")
			if r.URL.Path == "/events" {
				_, _ = fmt.Fprintf(w, `[{
					"id": %q,
					"commence_time": "2026-11-08T18:00:00Z",
					"home_team": "Buffalo Bills",
					"away_team": "Miami Dolphins"
				}]`, eventID)
				return
			}
			_, _ = fmt.Fprintf(w, `[{
				"id": %q,
				"commence_time": "2026-11-08T18:00:00Z",
				"home_team": "Buffalo Bills",
				"away_team": "Miami Dolphins",
				"bookmakers": [{
					"key": "draftkings",
					"sid": "dk-event",
					"markets": [{
						"key": "h2h",
						"outcomes": [{"name": "Buffalo Bills", "price": -150, "link": "https://dk/buf", "sid": "dk-buf"}]
					}]
				}]
			}]`, eventID)
		}))
	}
	primaryServer := server("evt-primary")
	t.Cleanup(primaryServer.Close)
	fallbackServer := server("evt-fallback")
	t.Cleanup(fallbackServer.Close)

	httpClient := &http.Client{Transport: archive.NewTransport(http.DefaultTransport, store, nil)}
	service := New(Deps{
		Store:         store,
		Lines:         odds.NewClient(primaryServer.URL, "primary-key", httpClient),
		FallbackLines: odds.NewClient(fallbackServer.URL, "fallback-key", httpClient),
		Logger:        zerolog.Nop(),
	})

	ctx := archive.WithKey(t.Context(), OddsLinesSource)
	_, err := service.syncOdds(ctx, []db.Game{game}, now)
	require.NoError(t, err)

	responses, err := store.ListAPIResponses(t.Context(), db.APIResponseFilter{})
	require.NoError(t, err)
	require.Len(t, responses, 4)

	counts := make(map[string]int, 2)
	for _, response := range responses {
		counts[response.Key]++
		assert.Equal(t, archive.SourceOdds, response.Source)
	}
	assert.Equal(t, map[string]int{OddsLinesSource: 2, TheOddsAPISource: 2}, counts)
}

func TestSyncOddsSkipsEnrichmentBelowTheOddsAPIEnrichFloor(t *testing.T) {
	t.Parallel()

	now := novemberTime(7, 11, 0)
	store, game := oddsTestStore(t, now)
	primary := primaryOdds(game)
	fallback := secondaryOdds(game)
	fallback.eventsCredits = odds.Credits{Remaining: TheOddsAPIEnrichFloor - 20}
	service := New(Deps{
		Store: store, Lines: primary, FallbackLines: fallback, Logger: zerolog.Nop(),
	})

	_, err := service.syncOdds(t.Context(), []db.Game{game}, now)
	require.NoError(t, err)
	assert.Zero(t, fallback.lineCalls, "enrichment waits for a healthy reserve")

	rows, err := store.OddsForGame(t.Context(), game.ID)
	require.NoError(t, err)
	require.Len(t, rows, 3, "primary rows still land")

	fallbackState, err := store.SyncState(t.Context(), TheOddsAPISource)
	require.NoError(t, err)
	require.NotNil(t, fallbackState.OddsCreditsRemaining)
	assert.Equal(t, TheOddsAPIEnrichFloor-20, *fallbackState.OddsCreditsRemaining,
		"the free event list still refreshes the fallback balance")
	assert.Equal(t, now, fallbackState.FetchedAt)
}

func TestSyncOddsFallsBackBelowTheParlayCreditFloor(t *testing.T) {
	t.Parallel()

	now := novemberTime(7, 11, 0)
	store, game := oddsTestStore(t, now)
	primary := primaryOdds(game)
	primary.eventsCredits = odds.Credits{Remaining: OddsLinesFloor - 1}
	fallback := secondaryOdds(game)
	fallback.eventsCredits = odds.Credits{Remaining: TheOddsAPIFallbackFloor + 55}
	service := New(Deps{
		Store: store, Lines: primary, FallbackLines: fallback, Logger: zerolog.Nop(),
	})

	changed, err := service.syncOdds(t.Context(), []db.Game{game}, now)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Zero(t, primary.lineCalls, "the primary stays under its floor")
	assert.Equal(t, 1, fallback.lineCalls, "the fallback buys the week's lines")

	stored, err := store.GameByID(t.Context(), game.ID)
	require.NoError(t, err)
	assert.Equal(t, new("evt-primary"), stored.OddsEventID,
		"the primary's free match wins while it is healthy")

	rows, err := store.OddsForGame(t.Context(), game.ID)
	require.NoError(t, err)
	require.Len(t, rows, 4)
	bySelection := make(map[string]db.Odds, len(rows))
	for _, row := range rows {
		bySelection[row.Book+"|"+row.MarketType+"|"+row.Team+"|"+row.Side] = row
	}
	assert.Equal(t, new("dk-buf"), bySelection["draftkings|ML|BUF|TEAM"].Sid)
	assert.Equal(t, new("mgm-mia"), bySelection["betmgm|ML|MIA|TEAM"].Sid)

	fallbackState, err := store.SyncState(t.Context(), TheOddsAPISource)
	require.NoError(t, err)
	require.NotNil(t, fallbackState.OddsCreditsRemaining)
	assert.Equal(t, 317, *fallbackState.OddsCreditsRemaining)
}

func TestSyncOddsCommitsPrimaryWhenFallbackFails(t *testing.T) {
	t.Parallel()

	tests := map[string]error{
		"disabled": odds.ErrNoAPIKey,
		"outage":   errors.New("503 service unavailable"),
	}
	for name, fallbackErr := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			now := novemberTime(7, 11, 0)
			store, game := oddsTestStore(t, now)
			primary := primaryOdds(game)
			fallback := secondaryOdds(game)
			fallback.eventsErr = fallbackErr
			service := New(Deps{
				Store: store, Lines: primary, FallbackLines: fallback, Logger: zerolog.Nop(),
			})

			changed, err := service.syncOdds(t.Context(), []db.Game{game}, now)
			require.NoError(t, err, "one healthy provider is enough")
			assert.True(t, changed)

			rows, err := store.OddsForGame(t.Context(), game.ID)
			require.NoError(t, err)
			require.Len(t, rows, 3)

			state, err := store.SyncState(t.Context(), OddsLinesSource)
			require.NoError(t, err)
			assert.Equal(t, now, state.FetchedAt, "the sync is recorded, so it does not back off")
		})
	}
}

func TestSyncOddsCommitsFallbackWhenPrimaryFails(t *testing.T) {
	t.Parallel()

	tests := map[string]error{
		"disabled": odds.ErrNoAPIKey,
		"outage":   errors.New("503 service unavailable"),
	}
	for name, primaryErr := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			now := novemberTime(7, 11, 0)
			store, game := oddsTestStore(t, now)
			_, err := store.SetGameOddsEvent(t.Context(), game.ID, "evt-old")
			require.NoError(t, err)
			game, err = store.GameByID(t.Context(), game.ID)
			require.NoError(t, err)

			primary := primaryOdds(game)
			primary.eventsErr = primaryErr
			fallback := secondaryOdds(game)
			service := New(Deps{
				Store: store, Lines: primary, FallbackLines: fallback, Logger: zerolog.Nop(),
			})

			changed, err := service.syncOdds(t.Context(), []db.Game{game}, now)
			require.NoError(t, err, "the fallback serves the week")
			assert.True(t, changed)

			stored, err := store.GameByID(t.Context(), game.ID)
			require.NoError(t, err)
			assert.Equal(t, new("evt-old"), stored.OddsEventID,
				"a transient primary outage must not churn the event ID")

			rows, err := store.OddsForGame(t.Context(), game.ID)
			require.NoError(t, err)
			require.Len(t, rows, 4)
		})
	}
}

func TestSyncOddsReturnsTheErrorWhenEveryConfiguredProviderFails(t *testing.T) {
	t.Parallel()

	now := novemberTime(7, 11, 0)
	store, game := oddsTestStore(t, now)
	primary := primaryOdds(game)
	primary.eventsErr = errors.New("primary is down")
	fallback := secondaryOdds(game)
	fallback.eventsErr = errors.New("fallback is down")
	service := New(Deps{
		Store: store, Lines: primary, FallbackLines: fallback, Logger: zerolog.Nop(),
	})

	_, err := service.syncOdds(t.Context(), []db.Game{game}, now)
	require.Error(t, err)
	require.ErrorContains(t, err, "primary is down")
	require.ErrorContains(t, err, "fallback is down")

	state, stateErr := store.SyncState(t.Context(), OddsLinesSource)
	require.ErrorIs(t, stateErr, db.ErrSyncStateNotFound)
	assert.Zero(t, state.FetchedAt, "a failed sync does not look fresh")
}

func TestSyncOddsReturnsErrNoAPIKeyWhenEveryProviderIsDisabled(t *testing.T) {
	t.Parallel()

	now := novemberTime(7, 11, 0)
	store, game := oddsTestStore(t, now)
	service := New(Deps{Store: store, Logger: zerolog.Nop()})

	_, err := service.syncOdds(t.Context(), []db.Game{game}, now)
	require.ErrorIs(t, err, odds.ErrNoAPIKey)

	disabled := &fakeLines{eventsErr: odds.ErrNoAPIKey}
	service = New(Deps{
		Store: store, Lines: disabled, FallbackLines: disabled, Logger: zerolog.Nop(),
	})
	_, err = service.syncOdds(t.Context(), []db.Game{game}, now)
	require.ErrorIs(t, err, odds.ErrNoAPIKey)
}
