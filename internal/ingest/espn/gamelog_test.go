package espn_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/ingest/espn"
)

func TestGameLogFromJoshAllenFixture(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("testdata", "gamelog.json"))
	require.NoError(t, err)

	requestedPath := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		assert.Equal(t, "parlay-club", r.Header.Get("User-Agent"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
	t.Cleanup(server.Close)

	client := espn.NewClient(server.URL, server.Client())
	log, err := client.GameLog(t.Context(), "3918298")
	require.NoError(t, err)

	assert.Equal(t, "/athletes/3918298/gamelog", requestedPath)
	assert.Equal(t, "3918298", log.AthleteID)
	assert.Equal(t, 7, log.GamesPlayed)

	// Most recent game first
	assert.Equal(t, []float64{289, 255, 210, 253, 204, 248, 334}, log.Markets["PASS_YDS"])
	assert.Equal(t, []float64{2, 2, 1, 1, 0, 3, 2}, log.Markets["PASS_TDS"])
	assert.Equal(t, []float64{30, 1, 121, 4, 22, 69, 23}, log.Markets["RUSH_YDS"])
	assert.Equal(t, []float64{7, 5, 10, 5, 8, 14, 6}, log.Markets["RUSH_ATT"])
	assert.Equal(t, []float64{1, 0, 1, 1, 2, 2, 2}, log.Markets["ANYTIME_TD"])

	assert.NotContains(t, log.Markets, "REC_YDS")
	assert.NotContains(t, log.Markets, "RECEPTIONS")
	assert.NotContains(t, log.Markets, "KICKER_PTS")

	assert.Equal(t, []espn.GameLogEntry{
		{EventID: "401873026", Week: 8, Opponent: "BAL", AtVs: "vs", Result: "W", Score: "36-10"},
		{EventID: "401873005", Week: 6, Opponent: "LV", AtVs: "@", Result: "W", Score: "33-30"},
		{EventID: "401872994", Week: 5, Opponent: "LAR", AtVs: "@", Result: "W", Score: "37-34 OT"},
		{EventID: "401872971", Week: 4, Opponent: "NE", AtVs: "vs", Result: "L", Score: "29-26"},
		{EventID: "401872953", Week: 3, Opponent: "LAC", AtVs: "vs", Result: "W", Score: "24-16"},
		{EventID: "401872932", Week: 2, Opponent: "DET", AtVs: "vs", Result: "W", Score: "41-31"},
		{EventID: "401872660", Week: 1, Opponent: "HOU", AtVs: "@", Result: "W", Score: "36-31"},
	}, log.Entries)
}

func TestGameLogFromTylerBassFixture(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("testdata", "gamelog_kicker.json"))
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
	t.Cleanup(server.Close)

	client := espn.NewClient(server.URL, server.Client())
	log, err := client.GameLog(t.Context(), "3917232")
	require.NoError(t, err)

	assert.Equal(t, "3917232", log.AthleteID)
	assert.Equal(t, 7, log.GamesPlayed)

	// KICKER_PTS: FG made * 3 + XP made
	// Week 9: 3 FG, 3 XP -> 12
	// Week 8: 4 FG, 3 XP -> 15
	// Week 7: 3 FG, 2 XP -> 11
	// Week 6: 2 FG, 2 XP -> 8
	// Week 5: 1 FG, 3 XP -> 6
	// Week 4: 0 FG, 5 XP -> 5
	// Week 1: 3 FG, 3 XP -> 12
	assert.Equal(t, []float64{12, 15, 11, 8, 6, 5, 12}, log.Markets["KICKER_PTS"])
	assert.NotContains(t, log.Markets, "PASS_YDS")
	assert.NotContains(t, log.Markets, "RUSH_YDS")
	assert.NotContains(t, log.Markets, "ANYTIME_TD")
}

func TestGameLogReceiverAndReturner(t *testing.T) {
	t.Parallel()

	fixture := `{
		"names": [
			"receptions",
			"receivingYards",
			"receivingTouchdowns",
			"rushingAttempts",
			"rushingYards",
			"rushingTouchdowns",
			"returnTouchdowns"
		],
		"labels": ["REC", "YDS", "TD", "CAR", "YDS", "TD", "TD"],
		"categories": [
			{"name": "receiving", "displayName": "Receiving", "count": 3},
			{"name": "rushing", "displayName": "Rushing", "count": 3},
			{"name": "returns", "displayName": "Returns", "count": 1}
		],
		"seasonTypes": [
			{
				"displayName": "2026 Preseason",
				"categories": [
					{
						"displayName": "Preseason Stats",
						"splitType": "1",
						"events": [
							{"eventId": "999", "stats": ["10", "150", "2", "0", "0", "0", "0"]}
						]
					}
				]
			},
			{
				"displayName": "2026 Postseason",
				"categories": [
					{
						"displayName": "Postseason Stats",
						"splitType": "3",
						"events": [
							{"eventId": "200", "stats": ["8", "110", "1", "2", "15", "0", "1"]}
						]
					}
				]
			},
			{
				"displayName": "2026 Regular Season",
				"categories": [
					{
						"displayName": "Regular Season Stats",
						"splitType": "2",
						"events": [
							{"eventId": "100", "stats": ["6", "75", "1", "1", "5", "1", "0"]}
						]
					}
				]
			}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixture))
	}))
	t.Cleanup(server.Close)

	client := espn.NewClient(server.URL, server.Client())
	log, err := client.GameLog(t.Context(), "4444")
	require.NoError(t, err)

	// Preseason game 999 must be skipped. Postseason (200) followed by regular season (100).
	assert.Equal(t, 2, log.GamesPlayed)

	assert.Equal(t, []float64{8, 6}, log.Markets["RECEPTIONS"])
	assert.Equal(t, []float64{110, 75}, log.Markets["REC_YDS"])
	assert.Equal(t, []float64{2, 1}, log.Markets["RUSH_ATT"])
	assert.Equal(t, []float64{15, 5}, log.Markets["RUSH_YDS"])

	// ANYTIME_TD = rushTDs + recTDs + retTDs
	// Game 200: 0 rush + 1 rec + 1 ret = 2
	// Game 100: 1 rush + 1 rec + 0 ret = 2
	assert.Equal(t, []float64{2, 2}, log.Markets["ANYTIME_TD"])
	assert.Equal(t, []espn.GameLogEntry{
		{EventID: "200"},
		{EventID: "100"},
	}, log.Entries)
}

func TestGameLogEntriesFromFixtures(t *testing.T) {
	t.Parallel()

	wantBengalsEntries := []espn.GameLogEntry{
		{EventID: "401873038", Week: 9, Opponent: "ATL", AtVs: "vs", Result: "W", Score: "34-17"},
		{EventID: "401873031", Week: 8, Opponent: "TEN", AtVs: "vs", Result: "W", Score: "43-36 OT"},
		{EventID: "401873012", Week: 7, Opponent: "BAL", AtVs: "@", Result: "W", Score: "35-13"},
		{EventID: "401872982", Week: 5, Opponent: "MIA", AtVs: "@", Result: "W", Score: "31-24"},
		{EventID: "401872969", Week: 4, Opponent: "JAX", AtVs: "vs", Result: "L", Score: "22-17"},
		{EventID: "401872950", Week: 3, Opponent: "PIT", AtVs: "@", Result: "L", Score: "30-27"},
		{EventID: "401872934", Week: 2, Opponent: "HOU", AtVs: "@", Result: "W", Score: "20-6"},
		{EventID: "401872925", Week: 1, Opponent: "TB", AtVs: "vs", Result: "W", Score: "33-27"},
	}

	cases := []struct {
		name      string
		fixture   string
		athleteID string
		market    string
	}{
		{name: "chase", fixture: "gamelog_chase.json", athleteID: "4362628", market: "REC_YDS"},
		{name: "burrow", fixture: "gamelog_burrow.json", athleteID: "3915511", market: "PASS_YDS"},
		{name: "mcpherson", fixture: "gamelog_mcpherson.json", athleteID: "4360234", market: "KICKER_PTS"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			raw, err := os.ReadFile(filepath.Join("testdata", tc.fixture))
			require.NoError(t, err)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(raw)
			}))
			t.Cleanup(server.Close)

			client := espn.NewClient(server.URL, server.Client())
			log, err := client.GameLog(t.Context(), tc.athleteID)
			require.NoError(t, err)

			assert.Equal(t, len(wantBengalsEntries), log.GamesPlayed)
			assert.Len(t, log.Markets[tc.market], len(wantBengalsEntries))
			assert.Equal(t, wantBengalsEntries, log.Entries)
		})
	}
}
