package weather_test

import (
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/ingest/weather"
)

// TestLiveForecast hits Open-Meteo's real forecast for a known venue and checks
// that the kickoff window parses into plausible weather. It skips in -short
// runs.
func TestLiveForecast(t *testing.T) {
	if testing.Short() {
		t.Skip("live API test")
	}
	t.Parallel()

	venue, ok := weather.VenueByID("3673")
	require.True(t, ok, "Lumen Field is in the venue map")

	kickoff := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Hour)
	forecast, err := weather.NewClient("", nil).Forecast(t.Context(), venue, kickoff)
	// A network that blocks Open-Meteo is not a parser regression, so skip
	// rather than fail. HTTP and parse errors still fail below.
	if _, ok := errors.AsType[*url.Error](err); ok {
		t.Skipf("Open-Meteo unreachable: %v", err)
	}
	require.NoError(t, err)

	assert.Greater(t, forecast.TempF, -60.0)
	assert.Less(t, forecast.TempF, 140.0)
	assert.GreaterOrEqual(t, forecast.WindMPH, 0.0)
	assert.GreaterOrEqual(t, forecast.WindGustMPH, forecast.WindMPH, "a gust is at least the sustained wind")
	assert.GreaterOrEqual(t, forecast.PrecipIN, 0.0)
}
