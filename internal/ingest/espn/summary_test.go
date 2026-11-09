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

func TestSummaryInjuriesFromFixture(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("testdata", "summary.json"))
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
	rawInjuries, err := client.SummaryInjuries(t.Context(), "401873051")
	require.NoError(t, err)

	assert.Equal(t, "event=401873051", requestedQuery)
	assert.NotEmpty(t, rawInjuries)
	assert.Equal(t, byte('['), rawInjuries[0])

	teams, err := espn.ParseInjuries(rawInjuries)
	require.NoError(t, err)
	require.NotEmpty(t, teams)

	// Check that key positions (RB, WR, CB, etc.) are included and non-key positions are filtered out.
	foundWR := false
	foundCB := false
	for _, team := range teams {
		assert.NotEmpty(t, team.Team)
		assert.NotEmpty(t, team.TeamID)
		for _, inj := range team.Injuries {
			assert.NotEmpty(t, inj.AthleteID)
			assert.NotEmpty(t, inj.FullName)
			assert.Contains(t, []string{"QB", "RB", "WR", "TE", "LT", "OT", "C", "CB", "S"}, inj.Position)
			if inj.Position == "WR" {
				foundWR = true
			}
			if inj.Position == "CB" {
				foundCB = true
			}
		}
	}
	assert.True(t, foundWR, "expected WR injury in fixture")
	assert.True(t, foundCB, "expected CB injury in fixture")
}

func TestSummaryInjuriesAbsent(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"header": {}}`))
	}))
	t.Cleanup(server.Close)

	client := espn.NewClient(server.URL, server.Client())
	raw, err := client.SummaryInjuries(t.Context(), "12345")
	require.NoError(t, err)
	assert.Equal(t, "[]", raw)

	parsed, err := espn.ParseInjuries(raw)
	require.NoError(t, err)
	assert.Empty(t, parsed)
}

func TestParseInjuriesFilterAndDetail(t *testing.T) {
	t.Parallel()

	raw := `[
		{
			"team": {
				"id": "2",
				"displayName": "Buffalo Bills",
				"abbreviation": "BUF"
			},
			"injuries": [
				{
					"status": "Out",
					"athlete": {
						"id": "100",
						"fullName": "Quarterback One",
						"position": {"abbreviation": "QB"}
					},
					"details": {
						"type": "Knee",
						"detail": "Surgery"
					}
				},
				{
					"status": "Questionable",
					"athlete": {
						"id": "101",
						"fullName": "Linebacker Two",
						"position": {"abbreviation": "LB"}
					},
					"details": {
						"type": "Ankle",
						"detail": "Not Specified"
					}
				},
				{
					"status": "Questionable",
					"athlete": {
						"id": "102",
						"fullName": "Left Tackle Three",
						"position": {"abbreviation": "LT"}
					},
					"details": {
						"type": "Hamstring",
						"detail": "Not Specified"
					}
				},
				{
					"status": "Doubtful",
					"athlete": {
						"id": "103",
						"fullName": "Center Four",
						"position": {"abbreviation": "C"}
					},
					"details": {
						"type": "Concussion"
					}
				}
			]
		}
	]`

	teams, err := espn.ParseInjuries(raw)
	require.NoError(t, err)
	require.Len(t, teams, 1)

	bills := teams[0]
	assert.Equal(t, "Buffalo Bills", bills.Team)
	assert.Equal(t, "2", bills.TeamID)
	assert.Equal(t, "BUF", bills.Abbreviation)

	// LB is skipped; QB, LT, and C are kept.
	require.Len(t, bills.Injuries, 3)

	assert.Equal(t, espn.Injury{
		AthleteID: "100",
		FullName:  "Quarterback One",
		Position:  "QB",
		Status:    "Out",
		Detail:    "Knee (Surgery)",
	}, bills.Injuries[0])

	assert.Equal(t, espn.Injury{
		AthleteID: "102",
		FullName:  "Left Tackle Three",
		Position:  "LT",
		Status:    "Questionable",
		Detail:    "Hamstring",
	}, bills.Injuries[1])

	assert.Equal(t, espn.Injury{
		AthleteID: "103",
		FullName:  "Center Four",
		Position:  "C",
		Status:    "Doubtful",
		Detail:    "Concussion",
	}, bills.Injuries[2])
}

func TestParseInjuriesEmpty(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"", "[]", "   ", "null"} {
		res, err := espn.ParseInjuries(input)
		require.NoError(t, err)
		assert.Empty(t, res)
	}

	_, err := espn.ParseInjuries("{invalid")
	require.Error(t, err)
}
