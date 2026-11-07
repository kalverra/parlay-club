package weather

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seasonVenues is the fixture's shape: one entry per unique venue on the
// season's scoreboards.
type seasonVenues struct {
	Season struct {
		Year int `json:"year"`
		Type int `json:"type"`
	} `json:"season"`
	Venues []struct {
		ID       string `json:"id"`
		FullName string `json:"fullName"`
		Indoor   bool   `json:"indoor"`
		Address  struct {
			City    string `json:"city"`
			State   string `json:"state"`
			Country string `json:"country"`
		} `json:"address"`
	} `json:"venues"`
}

func readSeasonVenues(t *testing.T) seasonVenues {
	t.Helper()

	raw, err := os.ReadFile("testdata/venues_2026.json")
	require.NoError(t, err)

	var fixture seasonVenues
	require.NoError(t, json.Unmarshal(raw, &fixture))
	require.NotEmpty(t, fixture.Venues)
	return fixture
}

func TestVenueMapCoversTheSeasonScoreboards(t *testing.T) {
	t.Parallel()

	fixture := readSeasonVenues(t)

	for _, want := range fixture.Venues {
		got, ok := VenueByID(want.ID)
		if !assert.True(t, ok, "venue %s %q is missing", want.ID, want.FullName) {
			continue
		}
		assert.Equal(t, want.ID, got.ID)
		assert.Equal(t, want.FullName, got.Name, "name comes from ESPN's scoreboard")
		assert.GreaterOrEqual(t, got.Latitude, -90.0)
		assert.LessOrEqual(t, got.Latitude, 90.0)
		assert.GreaterOrEqual(t, got.Longitude, -180.0)
		assert.LessOrEqual(t, got.Longitude, 180.0)
		assert.NotEqual(t, 0.0, got.Latitude, "a venue at 0,0 means the coordinates were missed")
		assert.NotEqual(t, 0.0, got.Longitude, "a venue at 0,0 means the coordinates were missed")
	}

	assert.Len(t, venues, len(fixture.Venues), "the map has no entries outside the season")
}

func TestVenueMapIncludesInternationalVenues(t *testing.T) {
	t.Parallel()

	fixture := readSeasonVenues(t)

	var international int
	for _, venue := range fixture.Venues {
		if venue.Address.Country == "USA" {
			continue
		}
		international++
		_, ok := VenueByID(venue.ID)
		assert.True(t, ok, "international venue %s %q is missing", venue.ID, venue.FullName)
	}
	assert.NotZero(t, international, "the fixture includes the season's international games")

	melbourne, ok := VenueByID("9119")
	require.True(t, ok)
	assert.Equal(t, "Melbourne Cricket Ground", melbourne.Name)
	assert.Less(t, melbourne.Latitude, 0.0)
	assert.Greater(t, melbourne.Longitude, 100.0)

	london, ok := VenueByID("5534")
	require.True(t, ok)
	assert.Equal(t, "Tottenham Hotspur Stadium", london.Name)
}

func TestVenueByIDReportsAnUnknownVenue(t *testing.T) {
	t.Parallel()

	venue, ok := VenueByID("999999")
	assert.False(t, ok)
	assert.Equal(t, Venue{}, venue)
}
