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

	require.NotNil(t, tylerBass, "Tyler Bass should be on the Bills roster")
	assert.Equal(t, "Tyler Bass", tylerBass.FullName)
	assert.Equal(t, "PK", tylerBass.Position)
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
