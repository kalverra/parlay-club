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

func findBoxPlayer(players []espn.PlayerBoxStats, playerID string) (espn.PlayerBoxStats, bool) {
	for _, p := range players {
		if p.PlayerID == playerID {
			return p, true
		}
	}
	return espn.PlayerBoxStats{}, false
}

func TestBoxScoreFromWeek9FeaturedFixture(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("testdata", "summary_week9_featured.json"))
	require.NoError(t, err)

	requestedQuery := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/summary", r.URL.Path)
		requestedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
	t.Cleanup(server.Close)

	client := espn.NewClient(server.URL, server.Client())
	box, err := client.BoxScore(t.Context(), "401873038")
	require.NoError(t, err)

	assert.Equal(t, "event=401873038", requestedQuery)
	assert.True(t, box.Completed)
	assert.True(t, box.HasStats)
	require.NotEmpty(t, box.Players)

	// Joe Burrow: passing + rushing merged into one PlayerBoxStats.
	burrow, ok := findBoxPlayer(box.Players, "3915511")
	require.True(t, ok, "expected Joe Burrow in boxscore")
	assert.Equal(t, "Joe Burrow", burrow.DisplayName)
	assert.Equal(t, "CIN", burrow.TeamAbbrev)
	assert.Equal(t, espn.PassingStats{PassingYards: 271, PassingTDs: 3}, burrow.Passing)
	assert.Equal(t, espn.RushingStats{RushingAttempts: 3, RushingYards: 7, RushingTDs: 1}, burrow.Rushing)
	assert.Equal(t, espn.ReceivingStats{}, burrow.Receiving)

	// Ja'Marr Chase: receiving only; rushing defaults to 0.
	chase, ok := findBoxPlayer(box.Players, "4362628")
	require.True(t, ok, "expected Ja'Marr Chase in boxscore")
	assert.Equal(t, "CIN", chase.TeamAbbrev)
	assert.Equal(t, espn.ReceivingStats{Receptions: 10, ReceivingYards: 155, ReceivingTDs: 1}, chase.Receiving)
	assert.Equal(t, espn.RushingStats{}, chase.Rushing)

	// Evan McPherson: kicking slash format (2/2, 4/4) and totalKickingPoints.
	mcpherson, ok := findBoxPlayer(box.Players, "4360234")
	require.True(t, ok, "expected Evan McPherson in boxscore")
	assert.Equal(t, espn.KickingStats{FieldGoalsMade: 2, ExtraPointsMade: 4, TotalKickingPoints: 10}, mcpherson.Kicking)

	// Bijan Robinson: rushing + receiving + fumbles merged cleanly.
	bijan, ok := findBoxPlayer(box.Players, "4430807")
	require.True(t, ok, "expected Bijan Robinson in boxscore")
	assert.Equal(t, "ATL", bijan.TeamAbbrev)
	assert.Equal(t, espn.RushingStats{RushingAttempts: 19, RushingYards: 100, RushingTDs: 1}, bijan.Rushing)
	assert.Equal(t, espn.ReceivingStats{Receptions: 6, ReceivingYards: 47, ReceivingTDs: 0}, bijan.Receiving)

	// Nick Folk: 1/2 FG + 2/2 XP = 5 pts.
	folk, ok := findBoxPlayer(box.Players, "10621")
	require.True(t, ok, "expected Nick Folk in boxscore")
	assert.Equal(t, espn.KickingStats{FieldGoalsMade: 1, ExtraPointsMade: 2, TotalKickingPoints: 5}, folk.Kicking)

	// Absent player when HasStats is true indicates DNP.
	_, dnp := findBoxPlayer(box.Players, "99999999")
	assert.False(t, dnp)
}

func TestBoxScoreFromEmptyFixture(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("testdata", "summary_empty.json"))
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
	t.Cleanup(server.Close)

	client := espn.NewClient(server.URL, server.Client())
	box, err := client.BoxScore(t.Context(), "401873000")
	require.NoError(t, err)

	assert.False(t, box.Completed)
	assert.False(t, box.HasStats)
	assert.Empty(t, box.Players)
}

func TestBoxScoreFromPregameSummaryFixture(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("testdata", "summary.json"))
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
	t.Cleanup(server.Close)

	client := espn.NewClient(server.URL, server.Client())
	box, err := client.BoxScore(t.Context(), "401873051")
	require.NoError(t, err)

	assert.False(t, box.Completed)
	assert.False(t, box.HasStats)
	assert.Empty(t, box.Players)
}

func TestBoxScoreParsesByKeyNameAndFallbacks(t *testing.T) {
	t.Parallel()

	payload := `{
		"header": {
			"competitions": [{"status": {"type": {"completed": true}}}]
		},
		"boxscore": {
			"players": [
				{
					"team": {"abbreviation": "BUF"},
					"statistics": [
						{
							"name": "kicking",
							"keys": ["extraPointsMade-extraPointAttempts", "fieldGoalsMade-fieldGoalAttempts"],
							"athletes": [
								{
									"athlete": {"id": "k1", "displayName": "Tyler Bass"},
									"stats": ["3-3", "2-3"]
								}
							]
						},
						{
							"name": "kickReturns",
							"keys": ["kickReturnTouchdowns", "kickReturns"],
							"athletes": [
								{
									"athlete": {"id": "ret1", "displayName": "Return Guy"},
									"stats": ["1", "4"]
								}
							]
						},
						{
							"name": "defensive",
							"keys": ["defensiveTouchdowns", "totalTackles"],
							"athletes": [
								{
									"athlete": {"id": "ret1", "displayName": "Return Guy"},
									"stats": ["1", "2"]
								}
							]
						},
						{
							"name": "interceptions",
							"keys": ["interceptionTouchdowns"],
							"athletes": [
								{
									"athlete": {"id": "db1", "displayName": "Corner One"},
									"stats": ["1"]
								}
							]
						}
					]
				}
			]
		}
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(server.Close)

	client := espn.NewClient(server.URL, server.Client())
	box, err := client.BoxScore(t.Context(), "401999999")
	require.NoError(t, err)

	assert.True(t, box.Completed)
	assert.True(t, box.HasStats)
	require.Len(t, box.Players, 3)

	kicker, ok := findBoxPlayer(box.Players, "k1")
	require.True(t, ok)
	assert.Equal(t, espn.KickingStats{
		FieldGoalsMade:     2,
		ExtraPointsMade:    3,
		TotalKickingPoints: 9, // 2*3 + 3 fallback when totalKickingPoints omitted
	}, kicker.Kicking)

	ret, ok := findBoxPlayer(box.Players, "ret1")
	require.True(t, ok)
	assert.Equal(t, 2, ret.OtherTDs)
	assert.Equal(t, espn.ReceivingStats{}, ret.Receiving)

	db, ok := findBoxPlayer(box.Players, "db1")
	require.True(t, ok)
	assert.Equal(t, 1, db.OtherTDs)
}

func TestBoxScoreReportsHTTPFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(server.Close)

	client := espn.NewClient(server.URL, server.Client())
	_, err := client.BoxScore(t.Context(), "401873038")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boxscore 401873038")
}

func TestParseBoxScoreJSON(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("testdata", "summary_week9_featured.json"))
	require.NoError(t, err)

	box, err := espn.ParseBoxScoreJSON(raw)
	require.NoError(t, err)
	assert.True(t, box.Completed)
	assert.True(t, box.HasStats)
	burrow, ok := findBoxPlayer(box.Players, "3915511")
	require.True(t, ok)
	assert.Equal(t, 271, burrow.Passing.PassingYards)

	emptyRaw, err := os.ReadFile(filepath.Join("testdata", "summary_empty.json"))
	require.NoError(t, err)
	emptyBox, err := espn.ParseBoxScoreJSON(emptyRaw)
	require.NoError(t, err)
	assert.False(t, emptyBox.HasStats)

	_, err = espn.ParseBoxScoreJSON([]byte("{invalid"))
	require.Error(t, err)
}
