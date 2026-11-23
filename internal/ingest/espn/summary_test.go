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
	foundTeamLogo := false
	foundHeadshot := false
	for _, team := range teams {
		assert.NotEmpty(t, team.Team)
		assert.NotEmpty(t, team.TeamID)
		if team.LogoURL != "" {
			foundTeamLogo = true
			assert.Contains(t, team.LogoURL, "https://")
		}
		for _, inj := range team.Injuries {
			assert.NotEmpty(t, inj.AthleteID)
			assert.NotEmpty(t, inj.FullName)
			assert.Contains(t, []string{"QB", "RB", "WR", "TE", "LT", "OT", "C", "CB", "S"}, inj.Position)
			if inj.HeadshotURL != "" {
				foundHeadshot = true
				assert.Contains(t, inj.HeadshotURL, "https://")
			}
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
	assert.True(t, foundTeamLogo, "expected team logo URL in fixture")
	assert.True(t, foundHeadshot, "expected athlete headshot URL in fixture")
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

func TestParseInjuriesMediaNormalization(t *testing.T) {
	t.Parallel()

	raw := `[
		{
			"team": {
				"id": "2",
				"displayName": "Buffalo Bills",
				"abbreviation": "BUF",
				"logo": "  https://a.espncdn.com/i/teamlogos/nfl/500/buf.png  "
			},
			"injuries": [
				{
					"status": "Out",
					"athlete": {
						"id": "100",
						"fullName": "Valid Headshot",
						"position": {"abbreviation": "QB"},
						"headshot": {"href": " https://a.espncdn.com/100.png "}
					}
				},
				{
					"status": "Questionable",
					"athlete": {
						"id": "101",
						"fullName": "HTTP Headshot",
						"position": {"abbreviation": "WR"},
						"headshot": {"href": "http://a.espncdn.com/101.png"}
					}
				},
				{
					"status": "Doubtful",
					"athlete": {
						"id": "102",
						"fullName": "Missing Headshot",
						"position": {"abbreviation": "TE"}
					}
				}
			]
		},
		{
			"team": {
				"id": "1",
				"displayName": "Atlanta Falcons",
				"abbreviation": "ATL",
				"logo": "http://a.espncdn.com/i/teamlogos/nfl/500/atl.png"
			},
			"injuries": []
		}
	]`

	teams, err := espn.ParseInjuries(raw)
	require.NoError(t, err)
	require.Len(t, teams, 2)

	assert.Equal(t, "https://a.espncdn.com/i/teamlogos/nfl/500/buf.png", teams[0].LogoURL)
	require.Len(t, teams[0].Injuries, 3)
	assert.Equal(t, "https://a.espncdn.com/100.png", teams[0].Injuries[0].HeadshotURL)
	assert.Empty(t, teams[0].Injuries[1].HeadshotURL)
	assert.Empty(t, teams[0].Injuries[2].HeadshotURL)

	assert.Empty(t, teams[1].LogoURL)
}

func TestIsKeyInjuryPosition(t *testing.T) {
	t.Parallel()

	for _, pos := range []string{"QB", "RB", "WR", "TE", "LT", "OT", "C", "CB", "S", " wr ", "te"} {
		assert.True(t, espn.IsKeyInjuryPosition(pos), "expected %q to be a key injury position", pos)
	}

	for _, pos := range []string{"", "LB", "G", "RG", "DE", "DT", "PK", "P", "LS"} {
		assert.False(t, espn.IsKeyInjuryPosition(pos), "expected %q not to be a key injury position", pos)
	}
}

func TestMergeTeamInjuries(t *testing.T) {
	t.Parallel()

	summary := []espn.Injury{
		{AthleteID: "1", FullName: "Josh Allen", Position: "QB", Status: "Questionable", Detail: "Hand (Full)"},
		{AthleteID: "2", FullName: "Keon Coleman", Position: "WR", Status: "Out", Detail: "Wrist"},
		{AthleteID: "3", FullName: "Curtis Samuel", Position: "WR", Status: "Questionable", Detail: "Shoulder"},
		{AthleteID: "4", FullName: "Dion Dawkins", Position: "LT", Status: "Questionable", Detail: "Ankle"},
		{AthleteID: "5", FullName: "Taron Johnson", Position: "CB", Status: "Out", Detail: "Knee"},
	}
	extra := []espn.Injury{
		// Matches by AthleteID even with different name formatting -> skipped; summary Detail kept.
		{AthleteID: "2", FullName: "K. Coleman", Position: "WR", Status: "Out"},
		// Matches by case-insensitive FullName with missing AthleteID -> skipped.
		{AthleteID: "", FullName: "  taron johnson ", Position: "CB", Status: "Out"},
		// Non-key positions -> filtered out.
		{AthleteID: "90", FullName: "Matt Milano", Position: "LB", Status: "Out"},
		{AthleteID: "91", FullName: "Connor McGovern", Position: "G", Status: "Questionable"},
		// 6th key-position player truncated from summary -> appended.
		{AthleteID: "6", FullName: "Dalton Kincaid", Position: "TE", Status: "Out"},
		// Duplicate of Kincaid in extra -> skipped.
		{AthleteID: "6", FullName: "Dalton Kincaid", Position: "TE", Status: "Out"},
	}

	merged := espn.MergeTeamInjuries(summary, extra)
	require.Len(t, merged, 6)
	assert.Equal(t, summary, merged[:5])
	assert.Equal(t, espn.Injury{
		AthleteID: "6",
		FullName:  "Dalton Kincaid",
		Position:  "TE",
		Status:    "Out",
	}, merged[5])

	assert.Empty(t, espn.MergeTeamInjuries(nil, nil))
	assert.Equal(t, []espn.Injury{{AthleteID: "6", FullName: "Dalton Kincaid", Position: "TE", Status: "Out"}},
		espn.MergeTeamInjuries(nil, []espn.Injury{
			{AthleteID: "90", FullName: "Matt Milano", Position: "LB", Status: "Out"},
			{AthleteID: "6", FullName: "Dalton Kincaid", Position: "TE", Status: "Out"},
		}))
}
