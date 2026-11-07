package weather_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/ingest/weather"
)

func TestForecastParsesTheKickoffWindow(t *testing.T) {
	t.Parallel()

	venue, ok := weather.VenueByID("3673")
	require.True(t, ok, "Lumen Field is in the venue map")

	forecast, query, err := fetchForecast(t, "forecast_lumen.json", venue,
		time.Date(2026, time.November, 8, 21, 25, 0, 0, time.UTC))
	require.NoError(t, err)

	assert.Equal(t, "47.59535", query.Get("latitude"))
	assert.Equal(t, "-122.33164", query.Get("longitude"))
	assert.Equal(t, "temperature_2m,precipitation,wind_speed_10m,wind_gusts_10m", query.Get("hourly"))
	assert.Equal(t, "mph", query.Get("wind_speed_unit"))
	assert.Equal(t, "fahrenheit", query.Get("temperature_unit"))
	assert.Equal(t, "inch", query.Get("precipitation_unit"))
	assert.Equal(t, "UTC", query.Get("timezone"))
	assert.Equal(t, "2026-11-08T21:00", query.Get("start_hour"), "the hour containing kickoff")
	assert.Equal(t, "2026-11-08T23:00", query.Get("end_hour"), "the kickoff hour plus two")

	assert.Equal(t, weather.Forecast{
		TempF:       53.6,
		WindMPH:     11.3,
		WindGustMPH: 19.5,
		PrecipIN:    0.028,
	}, forecast)
}

func TestForecastUsesTheKickoffHourTemperature(t *testing.T) {
	t.Parallel()

	venue, ok := weather.VenueByID("11938")
	require.True(t, ok, "Highmark Stadium is in the venue map")

	// Highmark's window warms from 41.6 to 42.7, so the max is not the kickoff
	// temperature. Wind and gust peak at the kickoff hour.
	forecast, query, err := fetchForecast(t, "forecast_highmark.json", venue,
		time.Date(2026, time.November, 10, 1, 15, 0, 0, time.UTC))
	require.NoError(t, err)

	assert.Equal(t, "2026-11-10T01:00", query.Get("start_hour"))
	assert.Equal(t, "2026-11-10T03:00", query.Get("end_hour"))

	assert.InDelta(t, 41.6, forecast.TempF, 1e-9)
	assert.InDelta(t, 6.3, forecast.WindMPH, 1e-9)
	assert.InDelta(t, 18.3, forecast.WindGustMPH, 1e-9)
	assert.Zero(t, forecast.PrecipIN)
}

func TestForecastRejectsAWindowThatEndsEarly(t *testing.T) {
	t.Parallel()

	venue, ok := weather.VenueByID("3673")
	require.True(t, ok, "Lumen Field is in the venue map")

	_, _, err := fetchForecast(t, "forecast_short_window.json", venue,
		time.Date(2026, time.November, 8, 22, 30, 0, 0, time.UTC))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forecast ends before the kickoff window")
}

func TestForecastReportsAnHTTPFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)

	_, err := weather.NewClient(server.URL, server.Client()).Forecast(t.Context(), weather.Venue{
		Latitude:  42.77303,
		Longitude: -78.79219,
	}, time.Date(2026, time.November, 10, 1, 15, 0, 0, time.UTC))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "503 Service Unavailable")
}

// fetchForecast serves fixture and returns the forecast, the query it put on
// the wire, and any error.
func fetchForecast(
	t *testing.T,
	fixture string,
	venue weather.Venue,
	kickoff time.Time,
) (weather.Forecast, url.Values, error) {
	t.Helper()

	//nolint:gosec // the fixture name is a test constant.
	raw, err := os.ReadFile(filepath.Join("testdata", fixture))
	require.NoError(t, err)

	queries := make(chan url.Values, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/forecast", r.URL.Path)
		assert.Equal(t, "parlay-club", r.Header.Get("User-Agent"))
		queries <- r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
	t.Cleanup(server.Close)

	forecast, err := weather.NewClient(server.URL, server.Client()).Forecast(t.Context(), venue, kickoff)
	return forecast, <-queries, err
}
