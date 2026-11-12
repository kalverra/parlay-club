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

func TestRosterFromFixture(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("testdata", "roster.json"))
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
	players, err := client.Roster(t.Context(), "2")
	require.NoError(t, err)

	assert.Equal(t, "/teams/2/roster", requestedPath)
	assert.NotEmpty(t, players)

	var joshAllen *espn.RosterPlayer
	var tylerBass *espn.RosterPlayer
	for i := range players {
		if players[i].ID == "3918298" {
			joshAllen = &players[i]
		}
		if players[i].ID == "3917232" {
			tylerBass = &players[i]
		}
	}

	require.NotNil(t, joshAllen, "Josh Allen should be on the Bills roster")
	assert.Equal(t, "Josh Allen", joshAllen.FullName)
	assert.Equal(t, "QB", joshAllen.Position)
	assert.Equal(t, "https://a.espncdn.com/i/headshots/nfl/players/full/3918298.png", joshAllen.HeadshotURL)

	require.NotNil(t, tylerBass, "Tyler Bass should be on the Bills roster")
	assert.Equal(t, "Tyler Bass", tylerBass.FullName)
	assert.Equal(t, "PK", tylerBass.Position)
	assert.Equal(t, "https://a.espncdn.com/i/headshots/nfl/players/full/3917232.png", tylerBass.HeadshotURL)
}

func TestRosterSkipsEmptyAthletes(t *testing.T) {
	t.Parallel()

	fixture := `{
		"athletes": [
			{
				"position": "offense",
				"items": [
					{"id": "1", "fullName": "Valid Player", "position": {"abbreviation": "QB"}},
					{"id": "", "fullName": "No ID", "position": {"abbreviation": "RB"}},
					{"id": "2", "fullName": "", "position": {"abbreviation": "WR"}},
					{"id": "3", "fullName": "No Position"}
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
	players, err := client.Roster(t.Context(), "10")
	require.NoError(t, err)

	require.Len(t, players, 2)
	assert.Equal(t, espn.RosterPlayer{ID: "1", FullName: "Valid Player", Position: "QB"}, players[0])
	assert.Equal(t, espn.RosterPlayer{ID: "3", FullName: "No Position", Position: ""}, players[1])
}

func TestRosterMissingAndInvalidHeadshots(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("testdata", "roster_falcons.json"))
	require.NoError(t, err)

	falconsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
	t.Cleanup(falconsServer.Close)

	players, err := espn.NewClient(falconsServer.URL, falconsServer.Client()).Roster(t.Context(), "1")
	require.NoError(t, err)
	require.NotEmpty(t, players)

	var missingPlayer *espn.RosterPlayer
	hasHeadshot := false
	for i := range players {
		if players[i].ID == "5344782" {
			missingPlayer = &players[i]
		}
		if players[i].HeadshotURL != "" {
			hasHeadshot = true
		}
	}
	require.NotNil(t, missingPlayer, "Jack Strand should be in roster_falcons.json")
	assert.Empty(t, missingPlayer.HeadshotURL)
	assert.True(t, hasHeadshot)

	fixture := `{
		"athletes": [
			{
				"items": [
					{"id": "1", "fullName": "HTTPS Padded", "position": {"abbreviation": "QB"}, "headshot": {"href": "  https://a.espncdn.com/1.png  "}},
					{"id": "2", "fullName": "HTTP Rejected", "position": {"abbreviation": "RB"}, "headshot": {"href": "http://a.espncdn.com/2.png"}},
					{"id": "3", "fullName": "Relative Rejected", "position": {"abbreviation": "WR"}, "headshot": {"href": "/3.png"}},
					{"id": "4", "fullName": "Empty Host", "position": {"abbreviation": "TE"}, "headshot": {"href": "https://"}}
				]
			}
		]
	}`
	normServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixture))
	}))
	t.Cleanup(normServer.Close)

	normPlayers, err := espn.NewClient(normServer.URL, normServer.Client()).Roster(t.Context(), "2")
	require.NoError(t, err)
	require.Len(t, normPlayers, 4)
	assert.Equal(t, "https://a.espncdn.com/1.png", normPlayers[0].HeadshotURL)
	assert.Empty(t, normPlayers[1].HeadshotURL)
	assert.Empty(t, normPlayers[2].HeadshotURL)
	assert.Empty(t, normPlayers[3].HeadshotURL)
}

func TestRosterError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	client := espn.NewClient(server.URL, server.Client())
	_, err := client.Roster(t.Context(), "2")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status 500")
}
